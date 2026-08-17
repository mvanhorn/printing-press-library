# FINRA CLI Brief

## API Identity
- Domain: FINRA (Financial Industry Regulatory Authority) public API Platform — a generic dataset-query REST API ("DAPI") fronting ~35 datasets across Equity market data, Fixed Income market data, Registration/licensing records, Firm disclosures, and FINRA content, plus small Submission (filing) and Notification (change-polling) surfaces.
- Users: broker-dealer compliance/ops teams (registration/licensing checks, Reg SHO monitoring, 4530 complaint tracking), market-data researchers (fixed income market health, TRACE), and firms submitting U4/U5/BR registration filings programmatically.
- Data profile: mix of daily/weekly time-series (Reg SHO, TRACE, fixed income breadth/sentiment) and reference/snapshot data (individual & firm registration records), all queried through one generic parameterized endpoint shape rather than bespoke per-resource REST paths.

## Reachability Risk
- Low. No evidence of IP allowlisting or non-browser blocking. Several actively-maintained third-party wrappers (Python, Node, TypeScript) function against it as of 2026. Only caution in docs is a usage-policy warning against abusive 24/7 polling patterns, not a technical block.

## API Architecture (non-obvious — read before generating)

FINRA's Query API is **one generic dataset-query surface**, not per-resource REST endpoints. The generator must model it accordingly:

| Method | Path | Purpose |
|---|---|---|
| GET | `/datasets` | Catalog of all datasets; filter by `?group=`/`?name=`. Each entry carries capability flags: `supportsGetById`, `supportsQuery`, `supportsRecordLimit`, `supportsRecordOffset`, `supportedMethods`, `securityLevel` (`PUBLIC/CONF/RCI/PCI`), `entitlement`. |
| GET | `/metadata/group/{group}/name/{name}` | Field list (`name`, `type`, `format`, `description`, `nativeName`) + `partitionFields` + `primaryIdField` for a dataset. Live schema introspection — do not hand-transcribe field lists from docs prose. |
| GET | `/partitions/group/{group}/name/{name}` | Available partition values (usually dates) — the incremental-sync mechanism. |
| GET | `/data/group/{group}/name/{name}` | List/filter via query params: `fields`, `sortFields`, `offset` (default 0), `limit` (default 1000, sync max 5000), `delimiter`, `quoteValues`, `async`. **`Accept: application/json` must be sent explicitly — default is `text/plain`.** |
| POST | `/data/group/{group}/name/{name}` | Same as above with richer body filters (`DataRequest`): `fields`, `compareFilters` (`{fieldName, fieldValue, compareType}`, compareType enum `EQUAL/NOT_EQUAL/GREATER/LESSER/GTE/LTE`), `dateRangeFilters` (`{fieldName, startDate, endDate}`, format `YYYY-MM-DD` or `YYYY-MM-DD HH:mm:ss.SSS`), `domainFilters` (`{fieldName, values[]}`), `orFilters`, `sortFields` (prefix `-` for desc), `limit`, `offset`, `async`. |
| GET | `/data/group/{group}/name/{name}/id/{id}` | Single record by ID — only when `supportsGetById: true`. |
| GET/POST async | same paths + `async: true` | Job-based for bulk (up to 100,000 records/req vs 5,000 sync); poll `GET /async-requests/group/{group}/name/{name}/{requestId}`. |

**Sorting constraint:** `sortFields` requires an `EQUAL` compareFilter on every `partitionFields` entry (from `/metadata`), and is unsupported entirely on `*Historic`-suffixed datasets.

**Response shape:** flat JSON array of records, no envelope. Headers carry pagination signal: `Record-Total`, `Record-Offset`, `Record-Limit`, `Total-Records-On-Page`.

## Auth
- OAuth2 `client_credentials`. `POST https://ews.fip.finra.org/fip/rest/ews/oauth2/access_token?grant_type=client_credentials` with `Authorization: Basic base64(clientId:clientSecret)`. Response: `access_token` (Bearer, JWT-like), `expires_in` (seconds — trust the returned value, refresh a few minutes early; docs example shows ~12h but also recommends a conservative 30-min cache — reconcile by trusting `expires_in` dynamically).
- Credentials provisioned via FINRA's API Console after an entitlement request; no self-serve public signup.
- Two environments with **separate credentials**: Production (`https://api.finra.org`) and QA/Test (`https://api-int.qa.finra.org`, free, unmetered). Generated CLI should support an env toggle (`FINRA_ENV=prod|test` or `--env`).
- Canonical env vars: `FINRA_CLIENT_ID` / `FINRA_CLIENT_SECRET` (HTTP Basic pair for the token exchange, not a single bearer token).

## Rate Limits / Usage Restrictions (exact — encode as client defaults)
- Sync: 1,200 req/min/IP; default limit 1,000, max 5,000 records/request; 3MB response cap; max `offset` 500,000.
- Async: 20 req/min/dataset/account; up to 100,000 records/request; no payload cap.
- Monthly cap: 10GB/month/credential (production only — QA/test unmetered).

## Dataset Catalog (~35 datasets, use for resource enrichment; confirm exact `group`/`name` strings live via `/datasets` at generation time — casing is inconsistent, e.g. `OTCMarket`/`FixedIncomeMarket` vs lowercase `firm`/`finra`/`registration`)

- **Equity** (10): Alternative Display Facility, Blocks Summary, Consolidated Short Interest, Monthly Summary, OTC Block Summary, OTC Daily List, Over-the-Counter Reporting Facility, Reg SHO Daily Short Sale Volume, Threshold List, Weekly Summary.
- **Fixed Income** (10): Agency Debt Market Breadth, Agency Debt Market Sentiment, Corporate 144A Debt Market Breadth, Corporate 144A Debt Market Sentiment, Corporate And Agency Capped Volume, Corporate Debt Market Breadth, Corporate Debt Market Sentiment, Securitized Product Capped Volume, TRACE, Treasury Daily/Monthly Aggregates.
- **Registration** (18, largest family): Accounting, Altered SSN and DOB, Branch Delta, Branch List, Broker Dealer Firm List, Composite Branch, Composite Individual (+ Seed), Firm Disclosures, Firm Profile, Firm Registration Status History, Firm Registrations, Individual Delta, Individual Fingerprint, Individual Pre-Registration Search (v1 only — **v2 is deprecated, exclude**), Individual Registration Validation (+ Details), Registered Individual Search, U4 Form Prefill.
- **Firm** (1): 4530 Customer Complaints.
- **FINRA Content** (2): FINRA Rulebook, Industry Snapshot: Firm Registration Types.
- **Notification API** (2, polling model not webhooks): FINRA Rulebook Notification, Draft Registration Filing.
- **Submission API** (5, schemas not publicly documented — treat as best-effort/gap): BR, Create Individual, NRF, U4, U5.
- **FileX**: bulk file transfer (SFTP/HTTPS/S3), no documented REST surface — out of scope for command generation beyond a thin status-check stub if one surfaces.

**Do not generate against `*Mock`-suffixed dataset names** — those are API Explorer docs fixtures; real production names have `Mock` stripped.

## Top Workflows
1. Registration/licensing check — look up an individual by CRD via `registration/registrationValidationIndividual/id/{crd}` or Registered Individual Search / Composite Individual.
2. Reg SHO short-volume trend tracking — pull `OTCMarket/regShoDaily` by symbol + date range, track short/total volume ratio over time (local time-series + analytics command).
3. Firm 4530 complaint monitoring — poll `firm/4530filings`, diff against last-synced filing ID/date to surface only new complaints.
4. Fixed income market-health snapshotting — TRACE / Corporate Debt Market Breadth / Agency Debt Market Sentiment for daily/weekly market-condition summaries.
5. U4/U5 bulk filing submission + status polling (gap: exact payload schema unconfirmed; stub with explicit disclosure).

## Data Layer
- Primary entities: time-series datasets (Reg SHO, TRACE, fixed-income breadth/sentiment/capped-volume) synced by partition (usually date field, discovered via `/metadata` `partitionFields` + `/partitions`); registration/firm reference data (Composite Individual, Firm Profile, Broker Dealer Firm List) snapshot-and-refresh, with `*Delta` datasets (Individual Delta, Branch Delta) for incremental sync.
- Sync cursor: last-synced partition value per dataset (date-based for time series); delta-dataset cursor for registration records.
- FTS/search: individual/firm names, symbols, filing descriptions.
- Not sync-worthy: FINRA Rulebook (static reference — candidate for `pp:novel-static-reference`), one-off ID lookups.

## Codebase Intelligence
- FINRA's actual client-facing contract lives in an embedded Swagger 2.0 spec inside each API Explorer doc page's HTML (`drupalSettings` JSON blob), not in the rendered prose. Extracted and saved at `finra-dapi-swagger-extract.json` in this research dir — use it as the authoritative schema source for spec authoring (`DataRequest`, `Dataset`, `DatasetMetaData`, `CompareFilter`, `DateRangeFilter`, `DomainFilter`, `Field` definitions).

## Source Priority
- N/A — single source (FINRA official API).

## Product Thesis
- Name: FINRA CLI (`finra-pp-cli`)
- Why it should exist: No existing tool covers FINRA's Query API broadly — every competitor (chencindyj/finra_api_queries, nikhilxsunder/finra, samgozman/finra-short-api, cmaurer/finra-mcp-server) is Python/TypeScript and narrowly scoped to one dataset family (short interest, or a handful of hand-picked datasets). None cover Submission or Notification APIs. This would be the first Go CLI, the first to treat the dataset catalog as a generically-discoverable surface (via `/datasets` + `/metadata` + `/partitions`), and the first with offline SQLite-backed trend analysis across Reg SHO/TRACE/registration data.

## Build Priorities
1. Generic dataset query engine (group/name-based GET/POST, metadata-driven field discovery, partition-aware incremental sync) — this is the foundation everything else sits on.
2. Friendly per-workflow commands wrapping the generic engine for the highest-value datasets: Reg SHO, TRACE, Composite Individual / Registration Validation, 4530 Complaints, Fixed Income breadth/sentiment.
3. Transcendence: local trend/anomaly analytics across synced time series (short-volume spikes, complaint-filing surges) that no existing wrapper offers because none persist history locally.
