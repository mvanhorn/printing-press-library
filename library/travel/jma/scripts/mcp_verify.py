#!/usr/bin/env python3
import subprocess,pathlib,json,select,time,os,datetime,hashlib
root=pathlib.Path(__file__).resolve().parents[1];p=root/'evidence/mcp';p.mkdir(exist_ok=True)
env=os.environ.copy();env['JMA_HOME']=str(root/'evidence/mcp-home')
proc=subprocess.Popen([str(root/'bin/jma-pp-mcp')],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,bufsize=1,env=env)
counter=0
results=[]
def send(method,params=None,notification=False):
 global counter
 counter+=1;req={'jsonrpc':'2.0','method':method}
 if not notification:req['id']=counter
 if params is not None:req['params']=params
 proc.stdin.write(json.dumps(req)+'\n');proc.stdin.flush()
 if notification:return None
 end=time.monotonic()+60
 while time.monotonic()<end:
  ready,_,_=select.select([proc.stdout],[],[],max(0,end-time.monotonic()))
  if not ready:break
  line=proc.stdout.readline()
  if not line:raise RuntimeError('MCP closed stdout')
  response=json.loads(line)
  if response.get('id')==counter:return response
 raise RuntimeError('MCP request timeout '+method)
try:
 send('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'JMA verification','version':'1'}})
 send('notifications/initialized',notification=True)
 listing=send('tools/list');tools=listing['result']['tools'];(p/'tools.json').write_text(json.dumps(tools,indent=2));names={x['name'] for x in tools};assert {'forecast_get','warnings_get','typhoons_get','typhoons_list','areas_resolve','inventory_refresh'}<=names;assert 'sql' not in names and 'search' not in names
 for name,args in [('areas_resolve',{'area':'2610000'}),('warnings_get',{'area':'1340100'}),('typhoons_get',{'id':'TC2633'})]:
  schema=next(x['inputSchema']['properties'] for x in tools if x['name']==name)
  if 'cache-dir' in schema:args['cache-dir']=str(root/'evidence/cache')
  if 'refresh' in schema:args['refresh']=True
  r=send('tools/call',{'name':name,'arguments':args});(p/(name+'.json')).write_text(json.dumps(r,ensure_ascii=False,indent=2));assert 'error' not in r,r
  result=r['result'];assert not result.get('isError'),r;d=json.loads(''.join(x.get('text','') for x in result['content']));assert 'meta' in d and 'results' in d
  if name=='warnings_get':assert d['results']['area']['id']=='1340100' and d['results']['state']=='active'
  if name=='typhoons_get':assert d['meta']['requests']==3 and all('analysis_track_untimed_lat_lon_deg' not in x for x in d['results']['points']) and any(x['type']=='forecast' and x['probability_circle_center_probability_pct']==70 for x in d['results']['points'])
  results.append({'tool':name,'pass':True,'requests':d['meta']['requests']})
 # Deterministic failure-path proof, explicitly synthetic and kept outside live cases.
 synthetic=root/'evidence/mcp-synthetic-cache';(synthetic/'http').mkdir(parents=True,exist_ok=True)
 url='https://www.jma.go.jp/bosai/warning/data/r8/130000.json'
 original=json.loads((root/'evidence/cache/http'/(hashlib.sha256(url.encode()).hexdigest()+'.json')).read_text())['body']
 fixture=json.loads(json.dumps(original))
 target=next(x for x in fixture[0]['warning']['class20Items'] if x['areaCode']=='1340100');target['kinds']=[{'code':'10','status':'unknown synthetic lifecycle'}]
 (synthetic/'http'/(hashlib.sha256(url.encode()).hexdigest()+'.json')).write_text(json.dumps({'fetched_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'body':fixture}))
 r=send('tools/call',{'name':'warnings_get','arguments':{'area':'1340100','cache-dir':str(synthetic),'offline':True,'select':'municipalities.events'}})
 (p/'synthetic-incomplete-projection.json').write_text(json.dumps({'fixture_type':'synthetic mutation of captured JMA JSON; not live source verification','response':r},ensure_ascii=False,indent=2))
 assert r['result']['isError'] is True
 d=json.loads(r['result']['content'][0]['text']);assert d['meta']['provider']=='Japan Meteorological Agency' and 'municipalities' in d['results'] and any(x['lifecycle']=='unknown' for x in d['results']['municipalities'][0]['events'])

finally:
 proc.stdin.close()
 try:proc.wait(timeout=5)
 except subprocess.TimeoutExpired:proc.terminate();proc.wait()
 (p/'stderr.txt').write_text(proc.stderr.read())
(root/'evidence/mcp-verification.json').write_text(json.dumps({'verified_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'tools':len(tools),'cases':results},indent=2));print('Focused MCP:',len(tools),'tools;',len(results),'live calls passed')
# Manifest derived from the real registered tool catalog, not guessed schemas.
(root/'tools-manifest.json').write_text(json.dumps({'api_name':'jma','description':'JMA forecasts, municipality warnings and uncertain cyclone forecasts through the normalized companion CLI.','auth':{'type':'none'},'mcp_ready':'full','surface':'focused companion CLI mirror','tools':tools},indent=2))
