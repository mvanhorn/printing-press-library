import datetime,hashlib,http.server,json,os,pathlib,queue,sqlite3,subprocess,tempfile,threading,time,urllib.parse
ROOT=pathlib.Path('<source-project>')
BUNDLE=json.load(open('<temporary>/tabiwa-reviewer-round4-bundle.json'))
CLI=pathlib.Path(BUNDLE['peers']['tabiwa-pp-cli']['path']); MCP=pathlib.Path(BUNDLE['peers']['tabiwa-pp-mcp']['path'])
PRODUCTS=[{'id':'J0001900','name':'ポイント券','price':'2,000P','is_point_only':True,'overview':'※全額ポイント利用のみ。クレジットカードは利用できません。QRを提示。','ticket_type':'freepass'}, {'id':'J0000900','name':'岡山・香川ワイドパス','price':'3,600円','overview':'※直島は含まれません。','ticket_type':'freepass'}]
GEOGRAPHY=[{'id':16,'name':'富山県','areas':[{'id':'21','name':'富山・立山'}]}, {'id':17,'name':'石川県','areas':[{'id':'22','name':'金沢'}]}]
requests=[]
class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  u=urllib.parse.urlsplit(self.path);requests.append({'method':'GET','path':u.path,'query':urllib.parse.parse_qs(u.query,keep_blank_values=True),'region_cookie':self.headers.get('Cookie'),'authorization_present':bool(self.headers.get('Authorization'))})
  if u.path=='/ticketList/search':body={'response':PRODUCTS}
  elif u.path=='/ticketList/area':body={'response':GEOGRAPHY}
  else:self.send_error(404);return
  b=json.dumps(body,ensure_ascii=False).encode();self.send_response(200);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(b)));self.end_headers();self.wfile.write(b)
 def do_POST(self):requests.append({'method':'POST','path':self.path});self.send_error(405)
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=server.serve_forever,daemon=True).start()
home=pathlib.Path(tempfile.mkdtemp(prefix='tabiwa-reviewer-round4-runtime-'))
env={k:v for k,v in os.environ.items() if not(k.startswith('TABIWA_') or k.startswith('PRINTING_PRESS_') or k in ('PP_MCP_TRANSPORT','PP_MCP_CLI_PATH'))}
env.update(TABIWA_HOME=str(home),TABIWA_BASE_URL=f'http://127.0.0.1:{server.server_port}',TABIWA_NO_LEARN='true')
proof={'observed_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'home':str(home),'extracted_cli':str(CLI),'extracted_mcp':str(MCP),'binary_hashes':{str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in [CLI,MCP]},'checks':[],'cli':{},'mcp':{}}
def check(name,value):
 proof['checks'].append({'name':name,'pass':bool(value)})
 if not value:
  pathlib.Path('<temporary>/tabiwa-reviewer-round4-partial.json').write_text(json.dumps(proof,ensure_ascii=False,indent=2)+'\n');raise AssertionError(name)
def payload(x):return x.get('results',x) if isinstance(x,dict) else x
def run(name,args,exitcode=0):
 begin=len(requests);p=subprocess.run([str(CLI),*args],cwd=home,env=env,capture_output=True,text=True,timeout=25)
 try:data=json.loads(p.stdout)
 except Exception:
  try:data=[json.loads(s) for s in p.stdout.splitlines() if s.strip()]
  except Exception:data=p.stdout
 proof['cli'][name]={'args':args,'exit_code':p.returncode,'payload':data,'stderr':p.stderr,'request_indexes':list(range(begin,len(requests)))}
 check(name+' expected exit',p.returncode==exitcode)
 return data,p
# Unsupported live request must never emit false product matches or call provider.
begin=len(requests);_,refused=run('generic_live_refused',['search','zz_tabiwa_greptile_no_match_2267_a60db57a','--data-source','live','--no-cache','--limit','3','--json','--no-learn'],2)
check('root live refusal points to catalog search and sends zero requests','catalog search' in refused.stderr and len(requests)==begin)
run('default_sync',['sync','--strict','--json','--no-learn'])
# Give custom DB deliberately different contents to prove --db is honored.
GEOGRAPHY=[{'id':18,'name':'福井県','areas':[]},{'id':19,'name':'福井観光','areas':[]}]
custom=home/'custom-geography.db';run('custom_sync',['sync','--db',str(custom),'--strict','--json','--no-learn'])
GEOGRAPHY=[{'id':16,'name':'富山県','areas':[{'id':'21','name':'富山・立山'}]}, {'id':17,'name':'石川県','areas':[{'id':'22','name':'金沢'}]}]
for mode in ['auto','local']:
 begin=len(requests);x,_=run('generic_'+mode,['search','福井','--data-source',mode,'--db',str(custom),'--type','geography','--limit','1','--json','--no-learn'])
 check(mode+' local FTS preserves custom DB/type/limit/provenance',len(x['results'])==1 and '福井' in x['results'][0]['name'] and x['meta']['source']=='local')
 check(mode+' root search no HTTP',len(requests)==begin)
x,_=run('generic_negative',['search','zz_tabiwa_greptile_no_match_2267_a60db57a','--db',str(custom),'--type','geography','--json','--no-learn']);check('generic absent phrase yields no match',x['results']==[])
x,_=run('generic_type_filter',['search','福井','--db',str(custom),'--type','not-a-synced-type','--json','--no-learn']);check('generic resource type filters rows',x['results']==[])
x,_=run('native_positive',['catalog','search','ポイント','--region','20','--no-cache','--json','--no-learn']);check('native positive text selects only matching product',len(payload(x)['products'])==1 and payload(x)['products'][0]['id']=='J0001900')
x,_=run('native_negative',['catalog','search','zz_tabiwa_greptile_no_match_2267_a60db57a','--region','20','--no-cache','--json','--no-learn']);check('native absent phrase stays empty',payload(x)['products']==[])
cache=home/'cache'/'catalog'/'saved.db';check('all default no-save CLI reads left selected cache absent',not cache.exists())
x,_=run('agent_context',['agent-context','--json','--no-learn'])
proof['agent_commands']={}
def collect(items,prefix=''):
 for item in items:
  name=(prefix+' '+item['name']).strip();proof['agent_commands'][name]=item
  collect(item.get('subcommands',item.get('children',[])),name)
collect(x.get('commands',[]))
for name in ['catalog','catalog search','catalog inspect','catalog compare']:
 a=proof['agent_commands'].get(name,{}).get('annotations',{})
 check(name+' emitted local-write and live-happy annotation',a.get('mcp:read-only')=='false' and a.get('mcp:local-write')=='true' and a.get('pp:live-happy-path')=='true' and '--save=true' in a.get('pp:happy-args',''))
# Actual extracted MCP shell-outs to its extracted sibling.
p=subprocess.Popen([str(MCP),'--transport','stdio'],cwd=home,env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,bufsize=1)
lines=queue.Queue();errs=[]
def reader(stream,dest):
 for line in stream:
  if dest is lines:dest.put(line)
  else:dest.append(line)
threading.Thread(target=reader,args=(p.stdout,lines),daemon=True).start();threading.Thread(target=reader,args=(p.stderr,errs),daemon=True).start();nextid=0
def rpc(method,params):
 global nextid
 nextid+=1;i=nextid;p.stdin.write(json.dumps({'jsonrpc':'2.0','id':i,'method':method,'params':params})+'\n');p.stdin.flush();deadline=time.monotonic()+25
 while time.monotonic()<deadline:
  msg=json.loads(lines.get(timeout=max(0.1,deadline-time.monotonic())))
  if msg.get('id')==i:
   if 'error' in msg:raise RuntimeError(msg['error'])
   return msg['result']
 raise TimeoutError(method)
def call(label,name,args):
 begin=len(requests);x=rpc('tools/call',{'name':name,'arguments':args});check(label+' succeeds',not x.get('isError',False))
 data=json.loads('\n'.join(c.get('text','') for c in x.get('content',[]) if c.get('type')=='text'))
 proof['mcp'][label]={'tool':name,'args':args,'payload':data,'request_indexes':list(range(begin,len(requests)))}
 return payload(data)
try:
 rpc('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'tabiwa-same-reviewer-round4','version':'1'}})
 p.stdin.write(json.dumps({'jsonrpc':'2.0','method':'notifications/initialized'})+'\n');p.stdin.flush()
 ts=rpc('tools/list',{})['tools'];tools={t['name']:t for t in ts};proof['actual_tools']=ts
 check('exact actual tool count is 27',len(tools)==27)
 manifest=json.loads((ROOT/'tools-manifest.json').read_text());declared=manifest.get('tools',[])
 if isinstance(declared,dict):declared=list(declared.values())
 if not declared:
  for v in manifest.values():
   if isinstance(v,list) and v and isinstance(v[0],dict) and 'name' in v[0]:declared=v;break
 md={x['name']:x for x in declared}
 check('tool metadata names match actual 27 tools',set(md)==set(tools))
 for name,t in tools.items():
  check(name+' metadata matches schema description and hints',all(md[name].get(k)==t.get(k) for k in ['inputSchema','description','annotations']))
 check('exact actual domain tool partition',{x['name'] for x in declared if x.get('surface')=='domain'}=={'catalog_search','catalog_inspect','catalog_compare','catalog_saved','geography_list'})
 for name in ['catalog_search','catalog_inspect','catalog_compare']:
  check(name+' truthful may-write hint and save input',tools[name]['annotations'].get('readOnlyHint') is False and tools[name]['inputSchema']['properties'].get('save',{}).get('type')=='boolean')
 for name in ['catalog_saved','geography_list']:
  check(name+' read-only hint retained',tools[name]['annotations'].get('readOnlyHint') is True)
 check('actual compare ids schema string retained',tools['catalog_compare']['inputSchema']['properties']['ids']['type']=='string')
 desc=tools['sql']['inputSchema']['properties']['query']['description'];check('actual SQL exact geography example and separate saved guidance',"resource_type='geography'" in desc and 'catalog_saved' in desc and "resource_type='catalog'" not in desc)
 sql=call('sql_exact_advertised_example','sql',{'query':"SELECT json_extract(data,'$.name') FROM resources WHERE resource_type='geography'"})
 rows=sql.get('rows',sql.get('results',[]));check('actual emitted SQL example returns two default geography names',len(rows)==2 and '富山県' in json.dumps(rows,ensure_ascii=False))
 check('generic database and selected cache are separate',(home/'data'/'data.db').exists() and not cache.exists())
 cases=[('catalog_search',{'query':'ポイント','region':'20'}),('catalog_inspect',{'product-id':'J0001900','region':'20'}),('catalog_compare',{'ids':'J0001900,J0000900','region':'20','on':'2026-10-28'})]
 for name,args in cases:
  x=call(name+'_default',name,args)
  check(name+' default no-save and leaves cache absent',x['saved'] is False and not cache.exists())
 for name,args in cases:
  x=call(name+'_save',name,dict(args,save=True));items=x.get('products',x.get('comparisons',[]))
  check(name+' positive save nonempty and persistent',x['saved'] is True and len(items)>0 and cache.exists())
 saved_before=(hashlib.sha256(cache.read_bytes()).hexdigest(),cache.stat().st_mtime_ns,cache.stat().st_mode)
 x=call('catalog_saved','catalog_saved',{'region':'20'});saved_after=(hashlib.sha256(cache.read_bytes()).hexdigest(),cache.stat().st_mtime_ns,cache.stat().st_mode)
 check('saved actual two observations and read leaves bytes/clock/mode unchanged',len(x['products'])==2 and saved_before==saved_after)
 check('saved preserves original source observation clocks',all(p['observed_at'] for p in x['products']))
 geo=call('geography_list','geography_list',{'region':'20'});check('actual extracted geography native provider rows',len(geo['prefectures'])==2)
 ctx=call('context','context',{});proof['context']=ctx
 text=json.dumps(ctx,ensure_ascii=False).lower();check('context discloses local writes separate geography and selected evidence','local-write' in text and 'geography' in text and 'catalog/saved.db' in text and 'get-only' in text)
 check('context actual total and domain tool counts',ctx.get('tool_count')==27 and ctx.get('domain_tool_count')==5)
 # Provider-free SQLite introspection confirms separate actual stores and bundled runtime.
 version=call('sql_runtime_version','sql',{'query':'SELECT sqlite_version() AS version'});check('actual bundled SQLite runtime current', '3.53.4' in json.dumps(version))
 x,_=run('promoted_catalog_save',['catalog','--region','20','--query','ポイント','--limit','3','--save=true','--json','--no-learn']);check('promoted executable catalog saves matching nonempty evidence',payload(x)['saved'] is True and len(payload(x)['products'])==1)
finally:
 p.stdin.close()
 try:p.wait(timeout=3)
 except subprocess.TimeoutExpired:p.terminate();p.wait(timeout=3)
 proof['mcp_stderr']=''.join(errs)[:2000];server.shutdown()
proof['requests']=requests;check('all actual provider fixture calls GET-only within two public paths',all(x['method']=='GET' and x['path'] in ('/ticketList/search','/ticketList/area') and not x['authorization_present'] for x in requests))
proof['verdict']='PASS';proof['passed']=sum(x['pass'] for x in proof['checks']);proof['failed']=sum(not x['pass'] for x in proof['checks'])
pathlib.Path('<temporary>/tabiwa-reviewer-round4-runtime.json').write_text(json.dumps(proof,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'verdict':proof['verdict'],'checks':len(proof['checks']),'requests':len(requests),'tool_count':len(tools),'bundle':BUNDLE['bundle_sha256'],'home':str(home)},ensure_ascii=False))
