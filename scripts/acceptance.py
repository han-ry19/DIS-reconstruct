#!/usr/bin/env python3
"""Verify the workflow, all peers, DNS publication, and deployment recovery."""
import argparse
import json
import subprocess
import time
import urllib.request
from network import ROOT, WORK, RUNTIME, BIN, clientenv, endpoint, load, orderflags, peerpath, orgpath, run


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--restart', action='store_true', help='Restart the project services and network')
    parser.add_argument('--all-gateways', action='store_true', help='Verify discovery through every peer and endorsement by peer replicas')
    args = parser.parse_args()
    services = json.loads((RUNTIME / 'services/services.json').read_text())
    api = f'http://127.0.0.1:{services["api_port"]}'
    dns = f'127.0.0.1:{services["dns_port"]}'
    network = load()

    def command(values):
        return subprocess.run([str(v) for v in values], check=True, capture_output=True, text=True).stdout

    transcript = RUNTIME / 'replay-requests.json'
    demo = command([ROOT / 'bin/dis-demo', '-url', api, '-transcript', transcript])
    record = json.loads(demo.splitlines()[-1])
    print(demo)

    def verify():
        query = api + '/v1/authorizations?data_id=' + record['data_id'] + '&owner_id=' + record['owner_id'] + '&recipient=' + record['recipient']
        with urllib.request.urlopen(query, timeout=15) as response:
            assert json.load(response) == record
        for node in network['peers']:
            deadline = time.monotonic() + 30
            while True:
                try:
                    body = run([BIN / 'peer', 'chaincode', 'query', '-C', 'dis', '-n', 'dis', '-c', json.dumps({'Args': ['GetAuthorization', record['data_id'], record['owner_id'], record['recipient']]})], clientenv(node['org'], node['peer']))
                    assert json.loads(body) == record
                    break
                except (RuntimeError, AssertionError):
                    if time.monotonic() > deadline:
                        raise
                    time.sleep(.2)
        deadline = time.monotonic() + 30
        while True:
            try:
                command([ROOT / 'bin/dis-sync', '-fabric-config', WORK / 'gateway.json', '-redis', f'127.0.0.1:{services["redis_port"]}', '-check', '-verify', dns])
                break
            except subprocess.CalledProcessError:
                if time.monotonic() > deadline:
                    raise
                time.sleep(.2)

    verify()
    if args.all_gateways:
        profiles = WORK / 'acceptance-gateways'
        profiles.mkdir(exist_ok=True)
        for node in network['peers']:
            i, j = node['org'], node['peer']
            admin = orgpath(i) / f'users/Admin@org{i}.dis/msp'
            profile = dict(endpoint=endpoint(network, node), server_name=f'peer{j}.org{i}.dis', tls_cert=str(peerpath(i, j) / 'tls/ca.crt'),
                           msp_id=f'Org{i}MSP', certificate=str(next((admin / 'signcerts').glob('*'))), private_key=str(next((admin / 'keystore').glob('*'))), channel='dis', chaincode='dis')
            path = profiles / (node['name'] + '.json')
            path.write_text(json.dumps(profile))
            command([ROOT / 'bin/dis-check', '-fabric-config', path])
        # Select each replica index explicitly, so all configured peers must execute chaincode.
        for j in range(network['topology']['peers_per_organization']):
            flags = []
            for node in network['peers']:
                if node['peer'] == j:
                    flags += ['--peerAddresses', endpoint(network, node), '--tlsRootCertFiles', peerpath(node['org'], j) / 'tls/ca.crt']
            run([BIN / 'peer', 'chaincode', 'invoke', '-C', 'dis', '-n', 'dis', '-c', json.dumps({'Args': ['GetSnapshot']}), '--waitForEvent'] + orderflags(network) + flags, clientenv(0))
    if args.restart:
        for script, action in [('services.py', 'down'), ('network.py', 'down'), ('network.py', 'up'), ('services.py', 'up')]:
            command(['python3', ROOT / 'scripts' / script, action])
        verify()
    replay = json.loads(transcript.read_text())
    before = json.loads(command([ROOT / 'bin/dis-check', '-fabric-config', WORK / 'gateway.json']))['sequence']
    for step in replay:
        request = urllib.request.Request(api + '/v1/transactions', data=json.dumps(step['request']).encode(), headers={'Content-Type': 'application/json'})
        with urllib.request.urlopen(request, timeout=30) as response:
            assert json.load(response) == step['result']
    after = json.loads(command([ROOT / 'bin/dis-check', '-fabric-config', WORK / 'gateway.json']))['sequence']
    assert before == after
    report = dict(committed_event_publication='passed', workflow='passed', exact_replay='passed', all_peer_queries='passed',
                  all_gateways='passed' if args.all_gateways else 'not-requested', replica_endorsement='passed' if args.all_gateways else 'not-requested',
                  organizations=network['topology']['organizations'], peer_count=len(network['peers']), orderer_count=len(network['orderers']),
                  host_count=len(set(n['host'] for n in network['peers'] + network['orderers'])),
                  endorsement_threshold=network['topology']['endorsement_threshold'], complete_dns_axfr='passed',
                  restart_recovery='passed' if args.restart else 'not-requested', authorization=record)
    (RUNTIME / 'acceptance.json').write_text(json.dumps(report, indent=2))
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
