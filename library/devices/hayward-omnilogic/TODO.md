# TODO — hayward-omnilogic-pp-cli

Open work from the adversarial code review (2026-05-11). Ranked P0
(critical) → P3 (minor). Greptile's 10 review findings on PR #431 are
all closed; everything here is in addition.

## P0 — Critical

None. Auth tokens are 0o600, store is 0o700/0o600, secrets never logged.

## P1 — High priority

### `doctor` doesn't verify auth

- **Where:** `internal/cli/doctor_omnilogic.go:16-31`
- **Symptom:** A user with a typo'd password sees `auth: "env vars
  present"` and `token_cache: "empty (next command will log in)"` —
  reads as "all good." Then `sites list` fails with a 401 mid-task.
- **Fix:** Add `--verify-auth` flag (or make it the default) that calls
  `EnsureToken()`. The inline comment rationalizing the gap ("would slow
  doctor down and pollute the token cache") doesn't hold — a 1-second
  auth round trip is cheap and the token cache is exactly the artifact
  doctor should validate.
- **Test:** Mock the auth endpoint, assert doctor surfaces the 401.

### `buildOrderedRequest` doesn't XML-escape string param values

- **Where:** `internal/omnilogic/envelope.go:94` and `:120` (also
  `buildChlorRequest`)
- **Symptom:** Latent. Values pass through `fmt.Fprintf("%s", val)`
  unescaped. Today every caller passes int/bool or literal-string
  values, so practical risk is zero — but the contract of `paramRepr`'s
  `case string:` returns the raw value, so the next time someone wires
  a Set* op that accepts a user-supplied string (a custom heater label,
  a future feature flag), the XML breaks silently or could be injected.
  `buildRequest` routes through `encoding/xml.Marshal` which escapes
  char data; the ordered builder bypasses that.
- **Fix:** Run all `Value` strings through `xml.EscapeText` (or
  `html.EscapeString`) before `Fprintf`. Pattern:
  ```go
  var buf bytes.Buffer
  _ = xml.EscapeText(&buf, []byte(val))
  fmt.Fprintf(&b, `<Parameter name="%s" dataType="%s">%s</Parameter>`, p.Name, p.DataType, buf.String())
  ```
- **Test:** Add a fixture that passes `"<&\""` as a string param and
  asserts the output XML parses cleanly via `xml.Unmarshal`.

## P2 — Medium priority

### Replay of legacy command_log rows on VSPs mis-dispatches

- **Where:** `internal/cli/transcendence.go runReplay` /
  `dispatchReplay`
- **Symptom:** Pre-fix rows (commits before the `is_vsp` tracking
  landed) don't have `is_vsp` in params_json. The dispatcher returns
  `false` from `boolFromAny(params["is_vsp"])` and routes to
  `SetEquipment(IsOn=bool)` — which Hayward rejects for VSPs with the
  "Input string was not in a correct format" error.
- **Fix:** When `is_vsp` is missing from params, fall back to live
  MSP-config detection: load the MSP config via the resolver, call
  `IsVSPPump(cfg, eqID)`, then route. At replay time, write the new
  row WITH `is_vsp` set so future replays self-contain.
- **Severity:** Non-silent (user gets an error), edge case (only
  affects rows created before the IsOn-overload fix).

### Migration code's `schemaVersion` bump is theater

- **Where:** `internal/store/store.go migrate()`
- **Symptom:** `migrate()` runs every migration statement on every
  Open regardless of `schemaVersion`. Idempotency comes from
  `CREATE TABLE IF NOT EXISTS` / `CREATE INDEX IF NOT EXISTS` hand-
  coded throughout. The first future migration that needs
  `ALTER TABLE ADD COLUMN` will break on the second invocation
  because SQLite has no `IF NOT EXISTS` for ALTER.
- **Fix:** Migration ranges keyed by `cur` (current schema_version):
  ```go
  if cur < 1 { runV1Migrations() }
  if cur < 2 { runV2Migrations() }  // site_capabilities
  if cur < 3 { runV3Migrations() }  // future
  ```
  Or guard each `ALTER TABLE` with a runtime "does column exist" probe.

### `classifyOmnilogicError` leaks Hayward auth body verbatim

- **Where:** `internal/cli/omnilogic_bridge.go:138`
- **Symptom:** `omnilogic.Truncate(authErr.Body, 200)` embedded in the
  user-facing error. Hayward's auth-failure body can include
  identity-shaped fields. If a user pipes stderr to a shared log,
  email/phone could land there.
- **Fix:** Strip / redact identity-shaped fields from the body before
  embedding. Or just include `HTTP %d` + a generic hint, not the raw
  body.

### `HAYWARD_DEBUG=1` logs `UserID` and `MspSystemID` to stderr

- **Where:** `internal/omnilogic/client.go sendOpRequest`
- **Symptom:** Every XML payload contains MspSystemID; `GetSiteList`
  also contains UserID. Both are sensitive (Hayward account
  identifier). The env-var is opt-in for debugging, but the help
  doesn't disclose what's logged.
- **Fix:** Document the leak surface in the `HAYWARD_DEBUG` comment +
  README troubleshooting section. Optionally redact UserID /
  MspSystemID in debug output (their values aren't needed to
  troubleshoot the protocol; the shape is what matters).

## P3 — Minor

### `chemistrySetupHint` recommends turning off all three sensors

- **Where:** `internal/cli/capabilities.go chemistrySetupHint`
- **Symptom:** Suggested command is "turn off ph + orp + salt". If a
  user has pH+ORP but no salt, the hint over-recommends.
- **Fix:** Tailor the suggested flags to which sensors actually
  returned null/-1 in the current telemetry snapshot.

### VSP detection is brittle string-match

- **Where:** `internal/omnilogic/resolve.go IsVSPPump`
- **Symptom:** Matches `strings.Contains(strings.ToUpper(p.Type),
  "VARIABLE_SPEED")`. If Hayward renames to `VSP_*` or `VAR_SPEED_*`,
  detection fails silently and `equipment on` for a VSP would re-route
  to `SetEquipment(IsOn=bool)` → API rejects → user error.
- **Fix:** Also accept `"VSP"` substring; or maintain a small allowlist
  of known VSP type strings.

### Replay output shape-confusing on error

- **Where:** `internal/cli/transcendence.go runReplay`
- **Symptom:** On dispatcher error, prints `{"status":"error", ...}` to
  stdout AND returns a non-zero error. Pipelines that check exit code
  AND parse `.status` see consistent signals; pipelines that just `jq`
  stdout see what looks like data.
- **Fix:** Either route error JSON to stderr, or document the contract
  ("always check exit code; the JSON is for the audit trail").

### No `context.Context` plumbing

- **Where:** Every HTTP call in `internal/omnilogic/client.go`
- **Symptom:** Long-running operations (`sweep` over many sites,
  `sync --full`) can't be cancelled cleanly. Rate limiter `Wait()`
  doesn't accept a context. Store queries don't either.
- **Fix:** Plumb `cmd.Context()` through every dispatcher; use
  `http.NewRequestWithContext`; add `WaitCtx(ctx)` to the limiter.
- **Severity:** Single-user CLI; edge case unless someone uses
  `--timeout`.

### `equipment on/off` for non-VSP equipment is untested live

- **Where:** `internal/omnilogic/operations.go SetEquipment`
- **Symptom:** SKILL banner groups this under "verified working" but
  the live test session only exercised the VSP routing path. The
  `IsOn=bool` path for accessory pumps + relays + lights was never
  fired against the real API. Hayward's MAY have a parallel quirk
  there.
- **Fix:** Live-test against an accessory pump or backyard relay (the
  user's pool has System-Ids 6 and 7 as single-speed pumps). Update
  the SKILL banner with the actual verification coverage.

### `sync` re-appends telemetry every invocation with no dedupe

- **Where:** `internal/store/writes.go AppendTelemetry`
- **Symptom:** Two `sync` calls within a minute both append the same
  readings. `chemistry log` / `drift` / `runtime` see double-rate.
  Compounds over time.
- **Fix:** Add a uniqueness constraint on
  `(site_msp_system_id, bow_system_id, equipment_system_id, metric, sampled_at)`
  with `INSERT OR IGNORE`, or skip-if-recent guard in `sync`.

### `TestOpenPermissions` assumes the test tmpdir honors POSIX mode

- **Where:** `internal/store/store_test.go TestOpenPermissions`
- **Symptom:** `runtime.GOOS == "windows"` skip exists. But Docker
  tmpfs with unusual mount options, or CI runners on filesystems
  without POSIX acls, can return different permission bits. Skip is
  too narrow.
- **Fix:** Pre-create a probe file with `os.Chmod(probe, 0o600)`,
  `os.Stat`, and skip the test if the filesystem returned anything
  other than `0o600`. Or use `t.Skipf` based on a runtime probe.

### Equipment-set dedupe vs heater dedupe inconsistency

- **Where:** `internal/omnilogic/resolve.go`
- **Symptom:** Pumps/lights/relays dedupe by `(kind, systemId)` so
  shared equipment collapses to one match. Heaters dedupe by
  `(bowID, heaterID)` so the same heater can show up multiple times
  if shared across BoWs. Intentional (Pool and Spa heaters can be
  separate physical units with the same name), but undocumented.
- **Fix:** Add a comment block explaining the asymmetry — heaters are
  typically distinct per BoW (Pool heater + Spa heater), shared pumps
  are typically the same physical unit.

### `sql` command lets users read PII without warning

- **Where:** `internal/cli/transcendence.go newSQLCmd`
- **Symptom:** Arbitrary SELECT over the store; the operator can dump
  `command_log` which includes everything logged. No warning.
- **Fix:** Add a brief note in the command's Long help that the store
  may contain PII (operator's own data; not a security issue but a
  documentation gap).

## Greptile findings (all closed)

See PR #431 history. Tracked here for context:

| ID | Finding | Commit |
|---|---|---|
| 3215869665 | `ready-by` advance-timing inverted guard | 85f61c18 |
| 3215869694 | XML param order non-determinism | 85f61c18 |
| 3215869726 | Cache files world-readable | 85f61c18 |
| 3215869760 | `bumpVerdict` drops findings | 85f61c18 |
| 3215951974 | `SetHeaterTemp` silent success | d3287d7a |
| 3216011168 | `sweep` ignored capabilities | d3287d7a |
| 3216047858 | `--replay` unwired | 540054ae |
| 3216091791 | SQLite store world-readable | 80b5f9af |
| 3216324918 | `schedule diff` heater-only | aac47ba5 |
| 3216365845 | `ready-by` -1 sentinel as real reading | 06a1fc2d |
