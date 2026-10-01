#!/usr/bin/env python3
"""Live read-only source assertions, with isolated cache and bounded calls."""
import datetime,json,pathlib,re,subprocess,sys,time,urllib.request
ROOT=pathlib.Path(__file__).resolve().parent.parent
OUT=ROOT/'evidence'/'live';OUT.mkdir(parents=True,exist_ok=True)
BIN=ROOT/'bin'/'pocket-concierge-pp-cli';CACHE=ROOT/'.cache'/'live'
rows=[]
def call(label,*args,code=0):
 started=time.monotonic();p=subprocess.run([str(BIN),*args,'--cache-dir',str(CACHE)],capture_output=True,text=True,timeout=125)
 (OUT/(label+'.stdout.json')).write_text(p.stdout);(OUT/(label+'.stderr.txt')).write_text(p.stderr)
 assert p.returncode==code,(label,p.returncode,p.stderr)
 j=json.loads(p.stdout) if code==0 else json.loads(p.stderr.splitlines()[-1])
 rows.append({'label':label,'command':list(args),'exit':p.returncode,'bytes':len(p.stdout.encode()),'elapsed_ms':round((time.monotonic()-started)*1000),'requests':j.get('meta',{}).get('requests',0)})
 return j

def source(query,variables=None,lang='en'):
 req=urllib.request.Request('https://www.pocket-concierge.jp/graphql',data=json.dumps({'query':query,'variables':variables}).encode(),headers={'Content-Type':'application/json','Accept-Language':lang,'X-Auth-Origin':'GUEST_ORIGIN'})
 with urllib.request.urlopen(req,timeout=20) as r:j=json.load(r)
 assert not j.get('errors'),j.get('errors')
 return j['data']
try:
 j=call('filters','filters','--refresh');assert any(x['id']=='19' and x['name']=='Kyoto/Osaka/Nara' and x['name_ja'] for x in j['areas'])
 j=call('name-search','restaurants','search','--query','Murase','--limit','3','--refresh');assert j['items'] and all('murase' in x['name'].lower() for x in j['items']);assert any(x['id']=='244725' for x in j['items'])
 j=call('mismatch-search','restaurants','search','--query','zzzzpocketnonexistent987654','--limit','3','--no-cache');assert j['items']==[]
 j=call('filters-search','restaurants','search','--area-id','19','--cuisine-id','1','--service','DINNER','--max-price','30000','--limit','3','--refresh');assert j['items'];assert all(x['area']['id']=='19' and any(c['id']=='1' for c in x['cuisines']) for x in j['items'])
 p1=call('pagination-1','restaurants','search','--limit','2','--page','1','--refresh');p2=call('pagination-2','restaurants','search','--limit','2','--page','2','--refresh');assert {x['id'] for x in p1['items']}.isdisjoint({x['id'] for x in p2['items']});assert p1['pagination']['next_page']==2
 j=call('detail','restaurants','get','--id','245672','--refresh');v=j['restaurant'];assert v['name_ja']=='ますます増田';assert v['conditions']['children'] and any('12' in x['text'] for x in v['conditions']['children']);assert any('1,000' in x['text'] for x in v['fee_statements'])
 j=call('courses','courses','list','--id','245672','--refresh');course=next(x for x in j['items'] if x['id']=='182402');assert course['price']['currency']=='JPY' and course['price']['per_guest']==21000 and course['price']['all_in_total'] is None;assert any('included' in x['text'] for x in course['fee_statements']);assert j['restaurant_fee_statements']
 direct=source('query { venue(id:"245672") { id name realTimeBooking courses { id name costPerGuest fixedPrice } availabilityCalendar { reservationDates waitlistDates } } }')['venue'];(OUT/'source-comparison.json').write_text(json.dumps(direct,ensure_ascii=False,indent=2)+'\n');dc=next(x for x in direct['courses'] if x['id']=='182402');assert dc['costPerGuest']==course['price']['per_guest'] and dc['fixedPrice']==course['price']['fixed_per_group']
 ja=call('japanese','restaurants','get','--id','245672','--lang','ja','--refresh');assert ja['restaurant']['name']==v['name_ja'] and ja['meta']['requests']==1
 dates=call('dates','availability','dates','--id','245672','--limit','10','--no-cache');assert dates['items'];assert all(x['party_eligible'] is None for x in dates['items'])
 date=next(x for x in direct['availabilityCalendar']['reservationDates'] if x>=datetime.datetime.now(datetime.timezone(datetime.timedelta(hours=9))).strftime('%Y-%m-%d'))
 slots=call('sessions','availability','slots','--id','245672','--date',date,'--party','2','--no-cache');assert slots['items'];slot=next(x for x in slots['items'] if x['session_id'] and x['course_id']=='182402');assert slot['status']=='instant_confirmation' and slot['min_party_size']<=2<=slot['max_party_size'];assert slot['start_time'].startswith(date)
 raw=source('query Q($d:ISO8601Date!) { availabilitySearch(venueId:"245672",date:$d) { __typename ... on ReservableAvailability {id startTime minPartySize maxPartySize course{id}} ... on WaitlistableAvailability {startTime course{id}} } }',{'d':date})['availabilitySearch'];assert any(x.get('id')==slot['session_id'] and x['course']['id']==slot['course_id'] and x['startTime']==slot['start_time'] for x in raw);(OUT/'source-sessions.json').write_text(json.dumps(raw,ensure_ascii=False,indent=2)+'\n')
 if direct['availabilityCalendar']['waitlistDates']:
  wd=direct['availabilityCalendar']['waitlistDates'][0];wait=call('waitlist','availability','slots','--id','245672','--date',wd,'--party','2','--no-cache');assert wait['items'] and all(x['status']=='waitlist' and x['session_id'] is None for x in wait['items'])
 req=call('request-mode','availability','dates','--id','245323','--limit','3','--no-cache');assert req['restaurant']['booking_mode']=='reservation_request'
 empty=call('unavailable-party','availability','slots','--id','245672','--date',date,'--party','20','--no-cache');assert empty['items']==[]
 j=call('date-search','restaurants','search','--date',date,'--party','2','--instant','--limit','3','--no-cache');assert all(x['booking_mode']=='instant_confirmation' for x in j['items'])
 hand=call('handoff','booking','handoff','--id','245672','--course-id','182402','--session-id',slot['session_id'],'--date',date,'--party','2','--no-cache');assert hand['session']['session_id']==slot['session_id'] and hand['url']=='https://www.pocket-concierge.jp/en/restaurants/245672' and hand['reservation_created'] is False
 call('wrong-course','booking','handoff','--id','245672','--course-id','1','--no-cache',code=3)
 call('invalid-date','availability','slots','--id','245672','--date','2026-02-29','--no-cache',code=2)
 call('missing-id','restaurants','get','--json',code=2)
 projection=call('projection','restaurants','search','--query','Murase','--limit','3','--select','items.id,items.name_ja');assert set(projection)=={'items','meta'} and set(projection['items'][0])=={'id','name_ja'}
 # Cached versus freshly retrieved measurements use process peak resident memory, not Go heap estimates.
 metrics=[]
 for label,flags in [('uncached',['--no-cache']),('prime',['--refresh']),('cached',[])]:
  command=[str(BIN),'restaurants','search','--query','Murase','--limit','3','--cache-dir',str(CACHE),*flags]
  started=time.monotonic();p=subprocess.run(['/usr/bin/time','-l',*command],capture_output=True,text=True,timeout=125);assert p.returncode==0,p.stderr;j=json.loads(p.stdout);m=re.search(r'(\d+)\s+maximum resident set size',p.stderr);assert m,p.stderr
  metrics.append({'case':label,'output_bytes':len(p.stdout.encode()),'source_requests':j['meta']['requests'],'source_response_bytes':j['meta']['response_bytes'],'cache_hits':j['meta']['cache_hits'],'wall_latency_ms':round((time.monotonic()-started)*1000),'peak_rss_bytes':int(m.group(1)),'source_elapsed_ms':j['meta']['elapsed_ms']})
 assert metrics[-1]['source_requests']==0 and metrics[-1]['cache_hits']==2
 (OUT/'performance.json').write_text(json.dumps(metrics,indent=2)+'\n')
 report={'status':'PASS','verified_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'provider':'Pocket Concierge','fixture_substitution':False,'rows':rows,'assertions':'query relevance, negative query, source filter IDs, pagination, Japanese identity, fee conflict, course/session source identity, live date/party, waitlist/request/instant distinctions, canonical handoff, ownership errors, projection, cached/uncached metrics','selected_date':date,'selected_session_id':slot['session_id']}
 (OUT/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'status':'PASS','rows':len(rows),'selected_date':date,'metrics':metrics}))
except Exception as e:
 (OUT/'report.json').write_text(json.dumps({'status':'FAIL','error':str(e),'rows':rows},indent=2)+'\n');raise
