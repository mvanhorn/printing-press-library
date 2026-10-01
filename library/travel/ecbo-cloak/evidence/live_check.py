#!/usr/bin/env python3
"""Bounded live reads; saves actual stdout, stderr, metrics and independent source comparisons."""
import subprocess,json,time,datetime,pathlib,urllib.request
root=pathlib.Path(__file__).resolve().parents[1]; ev=root/'evidence'; cache=ev/'runtime-cache'; binary=root/'ecbo-cloak-pp-cli'; results=[]

def run(name,args,expect=0):
 command=[str(binary),*args,'--cache-dir',str(cache),'--agent'];out=ev/(name+'.json');err=ev/(name+'.stderr');t=time.monotonic()
 p=subprocess.run(['/usr/bin/time','-l',*command],capture_output=True,text=True,timeout=70)
 out.write_text(p.stdout);err.write_text(p.stderr)
 import re
 rss=re.search(r'(\d+)\s+maximum resident set size',p.stderr)
 record={'case':name,'args':args,'exit_code':p.returncode,'expected_exit':expect,'elapsed_ms':round((time.monotonic()-t)*1000),'stdout_bytes':len(p.stdout.encode()),'peak_rss_bytes':int(rss.group(1)) if rss else None}
 try:d=json.loads(p.stdout);record['requests']=d.get('meta',{}).get('requests');record['cache_hits']=d.get('meta',{}).get('cache_hits')
 except json.JSONDecodeError:d=None
 results.append(record)
 assert p.returncode==expect,(name,p.returncode,p.stderr[:1500])
 return d

def source(url,payload=None):
 req=urllib.request.Request(url,data=json.dumps(payload).encode() if payload else None,headers={'Content-Type':'application/json','LOCALE':'en'})
 with urllib.request.urlopen(req,timeout=20) as r:return json.load(r)

near=['facilities','near','--lat','35.6812','--lon','139.7671','--limit','5']
u=run('near_uncached',near+['--no-cache']);c=run('near_cache_prime',near+['--refresh']);w=run('near_cached',near)
assert w['meta']['requests']==0 and w['meta']['cache_hits']==1
assert len(u['results'])==5 and u['results'][0]['name_ja'].find('JR')>=0
assert all(x['distance_km']<=5 and x['confirmed_available_capacity'] is None for x in u['results'])
project=run('near_projected',near+['--select','id,name,name_ja,booking_url']);assert set(project['results'][0])=={'id','name','name_ja','booking_url'}
page=run('near_next_page',near+['--offset','5']);assert not set(x['id'] for x in u['results'])&set(x['id'] for x in page['results'])
query=run('query_japanese',near+['--query','東京']);assert query['results'] and all('東京' in x['name_ja'] or '東京' in x['name'] for x in query['results'])
run('kyoto_window',['facilities','near','--lat','34.9858','--lon','135.7588','--limit','3','--refresh'])
run('osaka_window',['facilities','near','--lat','34.7025','--lon','135.4959','--limit','3','--refresh'])
run('filtered_window',near+['--from','2026-10-03T10:00','--to','2026-10-03T12:00','--large','1','--refresh'])
id='0c3fb1e9-ad5a-42de-bfda-3027ebe4921e'
du=run('detail_uncached',['facilities','get',id,'--no-cache']);run('detail_cache_prime',['facilities','get',id,'--refresh']);dw=run('detail_cached',['facilities','get',id]);assert dw['meta']['requests']==0 and dw['meta']['cache_hits']==2
jr=run('legacy_jr_detail',['facilities','get','GBy4uBrI','--refresh']);assert jr['id']=='31736291-0f63-437c-8e27-64f698c6d539' and jr['booking_url'].endswith('/spaces/'+jr['id']);assert jr['acceptance_cutoff'] is None and jr['pickup_cutoff'] is None
payload={'space_id':id,'from':'2026-10-03 19:00','to':'2026-10-03 21:00','reservation_items':{'small':1,'large':1}}
base=['offer','inspect',id,'--from','2026-10-03T19:00','--to','2026-10-03T21:00','--small','1','--large','1']
run('offer_uncached',base+['--no-cache'])
o=run('offer_valid',base);p=source('https://api.ecbo.io/api/web/reservations/price',payload);v=source('https://api.ecbo.io/api/web/reservations/validate',payload)
assert o['quote']['price']==p['price']==1300 and o['quote']['currency']==p['currency'] and o['validation']['valid']==v['valid']==True
assert o['confirmed_available_capacity'] is None
run('offer_overnight',['offer','inspect',id,'--from','2026-10-03T23:00','--to','2026-10-04T01:00','--large','1'])
overnight24=run('offer_24h_overnight',['offer','inspect','bYXqIesH','--from','2026-10-03T23:00','--to','2026-10-04T01:00','--large','1']);assert overnight24['quote']['currency']=='JPY'
run('offer_outside_hours',['offer','inspect',id,'--from','2026-10-03T10:00','--to','2026-10-03T12:00','--large','1'])
run('offer_excess_capacity',['offer','inspect',id,'--from','2026-10-03T19:00','--to','2026-10-03T21:00','--large','50'])
refreshed=run('inventory_refresh',['inventory','refresh','--lat','35.6812','--lon','139.7671','--limit','3']);assert len(refreshed['results'])==3 and refreshed['inventory_saved']
inv=run('inventory_offline',['inventory','list','--query','東京','--limit','3']);assert inv['results'] and inv['meta']['requests']==0
run('invalid_time',['offer','inspect',id,'--from','2026-02-30T12:00','--to','2026-03-01T12:00','--large','1'],2)
run('unknown_id',['facilities','get','ffffffff-ffff-ffff-ffff-ffffffffffff'],3)
(ev/'metrics.json').write_text(json.dumps({'observed_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'live':True,'measurements':results},indent=2))
(ev/'live-summary.json').write_text(json.dumps({'status':'pass','cases':len(results),'source_comparisons':{'price':p,'validation':v},'observed_at':datetime.datetime.now(datetime.timezone.utc).isoformat()},indent=2))
print(json.dumps({'status':'pass','cases':len(results),'metrics':'evidence/metrics.json'}))
