#!/usr/bin/env python3
"""Run signed data registration with Caliper's Fabric peer Gateway connector."""
import argparse
import json
import shutil
import os
from pathlib import Path
import random
import subprocess
from network import ROOT, WORK, load
from caliper_results import summarize


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--out',required=True)
    parser.add_argument('--rates',default='50,100,150,200,250,300,350,400,450')
    parser.add_argument('--duration',type=int,default=30)
    parser.add_argument('--repetitions',type=int,default=3)
    parser.add_argument('--workers',type=int,default=4)
    parser.add_argument('--tools',default=str(ROOT/'runtime/caliper'))
    args=parser.parse_args()
    rates=[int(v) for v in args.rates.split(',')]
    if min(rates+[args.duration,args.repetitions,args.workers])<1:
        parser.error('rates, duration, repetitions, and workers must be positive')
    output=Path(args.out).resolve()
    if output.exists():
        parser.error('output must be a new directory')
    output.mkdir(parents=True)
    source = output/'source'
    source.mkdir()
    for relative in ['benchmarks/caliper/register.js', 'benchmarks/caliper/package.json', 'scripts/benchmark-caliper.py', 'scripts/caliper_results.py']:
        target = source/relative
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT/relative, target)
    profile=json.loads((WORK/'gateway.json').read_text())
    manifest=load()
    network={'name':'DIS registration','version':'2.0.0','caliper':{'blockchain':'fabric'},
        'channels':[{'channelName':profile['channel'],'contracts':[{'id':profile['chaincode']}]}],
        'organizations':[{'mspid':profile['msp_id'],
            'identities':{'certificates':[{'name':'benchmark','clientSignedCert':{'path':profile['certificate']},
                'clientPrivateKey':{'path':profile['private_key']}}]},
            'peers':[{'endpoint':profile['endpoint'],'tlsCACerts':{'path':profile['tls_cert']},
                'grpcOptions':{'ssl-target-name-override':profile['server_name'],'grpc.max_receive_message_length':64*1024*1024}}]}]}
    schedule=[(rate,rep) for rep in range(args.repetitions) for rate in rates]
    random.Random(20261030).shuffle(schedule)
    rounds=[]
    for rate,rep in schedule:
        rounds.append({'label':f'register-{rate}-r{rep}','description':'Signed data.register; valid Fabric commit',
            'txDuration':args.duration,'rateControl':{'type':'fixed-rate','opts':{'tps':rate}},
            'workload':{'module':str(ROOT/'benchmarks/caliper/register.js'),
                'arguments':{'domain':manifest['domain'],'mspID':profile['msp_id'],'rate':rate,
                    'duration':args.duration,'output':str(output/'requests')}}})
    benchmark={'test':{'name':'DIS registration','description':'Valid committed data registrations',
        'workers':{'type':'local','number':args.workers},'rounds':rounds}}
    (output/'network.json').write_text(json.dumps(network,indent=2))
    (output/'benchmark.json').write_text(json.dumps(benchmark,indent=2))
    tools=Path(args.tools).resolve()
    env=dict(os.environ,NODE_PATH=str(tools/'node_modules'))
    command=[str(tools/'node_modules/.bin/caliper'),'launch','manager','--caliper-workspace',str(output),
        '--caliper-networkconfig',str(output/'network.json'),'--caliper-benchconfig',str(output/'benchmark.json'),
        '--caliper-flow-only-test','--caliper-report-path',str(output/'report.html')]
    environment={'arguments':vars(args),'topology':manifest['topology'],'source_revision':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),
        'source_diff':subprocess.check_output(['git','diff'],text=True),'command':command,'schedule':schedule,
        'client_placement':'same physical host as Fabric processes','caliper_version':'0.6.0','binding':'fabric:2.4',
        'gateway_sdk':'1.5.0','node_version':subprocess.check_output(['node','--version'],text=True).strip()}
    (output/'environment.json').write_text(json.dumps(environment,indent=2))
    with (output/'caliper.log').open('w') as log:
        result=subprocess.run(command,cwd=tools,env=env,stdout=log,stderr=subprocess.STDOUT)
    print(f'Caliper exit status: {result.returncode}; artifacts: {output}',flush=True)
    if result.returncode:
        print((output/'caliper.log').read_text()[-6000:])
    if result.returncode == 0:
        for row in summarize(output):
            print(json.dumps(row), flush=True)
    raise SystemExit(result.returncode)


if __name__=='__main__':main()
