from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import hashlib, importlib.metadata, json, math, os, subprocess, tempfile, threading
run = Path('/Users/zjsng/printing-press/.runstate/zjsng-a9b2d4f4/runs/20260927-212939-86ef661f')
os.environ['TIKTOKEN_CACHE_DIR'] = str(run/'proofs/tokenizer-cache')
import tiktoken
encoding = tiktoken.get_encoding('o200k_base')
binary = run/'working/tabelog-pp-cli/build/stage/bin/tabelog-pp-cli'
body = (run/'working/tabelog-pp-cli/e2e/testdata/tokyo-ranked.html').read_bytes()
requests = []
class Replay(BaseHTTPRequestHandler):
 def do_GET(self):
  requests.append(self.path)
  known = self.path.split('?',1)[0] == '/en/tokyo/rstLst/'
  payload = body if known else b'unexpected replay request'
  self.send_response(200 if known else 404)
  self.send_header('Content-Type','text/html; charset=utf-8')
  self.send_header('Content-Length',str(len(payload)))
  self.end_headers(); self.wfile.write(payload)
 def log_message(self,*args): pass
server=ThreadingHTTPServer(('127.0.0.1',0),Replay)
threading.Thread(target=server.serve_forever,daemon=True).start()
env={k:v for k,v in os.environ.items() if not k.startswith('TABELOG_') and k not in {'HTTP_PROXY','HTTPS_PROXY','ALL_PROXY','NO_PROXY'}}
env.update(TABELOG_TEST_MODE='1',TABELOG_TEST_BASE_URL=f'http://127.0.0.1:{server.server_port}',HTTP_PROXY='',HTTPS_PROXY='',ALL_PROXY='',NO_PROXY='127.0.0.1,localhost')
samples=[]
try:
 with tempfile.TemporaryDirectory(prefix='tabelog-projection-error-') as temporary:
  for i in range(20):
   before=len(requests)
   p=subprocess.run([str(binary),'find','--area','https://tabelog.com/en/tokyo/rstLst/','--limit','10','--select','items.id,items.name,items.rating','--agent','--home',str(Path(temporary)/f'projection-{i}')],env=env,capture_output=True,timeout=25)
   assert p.returncode == 0,p.stderr.decode()
   data=json.loads(p.stdout); assert len(data['items'])==10
   assert all(set(row)=={'id','name','rating'} for row in data['items'])
   samples.append({'tokens':len(encoding.encode(p.stdout.decode())),'stdout_bytes':len(p.stdout),'http_requests':len(requests)-before})
   if i==0:(run/'proofs/projection10-sample.json').write_bytes(p.stdout)
  before=len(requests)
  e=subprocess.run([str(binary),'find','--area','tokyo','--meal','dinner','--budget-max','1500','--agent','--home',str(Path(temporary)/'validation')],env=env,capture_output=True,timeout=25)
  assert e.returncode==2
  structured=False
  for stream in [e.stdout,e.stderr]:
   try:
    parsed=json.loads(stream); structured=isinstance(parsed,dict) and ('error' in parsed or 'errors' in parsed)
   except (ValueError,TypeError):continue
   if structured:break
  error={'exit_code':e.returncode,'tokens':len(encoding.encode((e.stdout+e.stderr).decode())),'stdout_bytes':len(e.stdout),'stderr_bytes':len(e.stderr),'http_requests':len(requests)-before,'structured':structured,'text':(e.stdout+e.stderr).decode()}
  assert error['http_requests']==0
  percentile=lambda key:sorted(s[key] for s in samples)[math.ceil(.95*len(samples))-1]
  p95={key:percentile(key) for key in ['tokens','stdout_bytes','http_requests']}
  result={'schema_version':1,'binary_sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),'tokenizer':{'package':'tiktoken','version':importlib.metadata.version('tiktoken'),'encoding':'o200k_base'},'method':'20 fresh-home projected10 subprocesses over existing sanitized loopback HTML; one validation failure; no origin requests. nearest-rank p95.','projection10':{'selection':'items.id,items.name,items.rating','samples':samples,'p95':p95,'checks':{'tokens_500':p95['tokens']<=500,'bytes_2kib':p95['stdout_bytes']<=2048}},'validation_error':error}
  (run/'proofs/projection-error-measurements.json').write_text(json.dumps(result,indent=2)+'\n')
  print(json.dumps({'projection10_p95':p95,'validation_error':error},indent=2))
finally:server.shutdown();server.server_close()
