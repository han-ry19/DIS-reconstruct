#!/usr/bin/env python3
"""Manage the DIS API, committed-event publisher, Redis, and CoreDNS."""
import argparse,fcntl,json,os,signal,socket,subprocess,time,urllib.request
from pathlib import Path
from network import stamp,live,RUNTIME
ROOT=Path(__file__).resolve().parents[1]
WORK=RUNTIME/'services'
def main():
 parser=argparse.ArgumentParser();parser.add_argument('action',choices=['up','down','status']);parser.add_argument('--api-port',type=int);parser.add_argument('--dns-port',type=int);parser.add_argument('--redis-port',type=int);args=parser.parse_args()
 WORK.mkdir(parents=True,exist_ok=True)
 with (WORK/'manager.lock').open('a') as lock:
  fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
  path=WORK/'services.json';m=json.loads(path.read_text()) if path.exists() else {'processes':{},'api_port':28080,'dns_port':28530,'redis_port':28679}
  m.setdefault('redis_port',28679)
  if args.action=='up':
   for field in ['api_port','dns_port','redis_port']:
    requested=getattr(args,field)
    if requested is not None and requested!=m[field]:
     if any(live(p) for p in m['processes'].values()):raise RuntimeError('Stop services before changing ports')
     m[field]=requested
  def save():path.write_text(json.dumps(m,indent=2))
  def stop():
   for p in reversed(list(m['processes'].values())):
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
  if args.action=='status':print(json.dumps(dict(api_port=m['api_port'],dns_port=m['dns_port'],redis_port=m['redis_port'],processes={name:live(p) for name,p in m['processes'].items()}),indent=2));return
  if args.action=='down':stop();return
  dns=RUNTIME/'dns';profile=RUNTIME/'fabric/gateway.json'
  sync=[ROOT/'bin/dis-sync','-fabric-config',profile,'-out',dns,'-redis',f'127.0.0.1:{m["redis_port"]}']
  try:
   redisdir=RUNTIME/'redis';redisdir.mkdir(parents=True,exist_ok=True)
   spawn('redis',[ROOT/'runtime/tools/redis-7.2.5/src/redis-server','--bind','127.0.0.1','--port',m['redis_port'],'--dir',redisdir,'--appendonly','yes','--appendfsync','always','--save',''])
   deadline=time.monotonic()+15
   while True:
    try:
     with socket.create_connection(('127.0.0.1',m['redis_port']),timeout=.2):break
    except OSError:
     if time.monotonic()>deadline:raise RuntimeError('Redis startup timeout')
     time.sleep(.1)
   subprocess.run([str(v) for v in sync]+['-once'],check=True)
   (WORK/'Corefile').write_text(f'''.:{m["dns_port"]} {{
 bind 127.0.0.1
 errors
 disredis {{
  address 127.0.0.1:{m["redis_port"]}
 }}
}}
''')
   spawn('coredns',[ROOT/'bin/dis-coredns','-conf',WORK/'Corefile'])
   spawn('sync',sync)
   spawn('api',[ROOT/'bin/dis-api','-listen',f'127.0.0.1:{m["api_port"]}','-fabric-config',profile])
   deadline=time.monotonic()+180
   while True:
    if any(not live(p) for p in m['processes'].values()):raise RuntimeError('Service exited; inspect runtime/services logs')
    try:
     with urllib.request.urlopen(f'http://127.0.0.1:{m["api_port"]}/healthz',timeout=1) as response:assert json.load(response)['backend']=='fabric'
     check=subprocess.run([str(v) for v in sync]+['-check','-verify',f'127.0.0.1:{m["dns_port"]}'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=120)
     if check.returncode==0:break
    except (OSError,subprocess.TimeoutExpired):pass
    if time.monotonic()>deadline:raise RuntimeError('Service readiness timeout')
    time.sleep(.5)
   print(json.dumps(dict(api=f'http://127.0.0.1:{m["api_port"]}',dns=f'127.0.0.1:{m["dns_port"]}',redis=f'127.0.0.1:{m["redis_port"]}',status='ready'),indent=2))
  except BaseException:stop();raise
if __name__=='__main__':main()
