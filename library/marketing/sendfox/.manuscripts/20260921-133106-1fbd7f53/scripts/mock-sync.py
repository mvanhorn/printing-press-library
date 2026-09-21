import os,pathlib,json,subprocess,http.server,threading,urllib.parse,sqlite3
w=pathlib.Path(os.environ['CLI_WORK_DIR']);r=pathlib.Path(os.environ['API_RUN_DIR']);requests=[]
class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  u=urllib.parse.urlsplit(self.path);page=int(urllib.parse.parse_qs(u.query).get('page',['1'])[0]);requests.append(self.path)
  if u.path=='/contact-fields':
   items=[{'id':i,'label':'Preference '+str(i),'name':'preference_'+str(i),'type':'text'} for i in (range(1,21) if page==1 else range(21,22))];result={'data':items,'current_page':page,'last_page':2,'per_page':20,'total':21,'next_page_url':f'{server.server_port}/contact-fields?page=2' if page==1 else None}
  else:result={'data':[],'current_page':1,'last_page':1,'total':0}
  self.send_response(200);self.send_header('Content-Type','application/json');self.end_headers();self.wfile.write(json.dumps(result).encode())
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=server.serve_forever,daemon=True).start();env={k:os.environ[k] for k in ['PATH','HOME','TMPDIR','GOCACHE','GOMODCACHE'] if k in os.environ};env.update(SENDFOX_HOME=str(r/'mock-sync-home'),SENDFOX_CONFIG=str(r/'isolated/empty.toml'),SENDFOX_BASE_URL=f'http://127.0.0.1:{server.server_port}',SENDFOX_API_TOKEN='unit-fixture-value',SENDFOX_NO_LEARN='true')
db=r/'proofs/sync-mock.db'
if db.exists():db.unlink()
def run(args):return subprocess.run([str(w/'sendfox-pp-cli')]+args,cwd=w,env=env,text=True,capture_output=True,timeout=30)
p=run(['sync','--resources','contact_fields','--db',str(db),'--full','--strict','--max-pages','5','--rate-limit','10000','--no-cache','--json']);server.shutdown()
conn=sqlite3.connect(db);count=conn.execute("SELECT count(*) FROM resources WHERE resource_type='contact_fields'").fetchone()[0];conn.close()
sql=run(['sql','SELECT count(*) AS count FROM resources','--db',str(db),'--json']);search=run(['search','Preference','--db',str(db),'--type','contact_fields','--json']);bad=run(['sql','DELETE FROM resources','--db',str(db),'--json']);analytics=run(['analytics','--type','contact_fields','--db',str(db),'--json'])
result={'sync_exit':p.returncode,'sync_output':p.stdout,'requests':requests,'records':count,'sql':json.loads(sql.stdout) if sql.returncode==0 else sql.stderr,'search_exit':search.returncode,'search':json.loads(search.stdout) if search.returncode==0 else search.stderr,'rejected_mutating_sql':bad.returncode!=0,'analytics':json.loads(analytics.stdout) if analytics.returncode==0 else analytics.stderr}
(r/'proofs/mock-sync.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,indent=2)[:4000]);assert p.returncode==0 and count==21 and len(requests)==2 and sql.returncode==0 and search.returncode==0 and bad.returncode!=0
