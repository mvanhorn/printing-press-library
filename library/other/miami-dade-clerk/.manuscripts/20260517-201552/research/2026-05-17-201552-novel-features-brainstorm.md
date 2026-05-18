# Novel Features Brainstorm — miami-dade-clerk

Spawned 2026-05-17 via Phase 1.5c.5 mandatory subagent (general-purpose).

## Customer model

### Persona 1: Alex Kleis — Foreclosure investor preparing Tuesday auction bids (BlueStone Capital)

**Today (without this CLI):** Three tabs on the Miami-Dade Clerk portal (Property search, Name search, document viewer), one on realforeclose.com case docket, one on PA folio page. For each of ~30 properties on Tuesday's calendar, manually pulls deeds by address, then for each grantee runs a separate Name search to enumerate mortgages/lis pendens/FTLs. Copies CFNs into a spreadsheet and eyeballs whether each lien has a satisfaction. Cannot answer "total surviving liens on this folio" without a $150-300 O&E from a title company.

**Weekly ritual:** Sun/Mon lien-due-diligence sweep: for every active FC or TD case on the upcoming calendar, pull the full lien chain, determine Max Safe Bid, feed into v3 underwriter.

**Frustration:** Reconstructing the lien chain. Property mode returns ONLY deeds; everything else (mortgages, liens, sats, lis pendens) is indexed by name. Has to deed-walk back through prior owners and run bounded Name search per owner, then manually filter back to target via subdivision+plat+block signature. 20-40 clicks, 15-30 minutes per address.

### Persona 2: Sarah — Title underwriter doing rush O&E reports for a small Miami title firm

**Today:** Same portal as Alex, plus paid vendors (TitlePoint, DataTrace) at $20-50/property. Each folio is one human session. Closing agents need 60-year chain of title + open liens within 24-48 hours.

**Weekly ritual:** 15-30 title orders/week. Each needs deeds-to-root (often 30+ years), open mortgages, open lis pendens, FTL/IRS liens, judgments by current owner, easements, restrictions.

**Frustration:** Vendor cost eats margin on small files; data still needs human re-keying into title commitment form. For folios not in vendor indexes (new subdivisions, condos), falls back to the free portal → same 20-40-click ordeal as Alex.

### Persona 3: A v3 frontend AI underwriter (downstream automation, not a human)

**Today:** v3 modal pulls `properties.underwriting_report` from Supabase. TD underwriter computes `uw_td_govt_liens` from a proxy (currently always 0 or hand-entered) — no machine-readable FTL/judgment feed per folio. "Surviving Liens" stat empty for 90% of TD cases.

**Weekly ritual:** Every property modal open renders surviving-liens panel; every `_stage-ai` run passes recordings to Gemini for narrative. Both need stable JSON: cents not dollars, ISO-8601 dates, `cfn_master_id` as primary key, viewer URL per record.

**Frustration:** No feed exists. Every recording on every folio gated behind single-use reCAPTCHA + 500-row cap. v3 modal stays empty until a CLI produces ingestible JSON keyed by folio.

## Candidates (pre-cut)

15 candidates generated across sources (a) (b) (c) (e); 7 killed inline. Full table preserved in the subagent's output and rendered in the absorb manifest below.

## Survivors and kills

### Survivors (transcendence features)

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Lien-chain reconstruction | `lien-chain --folio <folio>` | 10/10 | hand-code | Read deeds at folio from local SQLite → extract grantees → bounded Name search per grantee → filter back to target folio via subdivision+plat+block signature → unified timeline JSON | Brief Top Workflow #3 (killer feature); user vision explicit |
| 2 | Surviving-liens calculator | `surviving-liens --folio <folio>` | 10/10 | hand-code | Pair each MOR/LIE/FTL/JUD/LIS against SAT/REL by `linK_DOCTYPE`; static survivability table (FTL survives TD; junior MTG wiped on senior FC; HOA capped at FL Stat 720/718) | Brief Top Workflow #5; v3 `uw_td_govt_liens` column currently empty |
| 3 | Chain of title | `chain-of-title --folio <folio> [--since YYYY-MM-DD]` | 9/10 | hand-code | Local query on deeds (DEE/QCD/DAM/DM/ODE), ordered by recording_date; gap detector flags grantee-to-next-grantor mismatches | Brief Top Workflow #1; Sarah's 60-year chain standard deliverable |
| 4 | Owner portfolio scan | `owner-portfolio --name "<owner>"` | 8/10 | hand-code | Local FTS query on party fields, grouped into (a) folios on which owner appears, (b) active mortgages/liens taken, (c) lis pendens/judgments against | Brief Top Workflow #4; user vision killer feature |
| 5 | Litigation arc tracker | `case-arc --case <case-number>` | 7/10 | hand-code | Local query on `case_number`, ordered by date; classifier walks doc-type sequence (LIS→pending, JUD→entered, CTI→sale-complete, voluntary-dismissal→cancelled) | Brief data layer; v3 `foreclosure_cases` table parallel |
| 6 | Folio-batch enrichment | `enrich --folio-list folios.csv [--out lien-summary.json]` | 9/10 | hand-code | Orchestrator: for each folio in CSV, run lien-chain + surviving-liens + chain-of-title; emit one row with totals_cents, surviving_lien_count, oldest/last deed dates, current owner, FTL count | User vision: Supabase ingest pipeline; Alex's Tue-morning auction-prep |
| 7 | FTL hot-list | `ftl-scan --since YYYY-MM-DD [--folio-list folios.csv]` | 7/10 | hand-code | Local filter on `doc_type_code='FTL'` recorded after a date, optionally joined to folio watchlist via signature party match; returns owner + IRS amount + date | Brief: "FTL survives TD foreclosure"; v3 `uw_td_govt_liens` has no upstream feed today |
| 8 | Document downloader | `download-pdf <cfn_master_id> [--out ./pdfs]` | 6/10 | spec-emits | Resolves viewer URL from local row, fetches scanned PDF via Surf transport with browser-clearance session, writes to disk | Brief Build Priority 3; Sarah's title-file workflow needs the recorded image |

### Killed candidates

| Feature | Kill reason | Closest sibling |
|---------|-------------|---------|
| `sync-incremental` | Duplicate of absorb manifest #11 (`sync`) | `sync` (absorbed) |
| `watch --folio-list` | Scope creep — requires daemon/cron | `enrich --folio-list` |
| `breakdown --folio` | Thin reshape — agent computes with `jq` | `lien-chain` |
| `current-owners --folio` | Trivial subquery of last `chain-of-title` row | `chain-of-title` |
| `find-folio --address` | Duplicates absorb manifest #2 | `search-property --address` |
| `subdivision-sweep` | Fails weekly-use test (Alex never; Sarah occasionally) | `owner-portfolio` |
| `my-liens --name` | Wrong persona (county resident); identical mechanics | `owner-portfolio` |

## Hand-code count (Phase 1.5 gate commitment)

**7 hand-code + 1 spec-emits = 8 transcendence features.** The hand-code 7 are the Phase 3 build commitment: ~50-150 LoC per feature + `root.go` wiring + Cobra files.
