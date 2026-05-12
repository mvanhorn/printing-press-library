# WHOOP CLI Absorb Manifest

**Generated:** 2026-05-11
**Purpose:** Catalog every feature from every WHOOP tool we can find. Our CLI must match-or-beat each row in "Absorbed" and ship every "Novel" row.

---

## Ecosystem inventory

### Competitor CLIs and SDKs (Go/Python/TypeScript/Ruby)

| Tool | Lang | Type | URL |
|---|---|---|---|
| totocaster/whoopy | Go | CLI (closest competitor) | https://github.com/totocaster/whoopy |
| karl-cardenas-coding/mywhoop | Go | CLI + sync daemon | https://github.com/karl-cardenas-coding/mywhoop |
| ferueda/go-whoop | Go | SDK | https://github.com/ferueda/go-whoop |
| marekq/go-whoop | Go | Downloader | https://github.com/marekq/go-whoop |
| hedgertronic/whoop | Python | SDK | https://github.com/hedgertronic/whoop |
| felixnext/whoopy | Python | SDK | https://github.com/felixnext/whoopy |
| WhoopInc/whoop-pydantic-v2 | Python | Schemas (official) | https://github.com/WhoopInc/whoop-pydantic-v2 |
| jacc/whoopkit | TS | SDK | https://github.com/jacc/whoopkit |
| koala73/whoopskill | TS | CLI (npm) | https://github.com/koala73/whoopskill |
| TomasWard1/whoop-cli | TS | CLI | https://github.com/TomasWard1/whoop-cli |
| pelo-tech/whoop-api-spec | YAML | OpenAPI spec | https://github.com/pelo-tech/whoop-api-spec |
| ericaleman/whoop-plus | TS | Trends dashboard | https://github.com/ericaleman/whoop-plus |
| patrickloeber/whoop-analyzer | Python | Analytics notebook | https://github.com/patrickloeber/whoop-analyzer |

### MCP servers (9 confirmed)

| Repo | Lang | Auth | Notes |
|---|---|---|---|
| nissand/whoop-mcp-server-claude | TS | OAuth2 | 18+ endpoints, "complete coverage" |
| JedPattersonn/whoop-mcp | TS | Bearer + OAuth | Recovery deep-dive analysis tool |
| ctvidic/whoop-mcp-server | Python | Bearer | Cycle/recovery/strain/workout queries |
| shashankswe2020-ux/whoop-mcp | Python | OAuth | Natural language Q&A |
| RomanEvstigneev/whoop-mcp-server | Python | Bearer | Standard read endpoints |
| dpshde/whoop-mcp | ? | ? | (README 404, repo exists) |
| JasonBates/whoop-mcp-server | Python | OAuth | Claude Desktop focus |
| jd1207/whoop-mcp | TS | OAuth | "Write data to WHOOP" — unique writeback claim |
| k0va1/whoop-mcp | Ruby/Sinatra | OAuth | Streamable HTTP MCP transport |
| elizabethtrykin/whoop-mcp | ? | ? | (README 404) |

### Greg Van Horn's prior printing of `whoop-pp-cli` (1.0.0)

**Treat as feature reference, not competitor — we're replacing it.** Commands shipped:
- `auth login --client-id --client-secret --port`, `auth status/logout`
- `activity get-sleep-by-id`, `get-sleep-collection`, `get-workout-by-id`, `get-workout-collection`
- `activity-mapping get` (v1→v2 UUID lookup)
- `cycle get-by-id`, `get-collection`
- `recovery get-collection`
- `user get-body-measurement`, `get-profile-basic`, `revoke-oauth-access`
- `partner add-test-data`, `get-lab-requisition-by-id`, `get-service-request-by-id`, `request-token`, `update-service-request-status`, `upload-diagnostic-report-results`
- `doctor`
- Flags: `--json`, `--select`, `--dry-run`, `--agent`, `--human-friendly`, `--idempotent`, `--ignore-missing`, `--yes`, `--stdin`
- Bundle: pre-built binaries, MCPB for Claude Desktop, Hermes/OpenClaw skill install paths
- Known bugs: limit > 25 → 400 errors (sync fell over); no auto-refresh of OAuth token; `sync`/`search`/`sql`/analytics commands missing

---

## Absorb table — match or beat every row

| # | Feature | Source | Our CLI command | Match-or-beat note |
|---|---|---|---|---|
| 1 | OAuth2 PKCE login w/ browser redirect | whoopy, mywhoop, nissand-mcp, Greg's CLI | `auth login --client-id --client-secret --port 8085` | Match. Default port 8085 (user-registered). Beat with `auth refresh` and clearer redirect-uri error guidance. |
| 2 | Token persisted to XDG state dir | whoopy | `~/.config/whoop-pp-cli/tokens.json` (XDG-aware) | Match. |
| 3 | Auto token refresh | go-whoop (oauth2 lib), nissand-mcp | All commands auto-refresh 60s before expiry | Beat: Greg's CLI didn't refresh; we do. |
| 4 | Bearer env-var fallback | Greg's CLI, ctvidic-mcp | `WHOOP_ACCESS_TOKEN` (+ `WHOOP_OAUTH` back-compat) | Match. |
| 5 | `auth status` (expiry + scopes) | whoopy | `auth status` | Match. |
| 6 | `auth logout` revoke + clean | whoopy | `auth logout` (calls DELETE /user/access) | Match. |
| 7 | Profile + body measurement | whoopy, whoopskill, all MCPs | `user profile`, `user body-measurement` | Match. Beat: surface as single `whoami` summary too. |
| 8 | Workouts list + filters (sport, strain) | whoopy | `workout list --sport running --min-strain 12 --max-strain 18 --start --end` | Match. |
| 9 | Workout view (one workout) | whoopy, all MCPs, Greg's CLI | `workout get <uuid>` | Match. |
| 10 | Workouts "today" alias | whoopy | `workout today` | Match. |
| 11 | Workouts export (jsonl/csv, autoseek pages) | whoopy | `workout export --format jsonl --output -` | Match. Beat with `--format ndjson,csv,tsv,parquet`. |
| 12 | Sleep list/view/today | whoopy, whoopskill, Greg's CLI | `sleep list`, `sleep get <uuid>`, `sleep today` | Match. Greg shipped get-by-id+collection but no "today" alias. |
| 13 | Recovery list/view/today | whoopy, whoopskill | `recovery list`, `recovery get <id>`, `recovery today` | Match. Greg shipped only get-collection. |
| 14 | Cycle list/view | whoopy, whoopskill | `cycle list`, `cycle get <id>` | Match. |
| 15 | Daily stats aggregate | whoopy `stats daily`, whoopskill `summary` | `stats daily --date YYYY-MM-DD` | Match. Beat: also `stats weekly`, `stats monthly`. |
| 16 | Sport ID → name mapping | go-whoop, whoop-plus | Internal `data/sports.go` + `sports list` command | Match. |
| 17 | Pagination via `next_token` w/ `limit ≤ 25` clamp | whoopy, mywhoop, all SDKs | Generated client auto-paginates, clamps internally | Beat: fixes Greg's bug. Validate via `dogfood`. |
| 18 | V1→V2 ID mapping migration helper | Greg's CLI | `activity-mapping get <v1Id>` | Match. |
| 19 | Doctor / health check | whoopy `diag`, Greg's CLI `doctor` | `doctor` (config + token + API ping + DB integrity) | Match. Beat with explicit pass/fail per check, exit code per failure category. |
| 20 | Output formats: JSON, table, CSV | whoopy, Greg's CLI | `--json`, `--text`, `--csv`, `--select`, `--agent`, `--dry-run` | Match. |
| 21 | Non-interactive / agent-native | whoopy, Greg's CLI, all MCPs | `--agent` (JSON + compact + no prompts) | Match. |
| 22 | Exit codes per error class | Greg's CLI | 0/2/3/4/5/7/10 | Match. |
| 23 | Pre-built binaries (darwin/linux arm64+amd64) | whoopy, mywhoop, Greg's CLI | GoReleaser binaries via printing-press-library | Match. |
| 24 | MCP server bundle | nissand, JedPattersonn, 7 others, Greg's CLI | `whoop-pp-mcp` binary + `.mcpb` for Claude Desktop | Match. |
| 25 | Recovery deep-dive analysis | JedPattersonn MCP | `recovery analyze <date>` | Match. Beat: include contributing factors AND historical comparison. |
| 26 | Local SQLite cache for offline reads | mywhoop, discrawl pattern (press) | Auto-cache on all read commands; `--offline` to skip API | Beat: most competitors are stateless API wrappers. |
| 27 | Sync daemon / scheduled sync | mywhoop | `sync --since 7d --concurrency 2` + optional cron snippet | Match. |
| 28 | Long-term trend tracking | whoop-plus | `trends <metric> --window 30d` | Match. |
| 29 | "Write data to WHOOP" | jd1207 MCP | (out of scope — WHOOP write API undocumented/closed) | Skip. Document why. |
| 30 | Skill bundle for Claude Code | Greg's CLI | `pp-whoop` skill at `cli-skills/pp-whoop` | Match. Refresh to point at new CLI. |
| 31 | Partner endpoints (lab requisitions etc) | Greg's CLI | Behind `--include-partner` build tag (default off) | Match. |

---

## Novel features (Rung 4-5 transcendence)

These leverage local SQLite + cross-resource joins that no live API call can do. Top picks for the build, score ≥ 5/10.

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---|---|---|---|
| N1 | Recovery-vs-strain efficiency curve | `whoop-pp-cli analyze efficiency --window 90d` | Joins workouts.strain ⋈ next-day recoveries.recovery_score over arbitrary windows. Live API returns raw rows, not derivatives. Output: "you recover 12% faster from strain 15 than 6 months ago". | 9 |
| N2 | Sleep debt cumulative tracker | `whoop-pp-cli analyze sleep-debt --since 30d` | Cumulative sum of `need_from_sleep_debt_milli` over time, with weekly-bucket trend. Requires backfilled sleeps table. | 8 |
| N3 | Overtraining detector (1σ above 90d strain) | `whoop-pp-cli analyze overtraining --threshold 1.0` | Computes rolling 90d mean+stdev of `cycle.score.strain`; flags days outside band; correlates with subsequent recovery drops. | 8 |
| N4 | Recovery streak / declining-trend alert | `whoop-pp-cli analyze recovery-trend --alert 3` | Detects N consecutive days of declining recovery; outputs JSON event suitable for cron/Claude Code skill. | 7 |
| N5 | Sport-specific strain/recovery profile | `whoop-pp-cli analyze by-sport --sport running` | Groups workouts.score by sport_id; computes mean strain-per-minute, kj/min, zone distribution, and average recovery cost. | 7 |
| N6 | Sleep-consistency vs morning-recovery correlation | `whoop-pp-cli analyze correlate sleep_consistency recovery_score --window 90d` | Pearson correlation of any two metric columns over a window. Generic enough to surface ANY user-hypothesis. | 9 |
| N7 | "Why was today bad?" attribution | `whoop-pp-cli analyze why-today` | Compares today's recovery to user's baseline; ranks contributing deltas (HRV down, sleep debt up, strain yesterday high, RHR elevated, etc.). LLM-friendly structured output. | 10 |
| N8 | Raw `sql` escape hatch | `whoop-pp-cli sql "SELECT ..."` | Discrawl-style. Power-user (or Claude Code) writes arbitrary SELECTs against the local SQLite. Beats every competitor. | 8 |
| N9 | Cross-resource search | `whoop-pp-cli search "running" --since 30d` | FTS5 over sport_name + score_state + dates. Returns workouts, related cycle+recovery+sleep linked records. | 6 |

**Selected for build (top 7):** N1, N2, N3, N6, N7, N8, N9. (N4 and N5 fold into N7 and the per-sport variant of `stats`.)

**Headline:** "Every WHOOP feature plus local SQLite analytics and correlations no other WHOOP tool offers."

---

## Match-or-beat verdict

- **Absorbed feature count:** 31 rows. Every one will be present in generated CLI or rationalized.
- **Novel feature count (shipping):** 7 (N1, N2, N3, N6, N7, N8, N9).
- **Gap vs Greg's prior CLI:** Greg shipped 18 commands but missed OAuth-refresh, pagination, sync, search, analytics. We add ~15 commands (sync, search, sql, stats daily/weekly/monthly, analyze efficiency/sleep-debt/overtraining/recovery-trend/correlate/why-today, trends, today aliases) and fix the pagination bug.
- **Gap vs whoopy (top open-source competitor):** whoopy ships `stats daily` but no local SQLite, no sync, no analytics beyond daily aggregation, no correlations. We strictly dominate.
- **Gap vs every MCP server:** each MCP wraps the API surface and stops there. None do local persistence or analytics. We strictly dominate as MCP too.
