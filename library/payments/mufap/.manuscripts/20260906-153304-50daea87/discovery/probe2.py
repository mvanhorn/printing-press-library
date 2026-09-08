import re,html
from curl_cffi import requests
s=requests.Session(impersonate="chrome")
def rows(t):
    out=[]
    for r in re.findall(r'<tr[^>]*>(.*?)</tr>',t,re.S|re.I):
        c=[html.unescape(re.sub('<[^>]+>','',x)).strip() for x in re.findall(r'<td[^>]*>(.*?)</td>',r,re.S|re.I)]
        c=[x for x in c if x!='']
        if len(c)>=7: out.append(c)
    return out
print("### EARLIEST-DATE PROBE (tab=1) ###")
for d in ["2014-06-10","2010-06-10","2008-06-10","2005-06-10"]:
    u=f"https://www.mufap.com.pk/Industry/IndustryStatDaily?tab=1&AMCId=0&fundId=0&datefrom={d}&datetill={d}"
    r=s.get(u,timeout=60); rr=rows(r.text)
    print(f"  {d} status={r.status_code} rows={len(rr)} sample={rr[0][2:7] if rr else None}")
print("### TABS ###")
for tab in [1,2,3,4,5]:
    u=f"https://www.mufap.com.pk/Industry/IndustryStatDaily?tab={tab}"
    r=s.get(u,timeout=60)
    th=[html.unescape(re.sub('<[^>]+>','',x)).strip() for x in re.findall(r'<th[^>]*>(.*?)</th>',r.text,re.S|re.I)]
    # dedupe preserving order
    seen=[];[seen.append(x) for x in th if x not in seen]
    rr=rows(r.text)
    print(f"  tab={tab} status={r.status_code} rows={len(rr)} headers={seen[:14]}")
