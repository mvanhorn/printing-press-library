# NEPRA CLI — Absorb Manifest

## The competitive situation, stated plainly

**Nothing to absorb from a data tool, because no data tool exists.** PyPI: 0 packages.
npm: 0 packages. Zero GitHub repos fetch NEPRA data at runtime — *every* tariff figure in
the entire ecosystem is a hardcoded constant. A repo literally named `nepradataset` holds
one zip containing four hand-typed 10–14-row spreadsheets (one misspelled `Industerial`)
and a PDF.

So the absorb targets are not features to match — they are the seven consumer-side tools'
*good engineering ideas*, which we absorb and apply to a surface none of them touch.
The parity bar is low; the transcendence bar is the whole product.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Notified-tariff rate table per DISCO | `atifjan2019/ebillpakistan.pk` `lib/tariffs.js` (17,395 B, hardcoded) | `(generated endpoint) tariff get` | Fetched live from `/tariff/Distribution <DISCO>.php`, not a constant a human retypes; all 11 DISCOs incl. HAZECO |
| 2 | Source provenance per rate | `ebillpakistan.pk` `NEPRA_SOURCE` object (name/url/notifiedOn/effectiveFrom/supersedes) | `(behavior in nepra-pp-cli coverage) --provenance` records url, http_status, bytes, sha256, fetched_at per artifact | Machine-checkable provenance on every stored row, not prose in a JS object |
| 3 | Staleness as a hard build signal | `ebillpakistan.pk` `scripts/tariff-staleness.mjs` — `STALE_AFTER_DAYS=90`, CI exits 1 | `(behavior in nepra-pp-cli coverage) --check-stale` exits non-zero past the window | Applies to every surface, not just tariff; exit code is the contract so a cron can gate on it |
| 4 | Per-SRO effective-window tracking | `ebillpakistan.pk` `ADJUSTMENTS` with `appliesFrom`/`appliesTo` | `(generated endpoint) sro list` | Stored as rows with real date bounds, queryable; supersession chain preserved |
| 5 | Explicit refusal paths / stated scope limits | `Muhammad-Sohair/bijlicheck` — openly refuses K-Electric, ToU, unaccountable readings | `(behavior in nepra-pp-cli doctor) --scope` + SKILL.md anti-triggers | Refusals are declared in the machine-readable surface, not only the README |
| 6 | Separating provable breach from disputed | `bijlicheck` `recoverable` vs `disputed` | `(behavior in nepra-pp-cli reliability) value_state + conflict flag` | Generalised: every figure carries as-published state; conflicts surfaced, never reconciled |
| 7 | Peak / off-peak window table per DISCO | `farhanshahlabs/ha_wapda_peak_hours` `const.py` (2,008 B, "100% offline", hardcoded) | `(generated endpoint) tariff peak-windows` | Read from the notified tariff page rather than transcribed; drift becomes visible |
| 8 | T&D loss vs allowed target, per DISCO | NEPRA PER PDFs (no tool extracts these) | `(generated endpoint) reliability losses` | First machine-readable extraction; carries Actual, Allowed-in-Tariff AND Breach-of-Target |
| 9 | SAIFI / SAIDI with NEPRA target | NEPRA PER PDFs (no tool extracts these) | `(generated endpoint) reliability interruptions` | First machine-readable extraction across 7 report-years |
| 10 | Recovery %, complaints, fatalities, fault rate | NEPRA PER PDFs (no tool extracts these) | `(generated endpoint) reliability service` | The rest of the PER metric set, same store |
| 11 | Plant-level installed & dependable capacity | NEPRA generation workbooks (only hand-read) | `(generated endpoint) generation plants` | 7 fiscal years, 108–133 plants/yr, as rows |
| 12 | Monthly plant generation (GWh) + utilisation (%) | NEPRA generation workbooks (only hand-read) | `(generated endpoint) generation monthly` | 26 monthly fields/plant/yr with the five cell states preserved |
| 13 | Technology / fuel classification | NEPRA generation workbooks | `(behavior in nepra-pp-cli generation plants) --technology / --fuel` | 9 technology and 16 fuel values, verified stable across years, queryable as enums |
| 14 | Fuel Cost Adjustment series | NEPRA FCA workbooks (no tool extracts) | `(generated endpoint) fca list` | Machine-readable FCA, joinable to tariff |
| 15 | Load-shedding schedule ETL | `Noor-Rehman/VoltaIQ` `iesco_parser.py` (IESCO only, inputs hand-obtained) | `(stub — deferred, see below)` | Deliberately out of v1 scope; VoltaIQ's inputs are hand-downloaded XLSX, not a NEPRA surface |
| 16 | Bill audit / recompute from tariff | `bijlicheck` `lib/audit.ts` (20,878 B) | `(stub — deferred, see below)` | Needs per-consumer bill intake; a regulator-data CLI is the wrong home for it |
| 17 | Net-billing / prosumer simulation | `AE-Usama/NEPRA-Prosumer-Bill-Calculator-2025`, `harisahmadkhan/Solar-NetBilling-model-` | `(stub — deferred, see below)` | Simulation, not regulator data; correctly someone else's product |
| 18 | External cross-check of national generation | none in ecosystem | `(behavior in nepra-pp-cli verify) --crosscheck iea` | Reconciles our Σ Sum-GWh against IEA's 204-row 1990–2023 series; surfaces the gap rather than hiding it |

### Stubs and deferrals — read out at the gate for explicit approval

Rows 15, 16 and 17 ship as **documented non-goals**, not as stub commands. Each is a
different product (consumer billing, simulation, load-shed scheduling) whose inputs are not
NEPRA surfaces. They are disclosed in `SKILL.md` anti-triggers so an agent routes elsewhere
instead of calling us and getting nothing.

**Explicitly out of scope, with reasons:**
- **Power BI embed** — refuses every plain-HTTP probe (403 / curl 56). Viewer-only. Not budgeted.
- **`sir2023.pdf`** — text extraction yields **0 bytes**; no usable text layer. Recorded as
  unavailable, not silently skipped.
- **FY2023-24 PER** — the only published path 404s. A real hole, recorded as `UNAVAILABLE`.
- **TESCO reliability** — NEPRA excludes it on the record (unreliable metering). We show 10
  entities and say why, rather than an empty 11th row.
- **FY2010-11..FY2013-14 SAIDI** — exists only as truncated chart labels (`19,535.`); the
  trailing digits are absent from the text layer. Recorded `UNVERIFIED`, never parsed as a number.

## Transcendence (only possible with our approach)

From the novel-features subagent's adversarial cut: 16 candidates generated, 10 survived
(>= 5/10), 6 killed. Full audit trail incl. customer model, pre-cut candidates and kill
reasons is in `*-novel-features-brainstorm.md` beside this file.

| # | Feature | Command | Score | Persona | Buildability | Why only we can do this |
|---|---------|---------|-------|---------|--------------|--------------------------|
| 1 | Plant-month generation extract | `gen --fy 2023-24 [--format csv\|tsv\|json\|jsonl] [--rollup technology\|fuel]` | 9/10 | Zoya | **hand-code** | The payload's sole non-ASCII byte (0xA0, 132 occurrences) makes a naive UTF-8 read of 493,187 B report zero tables/rows/cells. We decode cp1252, expand the colspan/rowspan grid onto the frozen 32-column fingerprint, and prove `% age`-then-`GWh` order via `Sum == sum(12 GWh)` matching 118/118 vs 3/118 on percent columns. |
| 2 | Operator / ticker fleet panel | `fleet --parent HUBC \| --plant "<name>" [--fy-range 2017-18:2023-24]` | 9/10 | Zoya, Nadia | **hand-code** | Seven annual sheets share NO key — S.No is 0/106 stable into FY2023-24 (re-sorted by technology) and names mutate. Only a local store with a curated name↔alias↔parent↔ticker crosswalk produces a fleet panel; must refuse `KEL` because NEPRA writes "Kohinoor Energy Limited. (KEL)" for a 131 MW RFO plant while KEL is K-Electric's PSX symbol. |
| 3 | Conflict and break ledger | `conflicts [--surface gen\|per\|capacity] [--kind conflict\|break\|arithmetic]` | 9/10 | Nadia, Faraz | **hand-code** | Every entry is a same-key disagreement between two PUBLISHED sources: MEPCO SAIDI 3547.00 vs 1182.56 within one report (2.999425x), MEPCO 39733 vs 39.733 across four reports (exactly 1000.0000x), IESCO recovery 88.0→90 unannotated, 78 of 9,180 plant-months with GWh>0 at 0.00 load factor, Balloki Sum off by 39.56 GWh, four incompatible 30-Jun-2024 capacity figures. Only a store keeping both vintages at page-and-table granularity can print them side by side instead of silently choosing. |
| 4 | DISCO reliability panel | `disco --metric tnd\|recovery\|saifi\|saidi\|complaints\|safety [--fy 2024-25] [--variant headline\|comparison\|chart]` | 8/10 | Faraz, Nadia | **hand-code** | The ten-entity panel exists only as coordinate-bound tables in 13 unguessably-named PER PDFs — one path load-bearing on a trailing `%20`, SAIFI labelled Table 5/5/5/14/05+06/05 across six years — all with real text layers (39,416–82,813 chars), making this schema drift to cache once, not an OCR problem. |
| 5 | Regulatory determination event feed | `events [--company "Nishat Power"] [--disco LESCO] [--since 2026-01-01] [--docket TRF-71]` | 8/10 | Zoya, Imran | spec-emits | 16,405 accordion rows spanning 27 Mar 1999 → 7 Sep 2026 sit behind pages that silently truncate under a 30 s timeout without gzip (Wind: 1,028 rows truncated vs 2,440 complete, under an HTTP 200). Needs mandatory gzip, a completeness assertion, prose-date dedup (every determination listed twice) and verbatim hrefs. |
| 6 | Generation licence + capacity register | `licence [--search "Thar"] [--ticker HUBC] [--fuel Coal]` | 8/10 | Zoya, Nadia | spec-emits | 335 entities across 17 pages yield gross capacity only when key lookup is typo-aliased — exact matching on `Gross Capacity` silently drops WAPDA Hydel's 17,367.96 MW because the page says `Gross Capacityy`, and the K-Electric header reads `K-Elecric`. The 331/335 coverage assertion is the only thing that catches it. |
| 7 | Fetch, schema and staleness gate | `verify [--fy 2023-24] [--manifest] [--crosscheck iea]` | 8/10 | all four | **hand-code** | Reachability here is adversarial in ways only a stored fingerprint catches: a 9-byte 404 served as `charset=iso-8859-1`, a 9,838-byte `<frameset>` decoy returned 200 byte-identical across three fiscal years, and `SIR Data 2025.htm` serving FY2023-24 at md5 `37c27f4d…`, identical to the 2024 stub. |
| 8 | Capacity by status and system | `capacity --as-of 2024-06-30 [--by status\|system\|technology]` | 7/10 | Faraz, Nadia | **hand-code** | Four incompatible capacity numbers exist for one date; each is derivable only once every row carries a status enum (active/delicensed/decommissioned/listed_no_data) and a system enum: 40,614 MW active + 4,061 MW status rows = 44,675 MW, against 47,559.97 MW gross in the licence register. The two SIR-published figures are reported UNAVAILABLE, not approximated. |
| 9 | Fuel cost adjustment series | `fca [--entity cppag\|ke] [--cumulative] [--billing-month 2020-01]` | 7/10 | Imran, Faraz | spec-emits | The only numeric tariff surface in HTML anywhere on the site (1 table, 53 `<tr>`, 572 `<td>`, 48 gap-free months), carrying 49 accounting-parenthesised negatives that `float()` throws on or silently sign-flips — which is why the cumulative disallowance (CPPA-G 81.9748 requested vs 73.1323 allowed = 8.8425 Rs/kWh, below in 45 of 48 months) has never been computed by anyone. |
| 10 | Source catalogue + enumerator diff | `sources [--kind gen\|per\|sir\|fca\|tariff] [--diff] [--fetch <id>]` | 6/10 | Nadia, Faraz | spec-emits | NEPRA ships no working enumerator — both robots.txt sitemaps 404 with a 9-byte body, the generation directory 403s, `Main.htm` links 5 of 7 reachable years — while filenames carry load-bearing typos (`Genenration`, `DSICOs`, `(FFinal)`), a load-bearing trailing space, and case-sensitive extensions (`.pdf` 200 / `.PDF` 404 in June, reversed in July). |

### Killed at the adversarial cut (6)

| Feature | Kill reason | Closest survivor |
|---|---|---|
| `plant "<x>" --history` | Duplicate scope — identical input surface and alias table to `fleet`, differing only in aggregation. Folded into `fleet --plant`. | `fleet` |
| `ask "<question>"` | **LLM dependency at runtime** — prohibited. Also no citation path from answer back to source table/page. | `conflicts`, `disco` |
| `dashboard` | Requires the Power BI backend and a resident browser. `/public/reports/{rk}` → curl 56 empty reply; `/explore/.../modelsAndExploration` → 403, both probed twice with resource-key/Origin/Referer set. | `gen` |
| `sir --table sales\|consumers\|circular-debt` | Scope creep onto unverifiable input: 610.6 MB across ten reports at 68–164 KB/s (SIR 2025 = 316 MB), `sir2023.pdf` extracts 0 bytes, per-character kerning makes exact matching return false absences. Circular debt exists nowhere else, so it cannot be built honestly from cheap sources. | `sources` |
| `tariff-stack --disco LESCO --category B2` | **Not verifiable — emitting a rate would mean inventing one.** S.R.O. 41(1)2026 is 100 pages / 100 embedded images yielding 14,900 chars, every one a repeated registrar stamp; one of 13 near-identical SROs dated 13.01.2026 with no machine-determinable DISCO attribution. The only HTML consumer schedule is frozen at "Notified Tariff 01-01-2019". | `events` |
| `fca --extend` / `fpa --month` | Post-Jun-2022 source is OCR-corrupted and fails SILENTLY: five spellings of one entity (XWDISCOs 41x, XWDlSCOs 4x, XWDJSCOs 1x, XWDISCQs 1x), `Rs.l.2000/kWh` with letter l for digit 1, `Kl` for KE. | `fca`, `events` |

