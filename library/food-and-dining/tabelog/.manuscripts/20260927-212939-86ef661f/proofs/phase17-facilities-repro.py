from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import json
import os
import subprocess
import tempfile
import threading
run = Path(__file__).resolve().parents[1]
work = run / 'working/tabelog-pp-cli'
body = (work / 'e2e/testdata/tokyo-ranked.html').read_bytes()
class Replay(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200); self.send_header('Content-Type','text/html; charset=utf-8'); self.end_headers(); self.wfile.write(body)
    def log_message(self,*args): pass
server = ThreadingHTTPServer(('127.0.0.1',0), Replay)
threading.Thread(target=server.serve_forever,daemon=True).start()
env = dict(os.environ,TABELOG_TEST_MODE='1',TABELOG_TEST_BASE_URL=f'http://127.0.0.1:{server.server_port}',HTTP_PROXY='',HTTPS_PROXY='',ALL_PROXY='',NO_PROXY='127.0.0.1,localhost')
report = {'scope':'Unmodified real listing fixture; loopback only; disposable home; compare default summary with full-record projection.'}
try:
    with tempfile.TemporaryDirectory(prefix='tabelog-facilities-review-') as temp:
        for name,args in [('default',['find','--area','tokyo','--limit','1','--agent']),('projected',['find','--area','tokyo','--limit','1','--data-source','local','--agent','--select','items.id,items.facilities'])]:
            result = subprocess.run([str(work/'tabelog-pp-cli'),'--home',temp,*args],cwd=work,env=env,text=True,capture_output=True,timeout=25)
            report[name] = {'args':args,'exit_code':result.returncode,'stderr':result.stderr,'payload':json.loads(result.stdout)}
finally:
    server.shutdown();server.server_close()
(run/'proofs/phase17-facilities-repro.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({k:{'exit_code':v['exit_code'],'items':v['payload']['items'],'stderr':v['stderr']} for k,v in report.items() if isinstance(v,dict)},ensure_ascii=False))
