# servicetitan-pricebook-pp-cli — Build Log

## What was generated (Phase 2)
- `printing-press generate` (v4.6.1) from the enriched `tenant-pricebook-v2.enriched.json` spec.
- 40 endpoint commands across 10 resources (categories, client-specific-pricing, discounts-and-fees, equipment, pricebook-export, images, materials, materialsmarkup, pricebook bulk, services). The `export` resource was auto-renamed to `pricebook-export` to avoid shadowing the framework `export` command.
- MCP server (stdio+http, code orchestration, hidden endpoint tools — from the spec's `x-mcp` block).
- All generator quality gates passed first try (go mod tidy, govulncheck, go vet, go build, --help, version, doctor).

## Generator-bug patches applied (carry-forward from servicetitan-inventory retro)
The v4.6.1 generator wired the OAuth2 bearer half of composed auth but **not** the apiKey half or the sync registry — the #1303/#1305/#1332 carry-forward. Patched from the sibling template:
- **`internal/config/config.go`** — added `StAppKey` + `TenantID` fields, `ST_APP_KEY` / `ST_TENANT_ID` env reads, defensive `TrimSpace` on all four ST credentials (the known JKA `invalid_client` whitespace gotcha).
- **`internal/client/client.go`** — set the `ST-App-Key` header on every live request and in the `--dry-run` preview (the bearer half was already wired by v4.6.1).
- **`internal/cli/sync.go`** — populated `defaultSyncResources()` (7 cacheable resources) and `syncResourcePath()` with `{tenant}` substituted from `ST_TENANT_ID`. Resource names aligned to the strings the generated get-list commands use (`materialsmarkup`, `clientspecificpricing` — no hyphens).
- **`internal/cli/doctor.go`** — recognize composed auth from the credentials present (the OAuth bearer is minted lazily, so `AuthHeader()` is empty at doctor time); check all four required env vars.

Live-verified after patching: `doctor` reports Auth configured / Env Vars 4/4; `categories get-list` returned real JKA data; `sync` pulled 426 records across 7 resources.

## What was built (Phase 3)
**Foundation package `internal/pricebook/`** (hand-written, the data layer for all transcendence features; built, vetted, table-driven tests pass):
- `skus.go` — typed views (Material/Equipment/Service/Category/MarkupTier/SkuVendor/Warranty), store loaders, `CategoryRefs` resilient unmarshal, `StoreEmpty`.
- `markup.go` — `EvalTier`, `ExpectedPrice`, `ActualMarkupPercent`, `MarkupAudit`, `Reprice`.
- `costhistory.go` — `sku_cost_history` change-log table, `Snapshot` (change-only inserts), `CostDrift`.
- `match.go` — `Normalize`/`NormalizeTight`/`Tokens`/`Jaccard`/`Levenshtein`/`LevenshteinRatio`/`Similarity`/`TokenCoverage`/`PartMatch`.
- `audits.go` — `VendorPartGaps`, `WarrantyLint`, `OrphanSKUs`, `CopyAudit`, `Health`.
- `dedupe.go` — `Dedupe` (union-find clustering on fuzzy + exact-vendor-part match).
- `find.go` — `Find` (ranked NL part finder).
- `quote.go` — `ParseQuoteFile` (CSV/JSON), `Reconcile`, `BulkPlan`, `BulkUpdatePayload`.

**12 transcendence commands** in `internal/cli/` (thin wrappers over the foundation, registered in root.go):
markup-audit, cost-drift, vendor-part-gaps, warranty-lint, orphan-skus, copy-audit, dedupe, find, health, quote-reconcile, reprice (`--apply`), bulk-plan (`--apply`). Shared scaffolding in `pricebook_cmd.go`. All follow the verify-friendly RunE pattern (`dryRunOK` guard, positional-arg `cmd.Help()` fallback, `StoreEmpty` actionable error); `reprice`/`bulk-plan` add the `cliutil.IsVerifyEnv()` side-effect guard before any write.

Live-verified: all 12 command paths resolve via `--help`; `health`/`markup-audit`/`vendor-part-gaps`/`warranty-lint`/`find` all return correct, meaningful results against the synced JKA pricebook (199 markup-drift SKUs, 29 vendor-part gaps, 100 warranty issues, 286 orphan SKUs, 28 duplicate clusters).

## Codex delegation
The 12 command files were originally delegated to Codex (the run was invoked in `codex` mode). Codex returned `ERROR: Quota exceeded` — a hard billing block, not a code failure. Per the circuit-breaker intent, switched to standard mode and wrote the command files directly. No partial Codex output to revert.

## Intentionally deferred / known
- `sync` pulls 100 each of materials/equipment/services (one page). `sync --full` / `--max-pages 0` fetches the rest; the novel audits operate on whatever is synced. Documented behavior, not a bug.
- ST service descriptions contain HTML markup (`<b>`, `&mdash;`). `find`/`copy-audit` operate on the raw stored text; HTML adds minor token noise but does not break matching. Candidate for a `cliutil.CleanText` pass in polish if flagged.
- `reprice --apply` / `bulk-plan --apply` write paths are built but were NOT live-tested (write operations excluded from this run's testing per the API-key gate).

## No generator limitations blocked the build
Generation, build, and all quality gates were clean; the only manual work was the known composed-auth/sync carry-forward patches and the hand-built transcendence layer (which the generator never produces).
