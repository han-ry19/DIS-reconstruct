#!/usr/bin/env python3
"""Manage only processes identified by both PID and Linux process start time."""
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import sys
import time


def stamp(pid):
    try:
        fields = Path('/proc/' + str(pid) + '/stat').read_text().rsplit(')', 1)[1].split()
        return None if fields[0] == 'Z' else fields[19]
    except (FileNotFoundError, ProcessLookupError):
        return None


def live(process):
    return process.get('start') is not None and stamp(process['pid']) == process['start']


def main(request):
    action = request['action']
    if action == 'status':
        return {name: live(p) for name, p in request['processes'].items()}
    if action == 'connect':
        try:
            with socket.create_connection(('127.0.0.1', request['port']), timeout=.5):
                return {'ready': True}
        except OSError:
            return {'ready': False}
    if action == 'ports':
        sockets = []
        try:
            for port in request['ports']:
                sock = socket.socket()
                sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
                sock.bind(('0.0.0.0', port))
                sockets.append(sock)
        finally:
            for sock in sockets:
                sock.close()
        return {'available': True}
    if action == 'stop':
        processes = request['processes']
        for p in reversed(processes):
            if live(p):
                os.killpg(p['pid'], signal.SIGTERM)
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline and any(live(p) for p in processes):
            time.sleep(.1)
        for p in processes:
            if live(p):
                os.killpg(p['pid'], signal.SIGKILL)
        return {'stopped': True}
    if action == 'start':
        record = Path(request['record'])
        record.parent.mkdir(parents=True, exist_ok=True)
        if record.exists():
            previous = json.loads(record.read_text())
            if live(previous):
                return previous
        log = Path(request['log'])
        log.parent.mkdir(parents=True, exist_ok=True)
        env = dict(os.environ, **request.get('env', {}))
        with log.open('ab') as output:
            p = subprocess.Popen(request['command'], env=env, stdin=subprocess.DEVNULL,
                                 stdout=output, stderr=output, start_new_session=True)
        result = dict(pid=p.pid, start=stamp(p.pid))
        temporary = record.with_suffix('.tmp')
        temporary.write_text(json.dumps(result))
        temporary.replace(record)
        return result
    raise ValueError('Unknown agent action')


if __name__ == '__main__':
    try:
        print(json.dumps(main(json.load(sys.stdin))))
    except Exception as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
