"""Validate per-worker samples against Caliper's HTML summary."""
from html.parser import HTMLParser
import json
from pathlib import Path
import statistics


class TableParser(HTMLParser):
    def __init__(self):
        super().__init__();self.rows=[];self.row=[];self.cell=None
    def handle_starttag(self,tag,attrs):
        if tag=='tr':self.row=[]
        if tag in ['td','th']:self.cell=[]
    def handle_data(self,data):
        if self.cell is not None:self.cell.append(data)
    def handle_endtag(self,tag):
        if tag in ['td','th'] and self.cell is not None:
            self.row.append(''.join(self.cell).strip());self.cell=None
        if tag=='tr' and self.row:self.rows.append(self.row)


def summarize(output):
    output=Path(output)
    benchmark=json.loads((output/'benchmark.json').read_text())['test']
    parser=TableParser();parser.feed((output/'report.html').read_text())
    reports={r[0]:r for r in parser.rows if len(r)==8 and r[0].startswith('register-')}
    results=[]
    for index,round in enumerate(benchmark['rounds']):
        samples=[];owners=[]
        files=list((output/'requests').glob(f'round-{index}-worker-*.json'))
        if len(files)!=benchmark['workers']['number']:raise RuntimeError('Missing worker samples')
        for path in files:
            worker=json.loads(path.read_text());owners.append(worker['owner']);samples.extend(worker['samples'])
            if worker['requests']!=len(worker['samples']):raise RuntimeError('Missing transaction sample')
        success=[s for s in samples if s['status']=='success']
        if len({s['transaction_id'] for s in samples})!=len(samples):raise RuntimeError('Duplicate transaction ID')
        for sample in success:
            payload=json.loads(sample['result'])
            if payload['object_id']!=sample['object_id'] or payload['operation']!='data.register':
                raise RuntimeError('Unexpected committed business result')
            if payload.get('transaction_id') and payload['transaction_id']!=sample['transaction_id']:
                raise RuntimeError('Result transaction ID does not match Caliper')
        report=reports[round['label']]
        if int(report[1])!=len(success) or int(report[2])!=len(samples)-len(success):
            raise RuntimeError('Caliper summary and raw status counts disagree')
        results.append(dict(label=round['label'],offered_tps=round['rateControl']['opts']['tps'],duration=round['txDuration'],
            workers=len(files),submitted=len(samples),successful=len(success),failed=len(samples)-len(success),
            success_percent=100*len(success)/len(samples),send_rate_tps=float(report[3]),throughput_tps=float(report[7]),
            successful_commit_tps=len(success)*1000/(max(s['finalized'] for s in samples)-min(s['created'] for s in samples)),
            latency_mean_ms=statistics.mean(s['finalized']-s['created'] for s in success) if success else None,
            caliper_latency_mean_s=float(report[6]),latency_max_ms=max((s['finalized']-s['created'] for s in success),default=None),owners=owners))
    (output/'summary.json').write_text(json.dumps(results,indent=2)+'\n')
    return results


if __name__=='__main__':
    import sys
    print(json.dumps(summarize(sys.argv[1]),indent=2))
