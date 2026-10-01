#!/usr/bin/env python3
import json,subprocess,time,re,datetime,sys
from pathlib import Path
root=Path(__file__).resolve().parents[1];out=root/'evidence/live';out.mkdir(parents=True,exist_ok=True)
base=[str(root/'weathernews-pp-cli'),'--home',str(root/'evidence/runtime-home'),'--no-learn']
date=datetime.datetime.now(datetime.timezone(datetime.timedelta(hours=9))).date().isoformat()
cases=[
 ('places-kyoto',['places','resolve','--query','京都','--limit','3','--metrics']),
 ('places-mountain',['places','resolve','--query','高尾山','--limit','3','--metrics']),
 ('forecast',['weather','forecast','--lat','35.01167','--lon','135.76806','--hours','6','--days','3','--metrics']),
 ('forecast-outside',['weather','forecast','--lat','35.01167','--lon','135.76806','--date','2030-01-01','--metrics']),
 ('koyo-search',['season','search','--product','koyo','--area','kyoto','--query','嵐山','--limit','5','--metrics']),
 ('sakura-search',['season','search','--product','sakura','--query','清水','--limit','5','--metrics']),
 ('koyo-show',['season','show','--product','koyo','--id','26102','--metrics']),
 ('sakura-ended',['season','show','--product','sakura','--id','384','--metrics']),
 ('weather-compare',['weather','compare','--point','Kyoto=35.01167,135.76806','--point','Tokyo=35.681,139.767','--date',date,'--max-pop','40','--max-temp','30','--metrics']),
 ('season-compare',['season','compare','--product','koyo','--ids','26101,26111','--date','2026-11-26','--max-days-from-peak','3','--metrics']),
 ('ended-compare',['season','compare','--product','sakura','--ids','384,301','--date','2026-04-01','--max-days-from-peak','3','--metrics']),
 ('projection',['season','search','--product','koyo','--area','kyoto','--query','嵐山','--agent','--select','items.id,items.name_ja,season','--metrics']),
]
summary=[]
for name,args in cases:
 t=time.monotonic();r=subprocess.run(base+args,capture_output=True,text=True,timeout=100);secs=time.monotonic()-t
 (out/(name+'.json')).write_text(r.stdout);(out/(name+'.stderr')).write_text(r.stderr)
 try: d=json.loads(r.stdout);valid=True
 except Exception: d={};valid=False
 summary.append({'case':name,'args':args,'exit':r.returncode,'json':valid,'bytes':len(r.stdout.encode()),'latency_ms':round(secs*1000),'stderr':r.stderr[-700:]})
 print(name,r.returncode,len(r.stdout.encode()),d.get('status',d.get('total','')),flush=True)
(out/'summary.json').write_text(json.dumps(summary,ensure_ascii=False,indent=2))
if any(x['exit']!=0 or not x['json'] for x in summary):sys.exit(1)
