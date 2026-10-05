#!/usr/bin/env python3
"""Verify primer section 1b facts against the owner's HAR. Local only: no network."""
import base64
import collections
import json
import re
import sys
from urllib.parse import urlparse, parse_qs

HAR = "~/Downloads/jobs.uber.com.har"
doc = json.load(open(HAR))
log = doc["log"]
E = log["entries"]


def body(e):
    c = e["response"].get("content", {}) or {}
    t = c.get("text")
    if t is None:
        return ""
    if c.get("encoding") == "base64":
        try:
            return base64.b64decode(t).decode("utf-8", "replace")
        except Exception:
            return ""
    return t


def hdrs(lst):
    return {h["name"].lower(): h["value"] for h in lst}


print("== overall ==")
print("creator:", log.get("creator"), "browser:", log.get("browser"))
print("entries:", len(E))
times = sorted(e["startedDateTime"] for e in E)
print("first:", times[0], "last:", times[-1])
mimes = collections.Counter((e["response"].get("content", {}) or {}).get("mimeType", "").split(";")[0] for e in E)
print("mime top:", mimes.most_common(12))
b64 = sum(1 for e in E if (e["response"].get("content", {}) or {}).get("encoding") == "base64")
img = sum(1 for e in E if (e["response"].get("content", {}) or {}).get("mimeType", "").startswith("image/"))
print("base64 bodies:", b64, "image responses:", img)
img_bytes = sum(len((e["response"].get("content", {}) or {}).get("text") or "") for e in E
                if (e["response"].get("content", {}) or {}).get("mimeType", "").startswith("image/"))
all_bytes = sum(len((e["response"].get("content", {}) or {}).get("text") or "") for e in E)
print(f"image body chars: {img_bytes:,} of {all_bytes:,} total body chars")
hosts = collections.Counter(urlparse(e["request"]["url"]).hostname for e in E)
print("hosts:")
for h, n in hosts.most_common():
    print(f"  {n:4d} {h}")

print("\n== secrets ==")
bad_req = collections.Counter()
bad_resp = collections.Counter()
req_cookie_arrays = 0
resp_cookie_arrays = 0
for e in E:
    for h in e["request"].get("headers", []):
        n = h["name"].lower()
        if n in ("cookie", "authorization", "x-csrf-token", "x-xsrf-token", "proxy-authorization") or "token" in n or "auth" in n:
            bad_req[n] += 1
    for h in e["response"].get("headers", []):
        n = h["name"].lower()
        if n in ("set-cookie",):
            bad_resp[n] += 1
    if e["request"].get("cookies"):
        req_cookie_arrays += 1
    if e["response"].get("cookies"):
        resp_cookie_arrays += 1
print("request header hits (cookie/auth/token):", dict(bad_req))
print("response set-cookie hits:", dict(bad_resp))
print("entries with non-empty request.cookies:", req_cookie_arrays, "response.cookies:", resp_cookie_arrays)
cf_chl = [e["request"]["url"] for e in E if "challenges.cloudflare.com" in e["request"]["url"] or "/cdn-cgi/challenge-platform" in e["request"]["url"]]
print("challenge-platform requests:", len(cf_chl), cf_chl[:3])
cfcl = sum(1 for e in E if "cf_clearance" in json.dumps(e["request"]) or "cf_clearance" in json.dumps(e["response"].get("headers", [])))
print("entries mentioning cf_clearance in request/resp headers:", cfcl)

print("\n== uber.com first-party requests (non-image) ==")
for e in E:
    u = e["request"]["url"]
    h = urlparse(u).hostname or ""
    mt = (e["response"].get("content", {}) or {}).get("mimeType", "")
    if h.endswith("uber.com") and not mt.startswith("image/") and "/_next/static" not in u:
        rh = hdrs(e["response"].get("headers", []))
        print(f"  {e['startedDateTime']} {e['request']['method']} {e['response']['status']} {len(body(e)):>7} {mt[:24]:24} {u[:150]}")
        if "/api/" in u:
            print("     resp:", {k: rh.get(k) for k in ("server", "cache-control", "x-vercel-cache", "age", "cf-cache-status", "content-type", "location")})

print("\n== search API ==")
search = [e for e in E if "/api/jobs/search" in e["request"]["url"]]
for e in search:
    rq = hdrs(e["request"].get("headers", []))
    print("url:", e["request"]["url"], "status:", e["response"]["status"])
    print("  request header names:", sorted(rq.keys()))
    b = body(e)
    print("  body bytes:", len(b))
    if b:
        j = json.loads(b)
        print("  top keys:", {k: (type(v).__name__ if not isinstance(v, (int, str)) else v) for k, v in j.items()})
        jobs = j.get("jobs", [])
        print("  jobs:", len(jobs))
        if jobs:
            print("  row keys:", sorted(jobs[0].keys()))
            for r in jobs:
                locs = r.get("Locations") or []
                print(f"   Id={r.get('Id')} Ref={r.get('Reference')} same={r.get('Id')==r.get('Reference')} DisplayDate={r.get('DisplayDate')} nloc={len(locs)} "
                      f"CC={[l.get('CountryCode') for l in locs]} Country={[l.get('Country') for l in locs]} score={r.get('@search.score')} "
                      f"AddText={r.get('AdditionalText')!r} Urls={[x.get('Url') for x in (r.get('Urls') or [])]}")
            print("  Location keys:", sorted((jobs[0].get("Locations") or [{}])[0].keys()))
            print("  Salary:", jobs[0].get("Salary") and sorted(jobs[0]["Salary"].keys()))
            print("  Urls keys:", sorted((jobs[0].get("Urls") or [{}])[0].keys()))
            dd = [r.get("DisplayDate") for r in jobs]
            print("  DisplayDate newest-first:", dd == sorted(dd, reverse=True))

print("\n== recently-viewed / recommendations / subscribe ==")
for e in E:
    u = e["request"]["url"]
    if any(k in u for k in ("recently-viewed", "recommendations", "subscribe")):
        pd = (e["request"].get("postData") or {}).get("text")
        print(f"  {e['request']['method']} {e['response']['status']} {u}")
        print("    postData:", pd)
        rh = hdrs(e["response"].get("headers", []))
        print("    location:", rh.get("location"))
        b = body(e)
        if b:
            try:
                j = json.loads(b)
                print("    top:", list(j.keys()), "row keys:", sorted(j.get("jobs", [{}])[0].keys()) if j.get("jobs") else None, "n:", len(j.get("jobs", [])))
            except Exception:
                print("    body (non-json) bytes:", len(b))

print("\n== JS constants ==")
js = [e for e in E if e["request"]["url"].endswith(".js") or "javascript" in (e["response"].get("content", {}) or {}).get("mimeType", "")]
print("js entries:", len(js))
pat = {
    "JOB_QUERY_KEYS": re.compile(r"JOB_QUERY_KEYS"),
    "/api/jobs/search/?": re.compile(r"/api/jobs/search/\?"),
    "recently-viewed": re.compile(r"recently-viewed"),
    "recommendations": re.compile(r"/api/jobs/recommendations"),
    "subscribe": re.compile(r"/api/jobs/(subscribe|subscribeConfirm|unsubscribe)"),
    "oraclecloud": re.compile(r"oraclecloud"),
    "beamery": re.compile(r"beamery"),
    "myworkdayjobs": re.compile(r"myworkdayjobs"),
    "loadSearchJobsResults": re.compile(r"loadSearchJobsResults"),
}
for e in js:
    b = body(e)
    name = urlparse(e["request"]["url"]).path.rsplit("/", 1)[-1]
    hits = [k for k, p in pat.items() if p.search(b)]
    if hits:
        print(f"  {name}: {hits}")
        for k in ("JOB_QUERY_KEYS",):
            if k in hits:
                for m in re.finditer(k, b):
                    print("    ctx:", b[max(0, m.start()-100):m.start()+700].replace("\n", " ")[:900])
                    break
        if "/api/jobs/search/?" in hits:
            m = re.search(r"/api/jobs/search/\?", b)
            print("    search ctx:", b[max(0, m.start()-200):m.start()+120].replace("\n", " "))
        for k in ("oraclecloud", "beamery", "myworkdayjobs"):
            if k in hits:
                m = re.search(k, b)
                print(f"    {k} ctx:", b[max(0, m.start()-250):m.start()+250].replace("\n", " "))
        for k in ("recommendations",):
            if k in hits:
                m = pat[k].search(b)
                print(f"    {k} ctx:", b[max(0, m.start()-200):m.start()+250].replace("\n", " "))
        if "recently-viewed" in hits:
            m = re.search("recently-viewed", b)
            print("    rv ctx:", b[max(0, m.start()-200):m.start()+200].replace("\n", " "))

print("\n== legacy aliases / radius default (grep all JS) ==")
alljs = "\n".join(body(e) for e in js)
for k in ("mylocation", "subTeams", "subteam", "keyword", "radius", "pagesize"):
    ms = [m.start() for m in re.finditer(k, alljs)]
    print(f"  {k}: {len(ms)} hits")
for m in re.finditer(r"mylocation", alljs):
    print("    alias ctx:", alljs[max(0, m.start()-400):m.start()+300].replace("\n", " "))
    break
for m in re.finditer(r"radius", alljs):
    s = alljs[max(0, m.start()-120):m.start()+120].replace("\n", " ")
    if re.search(r"100", s):
        print("    radius ctx:", s)
        break

print("\n== /en/jobs/ HTML: RSC facets, JSON-LD, totals ==")
for e in E:
    u = e["request"]["url"]
    mt = (e["response"].get("content", {}) or {}).get("mimeType", "")
    if urlparse(u).hostname == "jobs.uber.com" and "html" in mt:
        b = body(e)
        print(f"  {u} status={e['response']['status']} bytes={len(b)} __next_f={b.count('self.__next_f')} json-ld={'application/ld+json' in b} __NEXT_DATA__={'__NEXT_DATA__' in b}")
        # Unescape the RSC string payloads crudely
        flat = b.replace('\\"', '"')
        for key in ("countries", "teams", "subTeams", "teamToSubTeamMappings", "contractTypes", "workPatterns", "departments", "companies", "experienceLevels", "employmentTypes", "totalJobs", "totalPages"):
            m = re.search(r'"%s":(\[[^\]]{0,4000}\]|\{[^}]{0,200}|\d+)' % key, flat)
            if m:
                v = m.group(1)
                if v.startswith("["):
                    try:
                        arr = json.loads(v)
                        print(f"    {key}: n={len(arr)} sample={arr[:6]}")
                    except Exception:
                        print(f"    {key}: {v[:200]}")
                else:
                    print(f"    {key}: {v[:120]}")
            else:
                print(f"    {key}: (not found)")

print("\n== RSC prefetch /en/jobs/{id}/ and radius ==")
rsc = [e for e in E if "_rsc" in e["request"]["url"]]
print("rsc entries:", len(rsc))
empty = sum(1 for e in rsc if not body(e))
print("rsc with empty body:", empty)
for e in rsc:
    u = e["request"]["url"]
    if "radius" in u or re.search(r"/en/jobs/\?", u):
        print("  ", e["response"]["status"], len(body(e)), u[:200])
job_stubs = [e for e in rsc if re.search(r"/en/jobs/\d+/", e["request"]["url"])]
print("job-id rsc stubs:", len(job_stubs), "sizes:", sorted(len(body(e)) for e in job_stubs)[:3], "...", sorted(len(body(e)) for e in job_stubs)[-3:])
qkeys = collections.Counter()
for e in E:
    u = e["request"]["url"]
    if urlparse(u).hostname == "jobs.uber.com":
        for k in parse_qs(urlparse(u).query).keys():
            qkeys[k] += 1
print("query keys seen on jobs.uber.com:", dict(qkeys))

print("\n== media host ==")
print("happydance hosts:", [h for h in hosts if h and "happydance" in h])
