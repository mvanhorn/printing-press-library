import re,html,statistics
from curl_cffi import requests
s=requests.Session(impersonate="chrome")
HDR=['Sector','Category','FundName','Rating','Benchmark','ValidityDate','NAV','YTD','MTD','D1','D15','D30','D90','D180','D270','D365','Y2','Y3']
def parse(t):
    out=[]
    for r in re.findall(r'<tr[^>]*>(.*?)</tr>',t,re.S|re.I):
        c=[html.unescape(re.sub('<[^>]+>','',x)).replace('\xa0',' ').strip()
           for x in re.findall(r'<td[^>]*>(.*?)</td>',r,re.S|re.I)]
        if len(c)>=18:
            out.append(dict(zip(HDR,c[:18])))
    return out
def num(v):
    v=(v or '').replace(',','').strip()
    try: return float(v)
    except: return None
print(f"{'date':12s} {'MMfunds':>7s} {'med30d':>8s} {'med90d':>8s} {'med365d':>8s}  universe")
for d in ["2020-08-14","2021-08-13","2022-08-12","2023-08-11","2024-08-09","2025-08-15","2026-09-04"]:
    u=f"https://www.mufap.com.pk/Industry/IndustryStatDaily?tab=1&AMCId=0&fundId=0&datefrom={d}&datetill={d}"
    try:
        r=s.get(u,timeout=60); rows=parse(r.text)
        mm=[x for x in rows if x['Category'].startswith('Money Market')]
        def med(k):
            v=[num(x[k]) for x in mm]; v=[y for y in v if y is not None and 0<y<60]
            return statistics.median(v) if v else float('nan')
        print(f"{d:12s} {len(mm):7d} {med('D30'):8.2f} {med('D90'):8.2f} {med('D365'):8.2f}  {len(rows)}")
    except Exception as e:
        print(d,"ERR",type(e).__name__,e)
