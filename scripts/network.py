#!/usr/bin/env python3
"""Create and manage single-host or SSH-distributed Fabric experiments."""
import argparse
import copy
import fcntl
import io
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import sys
import tarfile
import time
import yaml
from topology import endorsement, legacy_topology, load_topology, nodes
from transport import agent, deploy, translate

ROOT = Path(__file__).resolve().parents[1]
RUNTIME = Path(os.environ.get('DIS_RUNTIME', str(ROOT / 'runtime'))).expanduser().resolve()
TOOLS = ROOT / 'runtime/tools/fabric'
WORK = RUNTIME / 'fabric'
BIN = TOOLS / 'bin'
ENV = dict(os.environ, FABRIC_LOGGING_SPEC='warn')


def write(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2))


def save(manifest):
    write(WORK / 'network.json', manifest)


def load():
    path = WORK / 'network.json'
    if not path.exists():
        return None
    m = json.loads(path.read_text())
    if 'topology' not in m:
        m['topology'], m['peers'], m['orderers'] = legacy_topology(m)
    return m


def run(args, env=None, timeout=120):
    proc = subprocess.run([str(a) for a in args], env=env or ENV, capture_output=True, text=True, timeout=timeout)
    with (WORK / 'commands.log').open('a') as log:
        log.write(' '.join(str(a) for a in args) + '\n' + proc.stdout + proc.stderr + '\n')
    if proc.returncode:
        raise RuntimeError(proc.stderr[-3000:] or proc.stdout[-3000:])
    return proc.stdout


def stamp(pid):
    try:
        fields = Path('/proc/' + str(pid) + '/stat').read_text().rsplit(')', 1)[1].split()
        return None if fields[0] == 'Z' else fields[19]
    except (FileNotFoundError, ProcessLookupError):
        return None


def live(process):
    if process.get('remote'):
        return agent(process['remote'], dict(action='status', processes={'node': process}))['node']
    return process.get('start') is not None and stamp(process['pid']) == process['start']


def process_status(m):
    result = {}
    remote = {}
    for name, process in m['processes'].items():
        if process.get('remote'):
            remote.setdefault(process['remote']['ssh'] + ':' + process['remote']['root'], []).append((name, process))
        else:
            result[name] = live(process)
    for group in remote.values():
        host = group[0][1]['remote']
        result.update(agent(host, dict(action='status', processes=dict(group))))
    return result


def spawn(m, name, args, env, host_name=None):
    host = m['topology']['hosts'].get(host_name, {})
    if host.get('ssh'):
        request = dict(action='start', command=translate(args, host, ROOT, WORK, TOOLS),
                       env=translate(env, host, ROOT, WORK, TOOLS),
                       log=str(Path(host['root']) / 'fabric' / (name + '.log')),
                       record=str(Path(host['root']) / 'fabric/processes' / (name + '.json')))
        # Send only the node-specific environment, never controller credentials.
        request['env'] = {k: v for k, v in request['env'].items() if k in {'FABRIC_LOGGING_SPEC', 'FABRIC_CFG_PATH', 'CHAINCODE_ID', 'CHAINCODE_SERVER_ADDRESS', 'DIS_CHAINCODE_ADDRESS'}}
        record = agent(host, request)
        m['processes'][name] = dict(record, remote=host)
        save(m)
        return
    old = m['processes'].get(name)
    if old and live(old):
        return
    with (WORK / (name + '.log')).open('ab') as log:
        process = subprocess.Popen([str(a) for a in args], env=env, stdin=subprocess.DEVNULL,
                                   stdout=log, stderr=log, start_new_session=True)
    m['processes'][name] = dict(pid=process.pid, start=stamp(process.pid))
    save(m)


def endpoint(m, node):
    return f"{m['topology']['hosts'][node['host']]['address']}:{node['port']}"


def ready(m, node, name=None, port=None, timeout=60):
    name = name or node['name']
    port = port or node['port']
    address = m['topology']['hosts'][node['host']]['address']
    # External chaincode servers are reached only by their colocated peer.
    chaincode = name.startswith('chaincode-')
    host = m['topology']['hosts'][node['host']]
    remote_chaincode = chaincode and host.get('ssh')
    if chaincode:
        address = '127.0.0.1'
    until = time.monotonic() + timeout
    while time.monotonic() < until:
        if not live(m['processes'][name]):
            raise RuntimeError(name + ' exited; inspect its host runtime log')
        if remote_chaincode:
            if agent(host, dict(action='connect', port=port))['ready']:
                return
            time.sleep(.2)
            continue
        try:
            with socket.create_connection((address, port), timeout=.5):
                return
        except OSError:
            time.sleep(.2)
    raise RuntimeError(name + ' startup timeout')


def stop(m):
    remote = {}
    local = []
    for process in m['processes'].values():
        if process.get('remote'):
            host = process['remote']
            remote.setdefault(host['ssh'] + ':' + host['root'], (host, []))[1].append(process)
        else:
            local.append(process)
    for host, processes in remote.values():
        agent(host, dict(action='stop', processes=processes))
    for process in reversed(local):
        if live(process):
            os.killpg(process['pid'], signal.SIGTERM)
    until = time.monotonic() + 10
    while time.monotonic() < until and any(live(p) for p in local):
        time.sleep(.1)
    for process in local:
        if live(process):
            os.killpg(process['pid'], signal.SIGKILL)
    m['processes'] = {}
    save(m)


def orgpath(i):
    return WORK / f'crypto/peerOrganizations/org{i}.dis'


def peerpath(i, j=0):
    return orgpath(i) / f'peers/peer{j}.org{i}.dis'


def orderpath(i):
    return WORK / f'crypto/ordererOrganizations/orderer.dis/orderers/o{i}.orderer.dis'


def clientenv(i, j=0):
    m = load()
    node = next(p for p in m['peers'] if p['org'] == i and p['peer'] == j)
    return dict(ENV, FABRIC_CFG_PATH=str(WORK / node['folder']),
                CORE_PEER_MSPCONFIGPATH=str(orgpath(i) / f'users/Admin@org{i}.dis/msp'),
                CORE_PEER_ADDRESS=endpoint(m, node),
                CORE_PEER_TLS_ROOTCERT_FILE=str(peerpath(i, j) / 'tls/ca.crt'))


def orderflags(m):
    return ['-o', endpoint(m, m['orderers'][0]), '--tls', '--cafile', orderpath(0) / 'tls/ca.crt']


def peerflags(m):
    flags = []
    # One peer per organization is sufficient for lifecycle commit and initialization.
    for node in m['peers']:
        if node['peer'] == 0:
            flags += ['--peerAddresses', endpoint(m, node), '--tlsRootCertFiles', peerpath(node['org']) / 'tls/ca.crt']
    return flags


def check_ports(m):
    by_host = {}
    for node in m['orderers']:
        by_host.setdefault(node['host'], []).extend(node[k] for k in ['port', 'operations', 'admin'])
    for node in m['peers']:
        by_host.setdefault(node['host'], []).extend(node[k] for k in ['port', 'chaincode_listen', 'chaincode', 'operations'])
    for name, ports in by_host.items():
        host = m['topology']['hosts'][name]
        if host.get('ssh'):
            # Bootstrap the agent before checking remote ports.
            from transport import ssh_args
            import shlex
            subprocess.run(ssh_args(host) + [host['ssh'], shlex.join(['mkdir', '-p', host['root']])], check=True, timeout=30)
            subprocess.run(ssh_args(host) + [host['ssh'], shlex.join(['tee', str(Path(host['root']) / 'node-agent.py')])],
                           input=(ROOT / 'scripts/node-agent.py').read_bytes(), stdout=subprocess.DEVNULL, check=True, timeout=30)
            agent(host, dict(action='ports', ports=ports))
        else:
            sockets = []
            try:
                for port in ports:
                    sock = socket.socket()
                    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
                    sock.bind(('0.0.0.0', port))
                    sockets.append(sock)
            finally:
                for sock in sockets:
                    sock.close()


def configure(topology):
    if not (BIN / 'peer').exists():
        raise RuntimeError('Install Fabric tools using scripts/bootstrap.py first')
    peers, orderers = nodes(topology)
    m = dict(base=topology['port_base'], domain=topology['domain'], processes={}, phase='configured', packages={},
             ledger_schema=2, definition_sequence=1, publication_digest_version=2,
             topology=topology, peers=peers, orderers=orderers)
    check_ports(m)
    write(WORK / 'crypto.json', dict(
        OrdererOrgs=[dict(Name='Orderer', Domain='orderer.dis', EnableNodeOUs=True,
                         Specs=[dict(Hostname=f"o{p['index']}", SANS=['localhost', '127.0.0.1', topology['hosts'][p['host']]['address']]) for p in orderers])],
        PeerOrgs=[dict(Name=f'Org{i}', Domain=f'org{i}.dis', EnableNodeOUs=True,
                       Specs=[dict(Hostname=f"peer{p['peer']}", SANS=['localhost', '127.0.0.1', topology['hosts'][p['host']]['address']]) for p in peers if p['org'] == i],
                       Users={'Count': 1}) for i in range(topology['organizations'])]))
    run([BIN / 'cryptogen', 'generate', '--config', WORK / 'crypto.json', '--output', WORK / 'crypto'])
    sig = lambda rule: dict(Type='Signature', Rule=rule)
    imp = lambda rule: dict(Type='ImplicitMeta', Rule=rule)
    orgs = []
    for i in range(topology['organizations']):
        mid = f'Org{i}MSP'
        first = next(p for p in peers if p['org'] == i)
        orgs.append(dict(Name=mid, ID=mid, MSPDir=str(orgpath(i) / 'msp'),
                         AnchorPeers=[dict(Host=topology['hosts'][first['host']]['address'], Port=first['port'])],
                         Policies=dict(Readers=sig(f"OR('{mid}.member')"), Writers=sig(f"OR('{mid}.member')"),
                                       Admins=sig(f"OR('{mid}.admin')"), Endorsement=sig(f"OR('{mid}.peer')"))))
    orderorg = dict(Name='OrdererMSP', ID='OrdererMSP', MSPDir=str(WORK / 'crypto/ordererOrganizations/orderer.dis/msp'),
                    Policies=dict(Readers=sig("OR('OrdererMSP.member')"), Writers=sig("OR('OrdererMSP.member')"), Admins=sig("OR('OrdererMSP.admin')")))
    channel = dict(Capabilities={'V2_0': True}, Policies=dict(Readers=imp('ANY Readers'), Writers=imp('ANY Writers'), Admins=imp('MAJORITY Admins')))
    order = dict(OrdererType='etcdraft', Addresses=[endpoint(m, node) for node in orderers], BatchTimeout='500ms',
                 BatchSize=dict(MaxMessageCount=10, AbsoluteMaxBytes='10 MB', PreferredMaxBytes='512 KB'),
                 EtcdRaft=dict(Consenters=[dict(Host=topology['hosts'][p['host']]['address'], Port=p['port'],
                                              ClientTLSCert=str(orderpath(p['index']) / 'tls/server.crt'),
                                              ServerTLSCert=str(orderpath(p['index']) / 'tls/server.crt')) for p in orderers],
                              Options=dict(TickInterval='100ms', ElectionTick=10, HeartbeatTick=1, MaxInflightBlocks=5, SnapshotIntervalSize=20971520)),
                 Organizations=[orderorg], Capabilities={'V2_0': True},
                 Policies=dict(Readers=imp('ANY Readers'), Writers=imp('ANY Writers'), Admins=imp('MAJORITY Admins'), BlockValidation=imp('ANY Writers')))
    app = dict(Organizations=orgs, Capabilities={'V2_0': True},
               Policies=dict(Readers=imp('ANY Readers'), Writers=imp('ANY Writers'), Admins=imp('MAJORITY Admins'),
                             LifecycleEndorsement=imp('MAJORITY Endorsement'), Endorsement=imp('MAJORITY Endorsement')))
    write(WORK / 'configtx.yaml', dict(Organizations=orgs + [orderorg], Profiles=dict(
        System=dict(channel, Orderer=order, Consortiums=dict(DIS=dict(Organizations=orgs))),
        Application=dict(channel, Consortium='DIS', Application=app))))
    configenv = dict(ENV, FABRIC_CFG_PATH=str(WORK))
    run([BIN / 'configtxgen', '-profile', 'System', '-channelID', 'system', '-outputBlock', WORK / 'genesis.block'], configenv)
    run([BIN / 'configtxgen', '-profile', 'Application', '-channelID', 'dis', '-outputCreateChannelTx', WORK / 'dis.tx'], configenv)
    builder = WORK / 'builder/bin'
    builder.mkdir(parents=True, exist_ok=True)
    scripts = {
        'detect': "import json,sys\nfrom pathlib import Path\nsys.exit(0 if json.loads((Path(sys.argv[2])/'metadata.json').read_text())['type']=='external' else 1)\n",
        'build': "import json,os,sys\nfrom pathlib import Path\nc=json.loads((Path(sys.argv[1])/'connection.json').read_text())\nc['address']=os.environ.get('DIS_CHAINCODE_ADDRESS',c['address'])\n(Path(sys.argv[3])/'connection.json').write_text(json.dumps(c))\n",
        'release': "import sys,shutil\nfrom pathlib import Path\np=Path(sys.argv[2])/'chaincode/server';p.mkdir(parents=True)\nshutil.copyfile(Path(sys.argv[1])/'connection.json',p/'connection.json')\n"}
    for name, body in scripts.items():
        path = builder / name
        path.write_text('#!/usr/bin/env python3\n' + body)
        path.chmod(0o755)
    core = yaml.safe_load((TOOLS / 'config/core.yaml').read_text())
    ordering = yaml.safe_load((TOOLS / 'config/orderer.yaml').read_text())
    for node in orderers:
        i = node['index']
        folder = WORK / node['folder']
        identity = orderpath(i)
        cfg = copy.deepcopy(ordering)
        cfg['General'].update(ListenAddress='0.0.0.0', ListenPort=node['port'], LocalMSPID='OrdererMSP', LocalMSPDir=str(identity / 'msp'),
                              BootstrapMethod='file', BootstrapFile=str(WORK / 'genesis.block'))
        cfg['General']['TLS'].update(Enabled=True, PrivateKey=str(identity / 'tls/server.key'), Certificate=str(identity / 'tls/server.crt'), RootCAs=[str(identity / 'tls/ca.crt')])
        cfg['General']['Cluster'].update(ClientCertificate=str(identity / 'tls/server.crt'), ClientPrivateKey=str(identity / 'tls/server.key'), RootCAs=[str(identity / 'tls/ca.crt')])
        cfg['FileLedger']['Location'] = str(folder / 'ledger')
        cfg['Consensus'].update(WALDir=str(folder / 'wal'), SnapDir=str(folder / 'snap'))
        cfg['Operations']['ListenAddress'] = f"127.0.0.1:{node['operations']}"
        cfg['Admin']['ListenAddress'] = f"127.0.0.1:{node['admin']}"
        write(folder / 'orderer.yaml', cfg)
    for node in peers:
        i, j = node['org'], node['peer']
        folder = WORK / node['folder']
        identity = peerpath(i, j)
        cfg = copy.deepcopy(core)
        cfg['peer'].update(id=node['folder'], networkId='dis-reconstruct', address=endpoint(m, node), listenAddress=f"0.0.0.0:{node['port']}",
                           chaincodeListenAddress=f"127.0.0.1:{node['chaincode_listen']}", chaincodeAddress=f"127.0.0.1:{node['chaincode_listen']}",
                           localMspId=f'Org{i}MSP', mspConfigPath=str(identity / 'msp'), fileSystemPath=str(folder / 'ledger'))
        cfg['peer']['tls'].update(enabled=True, cert={'file': str(identity / 'tls/server.crt')}, key={'file': str(identity / 'tls/server.key')},
                                  rootcert={'file': str(identity / 'tls/ca.crt')}, clientRootCAs={'files': [str(identity / 'tls/ca.crt')]})
        siblings = [endpoint(m, p) for p in peers if p['org'] == i and p['key'] != node['key']]
        cfg['peer']['gossip'].update(bootstrap=' '.join(siblings), externalEndpoint=endpoint(m, node), useLeaderElection=False, orgLeader=True)
        cfg['peer']['gateway'] = {'enabled': True}
        cfg['peer']['limits']['concurrency']['gatewayService'] = topology.get('gateway_concurrency', 500)
        cfg['operations']['listenAddress'] = f"127.0.0.1:{node['operations']}"
        cfg['chaincode']['externalBuilders'] = [dict(name='dis-external', path=str(builder.parent), propagateEnvironment=['DIS_CHAINCODE_ADDRESS'])]
        cfg['vm']['endpoint'] = ''
        cfg['ledger']['snapshots']['rootDir'] = str(folder / 'snapshots')
        write(folder / 'core.yaml', cfg)
    save(m)
    return m


def prepare(m):
    for name, host in m['topology']['hosts'].items():
        if host.get('ssh'):
            deploy(host, name, ROOT, WORK, TOOLS, m['peers'], m['orderers'])


def basic(m):
    for node in m['orderers']:
        spawn(m, node['name'], [BIN / 'orderer'], dict(ENV, FABRIC_CFG_PATH=str(WORK / node['folder'])), node['host'])
        ready(m, node)
    for node in m['peers']:
        spawn(m, node['name'], [BIN / 'peer', 'node', 'start'], dict(ENV, FABRIC_CFG_PATH=str(WORK / node['folder']), DIS_CHAINCODE_ADDRESS=f"127.0.0.1:{node['chaincode']}"), node['host'])
        ready(m, node)


def package(node):
    def archive(values):
        stream = io.BytesIO()
        with tarfile.open(fileobj=stream, mode='w:gz') as tar:
            for name, value in values.items():
                value = value if isinstance(value, bytes) else json.dumps(value).encode()
                info = tarfile.TarInfo(name)
                info.size = len(value)
                tar.addfile(info, io.BytesIO(value))
        return stream.getvalue()
    code = archive({'connection.json': dict(address=f"127.0.0.1:{node['chaincode']}", dial_timeout='10s', tls_required=False)})
    path = WORK / f"chaincode-org{node['org']}.tgz"
    path.write_bytes(archive({'metadata.json': dict(type='external', label=f"dis-org{node['org']}"), 'code.tar.gz': code}))
    return path


def chaincodes(m):
    for node in m['peers']:
        if node['key'] in m['packages']:
            name = 'chaincode-' + node['key']
            env = dict(ENV, CHAINCODE_ID=m['packages'][node['key']], CHAINCODE_SERVER_ADDRESS=f"127.0.0.1:{node['chaincode']}")
            spawn(m, name, [ROOT / 'bin/dis-chaincode'], env, node['host'])
            ready(m, node, name=name, port=node['chaincode'])


def up(m):
    prepare(m)
    basic(m)
    if m['phase'] == 'configured':
        deadline = time.monotonic() + 60
        while True:
            try:
                run([BIN / 'peer', 'channel', 'create', '-c', 'dis', '-f', WORK / 'dis.tx', '--outputBlock', WORK / 'dis.block'] + orderflags(m), clientenv(0))
                break
            except RuntimeError as error:
                if 'no Raft leader' not in str(error) or time.monotonic() > deadline:
                    raise
                time.sleep(.5)
        for node in m['peers']:
            run([BIN / 'peer', 'channel', 'join', '-b', WORK / 'dis.block'], clientenv(node['org'], node['peer']))
        m['phase'] = 'channel'
        save(m)
    if not m.get('anchors_updated'):
        for i in range(m['topology']['organizations']):
            path = WORK / f'anchor-{i}.tx'
            run([BIN / 'configtxgen', '-profile', 'Application', '-channelID', 'dis', '-asOrg', f'Org{i}MSP', '-outputAnchorPeersUpdate', path], dict(ENV, FABRIC_CFG_PATH=str(WORK)))
            run([BIN / 'peer', 'channel', 'update', '-c', 'dis', '-f', path] + orderflags(m), clientenv(i))
        m['anchors_updated'] = True
        save(m)
    policy = endorsement(m['topology'])
    definition = ['--channelID', 'dis', '--name', 'dis', '--version', '1', '--sequence', '1', '--signature-policy', policy, '--init-required']
    if m['phase'] == 'channel':
        for node in m['peers']:
            path = WORK / f"chaincode-org{node['org']}.tgz"
            if not path.exists():
                package(node)
            run([BIN / 'peer', 'lifecycle', 'chaincode', 'install', path], clientenv(node['org'], node['peer']))
            m['packages'][node['key']] = run([BIN / 'peer', 'lifecycle', 'chaincode', 'calculatepackageid', path], clientenv(node['org'], node['peer'])).strip()
            save(m)
        m['phase'] = 'installed'
        save(m)
    chaincodes(m)
    if m['phase'] == 'installed':
        for node in m['peers']:
            if node['peer'] == 0:
                run([BIN / 'peer', 'lifecycle', 'chaincode', 'approveformyorg'] + definition + ['--package-id', m['packages'][node['key']]] + orderflags(m), clientenv(node['org']))
        run([BIN / 'peer', 'lifecycle', 'chaincode', 'commit'] + definition + orderflags(m) + peerflags(m), clientenv(0))
        m['phase'] = 'committed'
        save(m)
    if m['phase'] == 'committed':
        run([BIN / 'peer', 'chaincode', 'invoke', '-C', 'dis', '-n', 'dis', '--isInit', '-c', json.dumps({'Args': ['Init', m['domain']]}), '--waitForEvent'] + orderflags(m) + peerflags(m), clientenv(0))
        m['phase'] = 'initialized'
        save(m)
    admin = orgpath(0) / 'users/Admin@org0.dis/msp'
    profile = dict(endpoint=endpoint(m, m['peers'][0]), server_name='peer0.org0.dis', tls_cert=str(peerpath(0) / 'tls/ca.crt'), msp_id='Org0MSP',
                   certificate=str(next((admin / 'signcerts').glob('*'))), private_key=str(next((admin / 'keystore').glob('*'))), channel='dis', chaincode='dis')
    write(WORK / 'gateway.json', profile)
    run([ROOT / 'bin/dis-check', '-fabric-config', WORK / 'gateway.json', '-wait', '60s'], timeout=75)
    print(json.dumps(dict(phase=m['phase'], gateway=str(WORK / 'gateway.json'), endorsement_policy=policy,
                         organizations=m['topology']['organizations'], peers=len(m['peers']), orderers=len(m['orderers']), hosts=list(m['topology']['hosts'])), indent=2))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=['up', 'down', 'status', 'plan'])
    parser.add_argument('--port-base', type=int, default=27000)
    parser.add_argument('--topology', help='YAML topology for a new experiment')
    args = parser.parse_args()
    if args.action == 'plan':
        topology = load_topology(args.topology, args.port_base)
        peers, orderers = nodes(topology)
        print(json.dumps(dict(runtime=str(RUNTIME), topology=topology, peers=peers, orderers=orderers, endorsement_policy=endorsement(topology)), indent=2))
        return
    WORK.mkdir(parents=True, exist_ok=True)
    with (WORK / 'manager.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        m = load()
        if args.action == 'status':
            print(json.dumps(dict(phase=m['phase'], topology=m['topology'], processes=process_status(m)) if m else {'phase': 'not-configured'}, indent=2))
            return
        if args.action == 'down':
            if m:
                stop(m)
            return
        if m is not None and args.topology and m['topology'] != load_topology(args.topology, args.port_base):
            raise ValueError('Existing runtime has a different topology; select a new DIS_RUNTIME directory')
        if m is None:
            m = configure(load_topology(args.topology, args.port_base))
        try:
            up(m)
        except BaseException:
            stop(m)
            raise


if __name__ == '__main__':
    main()
