#!/usr/bin/env python3
"""Manage a deployment using one configuration file."""
import argparse
from datetime import datetime
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import yaml
from configuration import ROOT, load_config
from topology import endorsement, nodes


def log(message):
    print(f"[{datetime.now().astimezone().isoformat(timespec='seconds')}] {message}", flush=True)


def startup_summary(config):
    network = config['network']
    peers, orderers = nodes(network)
    log('DIS startup configuration (requested settings)')
    log('Runtime: ' + config['runtime'])
    log(f"Network: {network['organizations']} organizations, {len(peers)} peers, {len(orderers)} Raft orderers")
    log(f"Domain: {network['domain']}; channel: dis; chaincode: dis")
    log('Endorsement policy: ' + endorsement(network))
    log(f"Gateway concurrency per peer: {network.get('gateway_concurrency', 500)}")
    for node in orderers + peers:
        address = network['hosts'][node['host']]['address']
        log(f"{node['name']}: {address}:{node['port']} (host: {node['host']})")
    log('Advertised nameserver IPv4: ' + config['services'].get('ns_address', '127.0.0.1'))
    for name, port in config['services'].items():
        if name == 'ns_address': continue
        transport = 'UDP/TCP' if name == 'dns_port' else 'TCP'
        log(f"{name.removesuffix('_port').upper()} listener: 127.0.0.1:{port} ({transport})")
    log('Fabric logs: ' + str(Path(config['runtime']) / 'fabric'))
    log('Service logs: ' + str(Path(config['runtime']) / 'services'))


def execute(config, action):
    env = dict(os.environ, DIS_RUNTIME=config['runtime'])
    def run(script, *args):
        subprocess.run([sys.executable, str(ROOT / 'scripts' / script), *args],
                       cwd=ROOT, env=env, check=True)
    if action == 'plan':
        peers, orderers = nodes(config['network'])
        print(json.dumps(dict(config, endorsement_policy=endorsement(config['network']),
                              peers=peers, orderers=orderers), indent=2))
    elif action == 'up':
        startup_summary(config)
        log('[1/2] Starting Fabric network and checking Gateway readiness...')
        # The topology is temporary; network.py persists its effective configuration.
        with tempfile.TemporaryDirectory(prefix='dis-topology-') as directory:
            topology = Path(directory) / 'topology.yaml'
            topology.write_text(yaml.safe_dump(config['network']))
            run('network.py', 'up', '--topology', str(topology))
        flags = []
        for name, value in config['services'].items():
            flags.extend(['--' + name.replace('_', '-'), str(value)])
        log('[2/2] Starting Redis, DNS publisher, CoreDNS, and API; checking readiness...')
        run('services.py', 'up', *flags)
        log('DIS is ready')
        log(f"API health: http://127.0.0.1:{config['services']['api_port']}/healthz")
        log('Processes run in the background; detailed logs are in the runtime directory.')
    elif action == 'down':
        log('Stopping application services...')
        run('services.py', 'down')
        log('Stopping Fabric network...')
        run('network.py', 'down')
        log('DIS stopped; runtime data retained at ' + config['runtime'])
    else:
        run('network.py', 'status')
        run('services.py', 'status')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['plan', 'up', 'down', 'status'])
    parser.add_argument('--config', default=str(ROOT / 'config.yaml'), help='Unified YAML configuration')
    args = parser.parse_args()
    try:
        config = load_config(args.config)
        if args.action == 'up':
            log('Configuration: ' + str(Path(args.config).expanduser().resolve()))
        execute(config, args.action)
    except (ValueError, OSError, yaml.YAMLError, subprocess.CalledProcessError) as error:
        parser.exit(1, f'DIS {args.action} failed: {error}\n')


if __name__ == '__main__':
    main()
