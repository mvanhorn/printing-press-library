"""Representative live CLI output/latency/RSS measurements on macOS."""
import json,pathlib,re,subprocess,time
root=pathlib.Path(__file__).resolve().parent.parent;binary=root/'bin/pocket-concierge-pp-cli';cache=root/'.cache/measure';rows=[]
cases={'sushi-five':['restaurants','search','--cuisine-id','2','--limit','5'],'detail':['restaurants','get','--id','245672'],'courses':['courses','list','--id','245672'],'slots':['availability','slots','--id','245672','--date','2026-10-05','--party','2','--cache-availability']}
for name,args in cases.items():
 for mode,flags in [('uncached',['--no-cache']),('prime',['--refresh']),('cached',[])]:
  start=time.monotonic();p=subprocess.run(['/usr/bin/time','-l',str(binary),*args,'--cache-dir',str(cache),*flags],capture_output=True,text=True,timeout=125);assert p.returncode==0,(name,p.stderr);j=json.loads(p.stdout);match=re.search(r'(\d+)\s+maximum resident set size',p.stderr);assert match
  rows.append({'scenario':name,'mode':mode,'output_bytes':len(p.stdout.encode()),'requests':j['meta']['requests'],'cache_hits':j['meta']['cache_hits'],'source_response_bytes':j['meta']['response_bytes'],'wall_latency_ms':round((time.monotonic()-start)*1000),'peak_rss_bytes':int(match.group(1))})
(root/'evidence/performance.json').write_text(json.dumps({'platform':'macOS arm64','live':True,'rows':rows},indent=2)+'\n')
print(json.dumps(rows))
