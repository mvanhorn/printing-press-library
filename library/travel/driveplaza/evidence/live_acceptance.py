#!/usr/bin/env python3
"""Read-only behavioral acceptance; real source requests, never fixtures."""
import json,subprocess,pathlib,time,re,sys
root=pathlib.Path(__file__).resolve().parent.parent
outdir=root/'evidence'/'live'
checks=[]
def run(name,args):
 started=time.monotonic()
 p=subprocess.run(['/usr/bin/time','-l',str(root/'driveplaza-pp-cli'),*args,'--agent','--no-learn'],capture_output=True,text=True,timeout=70)
 (outdir/(name+'.json')).write_text(p.stdout)
 (outdir/(name+'.stderr')).write_text(p.stderr)
 if p.returncode: raise AssertionError(f'{name}: exit{p.returncode}: {p.stderr[:600]}')
 value=json.loads(p.stdout)
 meta=value['meta']; result=value['results']
 rss=re.search(r'(\d+)\s+maximum resident set size',p.stderr)
 checks.append({'name':name,'status':'fetched','args':args,'output_bytes':len(p.stdout.encode()),'latency_ms':round(1000*(time.monotonic()-started)),'upstream_requests':meta.get('upstream_requests',0),'peak_rss_bytes':int(rss.group(1)) if rss else None,'warnings':meta.get('warnings',[])})
 return result
try:
 ic=run('interchanges',['interchanges','--query','nerima','--language','en','--limit','3'])
 assert ic['items'][0]['id']=='1800001' and ic['items'][0]['name_ja']=='練馬'
 road=run('roads',['roads','--query','1040'])
 assert len(road['items'])==1 and '東北' in road['items'][0]['name_ja']
 cond=run('conditions',['conditions']); assert cond['vehicle_classes']['standard']=='1' and cond['currency']=='JPY'
 base=['route','--from','nerima','--to','sendai-minami','--at','2026-10-10T08:00']
 route=run('route-standard',[*base,'--payment','etc','--detail'])
 assert len(route['alternatives'])==3 and route['assumptions']['vehicle']=='standard'
 r=route['alternatives'][0]
 assert r['standard_jpy']==8490 and r['distance_km']==345.2 and r['ignoring_traffic_minutes']==213
 assert r['selected_jpy']==r['etc_jpy'] and r['considering_traffic_minutes']>=r['ignoring_traffic_minutes']
 assert r['directional_stops'] and r['forecast_urls'] and r['warnings']
 light=run('route-light',[*base,'--vehicle','light'])
 assert light['assumptions']['vehicle']=='light' and light['alternatives'][0]['standard_jpy']<r['standard_jpy']
 arrival=run('route-arrival',[*base,'--time-kind','arrival','--priority','toll','--payment','etc2'])
 assert arrival['assumptions']['time_kind']=='arrival' and all(a['selected_jpy']==a['etc2_jpy'] for a in arrival['alternatives'])
 stops=run('sapa-hasuda',['sapa','list','--road','1040','--query','hasuda'])
 assert len(stops['items'])==2 and {s['direction'] for s in stops['items']}=={'up','down'}
 assert all('蓮田' in s['name_ja'] for s in stops['items'])
 up=next(s for s in stops['items'] if s['direction']=='up'); down=next(s for s in stops['items'] if s['direction']=='down')
 assert up['parking_large_spaces']==132 and up['facility_categories']['pets'] is True and down['facility_categories']['pets'] is False
 ev=run('sapa-ev',['sapa','list','--road','1040','--direction','up','--facility','9010','--limit','3'])
 assert ev['items'] and all(s['direction']=='up' and s['facility_categories']['ev_charging'] is True for s in ev['items'])
 empty=run('sapa-empty',['sapa','list','--road','1040','--query','no-match-zz999'])
 assert empty['items']==[] and empty['total']==0 and empty['scanned_items']>0 and empty['note']
 detail=run('sapa-detail',['sapa','detail','--id','1040/1040021/1'])
 assert detail['name_en']=='HASUDA-SA' and '蓮田' in detail['name_ja'] and detail['road_name']=='Tohoku Expwy'
 assert any(s['category']=='Gas station' and '24' in s['source_text'] for s in detail['facility_sections'])
 notices=run('notices',['notices','--limit','5'])
 assert notices['items'] and notices['feed_updated_at'] and all(n['active_restriction'] is None and n['published_at'] for n in notices['items'])
 empty_notices=run('notices-empty',['notices','--query','no-match-zz999'])
 assert empty_notices['items']==[] and empty_notices['note']
 selected=run('selection',['sapa','list','--road','1040','--query','hasuda','--select','items.id,items.name_ja,items.direction,items.url'])
 assert len(selected['items'])==2 and set(selected['items'][0])=={'id','name_ja','direction','url'}
 handoff=run('handoff',['handoff'])
 assert len(handoff)==8 and any(h['purpose']=='east_etc_lanes' and h['url'].endswith('/lane/') for h in handoff)
 for name,args in [('invalid-date',['route','--from','nerima','--to','sendai-minami','--at','2026-02-30T08:00']),('invalid-minute',['route','--from','nerima','--to','sendai-minami','--at','2026-10-10T08:05']),('invalid-facility',['sapa','list','--road','1040','--facility','invalid'])]:
  p=subprocess.run([str(root/'driveplaza-pp-cli'),*args,'--agent','--no-learn'],capture_output=True,text=True,timeout=30)
  assert p.returncode==2, (name,p.returncode,p.stderr)
  checks.append({'name':name,'status':'pass','expected_exit':2,'actual_exit':p.returncode})
except Exception as e:
 for c in checks: c['status']='pass'
 if checks: checks[-1]['status']='fail'
 checks.append({'status':'fail','reason':str(e)})
 (outdir/'acceptance.json').write_text(json.dumps({'status':'fail','checks':checks},indent=2,ensure_ascii=False)+'\n')
 print(json.dumps({'status':'fail','checks':len(checks),'error':str(e)},ensure_ascii=False));sys.exit(1)
for c in checks: c['status']='pass'
(outdir/'acceptance.json').write_text(json.dumps({'status':'pass','checks':checks},indent=2,ensure_ascii=False)+'\n')
print(json.dumps({'status':'pass','checks':len(checks),'requests':sum(c.get('upstream_requests',0) for c in checks),'max_output_bytes':max(c.get('output_bytes',0) for c in checks)},ensure_ascii=False))
