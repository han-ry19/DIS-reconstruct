#!/usr/bin/env python3
"""Export aggregate data and figures from retained benchmark run summaries."""
import argparse
import csv
import hashlib
import json
from collections import Counter, defaultdict
from pathlib import Path
import statistics


def summarize(runs, name):
    groups = defaultdict(list)
    for run in runs:
        key = run.get('organizations', run.get('offered_tps', run.get('offered_qps')))
        groups[key].append(run)
    output = []
    for key, items in sorted(groups.items()):
        dns = items[0]['kind'] == 'dns'
        throughput = [r['throughput_qps' if dns else 'valid_throughput_tps'] for r in items]
        latency = [r['latency_mean_ms'] if dns else r['valid_latency_ms']['mean'] for r in items]
        scheduled = sum(r['sent' if dns else 'scheduled'] for r in items)
        valid = sum(r['noerror'] if dns else r['status_counts'].get('valid', 0) for r in items)
        output.append(dict(series=name, x=key, repetitions=len(items), scheduled=int(scheduled), valid=int(valid),
                           success_percent=100*valid/scheduled, throughput_mean=statistics.mean(throughput),
                           throughput_sd=statistics.stdev(throughput) if len(items)>1 else 0,
                           latency_mean_ms=statistics.mean(latency), latency_sd_ms=statistics.stdev(latency) if len(items)>1 else 0,
                           in_window_throughput_mean=statistics.mean([r.get('valid_completions_during_schedule_tps', r.get('throughput_qps',0)) for r in items])))
    return output


def table(rows, xlabel):
    text = ['| '+xlabel+' | Successful throughput (requests/s) | Mean latency (ms) | Success (%) |',
            '|---:|---:|---:|---:|']
    for r in rows:
        text.append(f"| {r['x']} | {r['throughput_mean']:.2f} ± {r['throughput_sd']:.2f} | {r['latency_mean_ms']:.2f} ± {r['latency_sd_ms']:.2f} | {r['success_percent']:.4f} |")
    return '\n'.join(text)


def figures(series, output):
    import matplotlib
    matplotlib.use('Agg')
    import matplotlib.pyplot as plt
    plt.rcParams.update({'font.size':10, 'axes.spines.top':False, 'axes.spines.right':False, 'svg.fonttype':'none'})
    fig, axes = plt.subplots(2, 2, figsize=(11, 7), constrained_layout=True)
    for name, ax, title, xlabel in [('writes', axes[0,0], 'Registration: valid Fabric commit', 'Offered registrations/s'),
                                  ('dns', axes[0,1], 'CoreDNS: existing data URI queries', 'Offered queries/s'),
                                  ('reads', axes[1,0], 'Committed-state lookup through Gateway', 'Offered reads/s'),
                                  ('scaling', axes[1,1], 'Majority endorsement; Gateway limit 500', 'Peer organizations')]:
        rows=series[name]
        x=[r['x'] for r in rows]
        ax.errorbar(x,[r['throughput_mean'] for r in rows],yerr=[r['throughput_sd'] for r in rows],fmt='o-',capsize=3,color='#1864ab')
        if name!='scaling': ax.plot(x,x,'--',color='#999999',linewidth=1,label='Offered load')
        ax.set(title=title,xlabel=xlabel,ylabel='Successful requests/s')
        ax.grid(alpha=.2)
        ax.set_ylim(bottom=0)
    for extension in ['png','svg']:
        fig.savefig(output/f'throughput.{extension}',dpi=180)
    plt.close(fig)
    fig, axes = plt.subplots(1, 3, figsize=(11, 3.4), constrained_layout=True)
    for name,ax,label in zip(['writes','dns','scaling'],axes,['Offered registrations/s','Offered queries/s','Peer organizations']):
        rows=series[name]
        ax.errorbar([r['x'] for r in rows],[r['latency_mean_ms'] for r in rows],yerr=[r['latency_sd_ms'] for r in rows],fmt='o-',capsize=3,color='#087f5b')
        ax.set(xlabel=label,ylabel='Mean successful latency (ms)',title=name.capitalize())
        ax.grid(alpha=.2);ax.set_ylim(bottom=0)
    for extension in ['png','svg']:fig.savefig(output/f'latency.{extension}',dpi=180)
    plt.close(fig)
    for path in output.glob("*.svg"):
        path.write_text(chr(10).join(line.rstrip() for line in path.read_text().splitlines())+chr(10))


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--input',default='runtime/benchmarks/paper-20261030')
    parser.add_argument('--output',default='results/paper-20261030')
    parser.add_argument('--plots',action='store_true')
    args=parser.parse_args()
    source=Path(args.input); output=Path(args.output);output.mkdir(parents=True,exist_ok=True)
    series={};raw={}
    for name in ['writes','reads','dns','scaling','writes-confirmation','dns-confirmation']:
        path=source/name/'runs.json'
        if not path.exists():continue
        raw[name]=json.loads(path.read_text());series[name]=summarize(raw[name],name)
    (output/'runs.json').write_text(json.dumps(raw,indent=2)+'\n')
    rows=[row for group in series.values() for row in group]
    with (output/'summary.csv').open('w') as stream:
        writer=csv.DictWriter(stream,fieldnames=list(rows[0]),lineterminator=chr(10));writer.writeheader();writer.writerows(rows)
    manifest=[]
    for name in raw:
        for path in sorted((source/name).rglob('*')):
            if path.is_file():
                digest=hashlib.sha256(path.read_bytes()).hexdigest()
                manifest.append(dict(path=str(path.relative_to(source)),bytes=path.stat().st_size,sha256=digest))
    (output/'raw-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
    environment=json.loads((source/'writes/environment.json').read_text())
    (output/'environment.json').write_text(json.dumps({k:v for k,v in environment.items() if k!='source_diff'},indent=2)+'\n')
    errors=Counter()
    for name in ['reads','scaling','writes-confirmation']:
        for path in (source/name).rglob('requests.jsonl'):
            for line in path.open():
                sample=json.loads(line)
                if sample.get('error'):errors[sample['error']]+=1
    (output/'error-counts.json').write_text(json.dumps(dict(errors),indent=2)+'\n')
    text=['# Registration and DNS benchmark measurements, 2026-10-30',
          '', 'The reconstructed registration and DNS components were measured on a remote Linux server using real Fabric commits and authoritative CoreDNS responses. The experiment follows the offered-load and node-scaling questions in the two reference papers. Tables below separate the published measurements from this implementation.',
          '', '## Reference experiments', '',
          '| Paper | Registration at 300 offered TPS | DNS reference |',
          '|---|---|---|',
          '| Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership | 179.2 TPS; 8.62 s mean latency | 8,947.6 QPS at 9,000 offered QPS; success above 99%; latency below 4 ms |',
          '| Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications | 190.6 TPS; 6.82 s mean latency | 8,947.6 QPS at 9,000 offered QPS; success above 99%; latency below 4 ms |',
          '', 'References: COMPSAC 2023, DOI 10.1109/COMPSAC57700.2023.00065, experimental evaluation; World Wide Web 2025, DOI 10.1007/s11280-025-01337-z, Section 8. The papers use four 16-vCPU, 32-GB servers and a separate client, with Caliper for writes and dnsperf for DNS. Their scale studies include fixed-orderer peer scaling; the second paper also varies committee size.',
          '', '## Measurement contract', '',
          '- Fabric 2.4.9, CoreDNS 1.12.0 with the disredis plugin, Redis 7.2.5, and dnsperf 2.9.0. Fabric uses five Raft orderers, a 500-ms block timeout, and a 10-transaction block limit.',
          '- Load sweeps use five peer organizations, one peer per organization, and three-organization endorsement. Scale runs use 2, 4, 8, 12, and 16 organizations, one peer per organization, five orderers, and floor(N/2)+1 endorsement.',
          '- The server has 128 logical CPUs (Intel Xeon Gold 6338 at 2.00 GHz). Clients and server processes share this physical machine; CPU quotas are not applied. A separate four-organization deployment remains idle.',
          '- Registration submits signed data.register operations with distinct data identifiers and a common owner within each run. Owner setup, request signing, and full-state checks are outside the timed window. No automatic retry is used. The benchmark calls the Go Fabric Gateway client directly.',
          '- A successful write means the Gateway returned a valid Fabric commit. Throughput is successful calls divided by scheduling plus drain time. Latency covers each Gateway call through valid-commit return. DNS visibility latency is a separate completion condition and is not measured by this write latency.',
          '- DNS queries request URI records for 1,000 existing data identifiers. The projection is verified before the DNS sweep. Success means a NOERROR response; loss and other response codes count against success. dnsperf uses 16 clients, four threads, and a two-second query timeout.',
          '- Chain reads invoke GetData through the Gateway. They measure committed-state reads, rather than consensus submissions.',
          '- Each sweep point schedules load for five seconds and is repeated three times. Sweep order is deterministically shuffled. Scale sizes run in ascending order. Values are arithmetic means across runs; ± is sample standard deviation. Longer confirmation runs use 30 seconds and three repetitions.',
          '- Fresh ledgers store independent object records and retry receipts. Publication serials are derived from receipts, avoiding a shared write counter between independent registrations. Existing sequence-based deployments retain their format. This is the measured reconstruction guided by the papers and source code.',
          '', 'The hardware, software versions, client placement, load generator, and explicit workload above define these measurements. Published values provide context; speedup ratios against those values are not computed.',
          '', '![Throughput sweeps](throughput.png)', '', '![Latency sweeps](latency.png)', '',
          'Figures show five-second sweeps with three repetitions. Error bars denote sample standard deviation; dashed lines show offered load. The scale panel uses five Raft orderers and a Gateway concurrency limit of 500.',
          '', '## Registration load sweep', '', table(series['writes'],'Offered registrations/s'),
          '', '## Authoritative DNS load sweep', '', table(series['dns'],'Offered queries/s'),
          '', '## Committed-state read sweep', '',table(series['reads'],'Offered reads/s'),
          '', '## Organization scaling', '',table(series['scaling'],'Peer organizations'),
          '', 'All scale runs offer 300 registrations/s. Increasing organization count increases the majority endorsement requirement while preserving the five-orderer Raft group. Failed and ambiguous client completions remain in the denominator; a failed response does not prove that the transaction was absent from the ledger. Run summaries include before/after state counts and raw request errors.']
    if 'writes-confirmation' in series:
        text+=['','## Longer registration confirmation','',table(series['writes-confirmation'],'Offered registrations/s'),'',
               'Throughput above includes final request drain. The corresponding mean valid completions during the 30-second scheduling windows are '+', '.join(f"{r['valid_completions_during_schedule_tps']:.2f}" for r in raw['writes-confirmation'])+' requests/s. Timeout responses extend the drain interval; per-run summaries retain both rates.']
    if 'dns-confirmation' in series:text+=['','## Longer DNS confirmation','',table(series['dns-confirmation'],'Offered queries/s')]
    text+=['', '## Error interpretation', '',
           'At 9,000 offered chain reads/s, the Gateway rejects requests after its configured 500-concurrent-request limit is reached. These are recorded as request errors. The read sweep retains both successes and errors. The same concurrency-limit error occurs in the 12- and 16-organization runs. These measurements describe the deployed Gateway configuration, rather than an inherent Raft or consensus capacity bound. Longer registration runs also record DeadlineExceeded responses; all such calls remain failures in the client completion metric. Error strings and counts are exported in error-counts.json.',
           '', '## Artifacts and reproduction', '',
           'summary.csv contains aggregate values; runs.json contains every run summary; error-counts.json preserves error categories; raw-manifest.json records hashes and sizes for the full raw logs retained under runtime/benchmarks/paper-20261030. environment.json records the source base revision, host description, and topology. Source changes during measurement are retained in the raw environment files. The source commit containing this report records the measured implementation.',
           '', 'The interrupted DNS readiness trial is retained separately as dns-interrupted-readiness with INVALID.txt and is excluded from these aggregates. The early shared-counter diagnostic is retained under runtime/benchmarks/pilot. Valid sweeps and error samples are all included.',
           '', 'Build the commands following docs/runbook.md. Deploy the measured topology and run workloads sequentially:', '',
           '\x60\x60\x60bash',
           'export DIS_RUNTIME="$PWD/runtime/experiments/paper"',
           'python3 scripts/network.py up --topology deploy/topologies/paper-5org.yaml',
           'python3 scripts/services.py up --api-port 48080 --dns-port 48530 --redis-port 48679',
           'python3 scripts/benchmark-paper.py --kind writes --rates 50,100,150,200,250,300,350,400,450 --duration 5 --repetitions 3 --out runtime/benchmarks/new/writes',
           '# Create a dnsperf input file containing existing-data-id.data. URI, one query per line.',
           '# Restart publication to load and verify the completed registration dataset before read-only DNS trials.',
           'python3 scripts/services.py down',
           'python3 scripts/services.py up',
           'python3 scripts/benchmark-paper.py --kind dns --rates 1000,2000,3000,4000,5000,6000,7000,8000,9000,10000,11000,12000,13000,14000,15000 --duration 5 --repetitions 3 --dns-queries queries.txt --out runtime/benchmarks/new/dns',
           'python3 scripts/benchmark-paper.py --kind reads --rates 1000,3000,6000,9000 --duration 5 --repetitions 3 --out runtime/benchmarks/new/reads',
           'python3 scripts/benchmark-scale.py --counts 2,4,8,12,16 --out runtime/benchmarks/new/scaling',
           'python3 scripts/benchmark-paper.py --kind writes --rates 300 --duration 30 --repetitions 3 --out runtime/benchmarks/new/writes-confirmation',
           'python3 scripts/services.py down',
           'python3 scripts/services.py up',
           'python3 scripts/benchmark-paper.py --kind dns --rates 9000 --duration 30 --repetitions 3 --dns-queries queries.txt --out runtime/benchmarks/new/dns-confirmation',
           'python3 scripts/summarize-benchmarks.py --input runtime/benchmarks/new --output results/new --plots',
           '\x60\x60\x60', '',
           'Plot generation requires matplotlib. Run performance workloads sequentially. Keep the per-run JSONL request logs, dnsperf output, source diff, and one-second CPU/RSS samples for subsequent analysis.', '']
    (output/'README.md').write_text('\n'.join(text).replace('\x60','~').replace('~',chr(96)))
    if args.plots:figures(series, output)
    print(json.dumps(series,indent=2))


if __name__=='__main__':main()
