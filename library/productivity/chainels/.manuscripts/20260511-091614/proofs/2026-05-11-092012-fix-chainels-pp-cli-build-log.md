# Chainels Phase 3 Build Log

## Built

### Foundation (Priority 0) — framework auto
- SQLite store with FTS5 indexes on `issues`, `agreements`, `messages` (and more).
- `sync` + `search` + `analytics` + `export` framework commands.
- `agent-context`, `doctor`, `which`, `feedback`, `auth` scaffolding.

### Absorbed (Priority 1) — framework auto
- 236 endpoint-mirror subcommands across 35 resource groups (`accounts`, `agreements`, `alarms`, `bans`, `booking`, `communities`, `companies`, `discounts`, `entities`, `invoices`, `issues`, `messages`, `metrics`, `payments`, `replies`, `reporting`, `requests`, `service-accounts`, `spaces`, `turnover`, plus `drive`/`storeban` promoted, `alams` typo cluster preserved verbatim).

### Transcendence (Priority 2) — hand-built (9 features)
| # | Command | File | Persona |
|---|---------|------|---------|
| 1 | `search` (framework) | `internal/cli/search.go` (generated) | Maya |
| 2 | `issues load` | `internal/cli/issues_load.go` | Maya |
| 3 | `issues stale` | `internal/cli/issues_stale.go` | Maya |
| 4 | `turnover pending` | `internal/cli/turnover_pending.go` | Maya |
| 5 | `turnover variance` | `internal/cli/turnover_variance.go` | Maya, Rashid |
| 6 | `agreements renewals` | `internal/cli/agreements_renewals.go` | Maya, Devi |
| 7 | `members audit` | `internal/cli/members.go` | Maya, Devi |
| 8 | `alarms diff` | `internal/cli/alarms_diff.go` | Ines, Maya |
| 9 | `changed` | `internal/cli/changed.go` | Devi |

All 9 follow the verify-friendly RunE template (`dryRunOK` short-circuit, `len(args)==0` help fall-through where applicable), use `printJSONFiltered` so `--json --select --compact --csv --quiet` work for free, and carry `mcp:read-only=true` annotations because they only read from the local store.

### Polish (Priority 3)
- Added `auth client-credentials` subcommand: OAuth2 client_credentials grant (no browser) so headless agents / CI runners can authenticate when the default `authorization_code` flow's browser handshake is impossible. The generator picked `authorization_code` as the only auth login flow even though the spec advertises both; a generator improvement note is filed for retro.
- Added Cookbook section to README with five worked examples that exercise the novel features.
- Shared `parseDaysFlag` helper in `internal/cli/novel_helpers.go` so `issues stale --older-than 14d` and `agreements renewals --within 90d` parse the same way.

## Intentionally Deferred

None. Every feature approved in the Phase 1.5 absorb gate is shipping.

## Skipped Body Fields (Generator)

- `PATCH /messages/{msg_id}` — body contains `oneOf/anyOf`; generator skipped. Endpoint reachable via `messages update --stdin` reading raw JSON.
- `POST /communities/{community_id}/service-accounts` — `oneOf/anyOf` body.
- `POST /companies/{community_id}/messages` — `oneOf/anyOf` body.
- `POST /companies/{community_id}/timeline` — `oneOf/anyOf` body.
- `/access/saltoks/accounts/me` — no valid HTTP methods after generator's verb filter, path dropped.

## Generator Limitations Found (retro candidates)

1. **Duplicate body-variable + flag collision in `companies issues save`.** Spec carries both nested `service.id` and flat `service_id` on the issue body; the generator flattened both to `bodyServiceId` → duplicate Go declarations and duplicate `cmd.Flags().StringVar(&bodyServiceId, "service-id", ...)`. Required a manual dedupe (kept the richer description from the flat path). The generator should detect this name collision and either pick one canonical binding or rename one.
2. **OAuth flow selection.** Spec advertises three security schemes (`oauth_client` client_credentials, `oauth_code` authorization_code, `openid`). Generator wired only `authorization_code` into `auth login`, even though a CLI is precisely the case where `client_credentials` is the right default. Hand-added `auth client-credentials` subcommand as a workaround. Generator could emit both grants when both are present in the spec and prefer client_credentials as the headless-default.
3. **Description trimming.** Root.go `Short:` and goreleaser brews `description:` truncated the curated headline at ~140 chars even though the sources are markdown-safe; the trailing `…` shows up across surfaces. Worth a re-check.
