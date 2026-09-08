import re,html,collections
from curl_cffi import requests
s=requests.Session(impersonate="chrome")
def cells(t):
    out=[]
    for r in re.findall(r'<tr[^>]*>(.*?)</tr>',t,re.S|re.I):
        c=[html.unescape(re.sub('<[^>]+>','',x)).strip() for x in re.findall(r'<td[^>]*>(.*?)</td>',r,re.S|re.I)]
        c=[x for x in c if x!='']
        if c: out.append(c)
    return out
print("### MONTHLY AUM for past months ###")
for mth in ["2026-06","2025-12","2020-06"]:
    u=f"https://www.mufap.com.pk/Industry/IndustryStatMonthly?tab=1&filterDate={mth}"
    r=s.get(u,timeout=60); rr=cells(r.text)
    vals=[x[5] for x in rr if len(x)>5]
    npub=sum(1 for v in vals if 'Not Published' in v)
    num=[v for v in vals if re.match(r'^[\d,]+\.?\d*$',v)]
    print(f"  {mth}: rows={len(rr)} notpublished={npub} numeric={len(num)} sample={num[:4]}")
print()
print("### DAILY VALIDITY-DATE DISTRIBUTION (today's snapshot, tab=1) ###")
r=s.get("https://www.mufap.com.pk/Industry/IndustryStatDaily?tab=1",timeout=60)
rr=cells(r.text)
# validity date = the cell matching 'Mon DD, YYYY'
dc=collections.Counter()
for x in rr:
    for c in x:
        if re.match(r'^[A-Z][a-z]{2} \d{2}, \d{4}$',c): dc[c]+=1; break
print("  distinct validity dates:",len(dc))
for d,n in dc.most_common(8): print(f"    {d}: {n} funds")
