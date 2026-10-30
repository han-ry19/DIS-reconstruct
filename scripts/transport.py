"""SSH transport for dedicated Fabric experiment runtime directories."""
import json
from pathlib import Path
import shlex
import shutil
import subprocess


def ssh_args(host):
    args = ['ssh', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes',
            '-o', 'ConnectTimeout=10', '-p', str(host.get('ssh_port', 22))]
    if host.get('identity_file'):
        args += ['-i', str(Path(host['identity_file']).expanduser()), '-o', 'IdentitiesOnly=yes']
    return args


def agent(host, request, timeout=30):
    command = ssh_args(host) + [host['ssh'], shlex.join(['python3', str(Path(host['root']) / 'node-agent.py')])]
    result = subprocess.run(command, input=json.dumps(request), capture_output=True, text=True, timeout=timeout)
    if result.returncode:
        raise RuntimeError(f"Remote agent {host['ssh']}: {result.stderr[-2000:]}")
    return json.loads(result.stdout)


def translate(value, host, root, work, tools):
    if isinstance(value, dict):
        return {k: translate(v, host, root, work, tools) for k, v in value.items()}
    if isinstance(value, list):
        return [translate(v, host, root, work, tools) for v in value]
    if isinstance(value, (str, Path)):
        value = str(value)
        for source, target in [(work, Path(host['root']) / 'fabric'), (tools, Path(host['root']) / 'tools/fabric'), (root / 'bin', Path(host['root']) / 'bin')]:
            if value == str(source) or value.startswith(str(source) + '/'):
                return str(target) + value[len(str(source)):]
    return value


def deploy(host, name, root, work, tools, peers, orderers):
    stage = work / 'deployment' / name
    if stage.exists():
        shutil.rmtree(stage)
    stage.mkdir(parents=True)

    def copy(source, relative):
        target = stage / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        if source.is_dir():
            shutil.copytree(source, target, dirs_exist_ok=True)
        else:
            shutil.copy2(source, target)

    copy(root / 'scripts/node-agent.py', Path('node-agent.py'))
    copy(tools / 'bin/peer', Path('tools/fabric/bin/peer'))
    copy(tools / 'bin/orderer', Path('tools/fabric/bin/orderer'))
    copy(root / 'bin/dis-chaincode', Path('bin/dis-chaincode'))
    copy(work / 'builder', Path('fabric/builder'))
    copy(work / 'genesis.block', Path('fabric/genesis.block'))
    for node in peers + orderers:
        if node['host'] != name:
            continue
        if 'org' in node:
            identity = Path(f"crypto/peerOrganizations/org{node['org']}.dis/peers/peer{node['peer']}.org{node['org']}.dis")
            config = 'core.yaml'
        else:
            identity = Path(f"crypto/ordererOrganizations/orderer.dis/orderers/o{node['index']}.orderer.dis")
            config = 'orderer.yaml'
        copy(work / identity, Path('fabric') / identity)
        target = stage / 'fabric' / node['folder'] / config
        target.parent.mkdir(parents=True, exist_ok=True)
        data = json.loads((work / node['folder'] / config).read_text())
        target.write_text(json.dumps(translate(data, host, root, work, tools), indent=2))
    subprocess.run(ssh_args(host) + [host['ssh'], shlex.join(['mkdir', '-p', host['root']])], check=True, timeout=30)
    subprocess.run(['rsync', '-a', '-e', shlex.join(ssh_args(host)), str(stage) + '/',
                    host['ssh'] + ':' + shlex.quote(host['root'].rstrip('/') + '/')], check=True, timeout=180)
