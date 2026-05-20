# Figma CLI Absorb Manifest

Best Source legend: **DC** = direct OpenAPI surface, **G** = GLips/Figma-Context-MCP, **O** = Official Figma Dev Mode MCP, **FM** = mikaelvesavuori/figmagic, **FE-RMR** = RedMadRobot/figma-export, **FE-MM** = marcomontalbano/figma-export, **TS** = Tokens Studio, **NEW** = original to figma-pp-cli.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Get file (full tree) | DC | `figma files get <key>` | typed endpoint mirror |
| 2 | Get file by node ids | DC | `figma files nodes <key> --ids=...` | accepts both `1234:5678` and `1234-5678` formats |
| 3 | Get file metadata only | DC | `figma files meta <key>` | Tier-1-light, cheap on rate limit |
| 4 | List file versions | DC | `figma files versions <key>` | cursor-paginated |
| 5 | Render nodes as PNG/SVG/PDF/JPG | DC, G | `figma files render <key> --ids=... --format=svg --scale=2` | one command for the kick-render+poll round-trip |
| 6 | Get image-fill URLs | DC | `figma files image-fills <key>` | warns on 14-day expiry; cache `fetched_at` |
| 7 | List comments | DC | `figma comments list <key>` | |
| 8 | Post comment with optional pin | DC | `figma comments post <key> --message --x --y --node-id` | |
| 9 | Delete comment | DC | `figma comments delete <key> <id> --confirm` | |
| 10 | Comment reactions CRUD | DC | `figma reactions {list,add,remove}` | |
| 11 | List team projects | DC | `figma teams projects <team_id>` | |
| 12 | List project files | DC | `figma projects files <project_id>` | cursor pagination |
| 13 | List team components | DC | `figma components team <team_id>` | |
| 14 | List file components | DC | `figma components file <key>` | |
| 15 | Get component by key | DC | `figma components get <component_key>` | |
| 16 | Component sets (3 endpoints) | DC | `figma component-sets {team,file,get}` | |
| 17 | Styles (3 endpoints) | DC | `figma styles {team,file,get}` | |
| 18 | Variables: local | DC | `figma variables local <key>` | Enterprise-gated; doctor warns |
| 19 | Variables: published | DC | `figma variables published <key>` | |
| 20 | Variables: write (POST) | DC | `figma variables write <key> --from-json file.json` | bulk patch from JSON |
| 21 | Dev resources: list/CRUD | DC | `figma dev-resources {list,add,update,delete}` | path-safe local cache |
| 22 | Webhooks v2 CRUD | DC | `figma webhooks {list,create,get,update,delete}` | strips `v2` from command names |
| 23 | Webhook request log | DC | `figma webhooks requests <id>` | feeds the replay command |
| 24 | Activity logs | DC | `figma activity-logs --start --end --query` | OAuth-only; doctor enforces |
| 25 | Library analytics — components | DC | `figma analytics components <key> --kind=actions\|usages` | Enterprise; cache-friendly |
| 26 | Library analytics — styles | DC | `figma analytics styles <key>` | |
| 27 | Library analytics — variables | DC | `figma analytics variables <key>` | |
| 28 | Whoami / current user | DC | `figma me` | OAuth-preferred; PAT fallback with friendly error |
| 29 | oEmbed lookup | DC | `figma oembed --url=...` | public, unauthenticated |
| 30 | Payments status | DC | `figma payments` | plugin/widget payment status |
| 31 | Developer logs | DC | `figma developer-logs` | OAuth-app debug log |
| 32 | OAuth login flow | DC | `figma auth oauth` (browser PKCE) | |
| 33 | PAT setup | DC | `figma auth pat` (interactive) | validates against `/v1/files/<probe>` rather than `/me` (PAT 403 on /me) |
| 34 | Doctor: rate-limit + plan probe | DC | `figma doctor` | reads `X-Figma-Plan-Tier`, `X-Figma-Rate-Limit-Type`; warns on Enterprise endpoints attempted with PAT; runs probe matrix (token × {`/me`, `/files/X`, `/activity_logs`}) |
| 35 | Bulk image download (resumable, rate-limit-aware) | G | `figma images download <key> --ids=... --format=png --scale=2 --out=./assets` | imageRef vs gifRef distinction; respects `Retry-After` |
| 36 | Animated GIF export | G | `--format=gif --gif-ref=<ref>` | inherits imageRef vs gifRef |
| 37 | Crop-aware export | G | honors transform matrix in node fills, emits crop suffix | |
| 38 | Design tokens export | FM, FE-RMR, TS, Temzasse | `figma tokens export <key> --format=css\|scss\|json\|w3c` | one CLI replaces three; W3C DTCG default |
| 39 | Bulk component → SVG/PNG by project | FE-MM | `figma components export-batch <project_id> --format=svg --out=./icons` | walks projects → files → components → renders, rate-limit budgeted |
| 40 | Library inventory dump | FE-RMR | `figma library inventory <team_id>` | components × styles × variables across team libraries; emits Markdown + CSV |
| 41 | File watch (poll meta) | NEW | `figma files watch <key> --interval=5m` | polls `/files/{key}/meta` (Tier-1-light), notifies on `last_modified` change |
| 42 | Sync to local SQLite | NEW | `figma sync teams\|projects\|files\|components\|comments\|webhooks` | `last_modified` cursor on files; image-URL cache invalidation |
| 43 | Local SQL queries | NEW | `figma sql` (built-in) | cross-cutting joins (file × components × usage × variables) |
| 44 | FTS5 search | NEW | `figma search "checkout"` | full-text across files / comments / components / dev-resources / variables |
| 45 | Output formats | DC | global `--json`/`--yaml`/`--text`/`--select` | default agent-native shape |
| 46 | --confirm flag | DC | global flag for delete ops | |
| 47 | --dry-run flag | DC | global flag | |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Persona Served | Score | Buildability Proof |
|---|---|---|---|---|---|---|
| T1 | Compaction-aware frame extract for codegen prompts | `figma frame extract <key> --ids=1234-5678 --depth=4 --include=variables,dev-resources,code-connect` | GLips ships compaction in a Node MCP only; no other tool fuses node tree + in-scope variables + dev resources + Code Connect into one compact payload, accepts both `1234:5678` and `1234-5678`, and resolves instance-override chains like `I5666:180910;1:10515`. | Maya | 10/10 | `GET /v1/files/{key}/nodes` + `GET /v1/files/{key}/dev_resources` + locally-synced `variables_local` joined by node id; ported `simplifyRawFigmaObject` compaction in Go; emits compact JSON with `simplifiedNodeCount`. |
| T2 | Dev-mode resource bundle for a single node | `figma dev-mode dump <key> --node=<id> --format=md` | Official Dev Mode MCP is closed-source and Desktop-bound; no surveyed tool emits a portable Markdown bundle fusing dev-resource links, variables in scope, render permalink, and Code Connect mapping for one node. | Maya | 9/10 | `GET /v1/files/{key}/nodes?ids=<id>` + `GET /v1/files/{key}/dev_resources` filtered by node + `variables_published` joined by `variableId`; emits Markdown. |
| T3 | Cross-file unresolved comments audit | `figma comments audit --older-than=14d --group-by=file,author` | `figma comments list` ships per-file; no surveyed tool aggregates comments across every synced team file with age + group-by; brief Top Workflow #3. | Priya / design-ops | 10/10 | Locally-synced `comments`; SQL `WHERE resolved_at IS NULL AND created_at < now()-INTERVAL`. |
| T4 | Stale-component / style / variable orphans finder | `figma orphans <team_id> --kind=component,style,variable --window=30d` | Library analytics is per-entity-per-file in the UI; no surveyed tool joins `components` ⨝ `analytics.usages` to surface published-but-zero-usage entities across an entire team library. | Priya | 10/10 | Locally-synced `components`/`styles`/`variables_published` ⨝ `analytics_*_usages` on `key`; aggregate `SUM(total) = 0` over window. Enterprise-gated. |
| T5 | Semantic tokens diff between file versions | `figma tokens diff <key> --from=<v1> --to=<v2> --format=md` | No surveyed tool diffs Figma variables across two `file_versions` with mode-awareness. | Diego | 9/10 | `GET /v1/files/{key}/versions` resolves ids; `GET /v1/files/{key}/variables/local` snapshotted by `file_version`; set-diff + value-compare in Go. |
| T6 | Deterministic file fingerprint for CI contract | `figma fingerprint <key> --expect=<hash>` | No surveyed tool offers a stable hash of a Figma file's token + component + style surface; figmagic emits artifacts but not a contract-check exit code. | Diego | 7/10 | Locally-synced `variables_local` + `components` + `styles`; sort by `key`, content-hash canonical JSON via `crypto/sha256`; exit 0/2 on match/mismatch. |
| T7 | Webhook delivery replay against new target | `figma webhooks test <id> --replay-failed --target-url=https://localhost:3000/figma` | No surveyed tool pulls Figma's webhook request log and replays stored payloads (with original headers + HMAC) against an arbitrary target. | Sam | 9/10 | `GET /v2/webhooks/{id}/requests` fetches deliveries; filter `status >= 400`; replay via stdlib `net/http` with captured payload + original headers + HMAC re-signed under the webhook's `passcode`. |
| T8 | Variable usage tracer | `figma variables explain <key> --variable=<name>` | Figma's UI shows variable references per-node modally; no surveyed tool emits a flat list of every node + component that references a given variable across the file. | Priya | 8/10 | Locally-synced `variables_local` + `node_variable_refs` join table populated during `figma sync files` (extracted from each node's `boundVariables`); SQL JOIN on `variable_id`. |
