"""Configuration isolation and launcher routing checks."""
import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import yaml
from configuration import ROOT, load_config

spec = importlib.util.spec_from_file_location('dis_launcher', ROOT / 'scripts/dis.py')
launcher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(launcher)


class ConfigurationTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.path = Path(self.directory.name) / 'config.yaml'
        self.raw = yaml.safe_load((ROOT / 'config.yaml').read_text())

    def load(self):
        self.path.write_text(yaml.safe_dump(self.raw))
        return load_config(self.path)

    def test_relative_runtime_and_no_side_effects(self):
        cfg = self.load()
        expected = self.path.parent / 'runtime/configured'
        self.assertEqual(cfg['runtime'], str(expected))
        with patch.object(launcher.subprocess, 'run') as run, patch('builtins.print'):
            launcher.execute(cfg, 'plan')
        run.assert_not_called()
        self.assertFalse(expected.exists())

    def test_rejects_invalid_and_colliding_settings(self):
        cases = [
            ('unknown', True), ('version', True), ('runtime', '/'),
            ('network', []), ('services', {'api_port': 1}),
        ]
        original = dict(self.raw)
        for key, value in cases:
            with self.subTest(key=key):
                self.raw = dict(original, **{key: value})
                with self.assertRaises(ValueError): self.load()
        self.raw = dict(original)
        self.raw['services'] = dict(original['services'], api_port=37000)
        with self.assertRaises(ValueError): self.load()
        self.raw['services'] = dict(original['services'], dns_port=38080)
        with self.assertRaises(ValueError): self.load()

    def test_launcher_routes_runtime_ports_and_topology(self):
        cfg = self.load()
        commands = []
        def capture(command, **kwargs):
            commands.append((command, kwargs))
            if '--topology' in command:
                topology = Path(command[command.index('--topology') + 1])
                self.assertEqual(yaml.safe_load(topology.read_text()), cfg['network'])
        with patch.dict(os.environ, DIS_RUNTIME='/unrelated'), patch.object(launcher.subprocess, 'run', side_effect=capture):
            launcher.execute(cfg, 'up')
        self.assertEqual([Path(c[0][1]).name for c in commands], ['network.py', 'services.py'])
        for _, kwargs in commands:
            self.assertEqual(kwargs['env']['DIS_RUNTIME'], cfg['runtime'])
        self.assertIn('--ns-address', commands[1][0])
        self.assertIn('127.0.0.1', commands[1][0])
        self.assertIn('--api-port', commands[1][0])
        self.assertIn('38080', commands[1][0])
        commands.clear()
        with patch.object(launcher.subprocess, 'run', side_effect=capture):
            launcher.execute(cfg, 'down')
        self.assertEqual([Path(c[0][1]).name for c in commands], ['services.py', 'network.py'])


    def test_nameserver_address(self):
        self.raw['services']['ns_address'] = '192.0.2.53'
        self.assertEqual(self.load()['services']['ns_address'], '192.0.2.53')
        for value in ('not-an-ip', '::1', 123):
            self.raw['services']['ns_address'] = value
            with self.assertRaises(ValueError): self.load()
        del self.raw['services']['ns_address']
        self.load()

    def test_failed_start_does_not_report_ready(self):
        from contextlib import redirect_stdout
        from io import StringIO
        import subprocess
        cfg = self.load()
        output = StringIO()
        with redirect_stdout(output), patch.object(launcher.subprocess, 'run', side_effect=subprocess.CalledProcessError(1, ['network.py'])):
            with self.assertRaises(subprocess.CalledProcessError):
                launcher.execute(cfg, 'up')
        self.assertIn('API listener: 127.0.0.1:38080', output.getvalue())
        self.assertNotIn('DIS is ready', output.getvalue())


if __name__ == '__main__':
    unittest.main()
