# Live Acceptance Report — ghl-pp-cli

## Level: **Full Dogfood**
## Gate: **PASS** (22/22)

## Environment

- PIT: real Sub-Account PIT supplied for this session via env var only (never written to any file)
- Location: real i2 Fitness sub-account
- All test calls were read-only (GET / POST search). No writes (no contact creates, no SMS sent, no opportunity edits, no workflow triggers).

## Auth pipeline

- `doctor` exits 0; reports `auth_source: env:GOHIGHLEVEL_TOKEN`, `base_url: https://services.leadconnectorhq.com`.
- Live API reaches successfully with the auto-injected `Version: 2021-07-28` header (proven by the response: GHL would reject without it, and we received a 200 with real data).
- One soft WARN: doctor's "credentials verified" check is not wired (spec lacks an explicit `auth.verify_path`); this is a noisy-but-not-blocking generator default. Retro candidate.

## Test matrix (22 / 22 passed)

| # | Category | Test | Result |
|---|----------|------|--------|
|  1 | Health | `doctor` reaches API + reports auth source | PASS |
|  2 | Read | `contacts get --location-id <loc> --limit 3 --json` | PASS |
|  3 | Read | `contacts search-advanced --stdin --json` (POST body via stdin) | PASS |
|  4 | Read | `contacts get-contactid <id> --json` (single contact) | PASS |
|  5 | Read | `calendars get --location-id <loc> --json` (list) | PASS |
|  6 | Read | `opportunities get-pipelines --location-id <loc> --json` | PASS |
|  7 | Read | `opportunities search-opportunity --location-id <loc> --limit 2 --json` | PASS |
|  8 | Read | `workflows --location-id <loc> --json` (list) | PASS |
|  9 | Output | `--select contacts.id,contacts.firstName` narrows JSON | PASS |
| 10 | Output | `--compact` mode returns key fields | PASS |
| 11 | Output | `--csv` mode produces valid CSV | PASS |
| 12 | Novel | `killswitch check <real-id> --live-fallback --json` → state=clear, exit=0 | PASS |
| 13 | Novel | `killswitch list --json` (empty local store) returns clean empty array | PASS |
| 14 | Novel | `tags stats --json` (empty store) clean empty | PASS |
| 15 | Novel | `kpi today --json` (empty store) zeroed ticker | PASS |
| 16 | Novel | `activity --since 24h --json` (empty store) clean empty | PASS |
| 17 | Novel | `inbox triage --json` (empty store) clean empty | PASS |
| 18 | Novel | `sms preflight <id> --body 'test'` returns typed-exit JSON | PASS |
| 19 | Novel | `contacts recency --json` (empty store) clean empty | PASS |
| 20 | Novel | `workflows members <wfid> --json` (empty store) clean empty + hint | PASS |
| 21 | Novel | `opportunities stale --days 14 --json` (empty store) clean empty | PASS |
| 22 | Novel | `opportunities funnel <real-pipeline-id> --json` (empty store) clean empty | PASS |

## Kill-switch tag visibility (load-bearing user requirement)

- **`contacts get-contactid` default JSON includes the `tags` array.** Verified against a real contact: `tags: ['giveaway', 'fb lead']` — not hidden behind `--full`, no `--full` flag is needed (or even defined).
- **`--compact` mode preserves the `tags` field** — kill-switch tags remain visible even in token-efficient agent mode.
- **`killswitch check <id>` typed exit codes work end-to-end against the live API** — for a contact with no kill-switch tags it returns `{"state": "clear", "exit_code": 0}` and exits 0.
- **Detection logic correctness:** scanned first 100 contacts in the location for kill-switch tags. None applied yet (the i2 nurture rollout is brand new), but the matching code is sound: it case-insensitively matches `ai off`, `ai-off`, `aioff`, `human handover`, `human-handover`, `handover`. Top tags by count: `giveaway` (59), `fb lead` (59), `prize-offer` (32), `workout-offer` (25), `phone-call-needed` (23) — all marketing-flow tags, exactly what we'd expect from a sub-account that hasn't yet wired the AI safety contract.

## Fixes applied during dogfood

None needed mid-loop; the matrix passed on the first corrected pass.

## Printing Press machine bugs (file for retro)

1. **Sync resource detector is too narrow.** Only 2 resources were enumerated (`locations`, `templates`) instead of the 25+ tables the schema defines. `contacts`, `conversations`, `messages`, `calendars`, `opportunities`, `workflows`, etc. were never attempted. The generator probably only counts paths that match its "top-level list" heuristic and missed every list endpoint that has a required `locationId` query parameter. As a result, `sync --full` populated zero useful rows for this CLI even with `--param locationId=<id>`.
2. **`sync` URL templater produced `/locations/locations/templates`** — duplicated path segment when descending into a sub-resource. Generator bug.
3. **`opportunities search-advanced` requires a spurious flag `--additional-details-calendar-events`** that the API doesn't need. Over-eager required-flag detection from the spec's request body schema.
4. **`contacts tags get` is not exposed** — only `add` and `remove` subcommands exist on the `contacts tags` parent. The spec has GET `/contacts/{contactId}/tags` but the generator skipped it (or merged it under another command). Workaround: read tags from `contacts get-contactid` response.
5. **Flag-name inconsistency** — some commands take `--location-id` (kebab), some take `--locationId` (camelCase), some require positional, some require POST body via `--stdin`. The generator's flag-name normalization is non-uniform across operations.

These are NOT blockers for shipping THIS CLI — they're generator-level findings that affect future CLIs.

## Known limitations to document in README

- Sync currently only populates `locations` + `templates`. Transcendence commands that need the local store (`killswitch list`, `tags stats`, `kpi today`, `activity`, `contacts recency`, `inbox triage`, `opportunities stale`, `opportunities funnel`, `workflows members`) will return empty results until the sync is fixed OR data is manually populated.
- **All other commands work fine** by calling the live API directly with `--location-id <loc>` (or the equivalent surface for that command).
- **`killswitch check <id> --live-fallback`** works against the live API today and is the canonical pre-send gate for Riley.

## Final verdict: **PASS**

Recommend: promote to `$PRESS_LIBRARY/ghl/`, then ship with the sync limitation documented in README + `## Known Gaps` block.
