#!/usr/bin/env python3
"""Second pass: detail RSC payloads, HTML-embedded rows, byte breakdown. Local only."""
import base64
import collections
import json
import re
from urllib.parse import urlparse

HAR = "~/Downloads/jobs.uber.com.har"
E = json.load(open(HAR))["log"]["entries"]


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


print("== body bytes by mime ==")
by = collections.Counter()
cnt = collections.Counter()
for e in E:
    c = e["response"].get("content", {}) or {}
    mt = (c.get("mimeType") or "").split(";")[0]
    by[mt] += len(c.get("text") or "")
    cnt[mt] += 1
tot = sum(by.values())
for mt, n in by.most_common(10):
    print(f"  {mt:28} {cnt[mt]:4d} entries {n:>12,} chars ({100*n/tot:.1f}%)")

print("\n== large job-id RSC payloads ==")
big = [e for e in E if re.search(r"jobs\.uber\.com/en/jobs/\d+/\?_rsc=", e["request"]["url"]) and len(body(e)) > 50000]
print("count:", len(big))
for e in big:
    b = body(e)
    jid = re.search(r"/en/jobs/(\d+)/", e["request"]["url"]).group(1)
    flat = b.replace('\\"', '"')
    has_desc = '"Description"' in flat
    has_id = f'"Id":"{jid}"' in flat or f'"Id":{jid}' in flat
    oracle = sorted(set(re.findall(r"https?://[a-z0-9.-]*oraclecloud\.com[^\"'\\ ]{0,160}", flat)))[:4]
    keys = sorted(set(re.findall(r'"(DisplayDate|DatePosted|datePosted|PostedDate|ApplyUrl|applyUrl|ApplyLink|ExternalUrl|Reference|QuestionsRaw|ValidThrough|validThrough|ExpiryDate|CreatedDate|PublishDate|StartDate)"', flat)))
    print(f"  {jid} {e['request']['url'].split('_rsc=')[1]} bytes={len(b):,} desc={has_desc} self-id={has_id} keys={keys}")
    print("    oracle urls:", oracle)
    ld = "ld+json" in flat
    print("    json-ld:", ld)

# Dump the field names near the job's own record in one payload
e = next((x for x in big if "/302906/" in x["request"]["url"]), big[0] if big else None)
if e:
    flat = body(e).replace('\\"', '"')
    jid = re.search(r"/en/jobs/(\d+)/", e["request"]["url"]).group(1)
    i = flat.find(f'"Id":"{jid}"')
    if i < 0:
        i = flat.find(jid)
    print(f"\n  context near own id {jid} (pos {i}):")
    seg = flat[max(0, i-300):i+2500]
    seg = re.sub(r'"Description":"(.{0,120}).*?","', r'"Description":"\1…","', seg, flags=re.S)
    print("   ", seg[:2800])
    # all PascalCase keys in the payload
    ks = collections.Counter(re.findall(r'"([A-Z][A-Za-z0-9]+)":', flat))
    print("\n  PascalCase keys:", ks.most_common(60))
    # any apply link patterns
    applies = sorted(set(re.findall(r'(https?://[^"\\ ]*(?:apply|Apply|CandidateExperience)[^"\\ ]{0,160})', flat)))
    print("\n  apply-ish urls:", applies[:8])
    dates = sorted(set(re.findall(r'"([A-Za-z]*Date[A-Za-z]*)":"([^"]{4,40})"', flat)))
    print("\n  date fields:", dates[:20])

print("\n== HTML page embedded rows ==")
for e in E:
    u = e["request"]["url"]
    if u == "https://jobs.uber.com/en/jobs/?page=2&pagesize=10":
        flat = body(e).replace('\\"', '"')
        ids = re.findall(r'"Id":"(\d+)"', flat)
        print("  ids in HTML (order, first 30):", ids[:30], "unique:", len(set(ids)))
        for k in ("page", "pageSize", "totalJobs", "totalPages"):
            m = re.findall(r'"%s":(\d+)' % k, flat)
            print(f"  {k}:", m[:5])
api = next(e for e in E if "/api/jobs/search/?page=2" in e["request"]["url"])
aids = [r["Id"] for r in json.loads(body(api))["jobs"]]
print("  API page=2 ids:", aids)

print("\n== other date shapes in API rows ==")
rows = json.loads(body(api))["jobs"]
for r in rows[:3]:
    print("  ", r["Id"], {k: r.get(k) for k in ("DisplayDate", "ContractType", "WorkPattern", "ExperienceLevel", "Remote", "Teams", "Summary")})
    print("     Salary:", r.get("Salary"))
    print("     Loc0:", {k: v for k, v in (r.get("Locations") or [{}])[0].items() if k != "LocationPoint"}, "LP type:", type((r.get("Locations") or [{}])[0].get("LocationPoint")).__name__)
    print("     AdditionalDescription1:", (r.get("AdditionalDescription1") or "")[:160])
    print("     QuestionsRaw:", str(r.get("QuestionsRaw"))[:160])
    print("     Description head:", (r.get("Description") or "")[:200])

print("\n== recently-viewed body ==")
for e in E:
    if e["request"]["url"].endswith("/recently-viewed/") and body(e):
        j = json.loads(body(e))
        print("  ", json.dumps(j["jobs"][0])[:600])

print("\n== sitemap RSC: job links? ==")
for e in E:
    if "/en/sitemap/?_rsc=4jzs8" in e["request"]["url"]:
        flat = body(e)
        print("  job links:", len(re.findall(r"/en/jobs/\d+/", flat)))

print("\n== iaziqy mentions ==")
for e in E:
    b = body(e)
    if "iaziqy" in b:
        u = e["request"]["url"]
        m = re.search("iaziqy", b)
        print("  ", urlparse(u).path[-60:], "ctx:", b[max(0, m.start()-80):m.start()+160].replace("\n", " "))
