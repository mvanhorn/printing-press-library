# Customer.io CLI — Absorb Manifest

The CLI absorbs every feature exposed by competing tools (the official `customerio/cli` plus the Customer.io ecosystem of SDKs), then transcends with 8 commands no other tool offers.

## Source tools surveyed

| Tool | Coverage | Stars | Last release | Contribution to manifest |
|---|---|---|---|---|
| [customerio/cli](https://github.com/customerio/cli) | Journeys + CDP via SA-token | 0 | 2026-05-06 (v0.0.4) | Core absorb model: SA→JWT auth, region select, schema introspection |
| [customerio-node](https://www.npmjs.com/package/customerio-node) | Track + App | — | 2026-04-28 (v4.4.0) | Identify, transactional send, broadcast trigger |
| [customerio-go](https://github.com/customerio/go-customerio) | Track + App | 30 | 2026-04-28 | Same |
| [customerio-python](https://github.com/customerio/customerio-python) | Track + App | 65 | 2026-04-28 | Same |
| [customerio-ruby](https://github.com/customerio/customerio-ruby) | Track + App | 69 | 2026-04-28 | Same |
| [@customerio/cdp-analytics-js](https://github.com/customerio/cdp-analytics-js) | CDP analytics | 9 | — | CDP control-plane awareness |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Generic API passthrough | customerio/cli `cio api` | `customer-io api <method> <path>` typed at endpoint level | Typed flags/exit codes per endpoint; --dry-run; --json/--select |
| 2 | OpenAPI schema introspection | customerio/cli `cio schema` | `customer-io schema list/show` | Plus offline cache, version pinning |
| 3 | List/get/create/update/delete customers | SDKs | `customer-io customers <op>` typed | --json --select --csv; idempotent; --dry-run |
| 4 | Identify customer (App API) | SDK identify() | `customer-io customers update --id` | --dry-run; batch via stdin |
| 5 | Trigger transactional message (email) | SDK sendEmail() | `customer-io transactional send --template <id>` | --dry-run; metrics follow-up |
| 6 | Trigger transactional SMS | App API | `customer-io transactional send-sms` | Same agent-native plumbing |
| 7 | Trigger transactional push | App API | `customer-io transactional send-push` | Same |
| 8 | Trigger broadcast / campaign | App API | `customer-io broadcasts trigger <id>` | Adaptive 1/10s rate-limit handling |
| 9 | List campaigns | App API | `customer-io campaigns list` | --since/--until time filters |
| 10 | Get campaign metrics | App API | `customer-io campaigns metrics <id>` | --window for time-series |
| 11 | Get journey metrics (funnel) | App API | `customer-io campaigns journey-metrics <id>` | Same |
| 12 | List segments | App API | `customer-io segments list` | --json --select |
| 13 | Get segment | App API | `customer-io segments get <id>` | Same |
| 14 | Get segment members | App API | `customer-io segments members <id>` | Pagination, --csv |
| 15 | Export segment (start) | App API | `customer-io exports segment <id>` | Auto-poll + signed-URL download |
| 16 | Export deliveries | App API | `customer-io exports deliveries` | 19 export sub-types |
| 17 | Get export status | App API | `customer-io exports status <id>` | Auto-resume |
| 18 | Download export | App API | `customer-io exports download <id> --to <file>` | Signed-URL fetch |
| 19 | List deliveries | App API | `customer-io deliveries list` | --recipient, --campaign, --since |
| 20 | Get delivery | App API | `customer-io deliveries get <id>` | --json |
| 21 | Suppress customer | App API | `customer-io suppressions add <email|id>` | Bulk via stdin |
| 22 | Unsuppress customer | App API | `customer-io suppressions remove <email|id>` | Same |
| 23 | List suppressions | App API | `customer-io suppressions list` | --since for new suppressions |
| 24 | List webhooks | App API | `customer-io webhooks list` | Reporting Webhooks |
| 25 | Create / delete webhook | App API | `customer-io webhooks create/delete` | --dry-run |
| 26 | List integrations | App API | `customer-io integrations list` | --json |
| 27 | Get account / workspaces | App API | `customer-io workspaces list / current` | Workspace switching |
| 28 | List CDP sources | CDP API | `customer-io cdp sources list` | Endpoint-mirrored |
| 29 | List CDP destinations | CDP API | `customer-io cdp destinations list` | Endpoint-mirrored |
| 30 | List Reverse-ETL syncs | CDP API | `customer-io cdp reverse-etl list` | Premium feature |
| 31 | Trigger Reverse-ETL sync | CDP API | `customer-io cdp reverse-etl trigger <id>` | --dry-run |
| 32 | List in-app messages | App API | `customer-io in-app list` | Endpoint-mirrored |
| 33 | List objects (relationship graph) | App API | `customer-io objects list` | --type filter |
| 34 | List transactional templates | App API | `customer-io transactional templates list` | Search by name |
| 35 | Get transactional metrics | App API | `customer-io transactional metrics <id>` | Time-windowed |
| 36 | Auth: SA token → JWT exchange | customerio/cli | `customer-io auth login --sa-token` | Token cache; --region us|eu |
| 37 | Auth: doctor / status | (universal) | `customer-io doctor` | Validates token, lists workspaces, prints account_id |
| 38 | Region select us/eu | customerio/cli | `--region` global flag + config | Same |
| 39 | Workspace select | (universal) | `--workspace` flag + config | Same |
| 40 | Local cache: sync | (printing-press generic) | `customer-io sync` | Per-resource or full |
| 41 | Local cache: SQL | (printing-press generic) | `customer-io sql "SELECT ..."` | Read-only SQLite |
| 42 | Local cache: search FTS | (printing-press generic) | `customer-io search <term>` | Cross-resource FTS |
| 43 | Pagination, --json/--select/--csv/--compact | (printing-press generic) | universal flags | (table stakes) |
| 44 | --dry-run for mutations | (printing-press generic) | universal flag | (table stakes) |
| 45 | Adaptive rate limiter (broadcast 1/10s) | hand-rolled | per-source limiter on broadcasts | Avoids 429 storms |
| 46 | Bundled MCP server | (none — gap!) | `customer-io mcp` (stdio + http) | NO competing tool has MCP |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---|---|---|---|
| 1 | Journey funnel × segment | `customer-io campaigns funnel <id> [--segment <id>] [--since 7d]` | 9/10 | Local SQL join across `deliveries` × `segment_members`. The API exposes per-campaign journey_metrics but no per-segment breakdown — exactly the "what fraction of segment X opened journey Y" the brief flagged. |
| 2 | Segment overlap | `customer-io segments overlap <id-a> <id-b> [<id-c>...]` | 8/10 | Multi-way Venn over `segment_members`. UI offers no multi-segment intersection; no SDK exposes it. |
| 3 | Customer 360 timeline | `customer-io customers timeline <email-or-id> [--since 30d]` | 8/10 | Per-customer chronological merge across customers/deliveries/suppressions/segment_members. The UI buries this across 4+ pages. |
| 4 | Broadcast pre-flight | `customer-io broadcasts preflight <id> [--segment <id>]` | 8/10 | Pre-trigger safety: target size, suppression overlap, last-sent recency from local deliveries. The 1/10s broadcast throttle makes "safe to send?" a real risk. |
| 5 | Suppression audit | `customer-io suppressions audit [--since 30d] [--reason ...]` | 7/10 | Joins suppressions to triggering bounce/complaint deliveries — no API exposes the join. |
| 6 | Reverse-ETL health | `customer-io cdp reverse-etl health [--since 24h] [--watch]` | 7/10 | Named verb over RETL run history; the official `cio` CLI is generic passthrough only. Maps to Devon's daily check. |
| 7 | Bulk suppress with audit log | `customer-io suppressions bulk add --from-csv <file>` | 6/10 | Real suppress calls + adaptive throttle + append-only local JSONL audit log keyed by date. |
| 8 | Delivery triage bundle | `customer-io deliveries triage --template <id> --status bounced --since 1h --bundle <dir>` | 6/10 | Writes self-contained `bundle/{summary.md, deliveries.jsonl, recipients.txt}` for incident handoff. summary.md uses SQL group-by on error reasons. |

All 8 transcendence features ship as **shipping-scope** (fully implemented, not stubs). No row carries `(stub)` status.

## Stubs

None.

## Killed novel candidates (audit trail)

8 candidates were generated but cut in adversarial Pass 3 (full reasons in the brainstorm file). Summary: transactional test-matrix (sub-weekly use), segment diff (data-model lift), recipient fatigue (overlaps preflight), schema diff (one-shot use), identify-from-jsonl (covered by manifest #4), engagement matrix (overlaps funnel), doctor --deep (fold into existing doctor).

## Total scope

- **46 absorbed features** (everything cio + SDKs + UI workflows expose)
- **8 transcendence features** (3 ≥8/10, 3 ≥7/10, 2 ≥6/10)
- **Total surface: 54 named commands**, plus the runtime Cobra-tree-walked endpoint mirror (estimated 100+ tools at MCP server start, mitigated by Cloudflare-pattern enrichment)
