# PCGS CLI Brief

## API Identity
- Domain: PCGS Public API — numismatic certification, grading, and population data for PCGS-graded coins/banknotes.
- Base: `https://api.pcgs.com/publicapi`
- Users: collectors verifying PCGS certs and ingesting CoinFacts metadata; dealers checking auction prices and population data; submitters checking order status.
- Data profile: cert-keyed coin records (CoinFacts), PCGS-spec/grade pairs (price guide + population), barcode lookups (cross-service), auction prices realized (APR), images, banknotes, and order/submission tracking.

## Reachability Risk
- **None.** Authoritative Swagger 2.0 spec retrieved from `https://api.pcgs.com/publicapi/swagger/docs/v1` (200, 20KB). Live probe of `GET /publicapi/coindetail/GetCoinFactsByGrade` with the user's bearer token returned 200 + valid JSON (1733 bytes) on the first call. No Cloudflare challenge for authenticated requests; unauthenticated traffic is rate-limited at the host (10,000/day shared) which surfaces as HTTP 429 returning an XML body with the message `API calls quota exceeded!`.

## Top Workflows
1. **Verify a PCGS cert is legit, extract every field.** Single-cert lookup against `/coindetail/GetCoinFactsByCertNo/{certNo}`; expose `IsValidRequest`/`ServerMessage` clearly; structured `--json` for downstream import. This is the headline workflow per VISION.md.
2. **Batch-verify and extract a list of certs.** Stdin/CSV/JSON/JSONL/plain text input → one PCGS call per cert → durable local cache → `--json` lines or table out. Fixtures already exist in `test-input-generated/` covering 11 valid and 7 deliberately-bad shapes (mixed CSV, JSON wrappers `{slabs:[…]}`/`{holdings:[…]}`, plus-grade slab IDs).
3. **Refresh cached records' mutable fields.** Re-query and update only Population/PopHigher, PriceGuideValue, AuctionList, Images, CoinFactsNotes; never overwrite cert/PCGSNo/Name/Year/Mintage/Designer/etc. Immutable-field drift is a data-integrity bug, not a refresh.
4. **Check the 1000/day quota before batching.** Persistent local counter keyed by UTC day (confirm reset window during P5 live test). Soft-warn at 80% / 95%; hard refuse with typed exit + reset-time message at 100%.
5. **Spec-number reverse lookup + grade/population sweep.** Pull every available grade row (`GetCoinFactsByGrade` for grades 1-70 with PlusGrade variants) for a given PCGSNo, build the local population curve, snapshot it. Underpins everything else — uses one batch instead of 70+ ad-hoc calls.

## Table Stakes
- Cert-based verification (`GetCoinFactsByCertNo`, `GetAPRByCertNo`, `GetImagesByCertNo`)
- Holder-barcode lookup (`GetCoinFactsByBarcode`, `GetAPRByBarcode`)
- PCGSNo + grade lookups (`GetCoinFactsByGrade`, `GetAPRByGrade`)
- Banknote variants of all three (`/banknotedetail/...`)
- Submission and order status (`/orderdetail/Get{OrdersBySubmissionNo,OrdersByDateRange}`)
- `--json`/`--select`/`--csv` output across every command (printing-press default)

## Data Layer
- Primary entities:
  - `coin` — keyed by `CertNo` (when known) AND `(PCGSNo, GradeNo, PlusGrade)` composite (cert-less rows from grade lookups).
  - `coin_image` — `CertNo → []url` (PCGS Images array)
  - `auction_record` — `(CertNo, AuctionLot)` from `AuctionList` JSON-blob in CoinFacts response and from `GetAPR*`.
  - `banknote` — keyed analogously by `CertNo` / `(PCGSNo, GradeNo)`.
  - `order` / `submission` — keyed by `SubmissionNo`.
  - `lookup_log` — every API call (timestamp, endpoint, request hash, status, IsValidRequest, ServerMessage). This is the data layer that powers the quota tracker.
- Sync cursor: per-cert `last_synced_at`, per-endpoint `last_called_at`. No upstream changelog; cache freshness drives sync decisions.
- FTS/search: text search across `Name`, `Country`, `SeriesName`, `Category`, `Designer`, `MintLocation`, `MajorVariety`, `MinorVariety`, `DieVariety`. Numeric/range search on `Year`, `Grade`, `PriceGuideValue`, `Population`, `PopHigher`, `Mintage`, `Weight`, `Diameter`.

## Codebase Intelligence
- Swagger 2.0 (not OpenAPI 3.x). Generator handles both; spec must be passed via `--spec` with a YAML/JSON path.
- **Auth missing from spec.** `securityDefinitions` is empty in the swagger. Bearer auth (`Authorization: bearer <token>`, env `PCGS_AUTH_TOKEN`) must be enriched into the spec before generation so every template (config, client, doctor, auth, README) emits correct env-var handling.
- No response schemas in the spec — every `responses` block is just `200`. Generator will produce loosely-typed responses (`map[string]interface{}` shape). Hand-written novel commands will need their own decoded types.
- Mixed parameter casing — `PCGSNo`/`pcgsNo`, `CertNo`/`certNo`. Acceptable but a public-param-audit pre-generation pass should canonicalize CLI flag spellings to `--cert`, `--pcgs-no`, `--grade`, `--plus-grade` while keeping wire names intact.
- Response envelopes: every payload includes `IsValidRequest` (bool-as-string in the wire) and `ServerMessage`. The wrapper is uniform across coin/banknote/order endpoints.

## User Vision
From `/Users/vinnypasceri/Projects/pcgs/VISION.md`:
- The CLI is **upstream of** the user's personal collection system — not a collection manager.
- A cert is "legitimate" iff the API returns a valid record (`IsValidRequest=true` + populated identity fields). The CLI surfaces that clearly.
- Capture **every field** PCGS exposes; do not curate. Importer downstream picks what it wants.
- **Auth:** `PCGS_AUTH_TOKEN` only. No other auth modes.
- **Rate limit: 1,000/day hard.** Central constraint. Track locally, persist count across invocations, warn at 80%/95%, refuse with typed exit + reset-time, expose `quota` command.
- **Mutable-only sync.** Update Population/PopHigher/PriceGuideValue/AuctionList/Images/CoinFactsNotes; never overwrite cert identity fields.
- **Typed exit codes + actionable messages** on every error path. No stack traces, no silent failures, no empty-on-error.
- Basics: single cert lookup + batch from stdin/file (CSV/JSON/JSONL/plain); quota status; local cache with `--no-cache` and `--max-age`; sync-update mutable-only.
- Local fixtures (`test-input-generated/`) and cert list (`pcgs-coin-list.csv`) are first-class — the CLI is the way the user feeds these into their collection.
- 11 PCGS-authored use-case articles included to inform novel-feature brainstorm — domain context, not API surface.
- **Out of scope:** any additional data source. PCGS only.

## Source Priority
Single source (`pcgs`). No multi-source ordering. No combo CLI.

## Product Thesis
- **Name:** `pcgs-pp-cli` (binary), `pcgs` (library slug, display name "PCGS").
- **Why it should exist:** The only ways to verify a PCGS cert today are (a) one-at-a-time pastes into `pcgs.com/cert`, (b) raw `curl` against the public API, or (c) an existing wrapper — but no community wrapper exists at all. `pcgs-pp-cli` is the first one. It bundles cert verification + full metadata extraction + batch-from-fixture + 1000/day budget enforcement + mutable-only sync into a tool that's actually pleasant to use, agent-friendly out of the box (`--json`, `--select`, typed exit codes, MCP-ready), and respectful of the central quota constraint. Upstream of a collection system, not a competitor to one.

## Build Priorities
1. **P0 — Foundation:** Swagger 2.0 spec enriched with bearer auth (`PCGS_AUTH_TOKEN`). Local store with `coin`, `coin_image`, `auction_record`, `banknote`, `order`, `lookup_log` tables. Quota tracker persisted across invocations, soft-warn at 80%/95%, typed exit 7 (rate-limit) at 100%.
2. **P1 — Absorbed (table stakes from the spec):** Cert/barcode/grade lookups for coins and banknotes; APR lookups; image retrieval; order/submission lookups. Every command supports `--json`/`--select`/`--csv`/`--cache`/`--no-cache`/`--max-age` and emits typed exit codes (0/2/3/4/5/7) per envelope state.
3. **P2 — Transcendence (only possible with this approach):** Quota-budgeting batch; spec-number population sweep; mutable-fields-only sync diff; cert-list ingest from VISION fixtures; pop-curve scarcity report; melt-vs-price advisor; lookup-log audit. Selected via the novel-features subagent (Phase 1.5c.5).
4. **P3 — Polish:** Description rewrites; README/SKILL prose; param flag canonicalization; quota/lookup-log integration tests.

## Key API Surface (12 endpoints, all GET, all return `IsValidRequest`/`ServerMessage` envelope)
- `GET /coindetail/GetCoinFactsByCertNo/{certNo}?retrieveAllData=bool` — headline endpoint
- `GET /coindetail/GetCoinFactsByGrade?PCGSNo&GradeNo&PlusGrade`
- `GET /coindetail/GetCoinFactsByBarcode?barcode&gradingService`
- `GET /coindetail/GetAPRByCertNo/{CertNo}`
- `GET /coindetail/GetAPRByGrade?PCGSNo&GradeNo&PlusGrade&StartDate&EndDate&NumberOfRecords`
- `GET /coindetail/GetAPRByBarcode?barcode&gradingService&StartDate&EndDate`
- `GET /coindetail/GetImagesByCertNo?certNo`
- `GET /banknotedetail/GetBanknoteByCertNo?certNo&languageCode`
- `GET /banknotedetail/GetBanknoteByGrade?pcgsNo&gradeNo`
- `GET /banknotedetail/GetBanknoteImagesByCertNo?certNo`
- `GET /orderdetail/GetOrdersBySubmissionNo?submissionNo`
- `GET /orderdetail/GetOrdersByDateRange?startDate&endDate&pageNo&pageSize`

Live response sample fields (from one authenticated probe call, PCGSNo=98836 GradeNo=66 PlusGrade=false):
- Identity (immutable): `PCGSNo`, `CertNo`, `Name`, `Year`, `Denomination`, `Mintage`, `MintMark`, `MintLocation`, `MetalContent`, `Diameter`, `Edge`, `Weight`, `Country`, `Grade`, `Designation`, `Designer`, `MajorVariety`, `MinorVariety`, `DieVariety`, `SeriesName`, `Category`, `CoinFactsLink`
- Mutable: `Population`, `PopHigher`, `PriceGuideValue`, `Images`, `CoinFactsNotes`, `AuctionList`
- Envelope: `IsValidRequest`, `ServerMessage`
