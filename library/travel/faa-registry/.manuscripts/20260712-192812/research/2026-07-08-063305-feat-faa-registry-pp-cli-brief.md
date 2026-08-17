# FAA Aircraft Registry CLI Brief

## API Identity
- Domain: FAA Civil Aviation Registry — the authoritative US aircraft registration database (registry.faa.gov). Two surfaces: (1) the live `aircraftinquiry` web app (10 inquiry types, ASP.NET MVC, HTML responses), (2) the daily-refreshed Releasable Aircraft Database bulk download (~530 MB uncompressed across 7 files).
- Users: pilots, aircraft owners (incl. fractional owners like NetJets shareowners), buyers doing pre-purchase research, planespotters, aviation journalists, escrow/title companies, ADS-B hobbyists mapping Mode S hex codes to tails.
- Data profile: ~315K active registrations, ~383K deregistered records, ~126K reserved N-numbers, 94K aircraft model refs (seats/weight/speed), 4.7K engine refs (HP/thrust). Updated each federal working day at midnight.

## Reachability Risk
- **Low.** `probe-reachability` = `standard_http` (0.95 confidence). Akamai blocks non-browser User-Agents (curl/Go-http-client → 403) — fixed with a required Chrome UA header, verified live. No auth, no cookies, no CSRF enforcement (anti-forgery token in forms is not validated). All inquiry endpoints accept plain GET with query params (verified: NNumberResult, NameResult, SerialNumberResult, MakeModelResult). Bulk DB zip: HEAD blocked (503) but GET works, supports Range.

## Top Workflows
1. **Tail-number lookup** — `N16QS` → full registration record (owner, serial, model, Mode S hex, expiration, temporary certificates, other/fractional owner names, airworthiness). The user's stated primary goal.
2. **Owner/fleet search** — all aircraft registered to "NETJETS SALES INC" or any person/LLC (live paginated search + instant offline query).
3. **Mode S hex ↔ tail decode** — ADS-B receivers give hex codes (A0F0C4); mapping to N-number requires the registry.
4. **Expiration tracking** — US registrations expire every 7 years; owners get letters, but a CLI check (`expiring --within 90d`) beats mail.
5. **Pre-purchase research** — serial-number history, deregistered records, reserved N-numbers, document index for liens.

## Table Stakes
- N-number lookup returning every field the website shows (merged when ecosystem agent reports; known incumbents: FlightAware/registry websites, python `faa-aircraft-registry` parsers of the bulk CSV, various dead GitHub scrapers).
- Bulk-database download + parse (several GitHub projects parse MASTER.txt; none pair it with live inquiries).
- JSON output for scripting.

## Data Layer
- Primary entities: aircraft (MASTER), deregistered, reserved N-numbers, aircraft models (ACFTREF), engines (ENGINE), dealers, document index.
- Sync cursor: whole-file replace on daily zip (Last-Modified/ETag check); no incremental API.
- FTS/search: FTS5 over owner names + model names; direct indexes on N-number, serial, Mode S hex.

## Live inquiry contract (discovered, verified)
Base `https://registry.faa.gov/aircraftinquiry` — all GET, `response_format: html`, semantic `data-label` markup:
- `/Search/NNumberResult?NNumberTxt=` · `/Search/SerialNumberResult?Serialtxt=&sort_option=` · `/Search/NameResult?nametxt=&sort_option=&Page=` · `/Search/MakeModelResult?Mfrtxt=&Modeltxt=&Page=` · `/Search/EngineReferenceResult?MfrNametxt=&Modeltxt=` · `/Search/DealerResult?Dealertxt=` · `/Search/DocumentIndexResult?Colltxt=` · `/Search/StateCountyResult?StateName=&CountyName=&Page=` · `/Search/CountryResult?Countrytxt=`
- N-number availability endpoint 302s without a session; availability is computable offline (MASTER ∪ RESERVED ∪ DEREG) instead.
- Full details in `discovery/browser-sniff-report.md`.

## User Vision
- From the /goal directive + example: "do a search on N number like N16QS and get the results page" — tail-number lookup returning the full structured record (the user co-owns a NetJets fractional; fractional-owner data like Other Owner Names and Temporary Certificates must parse correctly).

## Product Thesis
- Name: faa-registry (`faa-registry-pp-cli`)
- Why it should exist: the FAA website is the only authoritative source but is browser-only, form-driven, and un-scriptable; existing wrappers parse only the bulk CSV (stale tooling, no live inquiry) or are dead scrapers. Nobody pairs live inquiries with a daily-synced offline registry in SQLite — that combination unlocks fleet analytics, hex decoding, expiration alerts, and instant availability checks that neither the website nor any wrapper offers.

## Build Priorities
1. `aircraft lookup <N>` — flagship: live lookup, hand-parsed data-label tables → typed JSON (all sections incl. Other Owner Names, Temporary Certificates, Airworthiness).
2. Sync: download daily zip → SQLite (all 7 files), FTS5 on names/models.
3. Offline transcendence: hex↔tail decode, fleet reports, expiring registrations, N-number availability, deregistration history.
4. Remaining live inquiries (serial, name, make/model, engine, dealer, docs, state/county, country) with list-table parsing + pagination.
