import sys, os
from curl_cffi import requests
url = sys.argv[1]
out = sys.argv[2] if len(sys.argv) > 2 else None
try:
    r = requests.get(url, impersonate="chrome", timeout=45)
    print(f"STATUS={r.status_code} LEN={len(r.content)} CT={r.headers.get('content-type')}")
    if out:
        with open(out, "wb") as f:
            f.write(r.content)
        print(f"WROTE={out}")
    else:
        print(r.text[:2000])
except Exception as e:
    print("ERR", type(e).__name__, e)
