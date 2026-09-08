import re,html,collections
from curl_cffi import requests
s=requests.Session(impersonate="chrome")
HDR=['Sector','Category','FundName','Rating','Benchmark','ValidityDate','NAV','YTD','MTD','D1','D15','D30','D90','D180','D270','D365','Y2','Y3']
def parse(t):
    out=[]
    for r in re.findall(r'<tr[^>]*>(.*?)</tr>',t,re.S|re.I):
        c=[html.unescape(re.sub('<[^>]+>','',x)).replace('\xa0',' ').strip()
           for x in re.findall(r'<td[^>]*>(.*?)</td>',r,re.S|re.I)]
        if len(c)>=18: out.append(dict(zip(HDR,c[:18])))
    return out
for label,d in [("Thu 2026-09-03","2026-09-03"),("Fri 2026-09-04","2026-09-04"),
                ("Sat 2026-09-05","2026-09-05"),("Sun 2026-09-06","2026-09-06"),
                ("Wed 2025-06-18","2025-06-18")]:
    u=f"https://www.mufap.com.pk/Industry/IndustryStatDaily?tab=1&AMCId=0&fundId=0&datefrom={d}&datetill={d}"
    r=s.get(u,timeout=60); rows=parse(r.text)
    dc=collections.Counter(x['ValidityDate'] for x in rows)
    match=sum(n for v,n in dc.items() if v)
    print(f"{label}: rows={len(rows):4d} distinct_validity={len(dc)}")
    for v,n in dc.most_common(4): print(f"      {v}: {n}")
