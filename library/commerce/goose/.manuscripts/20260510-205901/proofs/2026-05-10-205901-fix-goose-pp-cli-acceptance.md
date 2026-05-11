# Goose CLI — Phase 5 Acceptance Report

## Level
SKIPPED (`auth_required_no_credential`)

## Auth context
- Type: `bearer_token` (AWS Cognito JWT)
- Live key in env at run time: **No**
- Cognito refresh token available: **No** (user is logged in to the browser session via the driven Chrome that browser-sniff used, but the refresh token was not piped into the CLI — that requires the user to run `goose-pp-cli auth login` themselves, which is a stdin paste flow)

## Why skipped
The Cognito access token observed during browser-sniff is 1-hour TTL. By the time this Phase 5 step runs (~30 minutes after capture), there is a non-trivial chance the token is expired. More importantly, automated stdin pastes during a /printing-press run are not the right shape — the user should run `auth login` themselves so the refresh token never enters the agent's context.

## Pre-ship verification still performed
Shipcheck umbrella (6/6 legs PASS, scorecard 85/100 Grade A) covered:
- `dogfood` — every leaf command runs --help, --dry-run, and one happy-path probe in mock mode (100% pass)
- `verify` — runtime breakage (31/31 commands PASS, 100%)
- `workflow-verify` — workflow manifest test (PASS)
- `verify-skill` — SKILL.md flags/paths exist (PASS)
- `validate-narrative` — README/SKILL recipe commands resolve in built CLI (PASS)
- `scorecard` — 85/100 Grade A

## Recommended first invocation by the user (not auto-run)
1. Run `goose-pp-cli auth login` (paste the refresh token from Chrome DevTools → Application → Local Storage → app.goose.pet, key ending `.refreshToken`).
2. Run `goose-pp-cli doctor` — confirms Cognito refresh succeeded and the API replies.
3. Run `goose-pp-cli today` — should render today's roster, mirroring the dashboard.
4. Run `goose-pp-cli reports list --json | jq '.results | length'` — should be ~50 report types.

## Gate
**SKIP** (per the skill's `auth_required_no_credential` rule)
