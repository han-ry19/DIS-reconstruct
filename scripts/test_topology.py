"""Topology invariants required for reproducible scale experiments."""
import importlib.util
from pathlib import Path
import subprocess
import tempfile
import unittest
import yaml
from topology import endorsement, legacy_topology, load_topology, nodes
from transport import translate


class TopologyTests(unittest.TestCase):
    def topology(self, value):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'topology.yaml'
            path.write_text(yaml.safe_dump(value))
            return load_topology(path)

    def test_scale_has_unique_ports_and_org_based_threshold(self):
        for orgs, replicas, orderers in [(1, 1, 1), (3, 2, 3), (5, 1, 3), (7, 3, 5)]:
            with self.subTest(orgs=orgs, replicas=replicas, orderers=orderers):
                topology = self.topology(dict(organizations=orgs, peers_per_organization=replicas, orderers=orderers))
                peers, ordering = nodes(topology)
                ports = [n[k] for n in peers for k in ['port', 'chaincode_listen', 'chaincode', 'operations']]
                ports += [n[k] for n in ordering for k in ['port', 'operations', 'admin']]
                self.assertEqual(len(ports), len(set(ports)))
                self.assertEqual(len(peers), orgs * replicas)
                self.assertEqual(len(ordering), orderers)
                self.assertEqual(endorsement(topology).count('.peer'), orgs)
                self.assertTrue(endorsement(topology).startswith(f'OutOf({orgs // 2 + 1},'))

    def test_invalid_configurations_are_rejected(self):
        for invalid in [dict(organizations=0), dict(orderers=True), dict(endorsement_threshold=5),
                        dict(peers_per_organization=-1), dict(port_base=65530), dict(organization=3),
                        dict(gateway_concurrency=0), dict(gateway_concurrency=True), dict(gateway_concurrency='2500'),
                        dict(placement={'peers': ['missing']}), dict(hosts={'local': {'address': '0.0.0.0'}}),
                        dict(hosts={'a': {'address': '127.0.0.1'}, 'b': {'address': 'worker', 'ssh': 'user@worker', 'root': '/tmp/run'}}),
                        dict(hosts={'remote': {'address': 'worker', 'ssh': 'user@worker', 'root': '/'}})]:
            with self.subTest(invalid=invalid), self.assertRaises(ValueError):
                self.topology(invalid)

    def test_replica_placement_is_in_organization_order(self):
        topology = self.topology(dict(organizations=3, peers_per_organization=2,
            hosts={'controller': {'address': 'controller'}, 'worker': {'address': 'worker', 'ssh': 'user@worker', 'root': '/tmp/experiment'}}))
        peers, _ = nodes(topology)
        self.assertEqual([(n['org'], n['peer'], n['host']) for n in peers],
                         [(i, j, ['controller', 'worker'][j]) for i in range(3) for j in range(2)])

    def test_legacy_ports_and_data_paths_are_preserved(self):
        topology, peers, orderers = legacy_topology({'base': 27000, 'domain': 'existing'})
        self.assertEqual([p['port'] for p in peers], [27010, 27011, 27012, 27013])
        self.assertEqual([p['folder'] for p in peers], ['p0', 'p1', 'p2', 'p3'])
        self.assertEqual([o['port'] for o in orderers], [27000, 27001, 27002, 27003])
        self.assertEqual(topology['domain'], 'existing')

    def test_remote_paths_preserve_unrelated_values(self):
        host = {'root': '/worker/run'}
        result = translate({'msp': '/controller/run/fabric/crypto/msp', 'binary': '/repo/bin/dis-chaincode',
                            'label': '/controller/run/fabrication', 'port': 22}, host,
                           Path('/repo'), Path('/controller/run/fabric'), Path('/repo/runtime/tools/fabric'))
        self.assertEqual(result, {'msp': '/worker/run/fabric/crypto/msp', 'binary': '/worker/run/bin/dis-chaincode',
                                  'label': '/controller/run/fabrication', 'port': 22})

    def test_stale_pid_record_cannot_stop_reused_process(self):
        spec = importlib.util.spec_from_file_location('node_agent', Path(__file__).with_name('node-agent.py'))
        agent = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(agent)
        child = subprocess.Popen(['sleep', '30'])
        try:
            agent.main({'action': 'stop', 'processes': [{'pid': child.pid, 'start': 'invalid-start-time'}]})
            self.assertIsNone(child.poll())
        finally:
            child.terminate()
            child.wait()


if __name__ == '__main__':
    unittest.main()
