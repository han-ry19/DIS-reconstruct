#!/usr/bin/env python3
"""Manage a deployment using one configuration file."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import yaml
from configuration import ROOT, load_config
from topology import endorsement, nodes


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
        # The topology is temporary; network.py persists its effective configuration.
        with tempfile.TemporaryDirectory(prefix='dis-topology-') as directory:
            topology = Path(directory) / 'topology.yaml'
            topology.write_text(yaml.safe_dump(config['network']))
            run('network.py', 'up', '--topology', str(topology))
        flags = []
        for name, value in config['services'].items():
            flags.extend(['--' + name.replace('_', '-'), str(value)])
        run('services.py', 'up', *flags)
    elif action == 'down':
        run('services.py', 'down')
        run('network.py', 'down')
    else:
        run('network.py', 'status')
        run('services.py', 'status')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['plan', 'up', 'down', 'status'])
    parser.add_argument('--config', default=str(ROOT / 'config.yaml'), help='Unified YAML configuration')
    args = parser.parse_args()
    try:
        execute(load_config(args.config), args.action)
    except (ValueError, OSError, yaml.YAMLError, subprocess.CalledProcessError) as error:
        parser.exit(1, str(error) + '\n')


if __name__ == '__main__':
    main()
