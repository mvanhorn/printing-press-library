Manifest transcendence rows: 7 planned, 0 built. Phase 3 will not pass until all 7 ship.

# Build Log — slack reprint (run 20260801-150754-271fdf29)

## Phase 2: Generate

Spec composition: curated internal YAML (62 endpoints) + archived official OpenAPI 2.0 pruned to 29 non-overlapping methods = **91 endpoints, no duplicate paths**.

### Generation blockers hit and fixed

1. **Reserved-name collision: `search`.** The spec's `search` resource collides with the framework's reserved `search` command (offline FTS5) — would overwrite `internal/cli/search.go` and emit a duplicate `newSearchCmd`. Renamed to `search_api`, which also disambiguates the live Slack search endpoint from the local index. The v1.2.1 press had no such check; the published CLI shipped with this latent collision.
2. **Reserved-name collision: `auth`.** Same class, same fix — renamed to `auth_api`. `/auth.*` also dropped from the OpenAPI side to avoid re-introducing it.
3. **Intent reference drift.** `mcp.intents[0]` referenced `auth.test`, stale after the rename. Updated to `auth_api.test`. Worth noting: the intent validator caught this, which is a machine capability the prior print did not have.
4. **Malformed Swagger `items`.** The archived 2021 official spec uses JSON-Schema draft-4 tuple form (`"items": [...]`) where Swagger 2.0 requires a single schema object; the generator's built-in cleanup could not repair it. Coerced 13 occurrences to single-schema form.
5. **Cross-spec duplication (no dedup on merge).** First successful generation produced **150 tools = 62 + 88 exactly** — the merge performed no overlap detection, so every method defined in both specs shipped twice under different names (`messages post_message` *and* `chat-post-message chat_post_message`). Fixed at the source by subtracting the 59 YAML-covered paths from the official spec before merge. Result: 91 tools, zero duplicate paths. **Flagged as a Printing Press issue for retro** — the generator should dedup by path across `--spec` inputs rather than leaving it to the operator.

### Generator warnings (retro candidates)

- `skipping workflow template workflows/comm_health.go.tmpl: file does not exist` — a referenced workflow template is absent from the installed generator.
- `novel feature command "users" maps to generated command path; skipping novel stub` — expected: `users activity` and `users whois` are children of the generated `users` resource parent. Scaffolds were still emitted.

### Verified after generation

- All Go gates pass: `go mod tidy`, `govulncheck`, `go vet`, `go build`, runnable binary, `--help`, `version`, `doctor`.
- MCP surface: `cobratree/`, `code_orch.go` (slack_search + slack_execute), `intents.go` with all three approved intents, streamable HTTP transport. The prior print had only a static `tools.go`.
- Descriptions resolve from the curated `narrative.headline`, not the upstream API blob, across `root.go` and `SKILL.md`.
- Creator attribution correctly preserved as Matt Van Horn (reprint guard working); operator recorded as contributor.

## Phase 3: Build — COMPLETE

Manifest transcendence rows: **7 planned, 7 built.** Zero stubs. Plus two features added mid-build to make approved scope actually work (see below), so nine hand-authored commands ship.

| # | Command | File | Notes |
|---|---|---|---|
| 1 | `archive recall` | `archive_recall.go` | FTS5 over the local mirror, thread context, ID→name resolution, retention-wall annotation |
| 2 | `archive coverage` | `archive_coverage.go` | per-channel ts range, gap detection, pre-wall holdings |
| 3 | `catchup` | `catchup.go` | volume + mentions + threads-awaiting-reply in one local pass |
| 4 | `threads stale` | `threads_stale.go` | thread grouping by `thread_ts`, ranked by age since last reply |
| 5 | `health` | `health.go` | msgs/day, distinct posters, median first-reply latency, idle days, `--dying` |
| 6 | `users activity` | `users_activity.go` | cross-channel per-user rollup |
| 7 | `users whois` | `users_whois.go` | resolves ID / @handle / email to one identity card |
| + | `sql` | `sql.go` | **added**: absorbed row 51 promised it; the generator emits no such command. Read-only, single-statement, SELECT/WITH/EXPLAIN/PRAGMA only |
| + | `archive sync` | `archive_sync.go` | **added**: nothing else writes messages to the mirror, so five of the seven features above had no data source |

Supporting package `internal/slackanalytics/` (search, stats, identity, mentions, timeutil) with table-driven tests.

### Phase 3 Completion Gate

- Per-row Cobra resolution: **14/14 approved command paths resolve** with the correct `Usage:` spec line (7 transcendence + 7 absorbed framework rows). Zero misses.
- Deterministic backstop: `dogfood --json` → `novel_features_check` = 7 planned / 7 found / 0 missing / not skipped.
- Test presence: `internal/slackanalytics` carries 20 test functions across 5 files; `internal/cli` novel commands carry scaffold + behaviour tests.

### Blockers found and fixed during Phase 3 and Phase 4

1. **Novel-command name collision the generator does not check.** The novel feature `recall` collided with the framework learn loop's own `recall` (`newRecallCmd` in `teach.go`). Two identically-named root commands shipped and 7 framework tests failed. The generator validates *spec resource* names against reserved commands but not *novel feature* names from `research.json`. Moved to `archive recall`; implementation relocated to `archive_recall.go` so future regens preserve rather than re-stub it.
2. **Auth entirely non-functional.** The secondary OpenAPI's `slackAuth` OAuth2 security definition displaced the curated `bearer_token` model. `AuthHeader()` returned only `AuthHeaderVal` or `Bearer <AccessToken>` and never read `SlackBotToken`, so `SLACK_BOT_TOKEN` was parsed into config and then ignored — every request went out unauthenticated. Stripped `securityDefinitions` and 29 per-operation `security` blocks from the contributing spec; rebased the working tree on a clean generation, restoring the 34 hand-authored files over it.
3. **`ok:false` store corruption (regression).** Documented in patch `slack-ok-false-detection`.
4. **`auth.revoke` revoked the live token.** Documented in patch `slack-auth-revoke-harness-guard` and the acceptance report.
5. **`info` → `get` verb convention.** Six generated commands used `info`; renamed at the spec level so wire paths stay `/conversations.info` while the CLI verb follows convention.
6. **Stale/incorrect help text.** `archive recall` advertised its pre-rename path; `auth-api` examples used the underscored resource key. Both would fail on copy-paste.

### Generator limitations encountered (retro candidates)

- No cross-spec dedup on `--spec` merge (150 tools = 62 + 88, same methods twice).
- Command safety derived from HTTP method — destructive-over-GET is mislabelled read-only.
- `ok:false`-in-HTTP-200 not modelled by the sync loop.
- Secondary-spec security definitions silently override the primary auth model.
- `destructive: true` in the internal spec is accepted and ignored.
- Mock-mode data pipeline yields 0 rows even with `response` / `response_path` / `types` declared.
- Missing template `workflows/comm_health.go.tmpl`.
