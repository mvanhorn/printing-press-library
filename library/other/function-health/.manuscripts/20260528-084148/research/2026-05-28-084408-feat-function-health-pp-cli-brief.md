# Function Health CLI Brief

## API Identity
- **Domain:** Consumer longevity/preventative health. Function Health is a $499/yr membership offering 100+ blood biomarkers (heart, hormones, thyroid, cancer signals, nutrients, heavy metals, metabolism, liver, kidney, pancreas) at Annual Test + 60+ at Mid-Year Test, with on-demand individual biomarker draws between rounds. Optional add-ons: Galleri cancer screening, MRI/CT.
- **Users:** ~250k+ paying members. Power users include Mark Hyman audience, longevity enthusiasts, biohackers, quantified-self practitioners. Adjacent: clinicians who get PDFs from patients.
- **Data profile:** Per member, multiple **test rounds** (grouped by `requisitionId`), each containing many **results** (biomarker measurements with value, unit, status, Quest reference range, Function "optimal" range), categorized into ~13 medical categories. Each result has history across rounds. Plus: biological age, BMI, personal notes, recommendations, full clinician report per visit.
- **No official API.** All four community tools (daveremy/function-health-mcp, bogini/function-health-exporter, Greenband1/biomarker_chrome_extension, LabSaver) use a reverse-engineered Firebase-authenticated REST API at `my.functionhealth.com/api/v1/*`. Core endpoint: `/api/v1/results-report`.

## Reachability Risk
- **Low.** `printing-press probe-reachability` returned `standard_http` (confidence 0.95) — no Cloudflare/WAF/DataDome/PerimeterX. Plain stdlib HTTP gets a 200 from the marketing root; the SPA shell renders for unauthenticated `/api/*` paths.
- **Auth fragility risk: MEDIUM.** daveremy/function-health-mcp issue #22 (opened 2026-05-01) reports Firebase id token refresh failing with `API_KEY_INVALID` after the 1-hour token expiry. Active competitor is currently broken on long-running sessions. Our CLI must (a) refresh tokens correctly, and (b) fall back to re-login gracefully when refresh dies. This is a competitive opening.

## User Vision (from briefing)
- **Historical biomarker store + comparison.** Pull every lab round into the local store. Query and compare any biomarker across rounds over time. This is the foundation requirement — without longitudinal history nothing else has compounding value.
- **Branded shareable PDF for doctors.** Render a PDF of the user's lab history with "Function" branding plus the user's name and date of birth, suitable for emailing to their personal physician. This is the headline export — the user has personal MD-facing workflows for which a Function-branded report carries authority a generic JSON dump doesn't.

## Top Workflows
1. **Initial pull + ongoing sync.** Member signs in once; CLI pulls every test round, every biomarker, every recommendation, persists locally. Subsequent syncs are incremental ("check for new results").
2. **Biomarker deep-dive.** "Show me ApoB across every draw." "What's my ferritin trend?" "Which of my biomarkers are out of Function's optimal range right now?"
3. **Round-over-round comparison.** "What changed between my Apr 2026 draw and my Oct 2025 draw?" — both directions, improvements + declines, with significance gating.
4. **LLM-ready export.** Power users feed labs into ChatGPT/Claude for second-opinion reading. JSON + Markdown export, per-category bundles, per-biomarker history files.
5. **Cross-cutting trend analysis.** "Show me every biomarker that's trended worse over the last 3 rounds." "Which biomarkers are oscillating in/out of optimal?" "What's my cardiovascular category trend?" — needs local SQL across all rounds.

## Table Stakes (must absorb from competitors)
From **daveremy/function-health-mcp** (13 MCP tools, full CLI):
- `login` (email/password → Firebase JWT)
- `status` (auth + data + last sync)
- `sync [--force]` (full pull)
- `check` (lightweight new-results probe)
- `results` (query with filters: category, biomarker, status, round)
- `biomarker <name>` (per-biomarker deep dive with full history)
- `summary` (overview incl. biological age, BMI)
- `categories` (list with counts)
- `changes [--from] [--to]` (round-over-round delta)
- `recommendations` (health guidance per category)
- `notifications` (read/ack change notifications)
- `report <visit>` (full clinician report)
- `export [--markdown]` (LLM-ready)

From **bogini/function-health-exporter** (CLI):
- `export` (19+ JSON files: profile, results, biomarkers, recommendations, reports, bio-age, BMI, notes)
- `markdown` (17 per-category health files for LLM input)
- `config` (persistent settings)
- `--max-biomarkers`, `--save-credentials`, retry+backoff, rate-limit delay

From **Greenband1/biomarker_chrome_extension** (Chrome extension):
- CSV/JSON/clipboard export
- Quest biomarker ID + LOINC-friendly fields
- Side-by-side Quest reference range AND Function optimal range
- Direction indicators (above/below)
- Date-range and category filtering, "latest only" view

## Data Layer
- **Primary entities:**
  - `members` — single row (you), with bio age, BMI, profile fields
  - `test_rounds` — keyed by `requisitionId`; round_type (annual/mid-year/on-demand), draw_date, status
  - `results` — biomarker measurements (round_id, biomarker_id, value, unit, status, quest_range_low/high, optimal_range_low/high, direction, category_id)
  - `biomarkers` — catalog (name, quest_test_code, category, description, units)
  - `categories` — ~13 medical categories (cardiovascular, metabolic, thyroid, etc.)
  - `recommendations` — per-category guidance, linked to rounds
  - `notifications` — change events (improved/declined/new)
  - `reports` — full clinician narrative per visit
  - `notes` — user-authored notes on biomarkers/rounds
- **Sync cursor:** `requisitionId` per round; round-level "last-modified" timestamp; recommendations updated_at.
- **FTS/search:** FTS5 over biomarker names, descriptions, categories, and clinician report narrative text. Enables `function-health search "thyroid"` and `function-health search "hs-CRP"` across all rounds and reports.
- **Cross-round joins (the killer feature):** SQLite makes trend analysis trivial. Every transcendence command is `SELECT … FROM results JOIN test_rounds … ORDER BY draw_date` — impossible in the JSON-file model the two competitors use.

## Codebase Intelligence
- **daveremy/function-health-mcp** is the most mature competitor; uses Firebase Auth REST endpoints + `my.functionhealth.com/api/v1/*` with 250ms rate-limit spacing, atomic writes to `~/.function-health/`, JSON-file persistence (no DB). Reads test rounds grouped by `requisitionId`. Optional recommendations endpoint. Reverse-engineered; `docs/api-reference.md` in repo.
- **bogini/function-health-exporter** uses the same Firebase JWT pattern, exposes `--save-credentials`, exponential backoff retry, supports `--max-biomarkers` for partial pulls.
- **Greenband1's extension** confirms `/api/v1/results-report` as a primary endpoint and shows the API returns both Quest reference ranges AND Function's proprietary optimal ranges in the same payload.
- **Auth model:** Firebase Auth (email/password) → ID token (1-hour TTL) + refresh token. Tokens go in `Authorization: Bearer <id_token>` header. Refresh via Firebase `securetoken.googleapis.com/v1/token` exchange. The 1-hour expiry + refresh-token revocation is the active bug in daveremy/#22.
- **Rate limits:** No published limits; competitors use 250ms client-side spacing. We'll do the same with `cliutil.AdaptiveLimiter`.

## Product Thesis
- **Name:** `function-health` (binary `function-health-pp-cli`; module under `mvanhorn/function-health-pp-cli` per Printing Press convention; library directory `~/printing-press/library/function-health`).
- **Why it should exist:** Every existing tool (1 CLI, 1 exporter, 1 Chrome extension, 1 web app integration) stops at "give me the data as JSON or PDF." None of them put your labs in a **queryable local SQLite store with FTS5 across years of draws.** None offer round-over-round drift detection that joins across the full history. None handle Firebase token refresh correctly (daveremy is currently broken on that). And none expose the surface as an **agent-native CLI** — `--json`, `--select`, typed exit codes, dry-run, MCP server with read-only annotations on every lookup tool — so Claude/ChatGPT can answer "what's happened to my ApoB in the last 4 draws and is it trending toward your optimal range?" without you uploading a PDF.

## Build Priorities
1. **Foundation (P0):** SQLite schema for all 8 entities; sync command that pulls every round, biomarker, result, recommendation, report; FTS5 indexes; Firebase JWT auth with **working refresh** + auto re-login on `API_KEY_INVALID`.
2. **Absorb (P1):** All 13 daveremy commands, all 4 bogini commands, all Chrome-extension export shapes (CSV/JSON/clipboard with Quest+optimal ranges side-by-side).
3. **Transcend (P2 — local SQL is what enables these):** biomarker trend analysis across all rounds, "drift toward optimal" scoring, oscillation detection, category-level health-score timelines, reorder-cadence reminder ("you usually draw every N months; last draw was M months ago"), LLM-ready bundle composer (Markdown report for any biomarker+window), and a `goat` command that picks the single most worrying biomarker right now given trend + range distance.
