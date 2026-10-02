#!/usr/bin/env python3
import subprocess,json,pathlib,time,csv,io
root=pathlib.Path(__file__).resolve().parent.parent
binary=root/'driveplaza-pp-cli';results=[]
def call(name,args,success=0):
 start=time.monotonic();p=subprocess.run([str(binary),*args,'--no-learn'],capture_output=True,text=True,timeout=35)
 assert p.returncode==success,(name,p.returncode,p.stderr)
 results.append({'name':name,'exit':p.returncode,'elapsed_ms':round(1000*(time.monotonic()-start)),'stdout_bytes':len(p.stdout.encode())})
 return p
base=['route','--from','nerima','--to','sendai-minami','--at','2026-10-10T08:00']
for vehicle in ['medium','large','extra-large']:
 p=call('vehicle-'+vehicle,[*base,'--vehicle',vehicle,'--agent']);v=json.loads(p.stdout)
 assert v['results']['assumptions']['vehicle']==vehicle and v['results']['alternatives'][0]['standard_jpy']>8490
 (root/'evidence/live'/('route-'+vehicle+'.json')).write_text(p.stdout)
 p_record=results[-1];p_record['upstream_requests']=v['meta']['upstream_requests'];p_record['standard_jpy']=v['results']['alternatives'][0]['standard_jpy']
p=call('five-waypoints',[*base,'--via','tokorozawa,kawagoe,higashi-matsuyama,hanazono,honjo-kodama','--agent'])
v=json.loads(p.stdout);assert len(v['results']['assumptions']['via'])==5 and v['results']['alternatives'];results[-1]['upstream_requests']=v['meta']['upstream_requests'];(root/'evidence/live/route-waypoints.json').write_text(p.stdout)
for name,args in [('empty-stops-csv',['sapa','list','--road','1040','--query','no-match-zz999','--csv']),('empty-notices-csv',['notices','--query','no-match-zz999','--csv'])]:
 p=call(name,args);assert list(csv.reader(io.StringIO(p.stdout)))==[] or not any(row for row in csv.DictReader(io.StringIO(p.stdout)))
p=call('timeout',['route','--from','nerima','--to','sendai-minami','--at','2026-10-10T08:00','--timeout','1ms','--agent'],5)
assert results[-1]['elapsed_ms']<1000 and 'deadline' in p.stderr.lower()
p=call('scan-cap-empty',['sapa','list','--road','1040','--max-scan-records','1','--query','no-match-zz999','--agent'])
v=json.loads(p.stdout);assert v['results']['scan_limited'] and v['results']['scanned_items']==1 and v['results']['items']==[] and '--max-scan-records' in v['results']['note'];results[-1]['upstream_requests']=v['meta']['upstream_requests']
(root/'evidence/live/variants.json').write_text(json.dumps({'status':'pass','checks':results},indent=2)+'\n')
print(json.dumps({'status':'pass','checks':len(results),'vehicle_prices':{r['name']:r.get('standard_jpy') for r in results if 'standard_jpy' in r}}))
