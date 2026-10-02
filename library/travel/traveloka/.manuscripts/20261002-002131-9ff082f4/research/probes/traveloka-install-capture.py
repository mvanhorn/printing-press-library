import urllib.parse, datetime, base64
capture_requests = {}
capture_responses = {}
allowed_prefixes = ['/api/v2/airport/', '/api/v2/flight/search/', '/api/v2/flight/summary', '/api/v2/flight/dateflow', '/api/v2/hotel/', '/api/v1/hotel/autocomplete']
def in_scope(url):
 p=urllib.parse.urlparse(url)
 return p.hostname=='www.traveloka.com' and any(p.path.startswith(x) for x in allowed_prefixes)
def capture_request(params, session_id):
 req=params.get('request', {})
 if in_scope(req.get('url','')):
  capture_requests[params['requestId']]={'request':req,'session_id':session_id,'timestamp':datetime.datetime.now(datetime.timezone.utc).isoformat()}
def capture_response(params, session_id):
 if params.get('requestId') in capture_requests:
  capture_responses[params['requestId']]={'response':params.get('response',{}),'session_id':session_id}
def capture_finished(params, session_id):
 rid=params.get('requestId')
 if rid not in capture_requests:return
 async def get_body():
  try:
   res=await browser._session.cdp_client.send.Network.getResponseBody(params={'requestId':rid},session_id=session_id)
   if res.get('base64Encoded'): res['body']=base64.b64decode(res['body']).decode('utf-8',errors='replace')
   capture_responses.setdefault(rid,{})['body']=res.get('body','')
  except Exception as e: capture_responses.setdefault(rid,{})['body_error']=type(e).__name__
 asyncio.create_task(get_body())
async def install():
 s=await browser._session.get_or_create_cdp_session()
 await s.cdp_client.send.Network.enable(session_id=s.session_id)
 register=browser._session.cdp_client.register
 register.Network.requestWillBeSent(capture_request)
 register.Network.responseReceived(capture_response)
 register.Network.loadingFinished(capture_finished)
browser._run(install())
print('Traveloka-only request/response capture installed; values retained in memory, output suppressed.')
