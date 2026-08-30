Manifest transcendence rows: 8 planned, 8 built. Phase 3 gate PASSED.

# PSX CLI — Phase 3 Build Log

Planned transcendence rows (all `hand-code`, none stubs):
1. actions   — corporate-action digest
2. diff      — snapshot delta
3. watchlist — local watchlist
4. unusual   — baseline-relative anomaly scan
5. rotation  — sector rotation
6. drift     — valuation drift
7. basis     — futures basis
8. docs search — regulatory document search (www.psx.com.pk)


## Built

**Shared infrastructure (hand-authored)**
- `internal/psx/table.go` — header-name-driven HTML table parser (x/net/html). Keys every row by
  its own `<th>` text, never by column position, because PSX reorders columns without notice and
  positional parsing fails silently with plausible numbers. 7 unit tests incl. a column-reorder
  regression test.
- `internal/psx/client.go` — rate-limited sibling client. Carries the portal's required headers,
  seeds `cliutil.AdaptiveLimiter` at the community-observed 2 req/s ceiling, and converts HTTP 429
  into a typed `*cliutil.RateLimitError` so throttling is never mistaken for "no data".
- `internal/store/extras.go` — `psx_watchlist` and append-only `psx_snapshots` tables.
- `internal/cli/psx_snapshot.go` — snapshot capture/read substrate + `snapshot take|list`.

**Absorbed surfaces (hand-authored; see Deviations)**
market watch/performers/status, quote, history, screener, debt, eligible-scrips, indices,
circuit-breakers, listings, board, announcements, payouts, company reports, sectors summary/top.

**Transcendence (8/8)**
actions, diff, watchlist (add/remove/show), unusual, rotation, drift, basis, docs search.

## Deviations from plan

**HTML surfaces moved from generated to hand-authored.** The generator's `response_format: html`
supports only `page | links | embedded-json` — there is no table-extraction mode, and `cliutil`
ships no table parser. Generated commands for the ~14 table-backed endpoints returned page/link
metadata (and `history` returned `{}`), i.e. wrong data. The spec was narrowed to JSON-only
endpoints and every table surface was hand-authored against `internal/psx`. No approved feature
was dropped; only the implementation mechanism changed. This was anticipated as Build Priority 3
in the brief. Hand-authored commands still reach MCP via the cobratree mirror.

**`sectors top` restored explicitly.** The generator promotes a lone endpoint onto its parent, so
`sectors` served top-10 data while the manifest-approved `sectors top` path did not resolve. Added
as an explicit subcommand rather than editing the manifest.

## Findings for retro (machine-level)

1. `generate --force` preserved a `DO NOT EDIT` generated file (`internal/cli/learn_init.go`)
   across a spec change, silently discarding a `learn.ticker_patterns` edit. The fix appeared not
   to work until the working tree was deleted and regenerated clean.
2. No HTML `<table>` extraction mode exists despite `response_format: html`; table-backed APIs
   therefore cannot use generated endpoint commands at all.
3. `crowd-sniff` returned nothing: npm downloads API 400, `ccxt`/`romdevtools` skipped on the
   10 MB tarball limit, and it does not mine PyPI where this API's ecosystem lives.
4. A generated framework test (`TestLearnEvents_AliasMediatedRecallCreditsTaughtRowID`) fails for
   any CLI whose `learn.ticker_patterns` match the 2-char token `WC` used as an org alias fixture.

## Evidence

- Per-row Cobra resolution: 32/32 manifest command paths resolve.
- `dogfood --json .novel_features_check`: planned 8, found 8, missing [].
- `go test ./...`: 13 packages ok.
- EOD tuple 4th element confirmed as **open** (not VWAP) by cross-checking OGDC 2026-08-19
  against the `/historical` table; the brief's open question is resolved from evidence.
