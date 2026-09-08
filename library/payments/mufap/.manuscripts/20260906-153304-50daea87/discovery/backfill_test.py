import re,html,hashlib
from curl_cffi import requests
s=requests.Session(impersonate="chrome")
def parse(t):
    rows=re.findall(r'<tr[^>]*>(.*?)</tr>',t,re.S|re.I)
    out=[]
    for r in rows:
        c=[html.unescape(re.sub('<[^>]+>','',x)).strip() for x in re.findall(r'<td[^>]*>(.*?)</td>',r,re.S|re.I)]
        c=[x for x in c if x!='']
        if len(c)>=7: out.append(c)
    return out
for d in ["2026-09-04","2025-06-16","2022-03-15","2019-08-14","2016-09-01"]:
    u=f"https://www.mufap.com.pk/Industry/IndustryStatDaily?tab=1&AMCId=0&fundId=0&datefrom={d}&datetill={d}"
    try:
        r=s.get(u,timeout=60)
        rows=parse(r.text)
        # sample: first row fundname/date/nav
        samp = rows[0][2:7] if rows else None
        dates = set(x[5] for x in rows if len(x)>5)
        h=hashlib.sha256("".join(x[6] for x in rows if len(x)>6).encode()).hexdigest()[:12]
        print(f"{d}  status={r.status_code} rows={len(rows):4d} navhash={h}")
        print(f"          sample={samp}")
        print(f"          distinct validity dates ({len(dates)}): {sorted(dates)[:4]}")
    except Exception as e:
        print(d,"ERR",type(e).__name__,e)
