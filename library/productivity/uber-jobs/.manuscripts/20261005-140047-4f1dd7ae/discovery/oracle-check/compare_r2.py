#!/usr/bin/env python3
"""Compare Oracle findReqs (r2) with the HAR's jobs.uber.com rows. Local only."""
import json
import sys

d = json.load(open("~/printing-press/uber-jobs-prep/oracle-check/r2_body"))
it = d["items"][0]
print("root keys:", sorted(d.keys()))
print("item keys:", sorted(k for k in it.keys() if k != "requisitionList"))
print("TotalJobsCount:", it.get("TotalJobsCount"), "| Offset:", it.get("Offset"), "| Limit:", it.get("Limit"), "| SortBy:", it.get("SortBy"))
rl = it.get("requisitionList") or []
print("requisitionList rows:", len(rl))
if rl:
    print("row keys:", sorted(rl[0].keys()))
for r in rl:
    print(f"  {r.get('Id'):>8} {r.get('PostedDate')} {str(r.get('PrimaryLocationCountry')):3} {str(r.get('PrimaryLocation'))[:40]:40} {str(r.get('Title'))[:60]}")

har_page1 = ["301974", "153856", "302500", "302496", "302497", "160013", "302278", "303121", "303008", "302447"]
har_page2 = ["302906", "160425", "302964", "303168", "302734", "302779", "301078", "300313", "302264", "302569"]
ids = [r.get("Id") for r in rl]
print("\nOracle order (first 20):", ids[:20])
print("HAR p1+p2 order       :", har_page1 + har_page2)
print("HAR ids found in Oracle top-25:", sum(1 for i in har_page1 + har_page2 if i in ids), "of 20")
print("Oracle top-25 ids not in HAR 20:", [i for i in ids if i not in har_page1 + har_page2])
dates = [r.get("PostedDate") for r in rl]
print("PostedDate non-increasing:", all(a >= b for a, b in zip(dates, dates[1:]) if a and b))
fac = [k for k in it.keys() if k.endswith("Facet") or k in ("categoriesFacet", "locationsFacet", "postingDatesFacet", "titlesFacet")]
print("facet-ish keys:", fac)
for k in ("categoriesFacet", "locationsFacet", "postingDatesFacet"):
    v = it.get(k)
    if isinstance(v, list):
        print(f"  {k}: n={len(v)} sample={[ (x.get('Name'), x.get('TotalCount')) for x in v[:6]]}")
