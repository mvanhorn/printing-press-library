import datetime, hashlib, http.server, json, os, pathlib, queue, sqlite3, subprocess, tempfile, threading, time, urllib.parse
CLI = pathlib.Path('<install-bin>/tabiwa-pp-cli')
MCP = pathlib.Path('<install-bin>/tabiwa-pp-mcp')
EXPECTED = {'tabiwa-pp-cli':'5d93ef94a811e9dfdb74f91be8b4f73f566dd65740a9b5bb99c8dc70531f6ed0','tabiwa-pp-mcp':'ed7a7df1004541111263090aa4b290fbf007720b10ffc8a95067ed9a371689dc'}
products = [{'id':'J0001900','name':'ポイント券','price':'2,000P','is_point_only':True,'overview':'※全額ポイント利用のみ。クレジットカードは利用できません。QRを提示。','ticket_type':'freepass'}, {'id':'J0000900','name':'岡山・香川ワイドパス','price':'3,600円','overview':'※直島は含まれません。','ticket_type':'freepass'}]
geography = [{'id':16,'name':'富山県','areas':[{'id':'21','name':'富山・立山'}]}, {'id':17,'name':'石川県','areas':[{'id':'22','name':'金沢'}]}]
requests=[]
class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self,*args): pass
    def do_GET(self):
        u=urllib.parse.urlsplit(self.path)
        requests.append({'method':'GET','path':u.path,'query':urllib.parse.parse_qs(u.query,keep_blank_values=True),'region_cookie':self.headers.get('Cookie'),'authorization_present':bool(self.headers.get('Authorization'))})
        if u.path=='/ticketList/search': body={'response':products}
        elif u.path=='/ticketList/area': body={'response':geography}
        else:
            self.send_error(404); return
        b=json.dumps(body,ensure_ascii=False).encode()
        self.send_response(200); self.send_header('Content-Type','application/json'); self.send_header('Content-Length',str(len(b))); self.end_headers(); self.wfile.write(b)
    def do_POST(self):
        requests.append({'method':'POST','path':self.path}); self.send_error(405)
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler)
threading.Thread(target=server.serve_forever,daemon=True).start()
home=pathlib.Path(tempfile.mkdtemp(prefix='tabiwa-greptile-a60db57a-'))
env={k:v for k,v in os.environ.items() if not (k.startswith('TABIWA_') or k.startswith('PRINTING_PRESS_') or k in ('PP_MCP_TRANSPORT','PP_MCP_CLI_PATH'))}
env.update(TABIWA_HOME=str(home), TABIWA_BASE_URL=f'http://127.0.0.1:{server.server_port}', TABIWA_NO_LEARN='true')
proof={'observed_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'head':'a60db57af56ae93ebd0300b741506ed2b355fe05','fixture_script':'<temporary>/tabiwa-greptile-triage-a60db57a.py','isolation_home':str(home),'checks':[], 'cli':{}}
def check(name,value):
    proof['checks'].append({'name':name,'pass':bool(value)})
    if not value: raise AssertionError(name)
def cli(name,args):
    begin=len(requests)
    p=subprocess.run([str(CLI),*args],cwd=home,env=env,capture_output=True,text=True,timeout=25)
    try: data=json.loads(p.stdout)
    except json.JSONDecodeError:
        data=[json.loads(s) for s in p.stdout.splitlines() if s.strip()]
    proof['cli'][name]={'argv':args,'exit_code':p.returncode,'stdout':data,'stderr':p.stderr,'request_indexes':list(range(begin,len(requests)))}
    check(name+' exits zero',p.returncode==0)
    return data
for p in [CLI,MCP]:
    d=hashlib.sha256(p.read_bytes()).hexdigest()
    proof.setdefault('binary_hashes',{})[str(p)]=d
    check(p.name+' exact approved installed bytes',d==EXPECTED[p.name])
ghost='zz_tabiwa_greptile_no_match_2267_a60db57a'
generic=cli('generic_live_search',['search',ghost,'--data-source','live','--no-cache','--limit','3','--json','--no-learn'])
native=cli('normalized_live_search',['catalog','search',ghost,'--region','20','--no-cache','--limit','3','--json','--no-learn'])
# Current generic search is an envelope with results; normalization is a products object.
hits=generic.get('results',generic.get('data',[])) if isinstance(generic,dict) else generic
proof['generic_hits']=hits
check('generic impossible query returns unrelated two rows',len(hits)==2 and all(ghost not in json.dumps(x,ensure_ascii=False) for x in hits))
check('native same impossible query returns zero products',native.get('products')==[])
check('generic forwards unsupported q without selected region',requests[0]['query'].get('q')==[ghost] and not requests[0]['region_cookie'])
check('native filters locally and selects region',requests[1]['region_cookie']=='regionId=20' and 'q' not in requests[1]['query'])
cli('default_sync',['sync','--strict','--json','--no-learn'])
db=home/'data'/'data.db'
check('default sync created only isolated generic store',db.exists())
with sqlite3.connect(f'file:{db}?mode=ro',uri=True) as con:
    counts=con.execute('SELECT resource_type,count(*) FROM resources GROUP BY resource_type').fetchall()
proof['generic_resource_counts']=counts
check('supported default sync yields geography only',counts==[('geography',2)])
# Invoke the real installed MCP peer, which shells out to its exact sibling CLI.
p=subprocess.Popen([str(MCP),'--transport','stdio'],cwd=home,env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,bufsize=1)
lines=queue.Queue(); err=[]
def reader(stream,dest):
    for line in stream:
        if dest is lines: dest.put(line)
        else: dest.append(line)
threading.Thread(target=reader,args=(p.stdout,lines),daemon=True).start()
threading.Thread(target=reader,args=(p.stderr,err),daemon=True).start()
nextid=0
def rpc(method,params):
    global nextid
    nextid+=1; i=nextid
    p.stdin.write(json.dumps({'jsonrpc':'2.0','id':i,'method':method,'params':params})+'\n');p.stdin.flush()
    deadline=time.monotonic()+25
    while time.monotonic()<deadline:
        line=lines.get(timeout=max(0.1,deadline-time.monotonic()))
        msg=json.loads(line)
        if msg.get('id')==i:
            if 'error' in msg: raise RuntimeError(msg['error'])
            return msg['result']
    raise TimeoutError(method)
def call(name,args):
    begin=len(requests)
    result=rpc('tools/call',{'name':name,'arguments':args})
    proof.setdefault('mcp_calls',{})[name]={'arguments':args,'result':result,'request_indexes':list(range(begin,len(requests)))}
    check('MCP '+name+' succeeds',not result.get('isError',False))
    text='\n'.join(c.get('text','') for c in result.get('content',[]) if c.get('type')=='text')
    return json.loads(text)
try:
    rpc('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'tabiwa-same-reviewer-triage','version':'1'}})
    p.stdin.write(json.dumps({'jsonrpc':'2.0','method':'notifications/initialized'})+'\n');p.stdin.flush()
    tool_list=rpc('tools/list',{})['tools']; tools={t['name']:t for t in tool_list}
    proof['runtime_tools']={n:tools[n] for n in ('sql','catalog_search','catalog_inspect','catalog_compare','catalog_saved')}
    sql=call('sql',{'query':"SELECT json_extract(data,'$.name') FROM resources WHERE resource_type='catalog'"})
    geo=rpc('tools/call',{'name':'sql','arguments':{'query':"SELECT json_extract(data,'$.name') AS name FROM resources WHERE resource_type='geography'"}})
    proof['mcp_geography_sql']=geo
    check('SQL advertised catalog example returns zero records',sql.get('rows')==[] or sql.get('results')==[])
    geodata=json.loads('\n'.join(c.get('text','') for c in geo.get('content',[]) if c.get('type')=='text'))
    georows=geodata.get('rows',geodata.get('results',[]))
    check('SQL geography example returns two records',len(georows)==2 and not geo.get('isError',False))
    props=tools['catalog_inspect']['inputSchema']['properties']
    check('save-capable tool advertises readOnlyHint true',tools['catalog_inspect']['annotations'].get('readOnlyHint') is True and 'save' in props)
    cache=home/'cache'/'catalog'/'saved.db'
    check('selected evidence cache initially absent',not cache.exists())
    idkey=next(k for k in props if k in ('product-id','product_id','id'))
    inspect=call('catalog_inspect',{idkey:'J0001900','region':'20','save':True})
    check('MCP accepted save true creates persistent selected cache',cache.exists() and inspect.get('saved') is True)
    saved=call('catalog_saved',{'region':'20'})
    check('saved tool returns selected persisted evidence',len(saved.get('products',[]))==1)
finally:
    p.stdin.close()
    try:p.wait(timeout=3)
    except subprocess.TimeoutExpired:p.terminate();p.wait(timeout=3)
    proof['mcp_stderr']=''.join(err)[:2000]
    server.shutdown()
proof['requests']=requests
check('all fixture provider operations are GET',all(r['method']=='GET' for r in requests))
proof['verdict']='three_findings_reproduced'
path=pathlib.Path('<temporary>/tabiwa-greptile-triage-a60db57a.json')
path.write_text(json.dumps(proof,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'proof':str(path),'home':str(home),'checks':len(proof['checks']),'pass':sum(c['pass'] for c in proof['checks']),'generic_hits':len(hits),'native_hits':len(native['products']),'resource_counts':counts,'request_count':len(requests),'verdict':proof['verdict']},ensure_ascii=False))
