#!/usr/bin/env python3
"""Assemble a HAR from browser-use eval captures of LoopNet pages."""
import json, os, sys

D = os.path.dirname(os.path.abspath(__file__))

PAGES = [
    ("search-sale.raw",  "https://www.loopnet.com/search/office/los-angeles-ca/for-sale/"),
    ("search-lease.raw", "https://www.loopnet.com/search/office/los-angeles-ca/for-lease/"),
    ("detail.raw",       "https://www.loopnet.com/Listing/2035-W-15th-St-Long-Beach-CA/38523625/"),
]

def load_html(fname):
    with open(os.path.join(D, fname), "r", encoding="utf-8", errors="replace") as f:
        raw = f.read()
    marker = "result: "
    idx = raw.find(marker)
    if idx == -1:
        sys.exit(f"no 'result: ' marker in {fname}")
    return raw[idx + len(marker):].strip()

entries = []
for fname, url in PAGES:
    html = load_html(fname)
    entries.append({
        "startedDateTime": "2026-05-20T11:42:00.000Z",
        "time": 800,
        "request": {
            "method": "GET", "url": url, "httpVersion": "HTTP/2",
            "headers": [
                {"name": "user-agent", "value": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"},
                {"name": "accept", "value": "text/html,application/xhtml+xml"},
            ],
            "queryString": [], "cookies": [], "headersSize": -1, "bodySize": 0,
        },
        "response": {
            "status": 200, "statusText": "OK", "httpVersion": "HTTP/2",
            "headers": [{"name": "content-type", "value": "text/html; charset=utf-8"}],
            "cookies": [], "redirectURL": "", "headersSize": -1, "bodySize": len(html),
            "content": {"size": len(html), "mimeType": "text/html", "text": html},
        },
        "cache": {},
        "timings": {"send": 0, "wait": 800, "receive": 0},
    })

har = {"log": {
    "version": "1.2",
    "creator": {"name": "printing-press-browser-sniff-manual", "version": "1.0"},
    "entries": entries,
}}

out = os.path.join(D, "loopnet-capture.har")
with open(out, "w", encoding="utf-8") as f:
    json.dump(har, f)
print(f"wrote {out} ({os.path.getsize(out)} bytes, {len(entries)} entries)")
