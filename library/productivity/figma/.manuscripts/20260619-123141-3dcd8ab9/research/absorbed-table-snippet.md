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

