import json,os,socket,subprocess,tempfile,time,urllib.request
from pathlib import Path
root=Path(__file__).resolve().parents[1]
with tempfile.TemporaryDirectory(prefix='dis-smoke-') as directory:
    path=Path(directory)
    with socket.socket() as sock:
        sock.bind(('127.0.0.1',0)); port=sock.getsockname()[1]
    url='http://127.0.0.1:'+str(port)
    def start():
        process=subprocess.Popen([str(root/'bin/dis-api'),'-listen','127.0.0.1:'+str(port),'-state',str(path/'state.json')],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        try:
            for attempt in range(100):
                if process.poll() is not None: raise RuntimeError('API exited')
                try:
                    with urllib.request.urlopen(url+'/healthz',timeout=1) as r:
                        if r.status==200:return process
                except OSError:time.sleep(.05)
            raise RuntimeError('API startup timeout')
        except BaseException:
            process.terminate();process.wait(timeout=5);raise
    process=start()
    try:
        demo=subprocess.run([str(root/'bin/dis-demo'),'-url',url],check=False,capture_output=True,text=True)
        assert demo.returncode==0, (demo.stdout,demo.stderr)
        record=json.loads(demo.stdout.splitlines()[-1]);assert record['status']=='approved'
        public=subprocess.check_output([str(root/'bin/dis-cli'),'keygen','-out',str(path/'identity.key')],text=True).strip()
        assert (path/'identity.key').stat().st_mode & 0o777 == 0o600
        request={'domain':'dis-local','operation':'identity.register','actor':'cli-user.uid','request_id':'cli-registration','timestamp':int(time.time()),'parameters':{'identity_id':'cli-user.uid','public_key':public,'kind':'user'},'signature':''}
        (path/'request.json').write_text(json.dumps(request))
        signed=subprocess.check_output([str(root/'bin/dis-cli'),'sign','-key',str(path/'identity.key'),'-request',str(path/'request.json')])
        req=urllib.request.Request(url+'/v1/transactions',data=signed,headers={'Content-Type':'application/json'})
        with urllib.request.urlopen(req) as response:assert response.status==200
        process.terminate();process.wait(timeout=5);process=start()
        query=url+'/v1/authorizations?data_id='+record['data_id']+'&owner_id='+record['owner_id']+'&recipient='+record['recipient']
        with urllib.request.urlopen(query) as response:assert json.load(response)==record
        with urllib.request.urlopen(url+'/v1/identities/cli-user.uid') as response:assert json.load(response)['public_key']==public
        print('PASS: independent API process, signed demo, CLI key generation and submission, restart and record lookup')
    finally:
        process.terminate();process.wait(timeout=5)
