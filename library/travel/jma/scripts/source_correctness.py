#!/usr/bin/env python3
"""Compare normalized live outputs with their captured first-party HTTP payloads."""
import pathlib,json,hashlib,datetime
root=pathlib.Path(__file__).resolve().parents[1]
def payload(path):
 u='https://www.jma.go.jp/bosai'+path
 p=root/'evidence/cache/http'/ (hashlib.sha256(u.encode()).hexdigest()+'.json')
 x=json.loads(p.read_text());return x['body'],u,x['fetched_at']
checks=[]
def check(name,ok,detail):
 checks.append({'check':name,'pass':bool(ok),'evidence':detail})
 if not ok:raise AssertionError(name+': '+str(detail))
f=json.loads((root/'evidence/live/01.json').read_text())['results'];raw,url,stamp=payload('/forecast/data/forecast/130000.json')
check('Tokyo district/station relevance',all(x['source_id'] in ['130010','44132'] for x in f['series']),{'source_url':url,'ids':[x['source_id'] for x in f['series']]})
for series in f['series']:
 b=raw[1 if series['kind'].startswith('weekly') else 0]
 check('forecast source issue '+series['kind'],series['issued_at']==b['reportDatetime'],b['reportDatetime'])
 for point in series['points']:
  candidate=[(ts,a) for ts in b['timeSeries'] for a in ts['areas'] if a['area']['code']==series['source_id'] and point['valid_at'] in ts['timeDefines']]
  if 'weather_code' in point:
   matches=[(ts,a) for ts,a in candidate if 'weatherCodes' in a];ts,a=matches[0];n=ts['timeDefines'].index(point['valid_at']);check('weather code '+point['valid_at'],point['weather_code']==a['weatherCodes'][n],a['weatherCodes'][n])
  if 'temperature_c' in point:
   matches=[(ts,a) for ts,a in candidate if 'temps' in a];ts,a=matches[0];n=ts['timeDefines'].index(point['valid_at']);value=a['temps'][n];check('temperature '+point['valid_at'],point['temperature_c']==(float(value) if value else None),value)
w=json.loads((root/'evidence/live/02.json').read_text())['results'];raw,url,stamp=payload('/warning/data/r8/130000.json');rows={p['dataTypeCode']:p for p in raw};area=w['municipalities'][0]
for event in area['events']:
 p=rows[event['product_id']];items=[x for x in p['warning']['class20Items'] if x['areaCode']==area['area_id']];kinds=items[0]['kinds'];check('warning hazard/status '+event['product_id'],any(x.get('code')==event['hazard_id'] and x['status']==event['status_ja'] for x in kinds),{'url':url,'hazard_id':event['hazard_id'],'status_ja':event['status_ja']});check('warning independent issue '+event['product_id'],event['issued_at']==p['reportDatetime'],p['reportDatetime'])
check('active and lifted are distinct',w['state']=='active' and any(x['lifecycle']=='lifted' for x in area['events']) and any(x['in_effect'] is True for x in area['events']),area['events'])
k=json.loads((root/'evidence/live/12.json').read_text())['results'];check('inland source applicability',k['state']=='none_reported' and k['municipalities'][0]['not_applicable_product_ids']==['VPWW59'],{'source_url':'https://www.jma.go.jp/bosai/warning/const/no_wave_tide.json','area_id':'1920100'})
f=json.loads((root/'evidence/live/11.json').read_text())['results'];check('north Izu broader weekly region',set(x['source_id'] for x in f['series'])=={'130100','44263'},{'source_url':'https://www.jma.go.jp/bosai/forecast/const/week_area05.json','series_ids':[x['source_id'] for x in f['series']]})
t=json.loads((root/'evidence/live/06.json').read_text())['results'];geometry,url,_=payload('/typhoon/data/TC2633/forecast.json');specs,surl,_=payload('/typhoon/data/TC2633/specifications.json');index={x['validtime']['JST']:x for x in specs[1:]};gindex={x['validtime']['JST']:x for x in geometry[1:]}
check('coherent cyclone issue',t['issued_at']==geometry[0]['issue']['JST']==specs[0]['issue']['JST'],{'geometry':url,'specifications':surl})
for point in t['points']:
 s=index[point['valid_at']];g=gindex[point['valid_at']];check('cyclone position '+point['valid_at'],[point['position']['latitude_deg'],point['position']['longitude_deg']]==g['center']==s['position']['deg'],g['center']);check('cyclone pressure hPa '+point['valid_at'],point['central_pressure_hpa']==float(s['pressure']),s['pressure'])
 if point['type']=='forecast':check('uncertainty radius units '+point['valid_at'],point['probability_circle_radius_m']==g['probabilityCircle']['radius'] and point['probability_circle_radius_km']==s['probabilityCircleRadius']['km'] and point['probability_circle_center_probability_pct']==70,{'geometry_m':g['probabilityCircle']['radius'],'specification_km':s['probabilityCircleRadius']['km']})
check('default cyclone detail omits historical track',all('analysis_track_untimed_lat_lon_deg' not in x for x in t['points']),{'case':'evidence/live/06.json'})
(root/'evidence/source-correctness.json').write_text(json.dumps({'verified_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'method':'Real live command outputs compared to captured JMA JSON; no fixture substitution','checks':checks},ensure_ascii=False,indent=2));print(str(len(checks))+' source-correctness checks passed')
