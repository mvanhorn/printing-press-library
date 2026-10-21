Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

# Build log — pinterest-pp-cli reprint (run 20261021-092559-b7e82418)

## Slice 0 — generation
- Generated with cli-printing-press @main 5a59a79 (v4.33.8 + #5131 go1.26.10 fix). 266 typed endpoints; Cloudflare MCP pattern auto-applied (search/read/execute + cobratree tools).
- Auth enrichment: PINTEREST_ACCESS_TOKEN canonical, PINTEREST_OAUTH2 fallback (both optional per_call).
- Prior-patch regressions found in fresh tree: token exchange/refresh send client_secret in body (needs HTTP Basic); callback 127.0.0.1:8085 (needs localhost:8875); no continuous_refresh; defaultSyncResources = resources/resources-delivery-metrics (needs bounded pins/boards).

## Slice 1 — prior-patch truths (Codex task A, verified by Claude)
- internal/cliutil/oauth_token.go: NewPinterestOAuthTokenRequest (HTTP Basic for confidential clients; client_id in body only for public/PKCE).
- internal/cli/auth.go: redirect http://localhost:<port>/callback, default --port 8875, continuous_refresh=true on code exchange.
- internal/client/client.go: refresh grant via the Basic-auth helper.
- internal/cli/sync.go: defaultSyncResources = boards, pins.
- doctor: env vars already reported optional (PINTEREST_ACCESS_TOKEN / PINTEREST_OAUTH2); unchanged.
- Tests: internal/cli/auth_pinterest_test.go, client_test.go refresh Basic-auth assertion.

## Slice 2 — local novel commands (Codex task B partial → Claude rewrite; Codex failure 1/3)
- pins audit (local), pins preflight (computed), trends gaps (live + local FTS). Shared helpers internal/cli/pinterest_local.go.
- Codex bugs fixed: trends response envelope is {"trends":[...]} not items; --group-by board unimplemented; missing-DB detection; nondeterministic error ordering; section resolution.
- Behavior tests: content assertions incl. empty store, missing DB, issue filter, field limits, stdin batch, FTS coverage via httptest.

## Slice 3 — live novel commands (Codex task C; Codex failure 2/3 on tests)
- perf compare, pins funnel, ads report.
- Claude fixes: ads report polling used cached GET (infinite IN_PROGRESS) → GetNoCache; tests string-matched pretty JSON → parsed; removed t.Skipf on loopback failure; case-insensitive --level/--granularity.
- Report download uses a separate http.Client with no Authorization header (test asserts).

## Gates
- go build/vet/test -count=1 ./... green.
- Per-row Cobra resolution: all 6 novel + auth login/sync/search resolve as leaf commands.
- dogfood novel_features_check: planned 6, found 6 (depth note: ads report vs ad-accounts ads — informational).
- Priority 1 review gate: boards list / pins get / user-account get — help examples, dry-run no request, stdout valid JSON.

## Deferred / notes
- Cache auto-refresh (x-cache 6h) triggers a bounded pins/boards refresh before stale reads; opt out with PINTEREST_NO_AUTO_REFRESH.
- 52 request bodies use --body-json fallback (bare arrays / oneOf) — generator limitation, unchanged.
- Generator limitation found: v4.33.8 go.mod template go1.26.9 (GO-2026-6809); used press@main (#5131).
