import re,html,hashlib
from curl_cffi import requests
s=requests.Session(impersonate="chrome")
def cells(t):
    out=[]
    for r in re.findall(r'<tr[^>]*>(.*?)</tr>',t,re.S|re.I):
        c=[html.unescape(re.sub('<[^>]+>','',x)).strip() for x in re.findall(r'<td[^>]*>(.*?)</td>',r,re.S|re.I)]
        c=[x for x in c if x!='']
        if c: out.append(c)
    return out
print("### MONTHLY AUM with correct params (datefrom/datetill) ###")
for d in ["2026-07-31","2026-06-30","2025-12-31","2022-06-30","2018-06-30"]:
    u=f"https://www.mufap.com.pk/Industry/IndustryStatMonthly?tab=1&AMCId=0&fundId=0&datefrom={d}&datetill={d}"
    r=s.get(u,timeout=60); rr=cells(r.text)
    vals=[x[5] for x in rr if len(x)>5]
    num=[v for v in vals if re.match(r'^[\d,]+\.?\d*$',v.replace(',',''))]
    npub=sum(1 for v in vals if 'Not Published' in v)
    h=hashlib.sha256("".join(num).encode()).hexdigest()[:12]
    tot=sum(float(v.replace(',','')) for v in num) if num else 0
    print(f"  {d}: rows={len(rr):4d} numeric={len(num):4d} notpub={npub:4d} hash={h} sumPKRmn={tot:,.0f}")
    print(f"        sample={num[:5]}")
