# FAA Aircraft Registry — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | N-number lookup, full registration record | FAA website; Apify parseforge scraper; tailnumberlookup.com | `aircraft lookup <N>` live HTML parse → typed JSON (Aircraft Description, Registered Owner, Other Owner Names, Temporary Certificates, Airworthiness, Fuel Modifications) | Scriptable JSON/`--select`; `--offline` fallback from local DB; parses fractional-owner fields no scraper exposes |
| 2 | Serial-number search | FAA website | `aircraft by-serial <serial>` | JSON + sort options |
| 3 | Owner-name search (live) | FAA website; aviationdb.com; adsbtrack `owner --name` (SQL LIKE) | `owners search --name X` | Auto-pagination (`--all-pages`), JSON; offline FTS5 fuzzy variant via local DB |
| 4 | Make/model reference search | FAA website | `models search --manufacturer X --model Y` | JSON incl. number-of-aircraft-assigned |
| 5 | Engine reference search | FAA website | `engines search` | JSON incl. horsepower/thrust from local ENGINE table |
| 6 | Dealer certificate search | FAA website | `dealers search` | JSON |
| 7 | Document index search | FAA website | `documents search` | JSON (lien/due-diligence records) |
| 8 | State/county aircraft listing | FAA website | `regions by-state WA --county KING` | JSON + pagination |
| 9 | Country listing | FAA website | `regions by-country CANADA` | JSON |
| 10 | Bulk DB download + parse (all 7 files) | ClearAerospace/faa-aircraft-registry (PyPI); adsbtrack `registry update`; simonw git-scraping | `sync` → SQLite + FTS5; decodes code fields (registrant type, aircraft type, engine type, status, region, airworthiness class) per ardata.pdf | ETag/Last-Modified-aware daily refresh; DEREG + RESERVED retained (most tools drop them); queryable via `sql`/`search` |
| 11 | N-number ⇄ ICAO hex conversion | guillaumemichel/icao-nnumber_converter; avionictools.com; grndcntrl.net; @squawk/icao-registry | `hex to-tail <hex>` / `hex from-tail <N>`: DB-authoritative + pure-algorithm fallback | Batch stdin, JSON, octal + hex forms |
| 12 | Hex → aircraft record (ADS-B enrichment) | SkyLink API (paid, 615K worldwide); adsbtrack `lookup --hex`; hexdb.io | `aircraft lookup --hex A0F0C4` from local DB | Free, offline, authoritative for US |
| 13 | Watch for registration changes by owner | Jxck-S/FAA-registry-checker (exact-caps owner list + Discord webhook) | `watch` over sync snapshots: new/removed/changed registrations for watched owners/tails | JSON output, fuzzy owner match, no external webhook infra needed |
| 14 | N-number availability check | FAA website availability inquiry (script-hostile: redirects with format errors) | `nnumber available <N>` computed offline: MASTER ∪ RESERVED ∪ DEREG status decode | Works programmatically; batch mode; explains WHY unavailable (assigned/reserved/dereg-pending) |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Persona / Evidence |
|---|---------|---------|-------|-------------|--------------------|
| 1 | Fleet composition report | `fleet report --owner "NETJETS SALES INC"` | 10/10 | Joins synced MASTER (owner's tails, incl. OTHER NAMES columns) to ACFTREF in local SQLite: count, model mix, jet/turboprop/piston split, avg seats/year | Fleet analyst persona; brief workflow #2; "become a spreadsheet" frustration |
| 2 | Batch Mode S hex resolution | `hex resolve` (stdin batch) | 10/10 | Reads hex codes from stdin, joins each to N-number + type + owner via indexed local MASTER; pure-algorithm fallback for unmatched | ADS-B hobbyist persona; brief workflow #3 |
| 3 | Aircraft ownership history | `aircraft history <N>` | 9/10 | Stitches current MASTER row + all DEREG records for tail/serial into chronological owner timeline with cancel dates | Pre-purchase researcher; brief workflow #5; open lane (faaDb excludes dereg) |
| 4 | Expiring registrations | `expiring --within 90 --owner X` | 9/10 | Local query over MASTER expiration dates filtered by owner/state, sorted soonest-first | Brief workflow #4; ecosystem gap: "checker roadmaps it, nobody ships" |
| 5 | Model-class fleet breakdown | `models fleet --manufacturer CIRRUS --model SR22` | 8/10 | Aggregates all registered examples of a make/model in local MASTER by registrant type + state | Fleet analyst / buyer personas |

No stubs. All 14 absorbed + 5 transcendence features are shipping scope. Killed candidates and full audit trail: `2026-07-08-063305-novel-features-brainstorm.md`.
