import os,pathlib,json,yaml
w=pathlib.Path(os.environ['CLI_WORK_DIR']); source=yaml.safe_load((pathlib.Path(os.environ['RESEARCH_DIR'])/'sendfox-enriched.yaml').read_text()); manifest=json.loads((w/'tools-manifest.json').read_text())
def resolve(x):
 if isinstance(x,list):return [resolve(y) for y in x]
 if not isinstance(x,dict):return x
 if '$ref' in x:
  v=source
  for part in x['$ref'][2:].split('/'):v=v[part]
  return resolve(v)
 r={k:resolve(v) for k,v in x.items() if k!='nullable'}
 if x.get('nullable') and 'type' in r:r['type']=[r['type'],'null']
 return r
ops=[]
for t in manifest['tools']:
 op=source['paths'][t['path']][t['method'].lower()]
 # Registry IDs use resource.endpoint, tool names use resource_endpoint.
 resource=source['paths'][t['path']][t['method'].lower()]['x-pp-resource'].replace('-','_')
 endpoint=t['name'][len(resource)+1:]
 params=resolve(source['paths'][t['path']].get('parameters',[])+op.get('parameters',[]))
 for p in params:
  p['wire_name']=p.get('x-url-name',p['name'])
 body=resolve(op.get('requestBody',{}).get('content',{}).get('application/json',{}).get('schema',{}))
 if body.get('type')=='object':body['additionalProperties']=False
 responses={code:resolve(resp) for code,resp in op.get('responses',{}).items()}
 ops.append(dict(id=resource+'.'+endpoint,method=t['method'],path=t['path'],summary=op.get('summary',''),description=op.get('description',''),parameters=params,body_schema=body,responses=responses))
(w/'internal/contract/operations.json').write_text(json.dumps(ops,indent=2)+'\n')
print('Embedded complete request/response contract for',len(ops),'operations')
