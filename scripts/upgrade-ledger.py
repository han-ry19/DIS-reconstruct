#!/usr/bin/env python3
"""Upgrade the installed DIS chaincode and migrate its v1 state in place."""
import fcntl,json,subprocess
from network import ROOT,WORK,BIN,run,clientenv,orderflags,peerflags,basic,chaincodes,save,load,RUNTIME,prepare
from topology import endorsement

def main():
 with (WORK/'manager.lock').open('a') as lock:
  fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
  m=load()
  if m['phase']!='initialized':raise RuntimeError('Initialize the network before upgrading')
  prepare(m);basic(m);chaincodes(m)
  sequence=m.get('definition_sequence',1)
  if m.get('ledger_schema',1)<2:
   policy=endorsement(m['topology'])
   definition=['--channelID','dis','--name','dis','--version','2','--sequence',str(sequence+1),'--signature-policy',policy]
   for i in range(m['topology']['organizations']):run([BIN/'peer','lifecycle','chaincode','approveformyorg']+definition+['--package-id',m['packages'][str(i)]]+orderflags(m),clientenv(i))
   run([BIN/'peer','lifecycle','chaincode','commit']+definition+orderflags(m)+peerflags(m),clientenv(0))
   m['definition_sequence']=sequence+1;m['ledger_schema']=2;save(m)
  subprocess.run([str(ROOT/'bin/dis-migrate'),'-fabric-config',str(WORK/'gateway.json')],check=True)
  if m.get('publication_digest_version',1)<2:
   status=RUNTIME/'dns/status.json'
   if status.exists():
    backup=RUNTIME/'reconstruction-backup';backup.mkdir(parents=True,exist_ok=True)
    status.rename(backup/'publication-status-before-v2.json')
   m['publication_digest_version']=2;save(m)
  print('Chaincode definition upgraded; business state and request receipts migrated')
if __name__=='__main__':main()
