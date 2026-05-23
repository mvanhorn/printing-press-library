# BuildAlert Absorb Manifest

## Ecosystem search results

Searched npm, PyPI, GitHub, claude-plugins-official, MCP marketplaces, lobehub, mcpmarket, fastmcp:
- **Zero competing CLIs found.** BuildAlert is a closed SaaS (no public API published, no published OpenAPI spec).
- **Zero MCP servers.**
- **Zero community SDK wrappers** on npm or PyPI.
- **Zero Claude plugins / skills.**

This means the absorb manifest matches the BuildAlert web dashboard itself, plus the cross-source workflows only a CLI can enable (transcendence).

## Absorbed (match every dashboard feature)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List planning-application leads matched to my filters | BuildAlert `/dashboard/leads` (`/dapi/leads/live-leads`) | `(generated endpoint) leads list` | Returns the same paginated payload, but `--json`/`--csv`/`--select` ready; supports composition with jq, fzf, ZAZU pipeline |
| 2 | Filter leads by project type (Extension, Loft_Conversion, etc.) | BuildAlert quickFilter tabs | `(behavior in buildalert-pp-cli leads list)` via `--project-types Extension,Loft_Conversion` | CSV composability; no clicking, scriptable from cron |
| 3 | Filter leads by minimum estimated value | BuildAlert API supports `minValue` (no obvious UI control) | `(behavior in buildalert-pp-cli leads list)` via `--min-value 50000` | Surfaces the API capability above what the dashboard exposes |
| 4 | Sort leads by date or estimated value | BuildAlert API `orderBy=createdDate|value` | `(behavior in buildalert-pp-cli leads list)` via `--sort value` (flag name; underlying param is `orderBy`) | Same as dashboard, scriptable |
| 5 | Paginate through all leads | BuildAlert dashboard pagination | `(behavior in buildalert-pp-cli leads list)` via `--page` + `--items-per-page` (or `sync --resources leads` for full mirror) | Bulk export via local sync; offline querying |
| 6 | Read planning description, address, applicant, council, lat/lng | BuildAlert lead row + Google Maps view | embedded in `leads list --json` (every field carried through verbatim from `/dapi/leads/live-leads.data[].application`) | All in one JSON; pipeable to ZAZU bd-mirror.sqlite |
| 7 | See AI-estimated project value band + reasoning | BuildAlert lead row | embedded in `leads list` (`estimationValueBand`, `estimationValueBandDescription`, `estimationReasoning`) | Same data, scriptable for value-floor filters across local store |
| 8 | View planning-portal URL for the lead | BuildAlert "View planning application" button | embedded in `leads list` (`application.url`) | Pipe into ZAZU pipeline; no manual click-through |
| 9 | View quickFilter category counts (149 total, 125 extensions, etc.) | BuildAlert quickFilters tab strip | `(generated endpoint) leads list` (`quickFilters` field in JSON) | Single command shows the breakdown |
| 10 | Inspect my BuildAlert user profile (postcode, radius, longitude, latitude, subscription state) | Dashboard sidebar | `(generated endpoint) user profile` | Includes activeSubscriber, credits, planningStatusToFilter — fields the web UI hides in dropdowns |
| 11 | Inspect my BuildAlert dashboard overview (newLeadsCount, totalPlanningApplications, lastLetterSentDate, credits) | `/dashboard/index` | `(generated endpoint) user dashboard` | Single command; pipeable to ZAZU status check |
| 12 | List my letter templates | `/dashboard/letter-templates` | `(generated endpoint) letter-templates list` | Same; supports `--json` for backup/version-control of template metadata |
| 13 | View transaction history (£2 letter charges) in a date window | `/dashboard/transactions` | `(generated endpoint) transactions list --date-from X --date-to Y` | Bulk export for accounting; pipe to xero, sheets, ZAZU spending tracker |
| 14 | View ROI tracking (letters sent, replies, conversion rate, work won, total return, chart data) | `/dashboard/tracking` | `(generated endpoint) tracking list --date-from X --date-to Y` | Aggregates exposed as structured JSON; can roll up across multiple windows |
| 15 | Run a backend liveness probe | (used internally by the dashboard, not user-facing) | `(behavior in buildalert-pp-cli doctor)` via `/dapi/healthcheck/keep-warm` | Cookie auth health check; useful in `buildalert-pp-cli doctor` |
| 16 | Local mirror of every lead in SQLite | (no equivalent — BuildAlert has no export feature) | `buildalert-pp-cli sync --resources leads` | Offline querying; historical retention as BuildAlert prunes old leads from the dashboard; agent-native |
| 17 | Full-text search across mirrored leads (description, address, applicant) | (no equivalent — dashboard has only category filters) | `buildalert-pp-cli search "loft conversion" --type leads` | Beats web filter; supports OR, NOT, phrase queries via SQLite FTS5 |
| 18 | SQL access to mirrored data | (no equivalent — dashboard is web-only) | `buildalert-pp-cli sql "SELECT ... FROM leads_fts WHERE ..."` | Compose arbitrary joins/aggregations; classic Steinberger bar |

## Mutation endpoints (out of v1 scope, explicit stubs)

The browser-sniff captured read-only traffic only. Mutations (filter update, letter send, schedule follow-up, template create/edit) need a deliberate second sniff pass where the user clicks the mutation UI elements. The user explicitly noted letter send is a real-money operation that should default to `--dry-run` anyway. For v1, these ship as documented stubs:

| # | Feature | Source | Disposition |
|---|---------|--------|-------------|
| 19 | Send a branded letter to a homeowner (£2) | "Secure this homeowner" button on each lead | `(stub) - mutation endpoint not browser-sniffed; £2/letter real-money op needs deliberate user consent. Future work after a second sniff pass.` |
| 20 | Schedule follow-up letter | BuildAlert automation UI | `(stub) - same reason as #19` |
| 21 | Update filter preferences (postcode, radius, project types) | Dashboard settings | `(stub) - mutation endpoint not browser-sniffed; users can update on the web for now` |
| 22 | Manage letter templates (create/edit/delete) | `/dashboard/letter-templates` editor | `(stub) - mutation endpoint not browser-sniffed` |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Score | Why Only We Can Do This |
|---|---------|---------|--------------|------:|------------------------|
| 1 | ZAZU diff — leads missing from ZAZU | `zazu-diff --zazu-db <path>` | hand-code | 10 | Left-anti-join local `applications` against ZAZU `bd-mirror.sqlite` on `(council_slug, reference)` derived from `internalUniqueReference`. No single API call answers "what does BuildAlert have that ZAZU missed?". |
| 2 | Pending-letter worklist | `pending-letters --zazu-db <path>` | hand-code | 10 | `canSendLetter=true AND letterBeenSent=false AND (council, ref) NOT IN zazu_sends`. The daily morning command. |
| 3 | Duplicate-letter guard | `letter-conflict --zazu-db <path>` | hand-code | 9 | Inner-join `letterBeenSent=true` rows with ZAZU's send log. Saves £2 + reputation per hit. |
| 4 | Council coverage gap map | `coverage --zazu-db <path>` | hand-code | 9 | Per-council volume delta across both stores; flags councils where BuildAlert sees many and ZAZU sees few (signal: ZAZU scraper missing) or vice versa. |
| 5 | Spend ledger by council/project-type | `analytics --type transactions --group-by council` | spec-emits | 9 | Generator-emitted analytics over the synced `transactions` resource. Answers "how much did I spend per council?". |
| 6 | Per-lead ROI joiner | `roi-per-lead --zazu-db <path>` | hand-code | 8 | Three-way local join `transactions × tracking × applications` keyed on lead reference; emits per-lead cost/reply/work-won. Optional ZAZU column. |
| 7 | Offline radius re-filter | `nearby --postcode HA1 --radius 10` | hand-code | 8 | Haversine on local lat/lng with a baked-in postcode→lat/lng prefix table; returns leads in radius without an API call. |

**Buildability count: 6 `hand-code` + 1 `spec-emits` = 6 hand-written novel-feature files.**

Audit trail (Customer model + full candidate list + killed candidates): see `2026-05-23-115243-novel-features-brainstorm.md`.
