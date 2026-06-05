# Power BI CLI — Absorb Manifest

## Sources Surveyed
- **powerbi-cli/powerbi-cli (`pbicli`)** — npm, 39 stars, cross-platform Node CLI. Closest competitor. 20 command groups.
- **Microsoft Remote MCP server** — hosted, AI-native, NL→DAX. Copilot-only, not a CLI.
- **`Az.PowerBI` PowerShell module** — official, complete, Windows-first, verbose, not Unix-friendly.
- **MinaSaad1/pbi-cli** — Windows-only Python, requires Power BI Desktop, model authoring. Different niche, not a competitor.
- **sulaiman013/powerbi-mcp** — community MCP. Same caveat as Microsoft's: MCP client only.
- **Microsoft Learn REST API docs** — canonical reference for the 17 operation groups.

## Absorbed (match or beat every read-only feature that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List workspaces (groups) | `pbicli group list` | `powerbi groups list` | `--json`, `--select`, `--csv`, exit-code typed |
| 2 | Get workspace | `pbicli group show` | `powerbi groups get` | Same + agent-native output |
| 3 | List datasets in workspace | `pbicli dataset list` | `powerbi datasets list-in-group` | Same |
| 4 | Get dataset metadata | `pbicli dataset show` | `powerbi datasets get-in-group` | Same |
| 5 | List dataset datasources | `pbicli dataset list-datasources` | `powerbi datasets list-datasources-in-group` | Same |
| 6 | Dataset refresh history | `pbicli dataset refresh history` | `powerbi datasets refresh-history-in-group --top N` | Same + filterable |
| 7 | Dataset parameters | `pbicli dataset list-parameters` | `powerbi datasets parameters-in-group` | Same |
| 8 | Execute DAX query | `pbicli dataset execute` (POST executeQueries) | `powerbi datasets execute-queries-in-group` | + `--csv`, `--select`, `--file`, `--out`, `--include-nulls`, structured 100K/120-rpm error surfacing |
| 9 | List reports in workspace | `pbicli report list` | `powerbi reports list-in-group` | Same |
| 10 | Get report | `pbicli report show` | `powerbi reports get-in-group` | Same |
| 11 | Get report pages | `pbicli report pages` | `powerbi reports get-pages-in-group` | Same |
| 12 | Export report to file (async POST) | `pbicli report export-to-file` | `powerbi reports export-to-file-in-group` (raw POST) | Same surface for raw use |
| 13 | Poll export status | `pbicli report export-to-file-status` | `powerbi reports export-status-in-group` | Same |
| 14 | Download exported file | `pbicli report export-to-file-get` | `powerbi reports export-download-in-group` | Same |
| 15 | List dashboards | `pbicli dashboard list` | `powerbi dashboards list-in-group` | Same |
| 16 | Get dashboard tiles | `pbicli dashboard tile list` | `powerbi dashboards get-tiles-in-group` | Same |
| 17 | List dataflows | `pbicli dataflow list` | `powerbi dataflows list-in-group` | Same |
| 18 | Dataflow refresh history (transactions) | `pbicli dataflow transactions` | `powerbi dataflows transactions-in-group` | Same |
| 19 | Get dataflow definition | `pbicli dataflow export` (model.json) | `powerbi dataflows get-definition-in-group` | Same |
| 20 | List installed apps | `pbicli app list` | `powerbi apps list` | Same |
| 21 | Get app reports / dashboards | `pbicli app *` | `powerbi apps list-reports / list-dashboards` | Same |
| 22 | Doctor / connectivity check | `pbicli login --azurecli` (no doctor) | `powerbi doctor` | Generator-emitted, beats pbicli's none |
| 23 | Local SQL query over cached resources | None | `powerbi sql "SELECT name FROM datasets"` | Generator-emitted, totally new in this space |
| 24 | Offline full-text search | None | `powerbi search "sales"` | Generator-emitted, FTS over workspace/dataset/report names |
| 25 | Agent context probe | None | `powerbi agent-context` | Generator-emitted |
| 26 | Local catalog sync | None | `powerbi sync` | Generator-emitted, pulls every entity into SQLite |
| 27 | MCP server bundled | Microsoft Remote MCP (hosted only) | `powerbi-pp-cli-mcp` stdio binary | Local, free, all CLI surface as MCP tools |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|-------------------------|
| 1 | One-shot report export | `report-export REPORT_ID --format pdf --output report.pdf --wait` | 9/10 | Wraps POST→poll→download in one synchronous command. Every competitor exposes the three async steps separately. |
| 2 | Saved DAX query catalog | `dax save NAME "EVALUATE ..."`, `dax run NAME --workspace W --dataset D` | 8/10 | Named, parameterizable, version-controllable DAX queries in local SQLite. `pbicli` accepts inline DAX only — no library. |
| 3 | Refresh failure dashboard | `refreshes failures --days 7` | 8/10 | Iterates the local catalog, pulls each dataset's refresh history, surfaces only the failing ones with error messages. Foundation: local SQLite. |
| 4 | DAX-from-file with CSV out | `dax run --file query.dax --csv > out.csv` | 7/10 | Pipe-friendly DAX execution. `pbicli execute` outputs only JSON; users hand-roll jq pipelines. |
| 5 | Auth doctor with tenant settings explainer | `auth doctor` | 7/10 | Decodes the JWT, surfaces scopes/expiry, checks "Allow service principals to use Power BI APIs" tenant setting, runs a `/groups` probe. Power BI has 5 distinct auth failure modes; this names which one is biting. |
| 6 | Best-effort dataset schema introspection | `dataset describe DATASET_ID` | 7/10 | Falls back gracefully: tries `INFO.TABLES()` DAX (Premium only), then `INFO.MEASURES()`, then dataset metadata. Outputs a tree. Microsoft's NL-MCP can't expose this programmatically. |
| 7 | AAD token exchange built in | `auth login --tenant T --client-id C --client-secret S` | 7/10 | Hand-built command. Caches token to `~/.config/powerbi-pp-cli/token.json`, refreshes when expired. Most competitors require BYO token via `az`. |
| 8 (stub) | Auto-pagination over 100K-row DAX cap | `dax run --auto-paginate --partition-by Date` | 5/10 | **Ships as stub for v1.** Rewriting arbitrary DAX with TOPN windowing requires partition-column UX and result-merge logic that is not trivial. Stub emits "not yet wired — use TOPN/SKIP manually". |

Total: 7 fully-built transcendence features + 1 stub.
