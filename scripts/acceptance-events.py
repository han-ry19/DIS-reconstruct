#!/usr/bin/env python3
"""Verify event catch-up while the publisher is offline, without a full refresh."""
import fcntl,json,os,signal,subprocess,time
from pathlib import Path
from network import ROOT,WORK,stamp,live

def main():
 servicesdir=ROOT/'runtime/services'
 with (servicesdir/'manager.lock').open('a') as lock:
  fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
  path=servicesdir/'services.json';m=json.loads(path.read_text());old=m['processes']['sync']
  if not live(old):raise RuntimeError('Start all services first')
  os.killpg(old['pid'],signal.SIGTERM)
  deadline=time.monotonic()+10
  while live(old):
   if time.monotonic()>deadline:raise RuntimeError('Publisher did not stop')
   time.sleep(.1)
  args=[ROOT/'bin/dis-sync','-fabric-config',WORK/'gateway.json','-redis',f'127.0.0.1:{m["redis_port"]}','-out',ROOT/'runtime/dns']
  def check():return subprocess.run([str(v) for v in args]+['-check','-verify',f'127.0.0.1:{m["dns_port"]}'],capture_output=True,text=True)
  try:
   subprocess.run([str(ROOT/'bin/dis-demo'),'-url',f'http://127.0.0.1:{m["api_port"]}'],check=True,capture_output=True,text=True)
   if check().returncode==0:raise RuntimeError('Projection changed while publisher was stopped')
  finally:
   output=(servicesdir/'sync.log').open('ab');p=subprocess.Popen([str(v) for v in args],stdout=output,stderr=output,start_new_session=True);output.close()
   m['processes']['sync']={'pid':p.pid,'start':stamp(p.pid)};path.write_text(json.dumps(m,indent=2))
  deadline=time.monotonic()+30
  while True:
   result=check()
   if result.returncode==0:break
   if not live(m['processes']['sync']) or time.monotonic()>deadline:raise RuntimeError(result.stderr)
   time.sleep(.2)
  report={'offline_commits':'passed','checkpoint_catch_up':'passed','authoritative_dns':'passed'}
  (ROOT/'runtime/acceptance-events.json').write_text(json.dumps(report,indent=2));print(json.dumps(report,indent=2))
if __name__=='__main__':main()
