"""Load and validate the unified deployment configuration without side effects."""
from pathlib import Path
import yaml
from topology import load_topology, nodes

ROOT = Path(__file__).resolve().parents[1]


def load_config(path):
    path = Path(path).expanduser().resolve()
    raw = yaml.safe_load(path.read_text())
    if not isinstance(raw, dict):
        raise ValueError('Configuration must be a mapping')
    required = {'version', 'runtime', 'network', 'services'}
    if set(raw) != required:
        raise ValueError('Configuration requires exactly: version, runtime, network, services')
    if type(raw['version']) is not int or raw['version'] != 1:
        raise ValueError('Unsupported configuration version; expected 1')
    runtime = raw['runtime']
    if not isinstance(runtime, str) or not runtime.strip():
        raise ValueError('runtime must be a nonempty path')
    runtime = Path(runtime).expanduser()
    if not runtime.is_absolute():
        runtime = path.parent / runtime
    runtime = runtime.resolve()
    if runtime == Path('/') or runtime == ROOT or runtime in ROOT.parents:
        raise ValueError('runtime must be a dedicated data directory')
    if not isinstance(raw['network'], dict) or not raw['network']:
        raise ValueError('network must be a nonempty topology mapping')
    topology = load_topology(raw['network'])
    services = raw['services']
    fields = {'api_port', 'dns_port', 'redis_port'}
    if not isinstance(services, dict) or set(services) != fields:
        raise ValueError('services requires exactly: api_port, dns_port, redis_port')
    for name, port in services.items():
        if type(port) is not int or not 1 <= port <= 65535:
            raise ValueError(name + ' must be an integer from 1 to 65535')
    if len(set(services.values())) != 3:
        raise ValueError('Service ports must be distinct')
    peers, orderers = nodes(topology)
    controller_ports = set()
    for node in peers + orderers:
        if topology['hosts'][node['host']].get('ssh'):
            continue
        fields = ('port', 'chaincode_listen', 'chaincode', 'operations') if 'org' in node else ('port', 'operations', 'admin')
        controller_ports.update(node[key] for key in fields)
    if controller_ports.intersection(services.values()):
        raise ValueError('Service ports overlap controller Fabric ports')
    return {'version': 1, 'runtime': str(runtime), 'network': topology, 'services': services}
