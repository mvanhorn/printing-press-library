# Zaptec CLI — Build Log

## Environment note
- Local Go is 1.24.4; generated project targets `go 1.26.3`. `GOTOOLCHAIN=go1.26.3` is exported for all builds/gates (the 1.26.3 toolchain is in the module cache). govulncheck/vet/build all pass under it.
- The generator's `--help` validation gate failed once on Windows only because it exec'd `zaptec-pp-cli-validation` without `.exe`; the real binary builds and runs. Not a code defect.

## Priority 0 — foundation
- **Auth gap fixed.** The generator mapped Zaptec's OAuth2 to a generic static-token flow (`auth set-token`, `ZAPTEC_LEGACY_AUTH`) and left `refreshAccessToken` a no-op. Added:
  - `internal/client/login.go` — `PasswordGrant()` implementing the OAuth2 ROPC (`grant_type=password`) exchange against `/oauth/token`.
  - `internal/cli/auth_login.go` — `auth login` command (flags / `ZAPTEC_USERNAME`+`ZAPTEC_PASSWORD` env / masked interactive prompt via `golang.org/x/term`), persists token via `cfg.SaveTokens`.
  - `maybeAutoLogin()` wired into `newClient()` — transparent login when env creds present and no usable token; no-op under `--dry-run`/verify.
- Generator-provided store (chargehistory/chargers/installation + others), sync, search, sql, doctor retained.

## Priority 1 — absorbed (generator + hand-built)
- Generator-emitted: `chargers`, `installation`, `chargehistory`, `session`, `constants`, `charger-firmware`, `user-groups`, sendCommand, updates, sync/search/analytics/tail/workflow/api.
- Hand-built friendly control wrappers over `POST /api/chargers/{id}/sendCommand/{commandId}` with baked command IDs (`internal/cli/controls.go`): `pause`(502), `resume`(507), `start`(501), `stop`(506 final), `restart`(102), `unlock`(708), `deauthorize`(10001), plus `firmware upgrade`(200).
- `internal/cli/zaptec_codes.go` — baked decode tables (command IDs, operation modes, observation names/units) from `/api/constants`, so state/control speak plain English offline.

## Priority 2 — transcendence (all 6 built)
| Command | File | Backing |
|---------|------|---------|
| `cost` | cost.go | local `chargehistory` rollup (kWh + sessions, optional `--price`) |
| `live` / `fleet` | state_live.go | one `/api/chargers` call, decoded modes |
| `current headroom` | current.go | `/api/installation/{id}` max vs available |
| `chargers stale` | chargers_stale.go | local `chargers` store query |
| `firmware drift` | firmware.go | `/api/chargerFirmware/installation/{id}` modal-version grouping |
| `sessions anomalies` | sessions.go | threshold SQL over local `chargehistory` |
- Also `state --watch`, decoded `state <id>`.

## Scope notes / deferred
- **`authorize` (friendly command) intentionally omitted.** Zaptec has no clean single `Authorize` commandId (native auth is per-session / charge-card based: `SetAuthenticationList`, `ConfirmChargeCardAdded`). `deauthorize` maps cleanly to `DeauthorizeAndStop` (10001) and is shipped. Authorization remains reachable via the raw `api` / `chargers send-command` path. No stub shipped.
- No monetary cost in the API → `cost` rolls up real kWh; cost only computed with user-supplied `--price`. Honest framing in help.
- Empty-result slices return `[]` not `null` (agent-friendly).

## Tests
- `internal/cli/zaptec_codes_test.go` — table tests for operation-mode/observation decode + round2. Pass.
