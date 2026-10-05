#!/usr/bin/env python3
"""Compare Oracle detail (r3) for 302906 with the HAR search row. Local only."""
import base64
import json
import re

d = json.load(open("~/printing-press/uber-jobs-prep/oracle-check/r3_body"))
print("root keys:", sorted(d.keys()), "count:", d.get("count"))
it = d["items"][0]
scalars = {k: v for k, v in it.items() if not isinstance(v, (list, dict))}
print("scalar keys:", sorted(scalars))
print("child lists:", {k: len(v) for k, v in it.items() if isinstance(v, list)})
for k in ("Id", "Title", "ExternalPostedStartDate", "ExternalPostedEndDate", "RequisitionId", "PrimaryLocation",
          "PrimaryLocationCountry", "JobFamily", "JobFunction", "Category", "Organization", "Department",
          "WorkplaceType", "JobSchedule", "JobType", "ContractType", "StudyLevel", "ManagerLevel", "HotJobFlag"):
    print(f"  {k}: {it.get(k)!r}")
for k in ("ExternalDescriptionStr", "ExternalQualificationsStr", "ExternalResponsibilitiesStr", "ShortDescriptionStr", "CorporateDescriptionStr", "OrganizationDescriptionStr"):
    v = it.get(k)
    print(f"  {k}: len={len(v) if v else 0} head={(v or '')[:140]!r}")
for k, v in it.items():
    if isinstance(v, list) and v:
        print(f"  child {k}[0] keys:", sorted(v[0].keys())[:25])

har = json.load(open("~/Downloads/jobs.uber.com.har"))["log"]["entries"]
row = None
for e in har:
    if "/api/jobs/search/?page=2" in e["request"]["url"]:
        c = e["response"]["content"]
        t = c["text"] if c.get("encoding") != "base64" else base64.b64decode(c["text"]).decode()
        row = next(r for r in json.loads(t)["jobs"] if r["Id"] == "302906")
print("\nHAR row: Title=%r DisplayDate=%s Country=%s City=%s" % (row["Title"], row["DisplayDate"], row["Locations"][0]["Country"], row["Locations"][0]["City"]))
print("Title match:", row["Title"] == it.get("Title"))
strip = lambda s: re.sub(r"\s+", " ", re.sub(r"<[^>]+>", " ", s or "")).strip()
hd, od = strip(row["Description"]), strip(it.get("ExternalDescriptionStr"))
print("Description: HAR text len", len(hd), "| Oracle text len", len(od), "| identical:", hd == od)
print("HAR desc starts in Oracle desc:", hd[:200] in od, "| Oracle starts in HAR:", od[:200] in hd)
