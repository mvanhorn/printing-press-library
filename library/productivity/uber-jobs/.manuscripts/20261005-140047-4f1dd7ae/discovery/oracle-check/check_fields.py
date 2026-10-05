#!/usr/bin/env python3
"""Field coverage in Oracle r2 rows and PII presence in r3, printing no PII values. Local only."""
import json

r2 = json.load(open("~/printing-press/uber-jobs-prep/oracle-check/r2_body"))["items"][0]["requisitionList"]
for k in ("ShortDescriptionStr", "ExternalQualificationsStr", "ExternalResponsibilitiesStr", "PrimaryLocationCountry",
          "PostedDate", "JobFamily", "JobFunction", "Organization", "Department", "ContractType", "WorkerType", "JobSchedule"):
    vals = [r.get(k) for r in r2]
    nonempty = [v for v in vals if v not in (None, "", [])]
    lens = sorted(len(v) for v in nonempty if isinstance(v, str))
    print(f"r2 {k}: non-empty {len(nonempty)}/{len(vals)} lens {lens[:3]}..{lens[-3:] if lens else ''} sample={sorted(set(map(str, nonempty)))[:4] if k not in ('ShortDescriptionStr','ExternalQualificationsStr','ExternalResponsibilitiesStr') else ''}")
r3 = json.load(open("~/printing-press/uber-jobs-prep/oracle-check/r3_body"))["items"][0]
for k in ("HiringManager", "ExternalContactName", "ExternalContactEmail", "InternalQualificationsStr", "InternalResponsibilitiesStr"):
    v = r3.get(k)
    print(f"r3 {k}: {'EMPTY' if v in (None, '') else 'PRESENT (len %d) - value not printed' % len(str(v))}")
wl = r3.get("workLocation") or []
print("r3 workLocation[0] Country:", wl[0].get("Country") if wl else None)
pc = r3.get("primaryLocationCoordinates") or []
print("r3 primaryLocationCoordinates[0] CountryCode:", pc[0].get("CountryCode") if pc else None)
