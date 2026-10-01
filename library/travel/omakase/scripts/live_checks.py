"""Read-only live checks; fixture documents are not used by this script."""
import json, os, pathlib, re, subprocess, time
root=pathlib.Path(__file__).resolve().parents[1]
binary=root/'omakase-pp-cli'; home=root/'evidence/live-runtime'; evidence=root/'evidence/live'
evidence.mkdir(exist_ok=True)
rows=[]
cases=[('find', ['restaurants','find','--query','Sugita']),('detail',['restaurants','show','hc778124']),('release',['release','jv742052']),('availability',['availability','hc778124','--date','2026-11-01','--party','2']),('compare',['compare','hc778124','qt951856']),('membership',['membership']),('page2',['restaurants','find','--page','2','--limit','3']),('ja',['courses','hc778124','--lang','ja']),('filters',['filters'])]
for name,args in cases:
 for state in ['cold','warm']:
  command=[str(binary),*args,'--no-learn','--home',str(home)]
  if state=='cold':command+=['--refresh']
  started=time.monotonic();p=subprocess.run(['/usr/bin/time','-l',*command],capture_output=True,text=True);elapsed=time.monotonic()-started
  (evidence/f'{name}-{state}.json').write_text(p.stdout)
  (evidence/f'{name}-{state}.stderr').write_text(p.stderr)
  assert p.returncode==0,(name,state,p.returncode,p.stderr)
  d=json.loads(p.stdout);m=d['meta'];r=d['results']
  if name=='find': assert len(r)==1 and r[0]['id']=='fa131638' and 'Sugita' in r[0]['name'] and m['source_total']>1 and m['partial']
  if name=='detail':assert r['name_ja']=='東麻布 天本' and r['courses'][0]['price']['amount']==52800 and r['courses'][0]['price']['minimum'] and r['service_charge']['percent']==10 and r['reservation_fee']['amount']==390 and r['cancellation'][-1]['percent']==100 and r['release']['state']=='undetermined'
  if name=='release':assert r['release']['next_round_at']=='2026-10-01T12:00:00+09:00' and r['seat_state']=='unknown'
  if name=='availability':assert r['state']=='unknown' and r['seats'] is None and r['query_evaluated'] is False and r['access']=='login_required' and r['party']==2
  if name=='compare':assert len(r)==2 and {x['id'] for x in r}=={'hc778124','qt951856'} and m['errors']==[]
  if name=='membership':assert r['green_monthly_jpy']==4980 and r['green_annual_jpy']==49980 and r['account_eligible'] is None and r['gold_price_jpy'] is None
  if name=='page2':assert len(r)==3 and m['source_page']==2 and m['next_offset']==3
  if name=='ja':assert r['name_ja']=='東麻布 天本' and r['reservation_fee']['amount']==390
  if name=='filters':assert any(x['value']=='kanto' for x in r['area']) and any(x['value']=='sushi' for x in r['cuisine'])
  if state=='warm':assert m['requests']==0 and m['cache_hits']>=1,(name,m)
  rss=re.search(r'(\d+)\s+maximum resident set size',p.stderr)
  rows.append(dict(case=name,state=state,output_bytes=len(p.stdout.encode()),requests=m['requests'],cache_hits=m['cache_hits'],latency_ms=round(elapsed*1000,1),peak_rss_bytes=int(rss[1]) if rss else None,response_bytes=m['response_bytes']))
# Explicit inventory, projections, offline miss, partial failure, invalid date and unknown projection.
for name,args in [('inventory-refresh',['inventory','refresh','--pages','2']),('inventory-find',['inventory','find','--query','Konno']),('inventory-status',['inventory','status']),('projection',['compare','hc778124','qt951856','--select','results.id,results.name']),('offline',['courses','hc778124','--offline'])]:
 p=subprocess.run([str(binary),*args,'--home',str(home),'--no-learn'],capture_output=True,text=True);assert p.returncode==0,(name,p.stderr);d=json.loads(p.stdout);(evidence/f'{name}.json').write_text(p.stdout)
 if name=='inventory-refresh':assert d['results']['count']>32 and d['meta']['requests']==2 and not d['results']['complete']
 if name=='inventory-find':assert all('konno' in x['name'].lower() for x in d['results']) and d['meta']['requests']==0
 if name=='projection':assert set(d)=={'results'} and all(set(x)=={'id','name'} for x in d['results'])
for name,args,expected in [('bad-date',['availability','hc778124','--date','2026-02-30','--party','2'],2),('bad-id',['courses','../users'],2),('missing',['courses','zz000000'],3),('offline-miss',['courses','zz000000','--offline'],5)]:
 p=subprocess.run([str(binary),*args,'--home',str(home),'--no-learn'],capture_output=True,text=True);assert p.returncode==expected,(name,p.returncode,p.stderr);(evidence/f'{name}.stderr').write_text(p.stderr)
report={'observed_at':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()),'live':True,'provider':'OMAKASE public pages','fixture_inputs':False,'metrics':rows,'checks_passed':len(rows)+9}
(root/'evidence/metrics.json').write_text(json.dumps(report,indent=2))
print(json.dumps({'live_checks_passed':report['checks_passed'],'metrics_rows':len(rows)}))
