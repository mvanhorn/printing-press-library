import json, datetime, urllib.parse, os
from pathlib import Path
run=Path(Path('/private/tmp/traveloka-run-path').read_text()); private=Path('/private/tmp/traveloka-private-session');private.mkdir(mode=0o700,exist_ok=True)
secret_fields={'sentinel','usercontext','authorization','proxy-authorization','cookie','set-cookie','t-a-v','tv-clientsessionid','tv-mcc-id','x-did','traceparent','clientSessionId','marketingContextCapsule','inventoryRateKey','ga_client_id','ga_session_id','amplitude_device_id','amplitude_session_id','fb_browser_id_fbp','rtbh_lid'}
def clean(v):
 if isinstance(v,dict):return {k:clean(x) for k,x in v.items() if k.lower() not in {s.lower() for s in secret_fields} and not any(t in k.lower() for t in ['token','password','secret','sessionid','apikey'])}
 if isinstance(v,list):return [clean(x) for x in v]
 return v
def cleanbody(s):
 try:return json.dumps(clean(json.loads(s)),separators=(',',':'))
 except Exception:return '[non-JSON response excluded]'
entries=[];replays=[]
for rid,rec in capture_requests.items():
 req=rec['request'];
 if not (urllib.parse.urlparse(req['url']).path.startswith('/api/v2/flight/') or urllib.parse.urlparse(req['url']).path.startswith('/api/v2/airport/') or urllib.parse.urlparse(req['url']).path in ['/api/v1/hotel/autocomplete','/api/v2/hotel/autocomplete/features','/api/v2/hotel/searchList','/api/v2/hotel/search/rooms']):continue
 res=capture_responses.get(rid,{}); response=res.get('response',{}); rawbody=res.get('body','')
 headers={k:v for k,v in req.get('headers',{}).items() if k.lower() not in secret_fields and not any(t in k.lower() for t in ['token','session','auth','key'])}
 entries.append({'method':req.get('method'),'url':req['url'],'request_headers':headers,'response_headers':{'content-type':response.get('mimeType','')},'request_body':cleanbody(req.get('postData','{}')),'response_body':cleanbody(rawbody) if rawbody else '', 'response_status':int(response.get('status',0)),'response_content_type':response.get('mimeType','')})
 replays.append({'method':req.get('method'),'url':req['url'],'headers':req.get('headers',{}),'body':req.get('postData','{}')})
file=private/'requests.json'; fd=os.open(file,os.O_WRONLY|os.O_CREAT|os.O_TRUNC,0o600)
with os.fdopen(fd,'w') as f:json.dump(replays,f)
out={'target_url':'https://www.traveloka.com/en-sg','captured_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'interaction_rounds':1,'auth':{'type':'browser-clearance-session','site':'traveloka.com'},'entries':entries}
(run/'discovery/browser-sniff-capture.json').write_text(json.dumps(out,indent=2)+'\n')
summary=[{'method':e['method'],'path':urllib.parse.urlparse(e['url']).path,'status':e['response_status'],'body_bytes':len(e['response_body'])} for e in entries]
print(json.dumps({'requests':len(entries),'endpoints':list({x['path'] for x in summary}),'last':summary[-12:]}))
