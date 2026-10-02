import datetime,json,os,pathlib,subprocess,time
from zoneinfo import ZoneInfo
proof=pathlib.Path(os.environ['PROOFS_DIR'])/'live-domain'
proof.mkdir(exist_ok=True)
binary=str(pathlib.Path(os.environ['CLI_WORK_DIR'])/'build/stage/bin/haneda-airport-pp-cli')
now=datetime.datetime.now(ZoneInfo('Asia/Tokyo'))
date=now.strftime('%Y-%m-%d')
env=dict(os.environ,HANEDA_AIRPORT_HOME=str(proof/'isolated-home'),HANEDA_AIRPORT_NO_LEARN='true')
records=[]
def run(name,args,expected=0):
 start=time.monotonic()
 r=subprocess.run([binary,*args,'--json'],capture_output=True,text=True,env=env,timeout=40)
 (proof/(name+'.json')).write_text(r.stdout)
 (proof/(name+'.stderr')).write_text(r.stderr)
 d=json.loads(r.stdout) if r.stdout else {}
 d=d.get('results',d)
 records.append({'name':name,'args':args,'exit_code':r.returncode,'elapsed_seconds':round(time.monotonic()-start,3),'output_bytes':len(r.stdout.encode()),'budget':d.get('budget',d.get('flight_board',{}).get('budget'))})
 assert r.returncode==expected,(name,r.returncode,r.stderr[:500])
 return d
q=['--kind','all','--direction','both','--date',date,'--limit','5']
b=run('all-four-boards',['flights','search',*q])
assert b['complete_source_scope'] and len(b['sources'])==4 and all(s['source_total']>0 for s in b['sources'])
assert {s['kind']+'/'+s['direction'] for s in b['sources']}=={'domestic/departure','domestic/arrival','international/departure','international/arrival'}
assert b['budget']['request_count']==8 and len(b['flights'])<=5
assert all(f['operating_flight'] is None and f['actual_at'] is None for f in b['flights'])
for name,number in [('detail-primary','NH849'),('detail-marketing','UA8003'),('detail-padded','NH0849')]:
 d=run(name,['flights','detail',number,'--kind','international','--direction','departure','--date',date])
 assert d['total_matches']==1 and d['flights'][0]['source_primary_flight']=='NH849'
 assert d['flights'][0]['actual_at'] is None and d['flights'][0]['operating_flight'] is None
 if number!='NH849':assert d['coverage']['query_mode']=='board_with_local_flight_lookup'
p=run('codeshare-terminal-plan',['plan','UA8003','--kind','international','--direction','departure','--date',date])
assert p['flight_board']['total_matches']==1 and p['terminal_floor_urls']
run('source-disruptions',['flights','disruptions',*q])
r=run('rollover',['flights','rollover','--kind','international','--direction','both','--date',date,'--limit','5'])
for f in r['flights']:
 assert f['service_date']!=date or (f['scheduled_at'] and f['revised_at'] and f['scheduled_at'][:10]!=f['revised_at'][:10])
a=run('japanese-airport',['catalog','airports','--kind','domestic','--query','札幌','--limit','5'])
assert any(x['airport_code']=='CTS' and x['search_value']=='SPK' and '札幌' in x['name_ja'] for x in a['airports'])
run('airline-identifiers',['catalog','airlines','--kind','international','--query','ANA','--limit','5'])
s=run('published-schedules',['schedule','search','--kind','all','--direction','both','--limit','5'])
assert len(s['sources'])==4 and s['total_matches']>0
negative=run('mismatch-is-empty',['flights','search','--kind','international','--flight','NH99999','--date',date,'--limit','5'])
assert negative['flights']==[] and negative['total_matches']==0
run('reject-empty-status',['flights','search','--status','canceled,'],2)
run('reject-board-id-in-schedule',['schedule','search','--flight','hnd:international:departure:'+date.replace('-','')+':NH849'],2)
before=proof/'before.json';after=proof/'after.json'
for name,path in [('save-before',before),('save-after',after)]:
 save=run(name,['snapshot','save','--kind','all','--direction','both','--date',date,'--file',str(path)])
 assert save['flight_groups']>1000
local=run('offline-codeshare',['snapshot','search','--file',str(after),'--flight','UA8003','--limit','5'])
assert local['budget']['request_count']==0 and local['total_matches']==1
cmp=run('offline-material-diff',['snapshot','diff','--before',str(before),'--after',str(after),'--limit','5'])
assert cmp['budget']['request_count']==0 and cmp['baseline_sufficient']
(proof/'summary.json').write_text(json.dumps({'date_jst':date,'observed_at':now.isoformat(),'status':'pass','checks':records,'all_source_totals':b['sources'],'snapshot_changes':cmp['comparison']['total_changes']},indent=2)+'\n')
print(json.dumps({'status':'pass','date_jst':date,'checks':len(records),'source_groups':sum(x['source_total'] for x in b['sources']),'snapshot_changes':cmp['comparison']['total_changes'],'max_elapsed_seconds':max(x['elapsed_seconds'] for x in records),'max_output_bytes':max(x['output_bytes'] for x in records)},indent=2))
