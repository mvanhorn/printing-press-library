import pathlib,os,re,json,shlex
w=pathlib.Path(os.environ['CLI_WORK_DIR']);ops=json.loads((w/'internal/contract/operations.json').read_text()); by={(x['method'],x['path']):x for x in ops}
def sample(schema,name):
 if 'example' in schema:return schema['example']
 if schema.get('enum'):return schema['enum'][0]
 typ=schema.get('type');typ=typ[0] if isinstance(typ,list) else typ
 if typ=='integer':return max(1,schema.get('minimum',1))
 if typ=='number':return 1
 if typ=='boolean':return True
 if typ=='array':return [sample(schema.get('items',{}),name)]
 if typ=='object':return {k:sample(v,k) for k,v in schema.get('properties',{}).items() if k in schema.get('required',[])}
 if schema.get('format')=='email':return 'reader@example.com'
 if schema.get('format')=='date-time':return '2027-01-01T12:00:00Z'
 if schema.get('format')=='uri':return 'https://example.com/newsletter'
 return {'title':'Weekly newsletter','subject':'A useful idea','html':'<p>Hello reader</p>','from_name':'Example Newsletter','email':'reader@example.com','from_email':'newsletter@example.com'}.get(name,'Newsletter')
for p in (w/'internal/cli').glob('*.go'):
 if p.name.endswith('_test.go'):continue
 text=p.read_text();m=re.search(r'"pp:method": "([A-Z]+)".*?"pp:path": "([^"]+)"',text)
 if not m or (m[1],m[2]) not in by:continue
 op=by[m[1],m[2]];use=re.search(r'Use:\s*"([^"]+)"',text)
 if not use:continue
 resource=op['id'].split('.')[0].replace('_','-');leaf=use[1].split()[0];args=['sendfox-pp-cli',resource,leaf];happy=[]
 for param in op['parameters']:
  if param['in']=='path':args.append('12');happy.append(param['name']+'=12')
  elif param['in']=='query' and param.get('required'):
   val=str(sample(param['schema'],param['name']));args.extend(['--'+param['name'].replace('_','-'),val]);happy.append('--'+param['name'].replace('_','-')+'='+val)
 for key in op['body_schema'].get('required',[]):
  val=sample(op['body_schema']['properties'][key],key);val=json.dumps(val,separators=(',',':')) if isinstance(val,(dict,list)) else str(val).lower() if isinstance(val,bool) else str(val)
  flag=key.replace('_','-');args.extend(['--'+flag,val]);happy.append('--'+flag+'='+val)
 if op['method']!='GET':args+=['--dry-run']
 example='  '+' '.join(shlex.quote(x) for x in args)
 if 'Example:' in text:text=re.sub(r'Example:\s*"(?:[^"\\]|\\.)*",', lambda _: 'Example: '+json.dumps(example)+',', text, count=1)
 else:text=text.replace(use[0],use[0]+',\n\t\tExample: '+json.dumps(example),1).replace(',,' ,',')
 # Preserve original values otherwise, no invented opaque live targets.
 if 'pp:happy-args' not in text and happy:text=text.replace('"pp:endpoint":', '"pp:happy-args": '+json.dumps(';'.join(happy))+', "pp:endpoint":',1)
 p.write_text(text)
p=w/'internal/cli/helpers.go';s=p.read_text()
for fn in ['readSecretFromStdin','handleBinaryResponseDelivery']:
 match=re.search(r'^func '+fn+r'\([\s\S]*?^}\n',s,re.M)
 if match:s=s[:match.start()]+s[match.end():]
p.write_text(s)
