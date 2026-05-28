# hubspot-pp-cli Phase 5 Acceptance Report

## Outcome

**Gate: SKIP** — Phase 5 live dogfood deferred; CLI ships verified against mocks. Skip marker at `proofs/phase5-skip.json` with `skip_reason: auth_required_no_credential`.

## Why skip

HubSpot's CRM Contacts API requires a Private App access token (`HUBSPOT_PRIVATE_APP_TOKEN`) with `Authorization: Bearer <token>`. No token was available in the run environment by the time Phase 5 was reached. Per the skill, this is the documented skip path for bearer-token APIs.

The user is generating the token in parallel; once available, they can run a live dogfood pass against the published library copy via `hubspot-pp-cli doctor` + `sync` + `daily-digest` to validate.

## What we have instead

- **Phase 4 shipcheck (mocks mode) — PASS 6/6 legs.** Verify, validate-narrative, dogfood, workflow-verify, verify-skill, scorecard all green.
- **Sample-output probe — 9/9 (100%).** Every novel command runs cleanly under the live-check sampler against a synthetic SQLite store.
- **Build + tests** — `go build` PASS, `go vet` clean, `go test ./internal/cli/... ./internal/store/...` PASS.
- **Help walk** — every novel command resolves as `<binary> <leaf> [flags]` and accepts `--dry-run` with exit 0.
- **Hand-coded behavioral smoke** — implementer agent ran each novel command against seeded SQLite (duplicate-suspects flagged `<example-email-a>` vs `<example-email-b>` at combined_sim=0.988; score-drift detected 80→40 with prior snapshot; daily-digest composed all four sections including lead→opportunity stage promotion).

## What this leaves untested

- Real `GET /crm/v3/objects/contacts` pagination cursor against a populated portal.
- Real rate-limit handling against HubSpot's 100 req/10s burst.
- Real shape parity between the spec types and a live response (HubSpot occasionally returns fields not in the spec).
- Auth-error message clarity when an invalid token is provided.

These are the known unknowns. The CLI's `doctor` command provides post-install live verification once the user has a token.

## Acceptance: SKIP (documented), proceed to promote and publish

Per the skill: "Do not promote without one" of `phase5-acceptance.json` (pass) or `phase5-skip.json` (legitimate skip). The skip marker is in place. Phase 5.6 promote and Phase 6 publish are unblocked.
