from pathlib import Path
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json, os, sqlite3, subprocess, tempfile, threading, time
r=Path('/Users/zjsng/printing-press/.runstate/zjsng-a9b2d4f4/runs/20260927-212939-86ef661f'); w=r/'working/tabelog-pp-cli'; b=w/'tabelog-pp-cli'; td=w/'e2e/testdata'
requests=[]
class Replay(BaseHTTPRequestHandler):
 def do_GET(self):
  requests.append(self.path)
  body=(td/('samboa-detail.html' if '13005012/' in self.path else 'ginza-bars.html')).read_bytes()
  self.send_response(200);self.send_header('Content-Type','text/html; charset=utf-8');self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body)
 def log_message(self,*a):pass
server=ThreadingHTTPServer(('127.0.0.1',0),Replay);threading.Thread(target=server.serve_forever,daemon=True).start()
env={k:v for k,v in os.environ.items() if not k.startswith('TABELOG_') and k not in {'HTTP_PROXY','HTTPS_PROXY','ALL_PROXY','NO_PROXY'}};env.update(TABELOG_TEST_MODE='1',TABELOG_TEST_BASE_URL=f'http://127.0.0.1:{server.server_port}',HTTP_PROXY='',HTTPS_PROXY='',ALL_PROXY='',NO_PROXY='127.0.0.1,localhost')
result={}
try:
 with tempfile.TemporaryDirectory(prefix='tabelog-review-') as tmp:
  def invoke(args):
   p=subprocess.run([str(b),*args,'--agent','--home',tmp],env=env,cwd=w,capture_output=True,text=True,timeout=25)
   return {'exit':p.returncode,'stdout':p.stdout,'stderr':p.stderr}
  find=['find','--area','https://tabelog.com/en/tokyo/A1301/A130101/rstLst/','--cuisine','bar','--meal','dinner','--budget-max','5000','--limit','5']
  x=invoke(find);assert x['exit']==0,x
  x=invoke(['lists','add','trip','13005012']);assert x['exit']==0,x
  before=json.loads(x['stdout'])['items'][0]
  x=invoke(['lists','refresh','trip','13005012']);assert x['exit']==0,x
  after=json.loads(x['stdout'])['items'][0]
  result['budget_provenance_only_changes']=[c for c in after['changes'] if c['field'] in ['dinner_budget','lunch_budget']]
  result['prior_budgets']={k:before[k] for k in ['dinner_budget','lunch_budget']}
  result['current_budgets']={k:after[k] for k in ['dinner_budget','lunch_budget']}
  count=len(requests)
  bad=invoke(find+['--data-source','local','--select','title,url'])
  result['documented_select_example']={**bad,'replay_requests':len(requests)-count}
  bad_all=invoke(find+['--data-source','local','--select','nonexistent'])
  result['all_invalid_projection_diagnostic']=bad_all
  paths=list(Path(tmp).rglob('*.db'));assert len(paths)==1,paths
  injection=sqlite3.connect(paths[0]);injection.execute("CREATE TRIGGER review_slow_sync BEFORE INSERT ON sync_state BEGIN SELECT sum(x) FROM (WITH RECURSIVE ticks(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM ticks WHERE x<1000000) SELECT x FROM ticks); END;");injection.commit();injection.close()
  count=len(requests);start=time.perf_counter()
  late=invoke(find+['--data-source','local','--timeout','100ms'])
  result['metadata_write_deadline_probe']={**late,'elapsed_ms':(time.perf_counter()-start)*1000,'requested_timeout_ms':100,'replay_requests':len(requests)-count,'scope':'Isolated actual CLI DB with an artificial slow INSERT trigger on generic sync_state to exercise the unbounded post-snapshot metadata write'}
finally:server.shutdown();server.server_close()
conn=sqlite3.connect(':memory:')
conn.executescript('CREATE TABLE tabelog_snapshots(id TEXT PRIMARY KEY,data TEXT NOT NULL,updated_at TEXT NOT NULL);CREATE TABLE tabelog_notebooks(name TEXT PRIMARY KEY);CREATE TABLE tabelog_memberships(list_name TEXT,restaurant_id TEXT,note TEXT,position INTEGER,PRIMARY KEY(list_name,restaurant_id));CREATE INDEX tabelog_membership_order ON tabelog_memberships(list_name,position);')
conn.executemany('INSERT INTO tabelog_snapshots VALUES(?,?,?)',[(str(13000000+i),'x'*128,'2026-09-27') for i in range(10000)])
conn.executemany('INSERT INTO tabelog_memberships VALUES(?,?,?,?)',[('trip'+str(i//500),str(13000000+i),'',i) for i in range(9000)])
q='SELECT COALESCE(SUM(length(CAST(data AS BLOB))),0) FROM tabelog_snapshots s WHERE NOT EXISTS (SELECT 1 FROM tabelog_memberships m WHERE m.restaurant_id=s.id)'
plan_before=conn.execute('EXPLAIN QUERY PLAN '+q).fetchall();start=time.perf_counter();total_before=conn.execute(q).fetchone()[0];elapsed_before=(time.perf_counter()-start)*1000
conn.execute('CREATE INDEX candidate_fix_membership_restaurant ON tabelog_memberships(restaurant_id)')
plan_after=conn.execute('EXPLAIN QUERY PLAN '+q).fetchall();start=time.perf_counter();total_after=conn.execute(q).fetchone()[0];elapsed_after=(time.perf_counter()-start)*1000
result['retention_index_probe']={'engine':'Python SQLite '+sqlite3.sqlite_version,'scope':'private synthetic SQL fixture using current production schema/query; no product DB modified','snapshots':10000,'memberships':9000,'payload_bytes_each':128,'before_ms':elapsed_before,'candidate_index_ms':elapsed_after,'same_total':total_before==total_after,'plan_before':plan_before,'plan_candidate_index':plan_after}
(r/'proofs/phase17-maintainability-reproducer.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps({'budget_changes':result['budget_provenance_only_changes'],'select_exit':bad['exit'],'select_requests':result['documented_select_example']['replay_requests'],'retention_probe':result['retention_index_probe'],'metadata_deadline':{'exit':result['metadata_write_deadline_probe']['exit'],'elapsed_ms':result['metadata_write_deadline_probe']['elapsed_ms'],'requests':result['metadata_write_deadline_probe']['replay_requests']}},indent=2))
