import urllib.request,urllib.parse,http.cookiejar,html.parser,re,json,pathlib,os,datetime
base="https://booking.ferry-sunflower.co.jp"
jar=http.cookiejar.CookieJar(); cli=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
class Forms(html.parser.HTMLParser):
 def __init__(self,s):
  super().__init__(); self.forms=[]; self.current=None;self.feed(s)
 def handle_starttag(self,t,a):
  a=dict(a)
  if t=="form":self.current={"action":a.get("action"),"hidden":{}};self.forms.append(self.current)
  if t=="input" and self.current is not None and a.get("type")=="hidden" and a.get("name"):self.current["hidden"][a["name"]]=a.get("value","")
 def handle_endtag(self,t):
  if t=="form":self.current=None
out=pathlib.Path(os.environ['DISCOVERY_DIR'])
entries=[]
def req(url,data=None):
 u=urllib.parse.urljoin(base,url); d=urllib.parse.urlencode(data).encode() if data is not None else None
 q=urllib.request.Request(u,data=d,headers={"User-Agent":"sunflower-ferry-cli source-contract-check/0.1","Referer":base+"/web/yoyaku/Reserve1030"})
 with cli.open(q,timeout=25) as r: s=r.read(2*1024*1024).decode("utf-8"); status=r.status; final=r.url;ct=r.headers.get('Content-Type','')
 safe=re.sub(r'(<input\b[^>]*\btype="hidden"[^>]*\bvalue=")[^"]*',r'\1REDACTED',s,flags=re.I)
 safe=re.sub(r'(<input\b[^>]*\bvalue=")[^"]*("[^>]*\btype="hidden")',r'\1REDACTED\2',safe,flags=re.I)
 request_data={k:v for k,v in (data or {}).items() if k not in ['__RequestVerificationToken','req_t']}
 entries.append({"method":"POST" if data is not None else "GET","url":u,"request_headers":{},"response_headers":{"Content-Type":ct},"request_body":urllib.parse.urlencode(request_data),"response_body":safe,"response_status":status,"response_content_type":ct,"classification":"","is_noise":False})
 print(json.dumps({"method":entries[-1]['method'],"path":urllib.parse.urlparse(u).path,"status":status,"final_path":urllib.parse.urlparse(final).path,"bytes":len(s),"heading":re.findall(r'<h[12][^>]*>(.*?)</h[12]>',s,re.S)[:2]},ensure_ascii=False))
 return s
s=req('/web/yoyaku/Reserve0000/IndexEnglish')
f=next(f for f in Forms(s).forms if f['action'].endswith('/Reserve'))
s=req(f['action'],f['hidden'])
f=next(f for f in Forms(s).forms if f['action'].endswith('/Reserve1030/MoveNext'))
d=f['hidden'].copy();d.update({'katamichiofuku':'katamichi','Ouro_BoardingDate':'2026/10/15(Thu)','Ouro_Line':'21','JosenNaiyo':'03','Car_Length':'','RiyoNaiyo.Number_Of_Bike_Over750cc':'0','RiyoNaiyo.Number_Of_Bike_Less750cc':'0','RiyoNaiyo.Number_Of_Scooter':'0','RiyoNaiyo.Number_Of_cycle':'0','RiyoNaiyo.Number_Of_Adults':'1','RiyoNaiyo.Number_Of_Children':'0','RiyoNaiyo.Number_Of_Yoji':'0','RiyoNaiyo.Number_Of_Nyuji':'0','Ouro_UseLiner':'false','RiyoNaiyo.Number_Of_PetCage':'0'})
s=req(f['action'],d)
assert 'Total Price' in s and 'SUNFLOWER KURENAI' in s and '14,620' in s,'No matching quote response'
for i,e in enumerate(entries):(out/f'portal-sample-{i}.html').write_text(e['response_body'])
(out/'browser-sniff-capture.json').write_text(json.dumps({'target_url':base+'/web/yoyaku/Reserve0000/IndexEnglish','captured_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'interaction_rounds':3,'auth':{'type':'none'},'entries':entries},indent=2)+'\n')
print('read_only_replay_passed; session values excluded')
