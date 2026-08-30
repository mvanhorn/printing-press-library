# Is Agentic CLI Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Retrieve one completed report by public URL | Official OpenAPI `GET /api/v1/report` | (generated endpoint) report get-is-agentic-v1 | Typed endpoint, `--json`, `--dry-run`, structured errors, MCP exposure |
| 2 | Accept domain or full URL | Official npm CLI | (behavior in is-agentic-pp-cli report get-is-agentic-v1) url normalization | Domain-friendly input while preserving explicit HTTP(S) URLs |
| 3 | Human-readable score and breakdown | Official npm CLI | is-agentic-pp-cli report | Readable score, tier points, failure/partial sections, evidence and fixes |
| 4 | Unchanged JSON output | Official npm CLI | (behavior in is-agentic-pp-cli report get-is-agentic-v1) --json | Stable machine output with no human prose on stdout |
| 5 | RFC 9457 problem details | Official OpenAPI/docs | (behavior in is-agentic-pp-cli report get-is-agentic-v1) structured errors | Preserve `code`, `title`, `status`, `detail`, and `resolution` |
| 6 | Rate-limit-aware requests | Official docs | (behavior in is-agentic-pp-cli report get-is-agentic-v1) retry handling | Honor `Retry-After` and surface quota failures instead of empty results |
| 7 | Stable report URL and scan timestamp | Official API/schema | is-agentic-pp-cli report | Make report provenance visible in terminal and JSON |
| 8 | Official MCP report/methodology/docs surfaces | Official MCP docs/server card | (generated endpoint) report get-is-agentic-v1 plus framework MCP tools | Keep read-only agent reachability and structured schemas |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---|---|---|---|---|
| 1 | Snapshot ledger | history | hand-code | Stores local observations separately from service `scanned_at`, preserving audit chronology across six-hour server caching. | Use this command to inspect locally retained report snapshots and provenance. |
| 2 | Cross-report diff | diff | hand-code | Joins two locally retained reports and normalizes issue IDs to show score, tier, eligibility, added, removed, and changed findings. | Compare two retained audits; do not use this command for a first-time fetch. |
| 3 | Explainable CI policy gate | check | hand-code | Evaluates local or freshly fetched reports against user-owned score, tier, and regression policies with typed exit codes. | Use this command in CI to enforce an explicit readiness policy. |
| 4 | Portfolio matrix | portfolio | hand-code | Fetches and aggregates many one-target reports under the documented quota, then emits sortable fleet-level output. | Use this command to compare multiple public targets; do not use it for one-target remediation detail. |
| 5 | Issue lifecycle queue | issues | hand-code | Derives first-seen, last-seen, fixed, and regressed state by joining issue IDs across stored snapshots. | Use this command to manage recurring readiness findings across time. |
| 6 | Rate-aware fleet scheduler | portfolio --file | hand-code | Adds bounded concurrency, deduplication, retry-after handling, and resumable local results absent from the one-target API. | Use `portfolio --file` for a bounded multi-target refresh that respects the service quota. |
| 7 | Evidence bundle/export | evidence | hand-code | Packages report JSON, provenance, and optional diff into a portable artifact for tickets and release evidence. | Export a self-contained audit artifact for review or attachment. |

## Scope Notes
- The website's browser-only scan initiation (`POST /scan/<target>` and `/api/scan/refresh`) is not promoted as a generated API endpoint. The CLI will use only the documented report GET for stable production transport; a missing report is reported honestly with the canonical browser URL.
- No approved stubs.
- All novel commands must remain read-only and non-interactive, support `--json` and `--dry-run` where applicable, and preserve empty lists as `[]`.
