# Clay CLI Absorb Manifest

Tools surveyed: Clay official `clay` CLI (clay-run/agent-plugins), Clay official MCP
(mcp.clay.com), bcharleson/clay-gtm-cli, bpw-civic/clay-mcp-server, Clay Public API
(OpenAPI 3.1). Excluded as wrong product: clay.earth MCPs, clay-run/clay-cli (FaaS),
clay/claycli (CMS), ClaySolutions, claydotio.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List tables | official clay CLI `tables list` | clay-pp-cli workbooks overview | Offline SQLite mirror, `--json`, no Enterprise gate |
| 2 | Get table + fields | official clay CLI `tables get` | clay-pp-cli tables get | Returns full field graph incl. formula + action bindings |
| 3 | Get column schema | official clay CLI `tables columns` | clay-pp-cli tables schema | Adds semanticType/dataType per column |
| 4 | Read rows | official clay CLI `tables rows list` | clay-pp-cli tables records | Works without Enterprise API table sync |
| 5 | Row count | official clay CLI (via query) | clay-pp-cli tables count | Single call, no sync required |
| 6 | Toggle query-enabled | official clay CLI `tables update --query-enabled` | (behavior in clay-pp-cli tables update) | Same PATCH, plus rename in one command |
| 7 | List workbooks | official clay CLI `workbooks list` | clay-pp-cli workspace get | Included in workspace fetch |
| 8 | Search GTM database | official clay CLI `search`, official MCP | (generated endpoint) public search_create | Public API parity |
| 9 | List search filter fields | Public API | (generated endpoint) public search_fields | Public API parity |
| 10 | Structured table query | Public API `/tables/query` | (generated endpoint) public tables_query | Public API parity |
| 11 | Authenticated user/workspace | Public API `/me` | (generated endpoint) public me | Public API parity |
| 12 | List connected accounts | Clay UI only | clay-pp-cli accounts list | Never before scriptable |
| 13 | Provider auth metadata | Clay UI only | clay-pp-cli accounts provider | Shows auth type + validate action per provider |
| 14 | Enrichment catalog search | official MCP (partial) | clay-pp-cli enrichments search | Full catalog incl. waterfalls, claygent, actions |
| 15 | Enrichment param schema | Clay UI only | clay-pp-cli enrichments params | Resolves dynamic inputs per action |
| 16 | Subroutines list | official MCP `list_subroutines` | clay-pp-cli workspace subroutines | Same data, scriptable + cached |
| 17 | Record sources | Clay UI only | clay-pp-cli workspace sources | Never before scriptable |
| 18 | Workspace permissions | Clay UI only | clay-pp-cli workspace permissions | Never before scriptable |
| 19 | **Create table** | NOBODY (Clay CLI: "Not supported") | clay-pp-cli tables create | First tool that can do this |
| 20 | **Create column** | NOBODY (Clay CLI: "Not supported") | clay-pp-cli columns create | text / formula / action / http-api |
| 21 | **Update column** | NOBODY | clay-pp-cli columns update | |
| 22 | **Create workbook** | NOBODY | clay-pp-cli workbooks create | |
| 23 | **Generate formula from prompt** | NOBODY | clay-pp-cli formulas | Natural language to Clay formula |
| 24 | Per-column run status | Clay UI only | clay-pp-cli tables runstatus | Enrichment progress counts per field |
| 25 | **Delete column** | NOBODY | clay-pp-cli columns delete | DELETE /fields/{id}, verified 200 |
| 26 | Workbook topology | Clay UI canvas only | clay-pp-cli workbooks overview | Returns nodes+edges graph with per-table credit estimate |
| 27 | **Change a column's formula** | NOBODY | (behavior in clay-pp-cli columns update) | PATCH typeSettings.formulaText on an existing column |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Table blueprint export | blueprint export | hand-code | Requires reading table details + schema, then serializing the whole column graph (types, formulaText, actionKey, inputsBinding, authAccountId) into portable YAML. No Clay surface exports this. | Use this to snapshot a proven table design into a file you can commit to git. Do NOT use it to export row data; use 'tables records' for rows. |
| 2 | Table blueprint apply | blueprint apply | hand-code | Requires replaying a blueprint through POST /tables then N x POST /fields with field-id remapping, because formulas reference columns by generated id. Only possible now that column creation is scriptable. | Use this to rebuild a captured table design in a new workbook or vertical. Do NOT use it to edit an existing table in place; use 'columns update'. |
| 3 | Column dependency graph | columns graph | hand-code | Formulas embed `{{f_...}}` references; building the DAG needs the full local field mirror plus a resolver that maps ids back to names. | Use this to see which columns feed which. Do NOT use it for enrichment run health; use 'runstatus'. |
| 4 | Broken-reference doctor | columns doctor | hand-code | Requires a local join of every formula's `{{f_}}` refs against the live field set to find dangling references and orphan columns. | Use this to find formulas pointing at deleted columns before a run wastes credits. |
| 5 | Enrichment provider compare | enrichments compare | hand-code | Requires joining catalog results with institutional-knowledge labels (GDPR, Cost Efficient, EMEA Accuracy) and the workspace's connected accounts to say which providers you can actually run today. | Use this to pick a provider for a job. Do NOT use it to run enrichment; it is advisory only. |
| 6 | Table schema diff | tables diff | hand-code | Requires two locally mirrored column graphs and a structural comparison across type, formula text, and action bindings. | Use this to see what changed between two tables or two blueprint versions. |
| 8 | Cross-table lookup column | columns link | hand-code | Requires creating an `action` column bound to `lookup-row-in-other-table` with the target table id and a join key, then remapping both tables' field ids. No Clay surface scripts this. | Use this to connect two tables so one pulls values from the other. Do NOT use it for same-table references; use a formula instead. |
| 9 | Workbook topology graph | workbooks graph | hand-code | Requires reading the workbook node/edge graph and joining it against each table's local column mirror to show which tables feed which and what each costs. | Use this to see how your tables and CSV sources connect. Do NOT use it for column-level dependencies; use 'columns graph'. |
| 10 | Formula edit in place | columns set-formula | hand-code | Requires reading the current formula, resolving `{{f_}}` refs to names, accepting an edited formula, and PATCHing typeSettings back with ids re-resolved. | Use this to change a formula without rebuilding the column. |
| 11 | Typed error surfacing | errors | hand-code | Clay returns a structured envelope (`type`, `message`, `details.bodyErrors.issues[]`) plus per-field run status counts; surfacing both together needs a local join. | Use this to see why a column failed. Do NOT use it to re-run enrichment. |
| 12 | Row read with column names | tables rows | hand-code | Clay needs two calls (view record ids, then bulk cell fetch) and returns cells keyed by generated field ids; only a local join with the schema yields readable rows. | Use this to read rows and spot failing cells. Do NOT use it to write data. |
| 7 | Enrichment run watch | watch | hand-code | Requires polling fields/runstatus and aggregating status counts per column until the table settles. | Use this after triggering enrichment to block until runs finish. |

## Known Gaps
- RESOLVED. Row listing is a two-step flow:
  `GET /tables/{t}/views/{v}/records/ids` then
  `POST /tables/{t}/bulk-fetch-records` with those ids. Originally missed because
  the traffic recorder was armed after the grid had already painted; re-capturing
  with a pre-page-load recorder found it.

## Stubs
None. Every row above ships fully implemented or is a generated endpoint command.

## Notes
- Rows 19-23 are the user's stated vision and are the reason this CLI exists.
- Auth is `composed`: `claysession` cookie for `/v3`, `clay-api-key` header for
  `/public/v0`. Verified that each API rejects the other's credential.
