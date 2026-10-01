#!/usr/bin/env python3
"""Independent comparisons with actual cached HTTP source bodies from live commands."""
import json,base64,hashlib,re
from pathlib import Path
root=Path(__file__).resolve().parents[1];live=root/'evidence/live'
# Runtime paths are discoverable; exact entry hash is from the public URL.
cache=list((root/'evidence/runtime-home').rglob('public-http'))[0]
def source(d):
 s=d['source'];p=cache/(hashlib.sha256(s['source_url'].encode()).hexdigest()+'.json');e=json.loads(p.read_text());assert e['url']==s['source_url'];return base64.b64decode(e['body']).decode()
def nuxt(raw):
 a=json.loads(re.search(r'<script[^>]*id="__NUXT_DATA__"[^>]*>(.*?)</script>',raw,re.S)[1]);memo={}
 def d(i):
  if i<0:return None
  if i in memo:return memo[i]
  v=a[i]
  if isinstance(v,dict):out={k:d(x) for k,x in v.items()}
  elif isinstance(v,list):out=(d(v[1]) if len(v)>1 else []) if v and isinstance(v[0],str) else [d(x) for x in v]
  else:out=v
  memo[i]=out;return out
 return d(0)['data']
checks=[]
f=json.loads((live/'forecast.json').read_text());raw=json.loads(source(f))
assert f['location']['name_ja']==raw['observation']['LNAME']
assert f['hourly'][0]['temperature_c']==raw['srf'][0]['AIRTMP']
assert f['hourly'][0]['precipitation_mm']==raw['srf'][0]['PREC']
assert f['hourly'][0]['wind_speed_m_s']==raw['srf'][0]['WNDSPD']
assert f['daily'][0]['precipitation_probability_percent']==raw['mrf'][0]['POP']
assert f['issued_at'] is None and f['observation']['kind']=='observation' and f['daily'][0]['kind']=='forecast'
checks.append('Kyoto hourly/daily numeric values and location match actual HTTP source; issue/valid/observation separation preserved')
for case in ['koyo-show','sakura-ended']:
 d=json.loads((live/(case+'.json')).read_text());raw=source(d);a=nuxt(raw)
 details=[v for v in a.values() if isinstance(v,dict) and str(v.get('spotid'))==d['location']['id']][0]
 assert d['location']['name_ja']==details.get('name',details.get('spotname'))
 assert d['normal']['period_ja']==details.get('reinennomigoro',details.get('reinen'))
 assert d['normal']['kind']=='historical_norm' and d['location']['elevation_m'] is None
 if case=='koyo-show':
  report=[v['data'] for v in a.values() if isinstance(v,dict) and isinstance(v.get('data'),dict) and 'rankText' in v['data']][0]
  assert d['report']['status_ja']==report['rankText'] and d['report']['source_date_ja']==report['obs']
  peak=[v for v in d['predictions'] if v['event']=='peak'][0];assert peak['source_date_ja']==report['migoro'] and peak['valid_date']=='2026-11-26'
 else:
  assert '2026年の桜開花情報の更新は終了しました' in raw and d['season']['updates_ended'] is True
  assert d['report']['status_ja']=='葉桜' and d['report']['valid_date']=='2026-04-15' and all(x['status']=='season_ended' for x in d['predictions'])
 checks.append(case+': Japanese source identity, report/date, normal and seasonal status agree with live source body')
d=json.loads((live/'koyo-search.json').read_text());assert d['total']==2 and d['inventory_total']==93
assert all('嵐山' in x['name_ja'] for x in d['items']);checks.append('Kyoto query relevance: both results contain 嵐山; bounded summary reads no details')
d=json.loads((live/'sakura-search.json').read_text());assert d['total']>=5
inventory=nuxt(source(d));rows=[]
for v in inventory.values():
 if isinstance(v,dict) and isinstance(v.get('data'),dict) and isinstance(v['data'].get('pointlist'),list):rows=v['data']['pointlist']
by_id={str(r.get('lcid')):r for r in rows}
for x in d['items']:
 r=by_id[x['id']];assert x['name_ja']==r['name'];assert any('清水' in str(r.get(k,'')) for k in ['name','name_kana','cityname','addr','pref_en'])
checks.append('Nationwide sakura relevance independently verified against each matching source name/kana/city/address/prefecture')
d=json.loads((live/'forecast-outside.json').read_text());assert d['status']=='out_of_horizon' and d['daily']==[] and d['hourly']==[]
d=json.loads((live/'ended-compare.json').read_text());assert all(x['status']=='unknown' and x['check']['reason']=='season_ended' for x in d['items'])
checks.append('Out-of-horizon weather and ended-season comparisons remain explicit unknowns')
d=json.loads((live/'projection.json').read_text());assert set(d['results'])=={'items','season'} and all(set(x)=={'id','name_ja'} for x in d['results']['items']);checks.append('Projection returns only requested fields with agent metadata')
(live/'source-correctness.json').write_text(json.dumps({'checks':checks,'passed':True,'basis':'actual HTTP cache bodies captured by live matrix; independent Python decoder; no fixtures'},ensure_ascii=False,indent=2))
print(len(checks),'source agreement/relevance checks passed')
