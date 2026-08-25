# Screener.pp CLI Build Log

Manifest transcendence rows: 5 planned, 5 built. Phase 3 complete.

## Priority 0 (foundation)
- Generated CLI with spec-driven resources: company (search/chart/peers/profile/profile-standalone/by-id), screens, explore, market, ipo, results, trades, filings, full-text-search
- Local SQLite store (sync/search/sql), learn/recall/playbook loop, auth login --chrome, doctor, export, workflow
- Cookie auth for gated pages (sessionid + csrftoken)

## Priority 1 (absorbed)
- All 23 absorbed features from the manifest: company search JSON, chart JSON, peers table, profile page (HTML parse), screens, explore, market sectors, IPO, results (auth), insider trades (auth), filings (auth), FTS (auth), consolidated/standalone views
- Framework: sync/search/sql, learn loop, auth, doctor, export, workflow

## Priority 2 (transcendence) — 5/5 built
1. **compare** — side-by-side fundamentals for 2-4 companies (live HTML parse: top-ratios, quarterly, shareholding, analysis). `internal/cli/compare.go`
2. **qtrend** — quarterly trend with YOY change, margin drift, acceleration flags. `internal/cli/qtrend.go`
3. **overlap** — intersect 2+ screen result tables by symbol. `internal/cli/overlap.go`
4. **rank** — re-score a screen with composite fundamentals. `internal/cli/rank.go`
5. **insider-flow** — aggregate insider trades into net per-company flows (auth-gated). `internal/cli/insider_flow.go`
- Shared HTML parser: `internal/cli/screener_parse.go` (top-ratios, fin tables, analysis, shareholding, screen tables)
- Registration: `internal/cli/novel_register.go`

## What was intentionally deferred
- None. All approved transcendence features shipped.

## Skipped body fields
- None (all GET endpoints, no request bodies).

## Generator limitations found
- `learn.ticker_patterns` with `^[A-Z0-9]{2,8}$` breaks the generated TestLearnEvents_AliasMediatedRecallCreditsTaughtRowID (matches "WC" alias). Workaround: `{3,8}`. Possibly a generator test-isolation issue with non-default learn configs.
- Go 1.26.5 stdlib has 5 vulns (net/url, crypto/tls, net/http, encoding/asn1); govulncheck gate fails. Fixed by upgrading to Go 1.26.6 + go.mod `go 1.26.6`.
