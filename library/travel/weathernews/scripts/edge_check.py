#!/usr/bin/env python3
import json,subprocess
from pathlib import Path
root=Path(__file__).resolve().parents[1];out=root/'evidence/edges';out.mkdir(parents=True,exist_ok=True)
base=[str(root/'weathernews-pp-cli'),'--home',str(root/'evidence/runtime-home'),'--no-learn']
cases=[('limit',['season','search','--product','koyo','--area','kyoto','--limit','51'],2),('bad-date',['weather','forecast','--lat','35.01','--lon','135.7','--date','2026-02-30'],2),('nan',['weather','forecast','--lat','NaN','--lon','135.7'],2),('no-criteria',['weather','compare','--point','A=35,135','--point','B=36,136','--date','2026-10-01'],2),('bad-point',['weather','compare','--point','A=35x,135','--point','B=36,136','--date','2026-10-01','--max-pop','40'],2),('duplicate',['season','compare','--product','koyo','--ids','26101,26101','--date','2026-11-26','--max-days-from-peak','3'],2),('wrong-product',['season','show','--product','winter','--id','1'],2),('cached-local',['season','show','--product','koyo','--id','26102','--data-source','local','--metrics'],0),('no-local',['season','show','--product','koyo','--id','99999999','--data-source','local','--metrics'],3),('projection-miss',['season','show','--product','koyo','--id','26102','--select','invented_field'],2),('page1',['season','search','--product','koyo','--area','kyoto','--limit','2','--offset','0','--data-source','local'],0),('page2',['season','search','--product','koyo','--area','kyoto','--limit','2','--offset','2','--data-source','local'],0),('empty',['season','search','--product','koyo','--area','kyoto','--query','存在しない地点名ZZZ','--data-source','local'],0)]
rows=[];results={}
for name,args,want in cases:
 r=subprocess.run(base+args,capture_output=True,text=True,timeout=15);(out/(name+'.stdout')).write_text(r.stdout);(out/(name+'.stderr')).write_text(r.stderr)
 assert r.returncode==want,(name,r.returncode,r.stderr);rows.append({'case':name,'expected_exit':want,'exit':r.returncode,'stdout_bytes':len(r.stdout.encode())})
 if r.returncode==0:results[name]=json.loads(r.stdout)
assert not ({x['id'] for x in results['page1']['items']} & {x['id'] for x in results['page2']['items']})
assert results['page1']['next_offset']==2 and results['empty']['total']==0
assert results['cached-local']['source']['cache_hit'] is True
(out/'summary.json').write_text(json.dumps({'passed':True,'cases':rows},indent=2));print(len(rows),'edge and cache/pagination checks passed')
