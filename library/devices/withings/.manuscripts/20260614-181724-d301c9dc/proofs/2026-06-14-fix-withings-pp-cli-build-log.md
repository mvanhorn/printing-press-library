# Withings CLI — Build Log

Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

## Built
**Foundation (hand-authored, durable extension files):**
- `internal/client/withings_call.go` — `WithingsForm` (form-encoded POST, `{status,body}` envelope unwrap, envelope-tolerant for verify mocks, auto-refresh on 401-class status, 429/5xx retry, dry-run).
- `internal/client/withings_token.go` — OAuth2 `ExchangeAuthCode` + `RefreshAccessToken` (action=requesttoken, single-use refresh rotation persisted).
- `internal/cli/withings_oauth.go` — `auth login` (localhost-callback + `--print-url`/`--code` manual flow, verify-friendly) + `auth refresh`; wired into `auth` parent (auth.go).
- `internal/cli/withings_decode.go` — measure type-code names + mantissa `scaleMeasure`, AFib/sleep-state/workout-category/appli decoders (+ tests).

**Priority 1 (absorbed) — rewired all 14 endpoint commands to `WithingsForm`** (form-encoding fix; the generic client JSON-POSTs which Withings rejects): measure, activity get/intraday, workouts, sleep series/summary, heart list/ecg, devices, goals, notify list/get/subscribe/revoke.

**Priority 2 (transcendence) — all 6 shipped, hand-coded, local SQLite analytics (+ synthetic-data tests):**
1. `recomp` — fat-vs-lean recomposition + rolling-avg weight + verdict.
2. `recovery` — workouts HR-zone load × resting HR × sleep score divergence flag.
3. `bp-report` — dated BP + AFib table with `--note DATE=TEXT` annotations (lazy `bp_notes` table).
4. `sleep debt` — cumulative deficit vs `--target` over `--window` (wired under `sleep`).
5. `digest` — agent-native "what changed since `<time>`" multi-metric snapshot.
6. `correlate` — Pearson + best-lag between two daily metric series.

**Foundation/sync:** `internal/cli/withings_sync.go` — Withings form-POST sync per resource (measure/activity/sleep/workouts/heart/devices) with stable ids; `root.go` routes `sync` to it (generated GET-based `newSyncCmd` is incompatible with Withings action-RPC).

## Verified (independent of the implementing agent)
- `go build ./...`, `go vet ./...`, `go test ./...` all pass.
- Command tree: all 6 transcendence commands resolve; `sleep debt` under `sleep`.
- Exit codes: unknown-metric → 2, missing-mirror → 0 (`{}`/`[]` + sync hint), `--dry-run` → 0; form-POST endpoint dry-run → 0.
- Synthetic-data unit tests assert computed values (recomp verdict "recomposing", correlate pearson≈1.0, recovery flag, sleep-debt positive, digest latest_weight, bp-report row).

## Intentionally deferred
- **Web tier (scalews cookie)** — P3 secondary source; needs a DevTools HAR to lock the exact action/param contract. Not yet built; will request HAR or ship cookie-import guidance.
- **Live API testing** — official API needs a Withings dev-app (client_id/secret) + OAuth consent; not available in-session. CLI verified against mocks + dry-run + synthetic local data.

## Known gaps for retro
- Generated `internal/cli/sync.go` `newSyncCmd` is now unwired dead code (DO NOT EDIT file; replaced by hand-authored sync). Generator does not fit action-RPC + form-POST + nested-envelope APIs — candidate machine improvement.
