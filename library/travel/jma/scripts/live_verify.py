#!/usr/bin/env python3
"""Live read-only JMA CLI verification. Captures are never substituted for network."""
import os,subprocess,pathlib,json,datetime,time,urllib.request
root=pathlib.Path(__file__).resolve().parents[1];p=root/'evidence/live';p.mkdir(exist_ok=True)
base=['--cache-dir',str(root/'evidence/cache'),'--home',str(root/'evidence/home')]
cmds=[['inventory','refresh'],['forecast','get','--area','130010','--days','3'],['warnings','get','--area','1340100','--detail'],['warnings','get','--area','1310100'],['typhoons','list'],['typhoons','get','--id','TC2632','--detail'],['typhoons','get','--id','TC2633'],['forecast','get','--area','270000','--period','week'],['forecast','get','--area','014030'],['forecast','get','--area','460040'],['warnings','get','--area','270000','--limit','2'],['forecast','get','--area','130020','--period','week'],['warnings','get','--area','1920100'],['areas','search','--query','nonexistent-source','--select','results.id']]
results=[]
for n,cmd in enumerate(cmds):
 start=time.monotonic();v=subprocess.run([str(root/'bin'/('jma-pp-cli.exe' if os.name == 'nt' else 'jma-pp-cli')),*cmd,*base,'--refresh'],capture_output=True,text=True,timeout=65)
 (p/f'{n:02}.json').write_text(v.stdout);(p/f'{n:02}.stderr').write_text(v.stderr)
 try:
  d=json.loads(v.stdout);m=d['meta'];info={'coverage':m['coverage'],'requests':m['requests'],'latency_ms':m['elapsed_ms']}
  if cmd[0]=='warnings':info.update({'state':d['results']['state'],'products':len(d['results']['products'])})
 except Exception:info={'stderr':v.stderr[:400]}
 results.append({'args':cmd,'exit':v.returncode,'bytes':len(v.stdout.encode()),**info})
(p/'matrix.json').write_text(json.dumps({'captured_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'cases':results},indent=2));print(json.dumps(results,indent=2))
if any(x['exit'] for x in results):raise SystemExit(1)
