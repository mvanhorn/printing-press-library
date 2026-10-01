#!/usr/bin/env python3
import json,subprocess
from pathlib import Path
root=Path(__file__).resolve().parents[1];base=[str(root/'weathernews-pp-cli'),'--home',str(root/'evidence/workflow-home'),'--no-learn'];steps=[]
def run(name,args):
 p=subprocess.run(base+args+['--metrics'],capture_output=True,text=True,timeout=60);assert p.returncode==0,(name,p.stderr);d=json.loads(p.stdout);m=[json.loads(x) for x in p.stderr.splitlines() if x.startswith('{')][0];steps.append({'step':name,'args':args,'exit':p.returncode,'metrics':m,'output':d});return d
s=run('discover-source-spot',['season','search','--product','koyo','--area','kyoto','--query','宝筐','--limit','1']);assert s['items'] and '宝筐' in s['items'][0]['name_ja']
id=s['items'][0]['id'];d=run('lazy-detail',['season','show','--product','koyo','--id',id]);assert d['location']['id']==id and d['normal']['kind']=='historical_norm'
loc=d['location'];f=run('weather-at-discovered-coordinate',['weather','forecast','--lat',str(loc['latitude']),'--lon',str(loc['longitude']),'--hours','3','--days','2']);assert f['status']=='available' and f['hourly'] and f['daily'] and f['location']['elevation_m'] is None
assert f['hourly'][0]['kind']=='forecast' and f['daily'][0]['kind']=='forecast' and f['issued_at'] is None
out=root/'evidence/live/workflow.json';out.write_text(json.dumps({'passed':True,'basis':'fresh real source IDs and coordinates passed between read-only commands','steps':steps},ensure_ascii=False,indent=2));print('live discovery -> lazy detail -> coordinate forecast passed')
