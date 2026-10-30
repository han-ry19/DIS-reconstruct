#!/usr/bin/env python3
"""Run the scoped, native Fabric network used by the DIS reconstruction."""
import argparse, copy, fcntl, io, json, os, signal, socket, subprocess, sys, tarfile, time
from pathlib import Path
import yaml

ROOT=Path(__file__).resolve().parents[1]
TOOLS=ROOT/'runtime/tools/fabric'
WORK=ROOT/'runtime/fabric'
BIN=TOOLS/'bin'
ENV=dict(os.environ,FABRIC_LOGGING_SPEC='warn')

def write(path,value):
    path.parent.mkdir(parents=True,exist_ok=True)
    path.write_text(json.dumps(value,indent=2))

def save(manifest):write(WORK/'network.json',manifest)

def run(args,env=None,timeout=120):
    proc=subprocess.run([str(a) for a in args],env=env or ENV,capture_output=True,text=True,timeout=timeout)
    with (WORK/'commands.log').open('a') as log:
        log.write(' '.join(str(a) for a in args)+'\n'+proc.stdout+proc.stderr+'\n')
    if proc.returncode:raise RuntimeError(proc.stderr[-3000:] or proc.stdout[-3000:])
    return proc.stdout

def stamp(pid):
    try:
        fields=Path('/proc/'+str(pid)+'/stat').read_text().rsplit(')',1)[1].split()
        return None if fields[0]=='Z' else fields[19]
    except (FileNotFoundError,ProcessLookupError):return None

def live(process):return stamp(process['pid'])==process['start']

def spawn(m,name,args,env):
    old=m['processes'].get(name)
    if old and live(old):return
    log=(WORK/(name+'.log')).open('ab')
    p=subprocess.Popen([str(a) for a in args],env=env,stdout=log,stderr=log,start_new_session=True);log.close()
    m['processes'][name]={'pid':p.pid,'start':stamp(p.pid)};save(m)

def ready(m,name,port,timeout=40):
    until=time.monotonic()+timeout
    while time.monotonic()<until:
        if not live(m['processes'][name]):raise RuntimeError(name+' exited; see '+str(WORK/(name+'.log')))
        try:
            with socket.create_connection(('127.0.0.1',port),timeout=.2):return
        except OSError:time.sleep(.1)
    raise RuntimeError(name+' startup timeout')

def stop(m):
    for name,p in reversed(list(m['processes'].items())):
        if live(p):os.killpg(p['pid'],signal.SIGTERM)
    end=time.monotonic()+10
    while time.monotonic()<end and any(live(p) for p in m['processes'].values()):time.sleep(.1)
    for name,p in m['processes'].items():
        if live(p):os.killpg(p['pid'],signal.SIGKILL)
    m['processes']={};save(m)

def orgpath(i):return WORK/f'crypto/peerOrganizations/org{i}.dis'
def peerpath(i):return orgpath(i)/f'peers/peer0.org{i}.dis'
def orderpath(i):return WORK/f'crypto/ordererOrganizations/orderer.dis/orderers/o{i}.orderer.dis'
def clientenv(i):
    return dict(ENV,FABRIC_CFG_PATH=str(WORK/f'p{i}'),CORE_PEER_MSPCONFIGPATH=str(orgpath(i)/f'users/Admin@org{i}.dis/msp'))
def orderflags(m):return ['-o',f"127.0.0.1:{m['base']}",'--tls','--cafile',orderpath(0)/'tls/ca.crt']
def peerflags(m):
    flags=[]
    for i in range(4):flags+=['--peerAddresses',f"127.0.0.1:{m['base']+10+i}",'--tlsRootCertFiles',peerpath(i)/'tls/ca.crt']
    return flags

def configure(base):
    if not (BIN/'peer').exists():raise RuntimeError('Install Fabric tools using scripts/bootstrap.py first')
    sockets=[]
    try:
        for offset in list(range(4))+list(range(10,14))+list(range(20,24))+list(range(30,34))+list(range(40,44))+list(range(50,54))+list(range(60,64)):
            s=socket.socket();s.bind(('127.0.0.1',base+offset));sockets.append(s)
    finally:
        for s in sockets:s.close()
    m={'base':base,'domain':'dis-local','processes':{},'phase':'configured','packages':{},'ledger_schema':2,'definition_sequence':1}
    write(WORK/'crypto.json',{'OrdererOrgs':[{'Name':'Orderer','Domain':'orderer.dis','EnableNodeOUs':True,'Specs':[{'Hostname':f'o{i}','SANS':['localhost','127.0.0.1']} for i in range(4)]}], 'PeerOrgs':[{'Name':f'Org{i}','Domain':f'org{i}.dis','EnableNodeOUs':True,'Specs':[{'Hostname':'peer0','SANS':['localhost','127.0.0.1']}],'Users':{'Count':1}} for i in range(4)]})
    run([BIN/'cryptogen','generate','--config',WORK/'crypto.json','--output',WORK/'crypto'])
    sig=lambda rule:dict(Type='Signature',Rule=rule)
    imp=lambda rule:dict(Type='ImplicitMeta',Rule=rule)
    orgs=[]
    for i in range(4):
        mid=f'Org{i}MSP'
        orgs.append(dict(Name=mid,ID=mid,MSPDir=str(orgpath(i)/'msp'),AnchorPeers=[dict(Host='127.0.0.1',Port=base+10+i)],Policies=dict(Readers=sig(f"OR('{mid}.member')"),Writers=sig(f"OR('{mid}.member')"),Admins=sig(f"OR('{mid}.admin')"),Endorsement=sig(f"OR('{mid}.peer')"))))
    orderorg=dict(Name='OrdererMSP',ID='OrdererMSP',MSPDir=str(WORK/'crypto/ordererOrganizations/orderer.dis/msp'),Policies=dict(Readers=sig("OR('OrdererMSP.member')"),Writers=sig("OR('OrdererMSP.member')"),Admins=sig("OR('OrdererMSP.admin')")))
    channel=dict(Capabilities={'V2_0':True},Policies=dict(Readers=imp('ANY Readers'),Writers=imp('ANY Writers'),Admins=imp('MAJORITY Admins')))
    order=dict(OrdererType='etcdraft',Addresses=[f'127.0.0.1:{base+i}' for i in range(4)],BatchTimeout='500ms',BatchSize=dict(MaxMessageCount=10,AbsoluteMaxBytes='10 MB',PreferredMaxBytes='512 KB'),EtcdRaft=dict(Consenters=[dict(Host='127.0.0.1',Port=base+i,ClientTLSCert=str(orderpath(i)/'tls/server.crt'),ServerTLSCert=str(orderpath(i)/'tls/server.crt')) for i in range(4)],Options=dict(TickInterval='100ms',ElectionTick=10,HeartbeatTick=1,MaxInflightBlocks=5,SnapshotIntervalSize=20971520)),Organizations=[orderorg],Capabilities={'V2_0':True},Policies=dict(Readers=imp('ANY Readers'),Writers=imp('ANY Writers'),Admins=imp('MAJORITY Admins'),BlockValidation=imp('ANY Writers')))
    app=dict(Organizations=orgs,Capabilities={'V2_0':True},Policies=dict(Readers=imp('ANY Readers'),Writers=imp('ANY Writers'),Admins=imp('MAJORITY Admins'),LifecycleEndorsement=imp('MAJORITY Endorsement'),Endorsement=imp('MAJORITY Endorsement')))
    write(WORK/'configtx.yaml',dict(Organizations=orgs+[orderorg],Profiles=dict(System=dict(channel,Orderer=order,Consortiums=dict(DIS=dict(Organizations=orgs))),Application=dict(channel,Consortium='DIS',Application=app))))
    configenv=dict(ENV,FABRIC_CFG_PATH=str(WORK))
    run([BIN/'configtxgen','-profile','System','-channelID','system','-outputBlock',WORK/'genesis.block'],configenv)
    run([BIN/'configtxgen','-profile','Application','-channelID','dis','-outputCreateChannelTx',WORK/'dis.tx'],configenv)
    builder=WORK/'builder/bin';builder.mkdir(parents=True,exist_ok=True)
    scripts={
    'detect':"import json,sys\nfrom pathlib import Path\nsys.exit(0 if json.loads((Path(sys.argv[2])/'metadata.json').read_text())['type']=='external' else 1)\n",
    'build':"import sys,shutil\nfrom pathlib import Path\nshutil.copyfile(Path(sys.argv[1])/'connection.json',Path(sys.argv[3])/'connection.json')\n",
    'release':"import sys,shutil\nfrom pathlib import Path\np=Path(sys.argv[2])/'chaincode/server';p.mkdir(parents=True)\nshutil.copyfile(Path(sys.argv[1])/'connection.json',p/'connection.json')\n"}
    for name,body in scripts.items():
        p=builder/name;p.write_text('#!'+sys.executable+'\n'+body);p.chmod(0o755)
    core=yaml.safe_load((TOOLS/'config/core.yaml').read_text());ordering=yaml.safe_load((TOOLS/'config/orderer.yaml').read_text())
    for i in range(4):
        folder=WORK/f'o{i}';folder.mkdir(exist_ok=True);cfg=copy.deepcopy(ordering);identity=orderpath(i)
        cfg['General'].update(ListenAddress='127.0.0.1',ListenPort=base+i,LocalMSPID='OrdererMSP',LocalMSPDir=str(identity/'msp'),BootstrapMethod='file',BootstrapFile=str(WORK/'genesis.block'))
        cfg['General']['TLS'].update(Enabled=True,PrivateKey=str(identity/'tls/server.key'),Certificate=str(identity/'tls/server.crt'),RootCAs=[str(identity/'tls/ca.crt')])
        cfg['General']['Cluster'].update(ClientCertificate=str(identity/'tls/server.crt'),ClientPrivateKey=str(identity/'tls/server.key'),RootCAs=[str(identity/'tls/ca.crt')])
        cfg['FileLedger']['Location']=str(folder/'ledger');cfg['Consensus'].update(WALDir=str(folder/'wal'),SnapDir=str(folder/'snap'))
        cfg['Operations']['ListenAddress']=f'127.0.0.1:{base+40+i}';cfg['Admin']['ListenAddress']=f'127.0.0.1:{base+50+i}'
        write(folder/'orderer.yaml',cfg)
        folder=WORK/f'p{i}';folder.mkdir(exist_ok=True);cfg=copy.deepcopy(core);identity=peerpath(i)
        cfg['peer'].update(id=f'p{i}',networkId='dis-reconstruct',address=f'127.0.0.1:{base+10+i}',listenAddress=f'127.0.0.1:{base+10+i}',chaincodeListenAddress=f'127.0.0.1:{base+20+i}',chaincodeAddress=f'127.0.0.1:{base+20+i}',localMspId=f'Org{i}MSP',mspConfigPath=str(identity/'msp'),fileSystemPath=str(folder/'ledger'))
        cfg['peer']['tls'].update(enabled=True,cert={'file':str(identity/'tls/server.crt')},key={'file':str(identity/'tls/server.key')},rootcert={'file':str(identity/'tls/ca.crt')},clientRootCAs={'files':[str(identity/'tls/ca.crt')]})
        cfg['peer']['gossip'].update(bootstrap='',externalEndpoint=f'127.0.0.1:{base+10+i}',useLeaderElection=False,orgLeader=True)
        cfg['peer']['gateway']={'enabled':True}
        cfg['operations']['listenAddress']=f'127.0.0.1:{base+60+i}'
        cfg['chaincode']['externalBuilders']=[dict(name='dis-external',path=str(builder.parent))];cfg['vm']['endpoint']=''
        cfg['ledger']['snapshots']['rootDir']=str(folder/'snapshots')
        write(folder/'core.yaml',cfg)
    save(m);return m

def basic(m):
    for i in range(4):
        spawn(m,f'orderer-{i}',[BIN/'orderer'],dict(ENV,FABRIC_CFG_PATH=str(WORK/f'o{i}')));ready(m,f'orderer-{i}',m['base']+i)
    for i in range(4):
        spawn(m,f'peer-{i}',[BIN/'peer','node','start'],dict(ENV,FABRIC_CFG_PATH=str(WORK/f'p{i}')));ready(m,f'peer-{i}',m['base']+10+i)

def package(i,base):
    def archive(values):
        stream=io.BytesIO()
        with tarfile.open(fileobj=stream,mode='w:gz') as tar:
            for name,value in values.items():
                value=value if isinstance(value,bytes) else json.dumps(value).encode()
                info=tarfile.TarInfo(name);info.size=len(value);tar.addfile(info,io.BytesIO(value))
        return stream.getvalue()
    code=archive({'connection.json':dict(address=f'127.0.0.1:{base+30+i}',dial_timeout='10s',tls_required=False)})
    path=WORK/f'chaincode-{i}.tgz';path.write_bytes(archive({'metadata.json':dict(type='external',label=f'dis-org{i}'),'code.tar.gz':code}));return path

def chaincodes(m):
    for i in range(4):
        if str(i) in m['packages']:
            env=dict(ENV,CHAINCODE_ID=m['packages'][str(i)],CHAINCODE_SERVER_ADDRESS=f"127.0.0.1:{m['base']+30+i}")
            spawn(m,f'chaincode-{i}',[ROOT/'bin/dis-chaincode'],env);ready(m,f'chaincode-{i}',m['base']+30+i)

def up(m):
    basic(m)
    if m['phase']=='configured':
        deadline=time.monotonic()+40
        while True:
            try:run([BIN/'peer','channel','create','-c','dis','-f',WORK/'dis.tx','--outputBlock',WORK/'dis.block']+orderflags(m),clientenv(0));break
            except RuntimeError as e:
                if 'no Raft leader' not in str(e) or time.monotonic()>deadline:raise
                time.sleep(.5)
        for i in range(4):run([BIN/'peer','channel','join','-b',WORK/'dis.block'],clientenv(i))
        m['phase']='channel';save(m)
    if not m.get('anchors_updated'):
        for i in range(4):
            path=WORK/f'anchor-{i}.tx'
            run([BIN/'configtxgen','-profile','Application','-channelID','dis','-asOrg',f'Org{i}MSP','-outputAnchorPeersUpdate',path],dict(ENV,FABRIC_CFG_PATH=str(WORK)))
            run([BIN/'peer','channel','update','-c','dis','-f',path]+orderflags(m),clientenv(i))
        m['anchors_updated']=True;save(m)
    policy="OutOf(3, 'Org0MSP.peer', 'Org1MSP.peer', 'Org2MSP.peer', 'Org3MSP.peer')"
    definition=['--channelID','dis','--name','dis','--version','1','--sequence','1','--signature-policy',policy,'--init-required']
    if m['phase']=='channel':
        for i in range(4):
            path=package(i,m['base'])
            run([BIN/'peer','lifecycle','chaincode','install',path],clientenv(i))
            pkg=run([BIN/'peer','lifecycle','chaincode','calculatepackageid',path],clientenv(i)).strip();m['packages'][str(i)]=pkg;save(m)
        m['phase']='installed';save(m)
    chaincodes(m)
    if m['phase']=='installed':
        for i in range(4):run([BIN/'peer','lifecycle','chaincode','approveformyorg']+definition+['--package-id',m['packages'][str(i)]]+orderflags(m),clientenv(i))
        run([BIN/'peer','lifecycle','chaincode','commit']+definition+orderflags(m)+peerflags(m),clientenv(0));m['phase']='committed';save(m)
    if m['phase']=='committed':
        run([BIN/'peer','chaincode','invoke','-C','dis','-n','dis','--isInit','-c',json.dumps({'Args':['Init',m['domain']]}),'--waitForEvent']+orderflags(m)+peerflags(m),clientenv(0));m['phase']='initialized';save(m)
    admin=orgpath(0)/'users/Admin@org0.dis/msp'
    profile=dict(endpoint=f"127.0.0.1:{m['base']+10}",server_name='peer0.org0.dis',tls_cert=str(peerpath(0)/'tls/ca.crt'),msp_id='Org0MSP',certificate=str(next((admin/'signcerts').glob('*'))),private_key=str(next((admin/'keystore').glob('*'))),channel='dis',chaincode='dis')
    write(WORK/'gateway.json',profile)
    run([ROOT/'bin/dis-check','-fabric-config',WORK/'gateway.json','-wait','45s'],timeout=60)
    print(json.dumps(dict(phase=m['phase'],gateway=str(WORK/'gateway.json'),endorsement_policy=policy),indent=2))

def main():
    parser=argparse.ArgumentParser();parser.add_argument('action',choices=['up','down','status']);parser.add_argument('--port-base',type=int,default=27000);args=parser.parse_args()
    WORK.mkdir(parents=True,exist_ok=True)
    with (WORK/'manager.lock').open('a') as lock:
        fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
        path=WORK/'network.json'
        m=json.loads(path.read_text()) if path.exists() else None
        if args.action=='status':
            print(json.dumps(dict(phase=m['phase'],processes={name:live(p) for name,p in m['processes'].items()}) if m else {'phase':'not-configured'},indent=2));return
        if args.action=='down':
            if m:stop(m)
            return
        if m is None:m=configure(args.port_base)
        try:up(m)
        except BaseException:stop(m);raise
if __name__=='__main__':main()
