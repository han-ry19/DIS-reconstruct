"""Validate experiment topologies and assign stable endpoints."""
from pathlib import Path
import re
import yaml


def positive(value, name, maximum):
    if type(value) is not int or not 1 <= value <= maximum:
        raise ValueError(f'{name} must be an integer from 1 to {maximum}')
    return value


def load_topology(path=None, port_base=27000):
    raw = yaml.safe_load(Path(path).read_text()) if path else {}
    if not isinstance(raw, dict):
        raise ValueError('Topology must be a mapping')
    allowed = {'organizations', 'peers_per_organization', 'orderers', 'endorsement_threshold', 'port_base', 'hosts', 'placement', 'domain'}
    if set(raw) - allowed:
        raise ValueError('Unknown topology fields: ' + ', '.join(sorted(set(raw) - allowed)))
    n = positive(raw.get('organizations', 4), 'organizations', 64)
    p = positive(raw.get('peers_per_organization', 1), 'peers_per_organization', 16)
    o = positive(raw.get('orderers', 4), 'orderers', 64)
    threshold = positive(raw.get('endorsement_threshold', n // 2 + 1), 'endorsement_threshold', n)
    base = positive(raw.get('port_base', port_base), 'port_base', 65535)
    domain = raw.get('domain', 'dis-local')
    if not isinstance(domain, str) or not domain.strip():
        raise ValueError('domain must be a nonempty string')
    hosts = raw.get('hosts', {'local': {'address': '127.0.0.1'}})
    if not isinstance(hosts, dict) or not hosts:
        raise ValueError('hosts must be a nonempty mapping')
    normalized = {}
    for name, host in hosts.items():
        if not isinstance(name, str) or not re.fullmatch(r'[A-Za-z0-9_-]+', name) or not isinstance(host, dict):
            raise ValueError('Host names must be simple identifiers with mapping values')
        if set(host) - {'address', 'ssh', 'ssh_port', 'identity_file', 'root'}:
            raise ValueError(f'Unknown host fields for {name}')
        address = host.get('address')
        if not isinstance(address, str) or not re.fullmatch(r'[A-Za-z0-9_.-]+', address):
            raise ValueError(f'{name}: provide an IPv4 address or DNS hostname')
        if address == '0.0.0.0' or (host.get('ssh') and (address == 'localhost' or address.startswith('127.'))):
            raise ValueError(f'{name}: advertise an address reachable from the controller and other nodes')
        spec = dict(host)
        if host.get('ssh'):
            if not isinstance(host['ssh'], str) or not re.fullmatch(r'[A-Za-z0-9_.@-]+', host['ssh']) or host['ssh'].startswith('-'):
                raise ValueError(f'{name}: invalid SSH target')
            remote_root = host.get('root', '')
            if not isinstance(remote_root, str) or not remote_root.startswith('/') or '..' in Path(remote_root).parts or remote_root == '/':
                raise ValueError(f'{name}: remote root must be a dedicated absolute runtime directory')
            spec['ssh_port'] = positive(host.get('ssh_port', 22), 'ssh_port', 65535)
        elif set(host) & {'root', 'ssh_port', 'identity_file'}:
            raise ValueError(f'{name}: SSH options require an ssh target')
        normalized[name] = spec
    local = [name for name, spec in normalized.items() if not spec.get('ssh')]
    if len(local) > 1:
        raise ValueError('Use one host entry per physical machine; at most one may be local')
    addresses = [spec['address'] for spec in normalized.values()]
    if len(set(addresses)) != len(addresses):
        raise ValueError('Each physical host must have a unique address')
    if len(hosts) > 1:
        for address in addresses:
            if address.lower() == 'localhost' or address.startswith('127.') or address == '0.0.0.0':
                raise ValueError('Multi-host addresses must be reachable from every machine')
    names = list(hosts)
    placement = raw.get('placement', {})
    if not isinstance(placement, dict) or set(placement) - {'peers', 'orderers'}:
        raise ValueError('placement accepts peers and orderers lists')
    peer_hosts = placement.get('peers', [names[i % len(names)] for i in range(n * p)])
    order_hosts = placement.get('orderers', [names[i % len(names)] for i in range(o)])
    for kind, values, count in [('peers', peer_hosts, n * p), ('orderers', order_hosts, o)]:
        if not isinstance(values, list) or len(values) != count or any(v not in hosts for v in values):
            raise ValueError(f'placement.{kind} must contain {count} known host names')
    if base + 1000 + (n * p - 1) * 10 + 3 > 65535 or base + (o - 1) * 10 + 2 > 65535:
        raise ValueError('Topology port range exceeds 65535')
    return dict(organizations=n, peers_per_organization=p, orderers=o,
                endorsement_threshold=threshold, port_base=base, hosts=normalized,
                placement=dict(peers=peer_hosts, orderers=order_hosts), domain=domain)


def nodes(topology):
    peers, orderers = [], []
    base = topology['port_base']
    for i, host in enumerate(topology['placement']['orderers']):
        orderers.append(dict(name=f'orderer-{i}', index=i, host=host, port=base + i * 10,
                             operations=base + i * 10 + 1, admin=base + i * 10 + 2, folder=f'o{i}'))
    for k, host in enumerate(topology['placement']['peers']):
        i, j = divmod(k, topology['peers_per_organization'])
        key = str(i) if j == 0 else f'{i}-{j}'
        peers.append(dict(name=f'peer-{key}', key=key, org=i, peer=j, host=host,
                          port=base + 1000 + k * 10, chaincode_listen=base + 1000 + k * 10 + 1,
                          chaincode=base + 1000 + k * 10 + 2, operations=base + 1000 + k * 10 + 3,
                          folder=f'p{key}'))
    return peers, orderers


def legacy_topology(manifest):
    topology = load_topology(port_base=manifest['base'])
    topology['domain'] = manifest['domain']
    peers, orderers = nodes(topology)
    base = manifest['base']
    for i, node in enumerate(orderers):
        node.update(port=base+i, operations=base+40+i, admin=base+50+i)
    for i, node in enumerate(peers):
        node.update(port=base+10+i, chaincode_listen=base+20+i, chaincode=base+30+i, operations=base+60+i)
    return topology, peers, orderers


def endorsement(topology):
    members = ', '.join(f"'Org{i}MSP.peer'" for i in range(topology['organizations']))
    return f"OutOf({topology['endorsement_threshold']}, {members})"
