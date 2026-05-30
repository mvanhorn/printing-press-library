# make-pp-cli Absorb Manifest

## Source Tools Catalogued
- **integromat/make-cli** (TypeScript, official, MIT, 97 stars) — full Management API CLI; categories: Scenarios, Credentials, Data Stores, Account Management, Custom App Development, Utilities
- **integromat/make-typescript-sdk (@makehq/sdk)** (TypeScript, MIT) — typed SDK surface: Enums, Blueprints, Connections, Credential Requests, Data Stores, Data Store Records, Data Structures, Executions, Folders, Functions, Hooks, Incomplete Executions, Keys, Organizations, Scenarios, Teams, Public Templates, Users, plus Custom Apps (SDK Apps/Modules/Connections/Functions/RPCs/Webhooks)
- **integromat/make-mcp-server (@makehq/mcp-server)** — legacy local MCP; exposes "On-Demand" scenario runs as agent tools
- **Cloud MCP at mcp.make.com** — Anthropic-hosted, paid-tier locked beyond scenario-run
- **zezutom/makker** (Kotlin/JVM) — abandoned

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | scenarios list/get/create/update/delete | make-cli | `scenarios list/get/create/update/delete` | `--json`, `--select`, `--csv`, offline cache |
| 2 | scenarios clone | make-cli | `scenarios clone` | `--json`, `--dry-run` |
| 3 | scenarios activate/deactivate | make-cli | `scenarios activate/deactivate` | bulk via `--all`/`--folder` |
| 4 | scenarios run (POST /run) | make-cli, cloud MCP | `scenarios run` | `--wait` blocking mode (transcendence S1) |
| 5 | scenarios logs | make-cli | `scenarios logs` | `--json`, FTS search |
| 6 | scenarios blueprint export | make-cli | `scenarios blueprint export` | `--git`, repo-aware (transcendence S2 supersedes) |
| 7 | scenarios blueprint import | make-cli | `scenarios blueprint import` | `--remap` (transcendence S3 supersedes) |
| 8 | scenarios executions list/get | make-cli, sdk | `executions list/get` | `--json`, `--since`, FTS |
| 9 | incomplete-executions (DLQ) list/get | make-cli, sdk | `dlq list/get` | offline cache, age filter |
| 10 | DLQ retry/resolve | make-cli | `dlq retry/resolve` | bulk by reason (transcendence S4 supersedes) |
| 11 | connections list/get/create/delete | make-cli, sdk | `connections list/get/create/delete` | `--json`, audit hook |
| 12 | connections verify | make-cli, sdk | `connections test` | bulk, `--stale` |
| 13 | keys list/get/create/delete | make-cli, sdk | `keys list/get/create/delete` | `--json` |
| 14 | credential-requests list/get/create | sdk | `credential-requests list/get/create` | `--json` |
| 15 | data-stores list/get/create/update/delete | make-cli, sdk | `data-stores list/get/create/update/delete` | `--json` |
| 16 | data-store records CRUD | make-cli, sdk | `data-stores records list/get/create/update/delete` | `--csv` import/export |
| 17 | data-structures list/get/create/update/delete | make-cli, sdk | `data-structures list/get/create/update/delete` | `--json` |
| 18 | scenarios-folders list/get/create/update/delete | make-cli, sdk | `folders list/get/create/update/delete` | `--json` |
| 19 | hooks list/get/create/delete | make-cli, sdk | `hooks list/get/create/delete` | `--json` |
| 20 | hooks enable/disable | make-cli | `hooks enable/disable` | bulk |
| 21 | hooks ping/learn | make-cli | `hooks ping/learn` | `--json` |
| 22 | devices list/get/delete | make-cli, sdk | `devices list/get/delete` | `--json` |
| 23 | teams list/get/create/update/delete | make-cli, sdk | `teams list/get/create/update/delete` | `--json` |
| 24 | organizations list/get/update | make-cli, sdk | `orgs list/get/update` | `--json` |
| 25 | users me/list/invite/remove | make-cli, sdk | `users me/list/invite/remove` | `--json` |
| 26 | templates list/get/create | make-cli, sdk | `templates list/get/create` | `--json` |
| 27 | enumerations | make-cli | `enums list` | `--json` |
| 28 | functions list/get/create/delete | make-cli, sdk | `functions list/get/create/delete` | `--json` |
| 29 | sdk-apps list/get | make-cli, sdk | `sdk-apps list/get` | `--json` (read-focused) |
| 30 | MCP scenario-run tools | mcp-server, cloud MCP | exposed via cobratree mirror | local MCP server, no cloud dep |
| 31 | sync + local SQLite mirror | NEW | `sync` populates all entities | offline, fast |
| 32 | FTS5 search across entities | NEW | `search "term"` | offline regex search |
| 33 | SQL passthrough | NEW | `sql "SELECT ..."` | composable analytics |
| 34 | doctor | NEW | `doctor` | auth + zone + scope check |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | How It Works | Persona |
|---|---------|---------|-------|--------------|---------|
| T1 | Blocking agent run with bundle return | `scenarios run <id> --wait --timeout 5m --json [--replay <exec>]` | 10/10 | POST /scenarios/{id}/run → poll /scenarios/{id}/executions with exponential backoff to terminal status → fetch execution detail + bundles → emit one JSON envelope | Riley (B) |
| T2 | Git-backed blueprint sync | `blueprint sync --repo ./make-blueprints [--all-teams]` | 9/10 | Iterate scenarios per team, GET blueprints, write canonical JSON + sidecar `metadata.expect/restore` per scenario into repo, commit with generated message | Sam (C), Wade (A) |
| T3 | Dev→prod promote with auto-remap | `blueprint promote --from-team <dev> --to-team <prod> --scenario <id> [--auto-suggest \| --map remap.yml] [--dry-run]` | 10/10 | Read source blueprint; fetch /connections, /hooks, /data-stores in target; propose name-match remap; rewrite connectionId/hookId/dataStoreId/folderId in blueprint JSON; POST to /scenarios in target team | Sam (C) |
| T4 | Cross-scenario DLQ inbox with bulk action | `dlq inbox [--team <id> \| --all-teams] [--age 24h] [--group-by reason] [--retry-all --match-reason <re>] [--resolve-all --match-reason <re>]` | 9/10 | SQL over synced dlqs + scenarios → group by extracted error-reason fingerprint → bulk POST retry/resolve filtered by regex match | Jordan (D), Wade (A) |
| T5 | Connections audit (stale/expiring/orphaned/errored) | `connections audit [--unused 30d] [--expiring 7d] [--errored 7d]` | 8/10 | Join local connections × walked-blueprint module references × executions errors | Wade (A), Sam (C) |
| T6 | Cross-team scenario list with stale filter | `scenarios list --all-teams [--active] [--stale 30d] [--folder <path>]` | 8/10 | Union scenarios across every team the token can see; left-join executions for last-run | Wade (A) |
| T7 | Webhook→scenario routing map | `hooks map [--team <id> \| --all-teams] [--orphans] [--shared]` | 7/10 | Walk locally-cached blueprints for `gateway:CustomWebHook` hookId references; join to hooks table; flag orphans (0 consumers) and shared (>1 consumers) | Wade (A), Jordan (D) |
| T8 | Blueprint diff & restore | `blueprint diff <scenarioId> [--from <snap>] [--to current]` and `blueprint restore <scenarioId> --snapshot <id>` | 7/10 | Read versioned snapshots from local `blueprint_snapshots` table; compute structural diff ignoring `metadata.expect/restore` noise; restore re-PUTs snapshot via /scenarios/{id}/blueprint | Sam (C) |
