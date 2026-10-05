#!/usr/bin/env python3
"""Third pass: JSON-LD in detail RSC, own ApplicationUrl, page-1 dates, all /api/ paths. Local only."""
import base64
import json
import re

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


api_rows = {}
for e in E:
    if "/api/jobs/search/?page=2" in e["request"]["url"]:
        for r in json.loads(body(e))["jobs"]:
            api_rows[r["Id"]] = r

print("== detail payloads: own record vs JSON-LD ==")
big = [e for e in E if re.search(r"jobs\.uber\.com/en/jobs/\d+/\?_rsc=", e["request"]["url"]) and len(body(e)) > 50000]
for e in big:
    jid = re.search(r"/en/jobs/(\d+)/", e["request"]["url"]).group(1)
    raw = body(e)
    flat = raw.replace('\\"', '"').replace("\\\\", "\\")
    # own record
    m = re.search(r'"DisplayDate":"([^"]+)","UpdateDate":"([^"]+)","Reference":"%s"' % jid, flat)
    disp, upd = (m.group(1), m.group(2)) if m else (None, None)
    app = re.search(r'"ApplicationUrl":"([^"]*%s[^"]*)"' % jid, flat)
    app_any = re.findall(r'"ApplicationUrl":"([^"]*)"', flat)
    # JSON-LD
    ld_i = flat.find("ld+json")
    ld = flat[ld_i:ld_i + 3000] if ld_i >= 0 else ""
    dp = re.search(r'"datePosted":"([^"]+)"', flat)
    vt = re.search(r'"validThrough":"([^"]+)"', flat)
    ident = re.search(r'"identifier":(\{[^}]{0,200}\}|"[^"]*")', flat)
    emp = re.search(r'"employmentType":("[^"]*"|\[[^\]]*\])', flat)
    print(f"  {jid}: DisplayDate={disp} UpdateDate={upd} | LD datePosted={dp and dp.group(1)} validThrough={vt and vt.group(1)} employmentType={emp and emp.group(1)}")
    print(f"      identifier={ident and ident.group(1)}")
    print(f"      own ApplicationUrl={app and app.group(1)} | all ApplicationUrl values={app_any[:5]}")
    a = api_rows.get(jid)
    if a:
        print(f"      API row DisplayDate={a['DisplayDate']} match={a['DisplayDate']==disp}")
    sub = re.search(r'"Teams":\[([^\]]*)\],"SubTeams":\[([^\]]*)\]', flat)
    addt = re.search(r'"AdditionalText":"([^"]*)"', flat)
    print(f"      Teams/SubTeams={sub and sub.groups()} AdditionalText={addt and addt.group(1)}")

e = big[0]
flat = body(e).replace('\\"', '"').replace("\\\\", "\\")
i = flat.find("ld+json")
print("\n== JSON-LD context (first payload) ==")
seg = flat[i:i + 2500]
seg = re.sub(r'"description":"(.{0,100}).*?","', r'"description":"\1…","', seg, flags=re.S)
print(seg)

print("\n== own job record keys/values (first payload), Description trimmed ==")
jid = re.search(r"/en/jobs/(\d+)/", e["request"]["url"]).group(1)
j = flat.find('"Reference":"%s"' % jid)
st = flat.rfind('{"', 0, j - 1500)
seg = flat[max(0, j - 2500): j + 400]
seg = re.sub(r'"Description":"(.{0,80}).*?","', r'"Description":"\1…","', seg, flags=re.S)
print(seg)

print("\n== HTML page-1 rows: ids + DisplayDate ==")
for e in E:
    if e["request"]["url"] == "https://jobs.uber.com/en/jobs/?page=2&pagesize=10":
        flat = body(e).replace('\\"', '"')
        for m in re.finditer(r'"Id":"(\d+)"', flat):
            seg = flat[max(0, m.start() - 4000): m.start() + 200]
            dd = re.findall(r'"DisplayDate":"([^"]+)"', seg)
            print("  ", m.group(1), dd[-1] if dd else None)

print("\n== every /api/ path literal in JS ==")
paths = set()
for e in E:
    mt = (e["response"].get("content", {}) or {}).get("mimeType", "")
    if "javascript" in mt or e["request"]["url"].endswith(".js"):
        for m in re.finditer(r'["`\'](/api/[A-Za-z0-9_./${}-]+)', body(e)):
            paths.add(m.group(1))
for p in sorted(paths):
    print("  ", p)

print("\n== keys in search row Locations/Identifier and Salary non-null counts ==")
nn = {k: sum(1 for r in api_rows.values() if r.get(k) not in (None, "", [])) for k in
      ("Summary", "AdditionalText", "AdditionalDescription1", "QuestionsRaw", "ExperienceLevel", "WorkPattern", "ContractType", "Remote")}
print("  non-empty counts over 10 rows:", nn)
print("  Remote values:", [r.get("Remote") for r in api_rows.values()])
print("  WorkPattern:", sorted(set(r.get("WorkPattern") for r in api_rows.values())))
print("  Salary MinValue non-null:", sum(1 for r in api_rows.values() if (r.get("Salary") or {}).get("MinValue") is not None))
print("  Salary.Description non-empty:", sum(1 for r in api_rows.values() if (r.get("Salary") or {}).get("Description")))
