# WeWork CLI — Production Auth Onboarding + Docs Fix

## Problem
- CLI had no `auth login --chrome` despite README/SKILL/research.json referencing it (doc bug).
- Composed auth needs 3 values (bearer token + weworkuuid + weworkmembertype) but the
  generated `set-token` persisted only the token; uuid/member-type had to be re-exported
  every shell (no persistence).

## Fix (durable, markerless hand files; survive/quarantine-restore across regen)
- `internal/config/wework_auth_persist.go` — `SaveComposedAuth(token,uuid,memberType)` writes
  all three to the credentials file via the same save path set-token uses; `ComposedAuthStatus()`
  + `JWTExpiry()` decode the token's exp. Unit-tested.
- `internal/cli/wework_auth.go` — new `auth import` (persist all three via flags or a
  {token,uuid,memberType} JSON on --stdin, tolerant of env/browser key spellings) and
  `auth whoami` (shows which values are set + token expiry/countdown, ready/expired state).
  Registered under the generated `auth` parent via the registerNovelCommand hook. Unit-tested.
- Docs corrected in research.json (source of truth) + generated README.md/SKILL.md: removed all
  `auth login --chrome` references; documented the real flow (DevTools snippet -> `auth import
  --stdin`, or --token/--uuid/--member-type; env vars still override; `auth whoami` for status).

## Verified (no real token needed — dummy JWT in an isolated --home)
- `auth import` (flags) persists all 3; fresh-process `auth whoami` still shows them set
  (persistence across shells confirmed).
- `auth import --stdin` with DevTools-JSON shape works.
- `auth whoami --json` reports ready:true + token_expires.
- `auth status` reports "Credentials present (Source: config)".
- go test (config+cli) pass; go vet clean.
- shipcheck: verify / validate-narrative / dogfood / workflow-verify / apify-audit /
  verify-skill all PASS; scorecard Grade A; only live_api_verification HOLD (no live token).

## Known minor gap
- MCPB manifest env prompt still lists only WEWORK_TOKEN (composed auth needs 3). The MCP
  binary reads the same persisted config, so `auth import` covers it; the prompt text is a
  cosmetic generator gap (retro candidate).
