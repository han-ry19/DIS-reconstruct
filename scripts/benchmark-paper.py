#!/usr/bin/env python3
"""Run paper-aligned Fabric write/read and dnsperf offered-load sweeps."""
import argparse
import csv
import json
import os
from pathlib import Path
import platform
import random
import re
import subprocess
import threading
import time
from network import ROOT, RUNTIME, WORK, load, live


def command(args, timeout=240):
    return subprocess.run([str(a) for a in args], check=True, capture_output=True, text=True, timeout=timeout)


def resources(stop, output):
    manifest = json.loads((WORK / 'network.json').read_text())
    services = json.loads((RUNTIME / 'services/services.json').read_text())
    processes = dict(manifest['processes'], **services['processes'])
    hz = os.sysconf('SC_CLK_TCK')
    page = os.sysconf('SC_PAGE_SIZE')
    previous = {}
    with output.open('w') as stream:
        while not stop.is_set():
            now = time.monotonic()
            for name, process in processes.items():
                if process.get('remote'):
                    continue
                try:
                    fields = Path(f"/proc/{process['pid']}/stat").read_text().rsplit(')', 1)[1].split()
                    if fields[19] != process['start']:
                        continue
                    ticks = int(fields[11]) + int(fields[12])
                    old = previous.get(name)
                    cpu = 0 if old is None else (ticks-old[1])/hz/(now-old[0])*100
                    previous[name] = (now, ticks)
                    stream.write(json.dumps(dict(time=time.time(), name=name, pid=process['pid'], cpu_percent=cpu, rss_bytes=int(fields[21])*page))+'\n')
                except (FileNotFoundError, ProcessLookupError):
                    pass
            stream.flush()
            stop.wait(1)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', required=True)
    parser.add_argument('--kind', choices=['writes', 'reads', 'dns'], required=True)
    parser.add_argument('--rates', default='50,100,150,200,250,300,350,400,450')
    parser.add_argument('--duration', type=int, default=10)
    parser.add_argument('--repetitions', type=int, default=3)
    parser.add_argument('--dns-queries', help='dnsperf input file with existing record names')
    args = parser.parse_args()
    output = Path(args.out).resolve()
    output.mkdir(parents=True, exist_ok=True)
    network = load()
    if args.kind == 'dns':
        services = json.loads((RUNTIME/'services/services.json').read_text())
        command([ROOT/'bin/dis-sync', '-fabric-config', WORK/'gateway.json', '-redis', f"127.0.0.1:{services['redis_port']}",
                 '-check', '-verify', f"127.0.0.1:{services['dns_port']}"], timeout=180)
    environment = dict(topology=network['topology'], platform=platform.platform(), cpu_count=os.cpu_count(),
                       cpu_info=Path('/proc/cpuinfo').read_text(), memory_info=Path('/proc/meminfo').read_text(),
                       source_revision=command(['git', '-C', ROOT, 'rev-parse', 'HEAD']).stdout.strip(),
                       source_diff=command(['git', '-C', ROOT, 'diff']).stdout, arguments=vars(args),
                       client_placement='same physical machine as server processes', started_at=time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()))
    (output/'environment.json').write_text(json.dumps(environment, indent=2))
    stop = threading.Event()
    sampler = threading.Thread(target=resources, args=(stop, output/'resources.jsonl'), daemon=True)
    sampler.start()
    runs = []
    try:
        rates = [int(v) for v in args.rates.split(',')]
        schedule = [(rate, repetition) for repetition in range(args.repetitions) for rate in rates]
        # A deterministic random order prevents a monotonic growing-ledger/order bias.
        random.Random(20261030).shuffle(schedule)
        for rate, repetition in schedule:
            path = output/f'{args.kind}-{rate}-r{repetition}'
            if path.exists():
                raise RuntimeError(f'Refusing to overwrite {path}')
            path.mkdir()
            if args.kind in ['writes', 'reads']:
                operation = 'data.register' if args.kind == 'writes' else 'data.read'
                result = command([ROOT/'bin/dis-bench', '-fabric-config', WORK/'gateway.json', '-operation', operation,
                                  '-rate', rate, '-duration', f'{args.duration}s', '-out', path])
                (path/'stdout.json').write_text(result.stdout)
                report = json.loads((path/'summary.json').read_text())
                report.pop('successful_ids', None)
                report.update(kind=args.kind, repetition=repetition)
            else:
                if not args.dns_queries:
                    raise ValueError('--dns-queries is required')
                services = json.loads((RUNTIME/'services/services.json').read_text())
                result = command(['dnsperf', '-s', '127.0.0.1', '-p', services['dns_port'], '-d', args.dns_queries,
                                  '-Q', rate, '-l', args.duration, '-c', 16, '-T', 4, '-q', 4096, '-t', 2, '-e'], timeout=args.duration+30)
                (path/'dnsperf.txt').write_text(result.stdout)
                (path/'stderr.txt').write_text(result.stderr)
                def number(pattern):
                    match = re.search(pattern, result.stdout)
                    if not match:
                        raise RuntimeError('Missing dnsperf metric: '+pattern)
                    return float(match.group(1))
                sent = number(r'Queries sent:\s+(\d+)')
                completed = number(r'Queries completed:\s+(\d+)')
                noerror_match = re.search(r'NOERROR\s+(\d+)', result.stdout)
                noerror = float(noerror_match.group(1)) if noerror_match else 0
                report = dict(kind='dns', repetition=repetition, offered_qps=rate, sent=sent, completed=completed,
                              noerror=noerror, success_rate=noerror/sent, throughput_qps=number(r'Queries per second:\s+([\d.]+)'),
                              latency_mean_ms=1000*number(r'Average Latency \(s\):\s+([\d.]+)'))
                (path/'summary.json').write_text(json.dumps(report, indent=2))
            runs.append(report)
            (output/'runs.json').write_text(json.dumps(runs, indent=2))
            print(json.dumps(report), flush=True)
            time.sleep(1)
    finally:
        stop.set()
        sampler.join(timeout=5)
    print('Results: '+str(output), flush=True)


if __name__ == '__main__':
    main()
