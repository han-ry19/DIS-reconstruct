#!/usr/bin/env python3
"""Verify Fabric/DNS workflow and restart recovery on a running DIS deployment."""
import argparse,importlib.util,json,subprocess,time,urllib.request
from pathlib import Path
from network import ROOT,WORK,run,clientenv,BIN

def main():
 parser=argparse.ArgumentParser();parser.add_argument('--restart',action='store_true',help='Restart the project services and network');args=parser.parse_args()
 services=json.loads((ROOT/'runtime/services/services.json').read_text());api=f'http://127.0.0.1:{services["api_port"]}';dns=f'127.0.0.1:{services["dns_port"]}'
 def command(values):return subprocess.run([str(v) for v in values],check=True,capture_output=True,text=True).stdout
 transcript=ROOT/'runtime/replay-requests.json'
 demo=command([ROOT/'bin/dis-demo','-url',api,'-transcript',transcript]);record=json.loads(demo.splitlines()[-1]);print(demo)
 def verify():
  query=api+'/v1/authorizations?data_id='+record['data_id']+'&owner_id='+record['owner_id']+'&recipient='+record['recipient']
  with urllib.request.urlopen(query,timeout=15) as response:assert json.load(response)==record
  for i in range(4):
   body=run([BIN/'peer','chaincode','query','-C','dis','-n','dis','-c',json.dumps({'Args':['GetAuthorization',record['data_id'],record['owner_id'],record['recipient']]})],clientenv(i))
   assert json.loads(body)==record
  deadline=time.monotonic()+20
  while True:
   try:
    command([ROOT/'bin/dis-sync','-fabric-config',WORK/'gateway.json','-redis',f'127.0.0.1:{services["redis_port"]}','-check','-verify',dns])
    break
   except subprocess.CalledProcessError:
    if time.monotonic()>deadline:raise
    time.sleep(.2)
 verify()
 if args.restart:
  for script,action in [('services.py','down'),('network.py','down'),('network.py','up'),('services.py','up')]:
   command(['python3',ROOT/'scripts'/script,action])
  verify()
 replay=json.loads(transcript.read_text())
 before=json.loads(command([ROOT/'bin/dis-check','-fabric-config',WORK/'gateway.json']))['sequence']
 for step in replay:
  request=urllib.request.Request(api+'/v1/transactions',data=json.dumps(step['request']).encode(),headers={'Content-Type':'application/json'})
  with urllib.request.urlopen(request,timeout=30) as response:assert json.load(response)==step['result']
 after=json.loads(command([ROOT/'bin/dis-check','-fabric-config',WORK/'gateway.json']))['sequence']
 assert before==after
 report={'committed_event_publication':'passed','workflow':'passed','exact_replay':'passed','four_peer_queries':'passed','complete_dns_axfr':'passed','restart_recovery':'passed' if args.restart else 'not-requested','authorization':record}
 output=ROOT/'runtime/acceptance.json';output.write_text(json.dumps(report,indent=2));print(json.dumps(report,indent=2))
if __name__=='__main__':main()
