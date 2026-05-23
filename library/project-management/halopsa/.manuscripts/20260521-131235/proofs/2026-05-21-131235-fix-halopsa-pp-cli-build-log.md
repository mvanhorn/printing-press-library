# HaloPSA Build Log

## What was built

### Generation (Phase 2)
- Source spec: enriched OpenAPI 3.0.1 (947 paths after stripping `/Control*`, derived from `https://haloacademy.halopsa.com/api/swagger/v2/swagger.json`)
- Auth: switched scheme to OAuth2 client_credentials with `x-auth-vars`: `HALOPSA_CLIENT_ID`, `HALOPSA_CLIENT_SECRET`, `HALOPSA_TENANT`, `HALOPSA_DOMAIN`, `HALOPSA_SCOPE`, `HALOPSA_TOKEN`.
- Server template: `https://{tenant}.{domain}/api`, with `domain` enum (halopsa.com / haloitsm.com / halocrm.com).
- MCP enrichment: `x-mcp: {transport: [stdio,http], orchestration: code, endpoint_tools: hidden}` — the Cloudflare pattern for the >900-tool surface.
- Result: 947 endpoint-mirrored Cobra commands + framework (auth, doctor, sync, search, stale, analytics, export, import, api, tail, which, feedback, profile, load, orphans, workflow).

### Phase 3 transcendence (hand-built)
14 hand-written commands across 11 files:

| # | File | Command(s) | Data source |
|---|------|-----------|-------------|
| 1 | `triage.go` | `triage` | local tickets join |
| 2 | `standup.go` | `standup` | local tickets + actions |
| 3 | `time.go` | `time gaps` | local set-diff tickets vs actions |
| 4 | `contracts.go` | `contracts burn` | local client_contract + actions |
| 5 | `rules.go` | `rules dump` | live `/TicketRules` + `/Workflow` |
| 6 | `tickets_ageout.go` | `tickets age-out`, `tickets changed-since` | local tickets; preview + `--apply` |
| 7 | `sla_breaching.go` | `sla breaching` | local tickets.targetdate |
| 8 | `agent_workload.go` | `agent workload` | local tickets + actions |
| 9 | `client_card.go` | `client card`, `client overlay` | local 6-table join |
| 10 | `asset_history.go` | `asset history` | local tickets filtered by asset_id |
| 11 | `kbarticle_suggest.go` | `kbarticle suggest` | live `/KBArticle?search=` ranked |
| 12 | `sqlcmd.go` | `sql` | local SQLite SELECT-only |
| 13 | `novel_register.go` | (registration helper) | attaches children to parents by Use |

All commands:
- Honor `dryRunOK(flags)` to short-circuit verify probes.
- Set `Annotations: {"mcp:read-only": "true"}` where they don't mutate.
- Emit JSON via `flags.printJSON` (picks up `--select`, `--csv`, `--compact`, `--quiet`).
- Use `cmd.Help()` when no args/flags are given.

### Generator workarounds applied
1. **Stripped `/Control*` (5 paths + the `Control` schema reduced to a stub)**: the Control resource had ~3,750 columns when promoted to a typed table, blowing past SQLite's 2,000-column default `SQLITE_MAX_COLUMN`. Control is a tenant-global singleton config object with no list/sync workflow value — safe drop.
2. **Resource shadowing renames** (auto-applied by generator): `feedback → halo-feedback`, `health → halo-health`, `search → halo-search`, `workflow → halo-workflow` to avoid collision with framework cobra commands.
3. **Description across surfaces verified**: `internal/cli/root.go` Short, `SKILL.md` frontmatter, `.goreleaser` brews description, `internal/cli/agent_context.go`, and `internal/mcp/tools.go` all render the curated `narrative.headline` ("Every HaloPSA, HaloITSM and HaloCRM feature, plus a local SQLite store and cross-entity views the API can't return.") — not the raw upstream OpenAPI info.description.

### Narrative validation (Phase 3 close-out)
`printing-press validate-narrative --strict --research $RESEARCH/research.json --binary $CLI/halopsa-pp-cli`:
```
OK: 11 narrative commands resolved against the CLI tree
```

## What was intentionally deferred

- **`assets/contracts/agent/timesheet` etc. as separate sync tables.** The generator only created 27 tables (the resources with clean list/get + reasonable schema width). My novel commands route through the tables that exist (`tickets`, `actions`, `client`, `client_contract`, `asset`, `site`, `users`) plus targeted live API calls for KB and rules. This is by design: forcing typed tables for every Halo resource would create more 800+-column tables (tickets and projects already sit at 847).
- **Token-cache file path defaults.** Defaults to `~/.local/share/halopsa-pp-cli/data.db`. Users can override with `--db`.
- **`tickets age-out --apply` resilience.** The current implementation issues one POST per stale ticket. Larger batches would benefit from `--concurrency`. Deferred until a paying user complains; safer for now to keep simple.

## Skipped body fields

The generator's lenient mode wrote some POST/PUT bodies as `[]interface{}` placeholders; downstream users will use `--stdin` with a real JSON payload. Listed in the generation log; not blocker-class.

## Generator limitations encountered

- SQLite column-cap blow-up on huge config objects (Control). Fixed in-spec by stripping; not retro-worthy unless a future Halo spec keeps growing the Control surface. The generator should consider falling back to JSON-only storage for resources beyond a width threshold.
