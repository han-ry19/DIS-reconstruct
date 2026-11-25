"""Load and validate the unified deployment configuration without side effects."""
from pathlib import Path
import yaml
import ipaddress
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
    service_fields = {'api_port', 'dns_port', 'redis_port'}
    if not isinstance(services, dict) or set(services) not in (service_fields, service_fields | {'ns_address'}):
        raise ValueError('services requires api_port, dns_port, redis_port and optional ns_address')
    for name in service_fields:
        port = services[name]
        if type(port) is not int or not 1 <= port <= 65535:
            raise ValueError(name + ' must be an integer from 1 to 65535')
    if len({services[name] for name in service_fields}) != 3:
        raise ValueError('Service ports must be distinct')
    address = services.get('ns_address', '127.0.0.1')
    if not isinstance(address, str) or ipaddress.ip_address(address).version != 4:
        raise ValueError('ns_address must be an IPv4 address')
    peers, orderers = nodes(topology)
    controller_ports = set()
    for node in peers + orderers:
        if topology['hosts'][node['host']].get('ssh'):
            continue
        fields = ('port', 'chaincode_listen', 'chaincode', 'operations') if 'org' in node else ('port', 'operations', 'admin')
        controller_ports.update(node[key] for key in fields)
    if controller_ports.intersection(services[name] for name in service_fields):
        raise ValueError('Service ports overlap controller Fabric ports')
    return {'version': 1, 'runtime': str(runtime), 'network': topology, 'services': services}
