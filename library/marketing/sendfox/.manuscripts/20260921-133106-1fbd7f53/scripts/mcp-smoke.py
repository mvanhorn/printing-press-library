import os,pathlib,json,subprocess,selectors
r=pathlib.Path(os.environ['API_RUN_DIR']);w=pathlib.Path(os.environ['CLI_WORK_DIR']);p=subprocess.Popen([str(r/'scripts/safe-run.sh'),str(w/'sendfox-pp-mcp')],cwd=w,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,text=True,bufsize=1)
selector=selectors.DefaultSelector();selector.register(p.stdout,selectors.EVENT_READ)
def ask(i,method,params):
 p.stdin.write(json.dumps({'jsonrpc':'2.0','id':i,'method':method,'params':params})+'\n');p.stdin.flush()
 while True:
  if not selector.select(20):raise RuntimeError('MCP response timeout')
  line=p.stdout.readline()
  if not line:raise RuntimeError('MCP exited')
  response=json.loads(line)
  if response.get('id')==i:return response
try:
 init=ask(1,'initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'local-verifier','version':'1'}})
 p.stdin.write(json.dumps({'jsonrpc':'2.0','method':'notifications/initialized'})+'\n');p.stdin.flush()
 listing=ask(2,'tools/list',{});tools=listing['result']['tools'];names=[x['name'] for x in tools]
 assert all(x in names for x in ['sendfox_search','sendfox_get','sendfox_execute','campaign_performance']),names
 lookup=ask(3,'tools/call',{'name':'sendfox_get','arguments':{'endpoint_id':'campaigns.create'}})
 search=ask(4,'tools/call',{'name':'sendfox_search','arguments':{'query':'campaign performance','limit':3}})
 workflow=[n for n in names if 'campaign_preflight' in n][0]
 local=ask(5,'tools/call',{'name':workflow,'arguments':{'input':str(w/'examples/snapshot.json')}})
 result={'tool_count':len(tools),'tools':tools,'schema_lookup':lookup,'search':search,'local_workflow':local}
 (r/'proofs/mcp-smoke.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({'tool_count':len(tools),'names':names,'schema_error':lookup.get('result',{}).get('isError',False),'local_error':local.get('result',{}).get('isError',False)},indent=2))
 assert not lookup['result'].get('isError') and not local['result'].get('isError')
finally:
 p.terminate();p.wait(timeout=10)
