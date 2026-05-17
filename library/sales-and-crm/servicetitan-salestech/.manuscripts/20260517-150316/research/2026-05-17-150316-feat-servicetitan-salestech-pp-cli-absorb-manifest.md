# servicetitan-salestech Absorb Manifest

## Source Inventory (no external competitor)

There is no existing CLI for the ServiceTitan Sales/Estimates API. The MCP family of `mcp__servicetitan__st_*` tools covers some adjacent surface (jobs, customers) but exposes the Sales/Estimates module as raw endpoint mirrors with high token cost. The "best source" for absorption is the spec itself plus six sibling ST-module CLI implementations (`servicetitan-{jpm,crm,dispatch,inventory,pricebook,memberships}`) for shape conformance.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List estimates with filters (jobId, projectId, jobNumber, totalGreater/Less, soldById, soldByEmployeeId, ids, page, modifiedOnOrAfter, etc.) | Estimates_GetList endpoint | `estimates get-list --job-id N --total-greater 5000 --json` | --json, --select dotted paths, --csv, --limit, --agent flag, agent-friendly defaults |
| 2 | Get single estimate by id | Estimates_Get | `estimates get <id>` | --json, --select with nested-items dotted paths, --compact |
| 3 | Create estimate | Estimates_Create | `estimates create --stdin file.json --dry-run` | --dry-run, --stdin batch, typed exit codes |
| 4 | Update estimate | Estimates_Update | `estimates update <id> --stdin patch.json --dry-run` | --dry-run + idempotent semantics |
| 5 | Dismiss estimate | Estimates_Dismiss | `estimates dismiss <id> --reason "<text>"` | --dry-run, --json |
| 6 | Sell estimate (close + mark sold) | Estimates_Sell | `estimates sell <id> --sold-on 2026-05-17 --sold-by <employee>` | --dry-run, --json |
| 7 | Unsell estimate (reverse close) | Estimates_Unsell | `estimates unsell <id>` | --dry-run, --json |
| 8 | Put estimate line item (create or update) | Estimates_PutItem | `estimates items put <estimate-id> --stdin item.json` | --dry-run, --stdin batch |
| 9 | Delete estimate line item | Estimates_DeleteItem | `estimates items delete <estimate-id> <item-id>` | --dry-run, --json |
| 10 | List estimate items with filters | Estimates_GetItems | `estimates items get-list --estimate-id N --active --json` | --json, --select, --csv, --sort, --limit |
| 11 | Export feed (current) — cursor + recent-changes | EstimatesExport_Estimates | `export estimates --from <iso> --include-recent-changes` | --json, structured cursor output |
| 12 | Export feed (legacy) — cursor + recent-changes | EstimatesExport_EstimatesAsyncLegacy | `estimates export --from <iso> --include-recent-changes` | --json (kept for compatibility) |
| 13 | Status change audit feed per estimate | EstimatesStatus_GetEstimateStatusChanges | `status estimates changes <id>` | --json with UTC timestamps |
| 14 | Composed auth: ST_APP_KEY + OAuth2 bearer | Sibling ST CLIs | `auth login`, `auth status`, `doctor` | TrimSpace defense, SHA12 cross-check guidance in docs |
| 15 | Local SQLite mirror of estimates + items + status changes | Generator framework | `sync`, FTS5 search across name/summary/jobNumber/sku fields | Offline composability with jq/agents |
| 16 | MCP intent surface (stdio + http transport) | x-mcp Cloudflare pattern | `servicetitan-salestech-pp-mcp` collapses 13 endpoint mirrors → 2 intent tools | ~95% per-turn token reduction vs raw mirrors |
| 17 | Reopen a Dismissed estimate (or reset Sold → Open) | Estimates_Update with status field | `estimates reopen <id> --dry-run` | Convenience wrapper that hides the PUT/status-Open ceremony; reverses Dismiss in one command |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score | Persona |
|---|---------|---------|-------------------------|-------|---------|
| 1 | Stale quotes | `estimates stale --older-than 3d --status Open --json` | SQLite filter on Open + sort by computed (now - createdOn) × total; no API call sorts by computed age×$ | 10/10 | Sales Manager Sam |
| 2 | Rep leaderboard | `reports rep-leaderboard --since 2026-01-01 --json` | Per-rep close-rate + avg days-to-sell + sold $ requires joining estimates × status_changes and aggregating | 10/10 | Sales Manager Sam |
| 3 | Close rate | `reports close-rate --group-by businessUnit --since 90d` | sold/(sold+dismissed) pivoted on arbitrary dimension; not a single API call | 10/10 | Sam, Pierce |
| 4 | Days to sell | `reports days-to-sell --percentiles --since 90d` | Percentiles of (Sold timestamp − createdOn) per rep/BU; needs status_changes join | 9/10 | Sales Manager Sam |
| 5 | Dismissed reasons | `reports dismissed-reasons --since 90d --top 20` | Exact-match group-by on reason text from status_changes; mechanical count, no NLP | 7/10 | Sales Manager Sam |
| 6 | Estimate audit | `audit estimate <id>` | Single-estimate forensic: header + every line item + full status timeline in one shaped output | 10/10 | Dispatcher Dana |
| 7 | Recent changes | `audit recent-changes --since 24h --json` | All estimates whose status changed in window with from→to + actor; sibling pattern from CRM | 10/10 | Dispatcher Dana |
| 8 | Find | `find "well pump" --status Open --min-total 5000` | FTS5 across name/summary/jobNumber/sku.name/sku.displayName with structured filters | 10/10 | Dana, Tech Tom |
| 9 | Pipeline snapshot | `reports pipeline --as-of 2026-05-17 --json` | Reconstructs total $ Open/Sold/Dismissed for arbitrary past date by replaying status_changes; impossible via API | 8/10 | Sam, Pierce |
| 10 | Health | `health` | Cross-source reconciliation: API count vs local count vs last cursor age per table | 8/10 | Pierce |
| 11 | SKU frequency | `reports sku-frequency --on sold --since 90d --top 50` | Joins estimate_items with estimates filtered by status; API only returns items per single estimate | 7/10 | Pierce, Sam |
| 12 | Rep follow-ups (user-added) | `reports follow-ups --rep <id\|all> --since 48h --json` | Per-rep open estimates from last N hours with customerId, jobId/jobNumber, soldByName, and deeplinks so reps can call customers back. Phone enrichment uses sibling `servicetitan-crm-pp-cli customers get` (documented in README recipe). | 10/10 | Sales Manager Sam, Rep |
| 13 | Record follow-up (user-added, local write) | `audit follow-up add <estimate-id> --note "..." --remind 2026-05-20` and `audit follow-ups list --due-by <date>` | Local SQLite log of follow-up activity against estimates with reminder dates. The ST API has no estimate-notes endpoint; this fills the gap without losing the cross-call audit trail. | 9/10 | Rep, Sam |
| 14 | CSV → ServiceTitan estimate ingest (user-added) | `estimates import --csv quotes.csv --dry-run` | Reads a defined CSV schema and generates Estimates_Create + Estimates_PutItem calls per row with --dry-run preview and --batch-size flow control. CSV schema documented in README; XLSX and Google Sheets are documented as future flags (`--xlsx`, `--sheet-id`) requiring extra parsers/auth. | 10/10 | Pierce, Sam |

## Stubs

None planned. Every transcendence feature is buildable from the synced SQLite store + sibling-CLI proven patterns. XLSX and Google Sheets import are explicitly v2 (CSV-first in v1).

## Known Concerns / Things to Worry About

- **Dismissed-reasons reason field:** The status_changes endpoint returns reason text on each change. If the field is empty / sparse for `Dismissed` transitions, `reports dismissed-reasons` returns mostly empty buckets. Mitigation: the command surfaces this honestly with a `<no reason recorded>` group rather than dropping rows. Live dogfood will tell us how dense the field is on JKA's tenant.
- **Pipeline-snapshot at-as-of-date** depends on a complete status_changes feed back to estimate creation. If the tenant only retains N days of changes (unknown), `--as-of` requests further back than retention return reconstructed state with a "warning: data older than oldest status_change" annotation. Live dogfood will pin the retention window.
- **Composed-auth patch carry-forward.** v4.8.0 closed #1303/#1305 — sync registry + apiKey wiring should be generator-emitted. We verify post-generate before touching `internal/config/config.go`. If still missing, the 4 standing patches from the sibling pattern get applied (mirror pricebook). Recorded in `.printing-press-patches.json` only if patches are needed.
- **MCP intent surface** is the headline value prop — at 13 endpoints the raw-mirror cost is already low (~13 tools), but the intent surface collapses to 2 (search + execute) and unlocks remote HTTP transport for hosted agents. The proven x-mcp block in this spec produces this automatically.
