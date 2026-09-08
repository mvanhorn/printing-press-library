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
# Sector/category distribution on tab=1 today
r=s.get("https://www.mufap.com.pk/Industry/IndustryStatDaily?tab=1",timeout=60)
rr=cells(r.text)
sec=collections.Counter(x[0] for x in rr if len(x)>2)
cat=collections.Counter(x[1] for x in rr if len(x)>2)
print("SECTORS:",dict(sec))
print("TOP CATEGORIES:",cat.most_common(14))
# Monthly industry stat
r2=s.get("https://www.mufap.com.pk/Industry/IndustryStatMonthly?tab=1",timeout=60)
th=[html.unescape(re.sub('<[^>]+>','',x)).strip() for x in re.findall(r'<th[^>]*>(.*?)</th>',r2.text,re.S|re.I)]
seen=[];[seen.append(x) for x in th if x not in seen]
print("MONTHLY HEADERS:",seen[:18])
m2=cells(r2.text)
print("monthly rows:",len(m2))
for x in m2[:4]: print("   ",x[:9])
for m in re.findall(r'<(?:input|select)[^>]*>',r2.text,re.I)[:8]:
    if re.search(r'name=|id=',m,re.I): print("   ctl:",m[:160])
