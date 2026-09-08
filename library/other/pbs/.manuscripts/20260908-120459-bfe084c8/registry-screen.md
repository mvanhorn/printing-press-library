# Live registry screen — 2026-09-08 ~10:5x PKT

Source: raw.githubusercontent.com/mvanhorn/printing-press-library/main/registry.json
schema_version 2, **504 entries**. Categories enumerated FROM THE REPO (not a hardcoded
list): **22** — accounting ai auth cloud commerce developer-tools devices education
food-and-dining health job-boards maps maps marketing media-and-entertainment monitoring
other payments productivity project-management sales-and-crm social-and-messaging travel.
Matches the brief's 504/22 exactly.

## Pakistani coverage — CONFIRMED, with the false positives run down
Pattern 1 `pakistan|karachi|lahore|islamabad|PKR|urdu|...` over api+name+category+path+
description+search_terms returned SIX hits. FOUR are false positives, all the same bug:
`simultan|eous` contains **multan**.
  - AnyList (food-and-dining)      "...collection simultaneously"
  - Etherpad (productivity)        "...thousands of simultaneous real time users"
  - Freshservice (productivity)    "...KB articles simultaneously"
  - Table Reservation GOAT         "...OpenTable and Tock simultaneously"
Real hits: **payments/psx** and **payments/nccpl** — both mine (qazmataz).

Pattern 1 WAS TOO NARROW. A second sweep on `\.pk\b|\bpk\b|south asia|rupee|lakh|crore|
saarc|frontier market` found one more: **commerce/daraz** (Daraz.pk, qazmataz) — a Pakistani
CLI whose description never says "Pakistan". This is the class of miss the brief warns about.

**FIELD STATE: 3 Pakistani CLIs in 504 entries. All three are mine.** cdc-pakistan and mufap
are open PRs, not yet in the registry, which will take it to 5.

## Candidate pool — screened on domain concepts, not slug substrings
CLEAR (no overlap of any kind): OGRA · DRAP · SECP · PTA · SBP · FBR · PMEX · PriceOye ·
Telemart · OLX · PakWheels · Graana · Bookme/Sastaticket · PIA/Airblue · SNGPL/SSGC · HEC ·
BISE · NTS/PPSC/FPSC · Upwork · Fiverr · Payoneer/remittance · AliExpress · Careem · Coursera

PRECEDENT-ONLY hits (validated shape, NOT a Pakistani duplicate):
  PPRA   -> sales-and-crm/eu-tenders (m91michel), sales-and-crm/tenderned (markvandeven)
  PBS    -> other/fred (LukeTheoJohnson), other/us-data (sdhilip200)   [US macro only]
  PMD    -> other/air-quality, other/nhc, other/open-meteo, productivity/opensnow,
            other/pvgis, other/weather-goat   [6 weather/AQI CLIs, zero PK]

FALSE POSITIVE run down: NEPRA matched **devices/bmw-cardata** on "electricity tariff" —
that is EV charging-cost reconciliation, not a utility regulator. Not an overlap.

ALREADY MINE, do not rebuild: psx · nccpl · daraz · 1688 · zameen · foodpanda · peekaboo ·
amazon-jobs (+ mufap, cdc-pakistan in flight).
