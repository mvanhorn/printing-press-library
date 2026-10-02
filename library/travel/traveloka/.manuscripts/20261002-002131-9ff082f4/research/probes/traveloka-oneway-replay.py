import json,urllib.request,urllib.error,http.cookiejar,gzip,zlib,time,datetime,uuid,decimal
from pathlib import Path
run=Path(Path('/private/tmp/traveloka-run-path').read_text());private=Path('/private/tmp/traveloka-private-session');records=json.loads((private/'requests.json').read_text());jar=http.cookiejar.CookieJar()
for c in json.loads((private/'cookies.json').read_text()):
 if c['domain'].lstrip('.') in ['traveloka.com','www.traveloka.com']:jar.set_cookie(http.cookiejar.Cookie(0,c['name'],c['value'],None,False,c['domain'],True,c['domain'].startswith('.'),c.get('path','/'),True,c.get('secure',True),None,True,None,None,{},False))
opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar));calls=[]
def choose(path):return next(r for r in reversed(records) if r['url'].endswith(path))
def call(path,payload,searchid):
 rec=choose(path);headers={k:v for k,v in rec['headers'].items() if k.lower() not in ['cookie','content-length','host','fpr-search-id']};headers['fpr-search-id']=searchid
 req=urllib.request.Request(rec['url'],data=json.dumps(payload).encode(),headers=headers,method='POST')
 try:
  with opener.open(req,timeout=25) as r:body=r.read(6000000);status=r.status;enc=r.headers.get('Content-Encoding','')
 except urllib.error.HTTPError as r:body=r.read(6000000);status=r.code;enc=r.headers.get('Content-Encoding','')
 if body.startswith(b'\x1f\x8b'):body=gzip.decompress(body)
 elif enc=='deflate':body=zlib.decompress(body)
 try:data=json.loads(body)
 except ValueError:raise RuntimeError('non_json_or_protection_response_status_'+str(status))
 calls.append({'path':path,'status':status,'bytes':len(body),'count':len(data.get('data',{}).get('searchResults',[]))})
 if status!=200 or not isinstance(data.get('data'),dict):raise RuntimeError('upstream_status_'+str(status))
 return data
payload=json.loads(choose('/api/v2/flight/search/initial')['body']);sid=str(uuid.uuid4());payload['data']['searchId']=sid;payload['data']['journeyIndex']=0;payload['data']['selectedFlights']=[];payload['data']['selectedFlightsContext']={}
def collect(p,index):
 inventory={};first=call('/api/v2/flight/search/initial' if index==0 else '/api/v2/flight/search/poll',p,sid)
 def merge(resp):
  for f in resp['data'].get('searchResults',[]):inventory[f['id']]=f
  if isinstance(resp.get('sentinel'),dict):p['sentinel']={**p.get('sentinel',{}),**resp['sentinel']}
 merge(first);response=first
 for _ in range(12):
  if response['data'].get('meta',{}).get('searchCompleted'):break
  time.sleep(1); response=call('/api/v2/flight/search/poll',p,sid);merge(response)
 if not response['data'].get('meta',{}).get('searchCompleted'):raise RuntimeError('polling_incomplete')
 return list(inventory.values()),response
outbounds,resp=collect(payload,0)
if not outbounds:raise RuntimeError('fresh_one_way_search_empty')
fare=min(outbounds,key=lambda f:int(f.get('fare',{}).get('display',{}).get('currencyValue',{}).get('amount') or '999999999'))
pair=json.loads(choose('/api/v2/flight/search/redirection')['body']);pair['data'].update({'journeyIds':[fare['id']],'searchId':sid,'isPrefetch':True,'journeyContextData':{fare['id']:{'isBaggageFilterEnabled':False}}});pair.pop('prefetchCacheKey',None)
if 'sentinel' in payload:pair['sentinel']=payload['sentinel']
time.sleep(1);details=call('/api/v2/flight/search/redirection',pair,sid)['data']
legs=[{k:s.get(k) for k in ['departureAirport','arrivalAirport','flightNumber','airlineCode','operatingAirlineCode','seatClass','departureDate','arrivalDate','departureTime','arrivalTime','tzDepartureMinuteOffset','tzArrivalMinuteOffset','durationMinutes']} for r in fare.get('connectingFlightRoutes',[]) for s in r.get('segments',[])]
proof={'live':True,'transport':'python-stdlib-http','observed_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'context':{'market':'SG','locale':'en_SG','currency':'SGD','journeys':payload['data']['journeys'],'passengers':payload['data']['numSeats'],'trip_type':payload['data']['tripType']},'inventory_count':len(outbounds),'legs':legs,'source_total_price':details.get('totalPrice'),'source_per_passenger_price':details.get('displayedPricePerPax'),'calls':calls}
(run/'discovery/standalone-one-way-workflow.json').write_text(json.dumps(proof,indent=2)+'\n');print(json.dumps(proof))
