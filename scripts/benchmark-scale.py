#!/usr/bin/env python3
"""Measure majority-endorsed registration as peer organizations scale."""
import argparse
import json
import os
from pathlib import Path
import subprocess
from network import ROOT


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--counts', default='2,4,8,12,16')
    parser.add_argument('--out', default='runtime/benchmarks/paper-20261030/scaling')
    parser.add_argument('--duration', type=int, default=5)
    parser.add_argument('--repetitions', type=int, default=3)
    args = parser.parse_args()
    output = Path(args.out).resolve()
    output.mkdir(parents=True, exist_ok=True)
    aggregate = output/'runs.json'
    runs = json.loads(aggregate.read_text()) if aggregate.exists() else []
    for count in [int(v) for v in args.counts.split(',')]:
        if sum(r['organizations'] == count for r in runs) == args.repetitions:
            continue
        runtime = ROOT/f'runtime/experiments/scale-{count}'
        runtime.mkdir(parents=True, exist_ok=True)
        topology = dict(organizations=count, peers_per_organization=1, orderers=5,
                        endorsement_threshold=count//2+1, port_base=25000,
                        hosts={'local': {'address': '127.0.0.1'}})
        config = runtime/'topology.json'
        config.write_text(json.dumps(topology, indent=2))
        env = dict(os.environ, DIS_RUNTIME=str(runtime))
        def call(command, timeout=300, check=True):
            result = subprocess.run([str(v) for v in command], env=env, capture_output=True, text=True, timeout=timeout)
            with (runtime/'runner.log').open('a') as log:
                log.write(json.dumps([str(v) for v in command])+'\n'+result.stdout+result.stderr+'\n')
            if check:
                result.check_returncode()
            return result
        try:
            call(['python3', ROOT/'scripts/network.py', 'up', '--topology', config])
            call(['python3', ROOT/'scripts/services.py', 'up', '--api-port', 49080, '--dns-port', 49530, '--redis-port', 49679])
            call(['python3', ROOT/'scripts/benchmark-paper.py', '--kind', 'writes', '--rates', '300', '--duration', args.duration,
                  '--repetitions', args.repetitions, '--out', output/f'peers-{count}'], timeout=600)
            reports=json.loads((output/f'peers-{count}'/'runs.json').read_text())
            for report in reports:
                report.update(organizations=count, orderers=5, endorsement_threshold=count//2+1)
            runs.extend(reports)
            aggregate.write_text(json.dumps(runs,indent=2))
            print(json.dumps(dict(organizations=count,valid=[r['valid_throughput_tps'] for r in reports],success=[r['success_rate'] for r in reports])),flush=True)
        finally:
            call(['python3', ROOT/'scripts/services.py', 'down'], check=False)
            call(['python3', ROOT/'scripts/network.py', 'down'], check=False)


if __name__=='__main__':
    main()
