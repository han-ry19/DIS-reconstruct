#!/usr/bin/env python3
"""Start the API, Fabric-to-DNS synchronizer, and CoreDNS."""
import argparse,fcntl,json,os,signal,socket,subprocess,time,urllib.request
from pathlib import Path
from network import stamp,live
ROOT=Path(__file__).resolve().parents[1]
WORK=ROOT/'runtime/services'
def main():
 parser=argparse.ArgumentParser();parser.add_argument('action',choices=['up','down','status']);parser.add_argument('--api-port',type=int,default=28080);parser.add_argument('--dns-port',type=int,default=28530);args=parser.parse_args()
 WORK.mkdir(parents=True,exist_ok=True)
 with (WORK/'manager.lock').open('a') as lock:
  fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
  path=WORK/'services.json';m=json.loads(path.read_text()) if path.exists() else {'processes':{},'api_port':args.api_port,'dns_port':args.dns_port}
  def save():path.write_text(json.dumps(m,indent=2))
  def stop():
   for p in m['processes'].values():
    if live(p):os.killpg(p['pid'],signal.SIGTERM)
   deadline=time.monotonic()+10
   while time.monotonic()<deadline and any(live(p) for p in m['processes'].values()):time.sleep(.1)
   for p in m['processes'].values():
    if live(p):os.killpg(p['pid'],signal.SIGKILL)
   m['processes']={};save()
  def spawn(name,command):
   old=m['processes'].get(name)
   if old and live(old):return
   output=(WORK/(name+'.log')).open('ab');p=subprocess.Popen([str(v) for v in command],stdout=output,stderr=output,start_new_session=True);output.close()
   m['processes'][name]={'pid':p.pid,'start':stamp(p.pid)};save()
  if args.action=='status':print(json.dumps(dict(api_port=m['api_port'],dns_port=m['dns_port'],processes={name:live(p) for name,p in m['processes'].items()}),indent=2));return
  if args.action=='down':stop();return
  dns=ROOT/'runtime/dns';profile=ROOT/'runtime/fabric/gateway.json'
  sync=[ROOT/'bin/dis-sync','-fabric-config',profile,'-out',dns]
  subprocess.run([str(v) for v in sync]+['-once'],check=True)
  core=[]
  for zone in ['uid','data']:
   core.append(f'{zone}:{m["dns_port"]} {{\n bind 127.0.0.1\n errors\n file {dns}/{zone}.zone {zone}. {{\n  reload 1s\n }}\n transfer {{\n  to 127.0.0.1\n }}\n}}\n')
  (WORK/'Corefile').write_text('\n'.join(core))
  try:
   spawn('coredns',[ROOT/'runtime/tools/coredns','-conf',WORK/'Corefile'])
   spawn('sync',sync)
   spawn('api',[ROOT/'bin/dis-api','-listen',f'127.0.0.1:{m["api_port"]}','-fabric-config',profile])
   deadline=time.monotonic()+45
   while True:
    if any(not live(p) for p in m['processes'].values()):raise RuntimeError('Service exited; inspect runtime/services logs')
    try:
     with urllib.request.urlopen(f'http://127.0.0.1:{m["api_port"]}/healthz',timeout=1) as response:assert json.load(response)['backend']=='fabric'
     check=subprocess.run([str(v) for v in sync]+['-once','-verify',f'127.0.0.1:{m["dns_port"]}'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=20)
     if check.returncode==0:break
    except (OSError,subprocess.TimeoutExpired):pass
    if time.monotonic()>deadline:raise RuntimeError('Service readiness timeout')
    time.sleep(.5)
   print(json.dumps(dict(api=f'http://127.0.0.1:{m["api_port"]}',dns=f'127.0.0.1:{m["dns_port"]}',status='ready'),indent=2))
  except BaseException:stop();raise
if __name__=='__main__':main()
