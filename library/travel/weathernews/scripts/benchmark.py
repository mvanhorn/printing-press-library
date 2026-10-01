#!/usr/bin/env python3
import subprocess,json,time,re
from pathlib import Path
root=Path(__file__).resolve().parents[1];out=root/'evidence/performance';out.mkdir(parents=True,exist_ok=True)
base=[str(root/'weathernews-pp-cli'),'--home',str(root/'evidence/benchmark-home'),'--no-learn']
work=[('forecast',['weather','forecast','--lat','35.01167','--lon','135.76806','--hours','6','--days','3','--agent','--metrics']),('inventory',['season','search','--product','koyo','--area','kyoto','--query','嵐山','--agent','--metrics']),('nationwide',['season','search','--product','sakura','--query','清水','--limit','5','--agent','--metrics'])]
rows=[]
for name,args in work:
 for mode,extra in [('uncached',['--no-cache']),('populate',['--refresh']),('cached',[])]:
  start=time.monotonic();r=subprocess.run(['/usr/bin/time','-l']+base+args+extra,capture_output=True,text=True,timeout=100);elapsed=time.monotonic()-start
  (out/f'{name}-{mode}.json').write_text(r.stdout);(out/f'{name}-{mode}.stderr').write_text(r.stderr)
  metric=[json.loads(x) for x in r.stderr.splitlines() if x.startswith('{')][0]
  peak=re.search(r'(\d+)\s+maximum resident set size',r.stderr)
  rows.append({'case':name,'mode':mode,'exit':r.returncode,'output_bytes':len(r.stdout.encode()),'wall_ms':round(elapsed*1000),'peak_memory_bytes':int(peak[1]) if peak else None,**metric})
(out/'summary.json').write_text(json.dumps(rows,ensure_ascii=False,indent=2));print(json.dumps(rows,indent=2))
assert all(r['exit']==0 for r in rows)
assert all(r['requests']==0 for r in rows if r['mode']=='cached')
