import sys, os, re
from curl_cffi import requests
s = requests.Session(impersonate="chrome")
pages = {
 "daily1":"https://www.mufap.com.pk/Industry/IndustryStatDaily?tab=1",
 "funddir":"https://www.mufap.com.pk/FundProfile/FundDirectory",
 "pkrv":"https://www.mufap.com.pk/WebRegulations/Index?Head=Pricing&title=PKRV/PKISRV/PKFRV",
 "netsales":"https://www.mufap.com.pk/Industry/WebMonthlyNetSales",
}
for k,u in pages.items():
    try:
        r = s.get(u, timeout=45)
        fn = f"p_{k}.html"
        open(fn,"wb").write(r.content)
        # crude table/count probe
        ntab = len(re.findall(r'<table', r.text, re.I))
        ntr  = len(re.findall(r'<tr', r.text, re.I))
        print(f"{k:10s} status={r.status_code} len={len(r.content):7d} tables={ntab} rows={ntr} -> {fn}")
    except Exception as e:
        print(f"{k:10s} ERR {type(e).__name__} {e}")
