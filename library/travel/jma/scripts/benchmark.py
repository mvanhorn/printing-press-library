#!/usr/bin/env python3
import os,subprocess,pathlib,json,re,datetime
root=pathlib.Path(__file__).resolve().parents[1];p=root/'evidence/benchmark';p.mkdir(exist_ok=True)
rows=[]
cases={'forecast':['forecast','get','--area','130010'],'warnings':['warnings','get','--area','1340100'],'typhoon':['typhoons','get','--id','TC2633'],'typhoon-list':['typhoons','list']}
base=['--cache-dir',str(root/'evidence/cache'),'--home',str(root/'evidence/home')]
for name,args in cases.items():
 for mode in ['uncached-forced-refresh','cached']:
  cmd=[str(root/'bin'/('jma-pp-cli.exe' if os.name == 'nt' else 'jma-pp-cli')),*args,*base]+(['--refresh'] if mode.startswith('uncached') else [])
  v=subprocess.run(['/usr/bin/time','-l',*cmd],capture_output=True,timeout=65)
  (p/(name+'-'+mode+'.json')).write_bytes(v.stdout);(p/(name+'-'+mode+'.time')).write_bytes(v.stderr)
  try:
   d=json.loads(v.stdout);m=d['meta'];rss=re.search(rb'(\d+)\s+maximum resident set size',v.stderr);rows.append({'case':name,'mode':mode,'exit':v.returncode,'output_bytes':len(v.stdout),'requests':m['requests'],'latency_ms':m['elapsed_ms'],'peak_rss_bytes':int(rss[1]) if rss else None})
  except Exception:rows.append({'case':name,'mode':mode,'exit':v.returncode,'error':v.stderr.decode()[:500]})
out={'captured_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'method':'macOS /usr/bin/time -l, sequential subprocesses; uncached forces refresh then cached immediately; source request counts from CLI meta; no fixture data','cases':rows};(root/'evidence/benchmark.json').write_text(json.dumps(out,indent=2));print(json.dumps(rows,indent=2))
if any(x['exit'] for x in rows):raise SystemExit(1)
