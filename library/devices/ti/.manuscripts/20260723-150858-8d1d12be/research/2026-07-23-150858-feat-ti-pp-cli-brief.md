# TI (Texas Instruments) CLI Brief

## API Identity
- Domain: ti.com — semiconductor manufacturer; compliance data (RoHS/REACH/MSL/packaging) per orderable part number (OPN)
- Users: the gdx cowork-agent CLI extraction lane (`lane: cli`); compliance operators verifying part status
- Data profile: per-OPN compliance attributes + supplier-published CoC/statement PDF documents

## Reachability Risk
- **None for the TI-1 scope.** All target surfaces probed 2026-07-23 with `probe-reachability`:
  - `https://www.ti.com/product/<generic>` → 200 stdlib HTTP, `mode: standard_http`. Page is
    server-rendered with a single `application/ld+json` block carrying per-OPN
    PropertyValues: `rohsCompliant`, `reachStatus`, `mslRatingPeakReflow`,
    lead finish/ball material, `packaging`, `pins`, `marketingStatusText`,
    orderable part numbers (`TUSB320RWBR.A`, ...).
  - `https://www.ti.com/lit/pdf/szzq087` (REACH statement) → 200 `application/pdf`, anonymous.
- **Login wall confirmed on materialcontent**: `https://www.ti.com/materialcontent/en/search?...`
  returns a raw 302 → `login.ti.com/as/authorization.oauth2` (myTI OAuth). The
  richer Environmental Ratings table + FMD XML (`/materialcontent/api/opn/pcid/standards`)
  used by the legacy `TIExtractor` is **NOT anonymous**. Out of TI-1 scope (TI-3 later,
  via deposited-cookie injection).
- Probe-safe endpoint used: `GET /product/TUSB320` (read-only page fetch).

## Top Workflows
1. `part compliance <opn> --json` — resolve an OPN (e.g. TUSB320RWBR) to its generic's product page, read JSON-LD, emit RoHS/REACH raw strings + description. THE lane contract.
2. Fetch the applicable blanket CoC/statement PDFs as evidence (`--coc <dir>`): RoHS `szzq088`, RoHS exemptions `szzq119`, REACH `szzq087`, Low-Halogen `szzq077`, IEC 62474 `szzq195`.
3. Batch check of several OPNs (nice-to-have; the lane calls one part per invocation).

## Table Stakes
- OPN → generic resolution (TUSB320RWBR → product page TUSB320; suffix stripping with verification against the page's orderables list, e.g. `TUSB320RWBR.A`).
- Raw-vocabulary fidelity: emit TI's strings verbatim (`Yes`) — normalization is the agent-side `MapToComplianceFacts`'s job; parity gate is `InterpretRatingsOracle`.
- Fail-closed: no RoHS **and** no REACH → non-zero exit (lane contract; matches mcmaster-pp-cli).

## Data Layer
- Primary entities: none worth a store for the lane scope. Statement PDF catalog is a static table in code (doc code → URL → what it covers).
- No sync/FTS needed; this is a deliberately narrow lane CLI like mcmaster (single `part compliance` command, framework baseline).

## Codebase Intelligence (prior art, internal)
- `agent/webextract/tiscrape/` — legacy scraper: materialcontent search/report URL shapes, cookie injection, `IsAuthRedirect`, FMD form endpoint `/materialcontent/api/opn/pcid/standards?opn&pcid&sitecode&standardsId=3`. Login-gated; reference for TI-3 only.
- `agent/webextract/ti_extractor.go:interpretRatings` — the RoHS/REACH cell-mapping oracle the CLI's raw output must reproduce through the agent's mapping.
- gdx `tools/gdxapi/cmd/ti-compliance` — Browserbase download-capture spine (TI-3 reference).
- **Vocabulary delta to flag for TI-2 parity**: the anonymous product page says `rohsCompliant: Yes` / `reachStatus: Yes` (coarse) while the login-gated ratings table has richer cell text (exemptions, SVHC sentence). Parity gate must decide whether coarse anonymous vocab maps to the same verdicts on the reference parts.

## User Vision
- TI-1 of the supplier CLI onboarding template (`thoughts/shared/plans/2026-07-23-supplier-cli-onboarding-template.md`):
  anonymous compliance CLI matching the lane:cli contract; evidence = real TI CoC/statement documents,
  NOT page renders (manufacturer pattern; page-render is the McMaster/distributor fallback).
  FMD/per-part signed certificate is TI-3 (two-phase Browserbase login; out of scope).

## Product Thesis
- Name: ti-pp-cli
- Why it should exist: makes TI a first-class supplier on the extraction lane — one command (`part compliance`), plain-HTTP, deterministic, with real supplier CoC documents as evidence; replaces two legacy in-repo TI paths after the TI-2 parity gate.

## Build Priorities
1. `part compliance <opn> --json` — OPN→generic resolution + JSON-LD parse (rohs/reach/msl/lead-finish/packaging + title), per-orderable match.
2. `--coc <dir>` (or `part coc`) — download the applicable blanket statement PDFs, emit their paths in the JSON.
3. Fail-closed exits + `doctor`.
