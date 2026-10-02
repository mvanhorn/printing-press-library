import json,urllib.request,urllib.error,http.cookiejar,gzip,zlib,time,datetime
from pathlib import Path
run=Path(Path('/private/tmp/traveloka-run-path').read_text()); private=Path('/private/tmp/traveloka-private-session')
requests=json.loads((private/'requests.json').read_text());cookies=json.loads((private/'cookies.json').read_text());jar=http.cookiejar.CookieJar()
for c in cookies:
 if c['domain'].lstrip('.') not in ['traveloka.com','www.traveloka.com']:continue
 jar.set_cookie(http.cookiejar.Cookie(0,c['name'],c['value'],None,False,c['domain'],True,c['domain'].startswith('.'),c.get('path','/'),True,c.get('secure',True),None,True,None,None,{},False))
opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar)); results=[]
def sanitize(v):
 if isinstance(v,dict):return {k:sanitize(x) for k,x in v.items() if not any(t in k.lower() for t in ['token','cookie','auth','password','secret','sentinel','usercontext','sessionid','apikey','traceparent','x-did','tv-mcc-id','marketingcontextcapsule','inventoryratekey'])}
 if isinstance(v,list):return [sanitize(x) for x in v]
 return v
def call(rec,body=None):
 payload=json.loads(rec['body']) if body is None else body
 headers={k:v for k,v in rec['headers'].items() if k.lower() not in ['cookie','content-length','host']}
 req=urllib.request.Request(rec['url'],data=json.dumps(payload).encode(),headers=headers,method=rec['method'])
 try:
  with opener.open(req,timeout=25) as r:status=r.status; response=r.read(6000000);ctype=r.headers.get('Content-Type','');enc=r.headers.get('Content-Encoding','')
 except urllib.error.HTTPError as r:status=r.code;response=r.read(6000000);ctype=r.headers.get('Content-Type','');enc=r.headers.get('Content-Encoding','')
 if response.startswith(b'\x1f\x8b'):response=gzip.decompress(response)
 elif enc=='deflate':response=zlib.decompress(response)
 try: data=json.loads(response)
 except ValueError:data={'non_json':response.decode(errors='replace')[:200]}
 inventory=data.get('data',{}).get('searchResults',[]) if isinstance(data.get('data'),dict) else []
 result={'method':rec['method'],'url':rec['url'],'observed_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'status':status,'content_type':ctype,'bytes':len(response),'request':sanitize(payload),'response':sanitize(data),'inventory_count':len(inventory),'hotel_entry_count':len(data.get('data',{}).get('entries',[])) if isinstance(data.get('data'),dict) else 0,'cookies_used':True,'sentinel_used':'sentinel' in payload}
 if isinstance(result['response'].get('data'),dict) and len(inventory)>3:result['response']['data']['searchResults']=result['response']['data']['searchResults'][:3];result['sample_truncated']=True
 results.append(result); print(json.dumps({k:result[k] for k in ['url','status','bytes','inventory_count','hotel_entry_count','sentinel_used']}));
 if status>=400: print('Upstream error:',json.dumps(sanitize(data))[:650])
 return data,status
rec=next(x for x in reversed(requests) if '/hotel/searchList' in x['url']); rec=dict(rec);rec['headers']=dict(rec['headers']);rec['headers'].update({'Origin':'https://www.traveloka.com','Sec-Fetch-Site':'same-origin','Sec-Fetch-Mode':'cors','Sec-Fetch-Dest':'empty','Accept':'application/json, text/plain, */*'});data,status=call(rec)
(run/'discovery/standalone-multiple-party-hotel-replay.json').write_text(json.dumps({'live':True,'transport':'python-stdlib-http','results':results},indent=2)+'\n')
