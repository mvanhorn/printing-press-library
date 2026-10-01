import re,json,sys
from pathlib import Path
for p in map(Path,sys.argv[1:]):
 s=p.read_text();m=re.search(r'<script[^>]*id="__NUXT_DATA__"[^>]*>(.*?)</script>',s,re.S)
 if not m:print(p.name,'no payload');continue
 a=json.loads(m.group(1));memo={}
 def d(i):
  if not isinstance(i,int):return i
  if i<0:return None
  if i in memo:return memo[i]
  v=a[i]
  if isinstance(v,dict):
   out={};memo[i]=out;out.update({k:d(x) for k,x in v.items()});return out
  if isinstance(v,list):
   if v and isinstance(v[0],str):return d(v[1]) if len(v)>1 else []
   out=[];memo[i]=out;out.extend(d(x) for x in v);return out
  return v
 data=d(0)['data'];p.with_suffix('.decoded.json').write_text(json.dumps(data,ensure_ascii=False))
 print(p.name)
 for k,v in data.items():
  print(k,list(v) if isinstance(v,dict) else type(v).__name__)
  if isinstance(v,dict):
   for j,x in v.items():
    if j not in ['data','listData']:continue
    print(j, (list(x) if isinstance(x,dict) else ('len',len(x)) if isinstance(x,list) else str(x)))
    print(str(x)[:1000])
