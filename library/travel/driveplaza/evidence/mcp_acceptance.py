#!/usr/bin/env python3
import os,json,subprocess,pathlib,selectors,time
root=pathlib.Path(__file__).resolve().parent.parent
p=subprocess.Popen([str(root/'driveplaza-pp-mcp')],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
selector=selectors.DefaultSelector();selector.register(p.stdout,selectors.EVENT_READ);buffer=b''
def send(obj):p.stdin.write((json.dumps(obj)+'\n').encode());p.stdin.flush()
def call(id,method,params):
 global buffer
 send({'jsonrpc':'2.0','id':id,'method':method,'params':params});end=time.monotonic()+40
 while time.monotonic()<end:
  while b'\n' in buffer:
   line,buffer=buffer.split(b'\n',1);obj=json.loads(line)
   if obj.get('id')==id:return obj
  if selector.select(1):buffer+=os.read(p.stdout.fileno(),65536)
 raise AssertionError('MCP response timeout')
checks=[];responses={}
try:
 call(1,'initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'readonly-acceptance','version':'1'}})
 send({'jsonrpc':'2.0','method':'notifications/initialized'})
 for id,name in [(2,'context'),(3,'reference_rest_form'),(4,'reference_route_form'),(5,'reference_schedule')]:
  response=call(id,'tools/call',{'name':name,'arguments':{}});assert 'result' in response and not response['result'].get('isError'),response
  text=response['result']['content'][0]['text'];v=json.loads(text);responses[name]=v
  if name=='context':
   assert 'cursor' not in json.dumps(v['query_tips']) and 'maximum 30' in json.dumps(v['query_tips'])
   assert any(f['cli_command']=='sapa list' and f['command']=='sapa_list' and f['mcp_tool']=='sapa_list' for f in v['command_mirror_capabilities'])
  elif name!='reference_schedule':
   data=v.get('results',v);assert data['title'] and data['canonical_url'].startswith('https://en.driveplaza.com') and isinstance(data['links'],list)
   assert '<html' not in text.lower() and '<script' not in text.lower()
  else:
   assert 'construction-regulation' in text and 'drivetraffic.jp/lane' in text
  checks.append({'tool':name,'status':'pass','output_bytes':len(text.encode())})
finally:
 p.terminate();p.wait(timeout=5)
(root/'evidence/mcp-acceptance.json').write_text(json.dumps({'status':'pass','checks':checks,'responses':responses},indent=2,ensure_ascii=False)+'\n')
print(json.dumps({'status':'pass','checks':len(checks),'max_output_bytes':max(c['output_bytes'] for c in checks)}))
