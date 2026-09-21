import os,pathlib,json,re,subprocess,http.server,threading
w=pathlib.Path(os.environ['CLI_WORK_DIR']);proof=pathlib.Path(os.environ['PROOFS_DIR']);ops=json.loads((w/'internal/contract/operations.json').read_text());seen=[]
class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def respond(self):
  body=self.rfile.read(int(self.headers.get('Content-Length',0)))
  seen.append({'method':self.command,'path':self.path,'body':json.loads(body) if body else None})
  self.send_response(200);self.send_header('Content-Type','application/json');self.end_headers();self.wfile.write(json.dumps({'id':12,'email':'reader@example.com','status':'completed','data':[],'current_page':1,'last_page':1,'total':0,'created':1,'updated':0}).encode())
 do_GET=do_POST=do_PATCH=do_DELETE=do_PUT=respond
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=server.serve_forever,daemon=True).start()
env={k:os.environ[k] for k in ['PATH','HOME','TMPDIR','GOCACHE','GOMODCACHE'] if k in os.environ};env.update(SENDFOX_HOME=str(pathlib.Path(os.environ['API_RUN_DIR'])/'mock-home'),SENDFOX_CONFIG=str(pathlib.Path(os.environ['API_RUN_DIR'])/'isolated/empty.toml'),SENDFOX_BASE_URL=f'http://127.0.0.1:{server.server_port}',SENDFOX_API_TOKEN='unit-fixture-value',SENDFOX_NO_LEARN='true')
def sample(s,k):
 if s.get('enum'):return s['enum'][0]
 typ=s.get('type');typ=typ[0] if isinstance(typ,list) else typ
 if typ=='integer':return max(1,s.get('minimum',1))
 if typ=='number':return 1
 if typ=='boolean':return False
 if typ=='array':return [sample(s.get('items',{}),k)]
 if typ=='object':return {key:sample(v,key) for key,v in s.get('properties',{}).items() if key in s.get('required',[])}
 if s.get('format')=='email':return 'reader@example.com'
 if s.get('format')=='uri':return 'https://example.com/news'
 if s.get('format')=='date-time':return '2027-01-01T12:00:00Z'
 return {'html':'<p>Hello</p>','subject':'News','title':'Newsletter','from_name':'Newsletter'}.get(k,'Reader')
results=[]
for op in ops:
 source=None
 for p in (w/'internal/cli').glob('*.go'):
  text=p.read_text()
  if '"pp:method": "'+op['method']+'"' in text and '"pp:path": "'+op['path']+'"' in text:source=text;break
 if source is None:raise AssertionError(op['id'])
 use=re.search(r'Use:\s*"([^"]+)"',source)[1];argv=[str(w/'sendfox-pp-cli'),op['id'].split('.')[0].replace('_','-')]+([] if op['id'].split('.')[0] in ['me','unsubscribe'] else [use.split()[0]])
 for p in op['parameters']:
  if p['in']=='path':argv.append('12')
  elif p['in']=='query' and p.get('required'):argv+=['--'+p['name'].replace('_','-'),str(sample(p['schema'],p['name']))]
 for k in op['body_schema'].get('required',[]):
  value=sample(op['body_schema']['properties'][k],k);v=json.dumps(value) if isinstance(value,(list,dict)) else str(value).lower() if isinstance(value,bool) else str(value);argv+=['--'+k.replace('_','-'),v]
 argv+=['--json','--yes','--approve-sensitive','--no-cache','--rate-limit','10000','--no-learn']
 before=len(seen);p=subprocess.run(argv,cwd=w,env=env,text=True,capture_output=True,timeout=15)
 parsed=None
 try:parsed=json.loads(p.stdout)
 except ValueError:pass
 success=p.returncode==0 and len(seen)==before+1 and seen[-1]['method']==op['method'] and parsed is not None
 results.append({'id':op['id'],'passed':success,'exit':p.returncode,'request':seen[-1] if len(seen)>before else None,'error':p.stderr[-900:] if not success else ''})
server.shutdown();(proof/'mock-operations.json').write_text(json.dumps({'operations':len(results),'passed':sum(r['passed'] for r in results),'results':results},indent=2)+'\n');print(json.dumps({'operations':len(results),'passed':sum(r['passed'] for r in results),'failures':[r for r in results if not r['passed']]},indent=2));raise SystemExit(0 if all(r['passed'] for r in results) else 1)
