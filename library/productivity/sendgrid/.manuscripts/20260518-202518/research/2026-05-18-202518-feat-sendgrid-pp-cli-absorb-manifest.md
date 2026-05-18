# SendGrid CLI Absorb Manifest

## Source Tools Surveyed
| Tool | Type | Surface | Repo |
|---|---|---|---|
| sendgrid/sendgrid-cli | Bash CLI (archive) | ~6 commands (subuser admin, webhooks toggle) | https://github.com/sendgrid/sendgrid-cli |
| tddschn/sendgrid-cli | Python CLI | send-only | https://github.com/tddschn/sendgrid-cli |
| twilio-cli email plugin | CLI plugin | email:set, email:send | https://www.twilio.com/docs/twilio-cli |
| sendgrid/sendgrid-go | Go SDK | full v3 raw client | https://github.com/sendgrid/sendgrid-go |
| sendgrid/sendgrid-nodejs | JS SDK | full v3 raw client | https://github.com/sendgrid/sendgrid-nodejs |
| sendgrid/sendgrid-python | Python SDK | full v3 raw client | https://github.com/sendgrid/sendgrid-python |
| Garoth/sendgrid-mcp | TS MCP | contacts, templates, single-sends, stats (~25 tools) | https://github.com/Garoth/sendgrid-mcp |
| garethcull/sendgrid-mcp | Python MCP | stats pull, template save | https://github.com/garethcull/sendgrid-mcp |
| deyikong/sendgrid-mcp | Node MCP | basic surface | https://github.com/deyikong/sendgrid-mcp |
| mcpmarket sendgrid (hosted) | hosted MCP | "59 tools" | https://mcpmarket.com |
| davepoon/buildwithclaude sendgrid-automation | Claude skill | composio passthrough | https://github.com/davepoon/buildwithclaude |

**No competitor covers the full 391-operation v3 surface. The biggest dedicated tool (mcpmarket) lists 59 tools; the official archive-quality CLI covers a handful.**

## Absorbed Features (full v3 surface — generator-emitted from spec)

The bundled `twilio/sendgrid-oai` spec (240 paths, 391 operations) covers every endpoint any competitor exposes. The Printing Press emits one command per endpoint automatically. The table below highlights load-bearing groups that must work in the printed CLI; the spec covers far more.

| Resource group | Endpoints | Our implementation | Beats competitor by |
|---|---|---|---|
| Mail Send | /v3/mail/send, /v3/mail/batch | spec-emit + --dry-run + idempotency key | every existing tool stops here |
| Suppressions (bounces, blocks, spam_reports, global, group) | ~25 endpoints | spec-emit + local SQLite mirror | offline diff/sync, FTS over reasons |
| Marketing Contacts | /v3/marketing/contacts (job-based) | spec-emit + sync + job-poller | local mirror, segment replay |
| Marketing Lists & Segments | ~15 endpoints | spec-emit + local mirror | dedup analytics |
| Marketing Single Sends | ~12 endpoints | spec-emit + schedule helpers | per-send post-mortem with local stats |
| Marketing Designs / Templates | ~20 endpoints | spec-emit + local mirror | template version diff |
| Stats (global, category, mailbox-provider, geo, browser, subuser) | ~15 endpoints | spec-emit + local time-series | offline rollups, WoW/MoM deltas |
| Email Activity (beta) | /v3/messages | spec-emit + rate-limit-aware tail | 6/min limit handled |
| API Keys & Scopes | /v3/api_keys, /v3/scopes | spec-emit | typed exit codes for scope errors |
| Subusers | /v3/subusers + nested IPs/credits/monitor | spec-emit + on-behalf-of header | per-subuser rollup |
| IPs & Pools | /v3/ips, /v3/ips/pools, /v3/ip_warmup | spec-emit | pool-assignment helpers |
| Domain Authentication | /v3/whitelabel/domains | spec-emit | DNS-record extraction helper |
| Link Branding | /v3/whitelabel/links | spec-emit | spec only |
| Reverse DNS | /v3/whitelabel/ips | spec-emit | spec only |
| Event Webhooks | /v3/user/webhooks/event/settings | spec-emit + signed-key helper | dual-key rotation handling |
| Alerts | /v3/alerts | spec-emit | spec only |
| Mail Settings | /v3/mail_settings/* | spec-emit | spec only |
| Scheduled Sends | /v3/user/scheduled_sends | spec-emit | local schedule index |
| SSO | /v3/sso/* | spec-emit | spec only |
| Recipients Data Erasure | /v3/recipients/data_erasure | spec-emit | spec only |

**All absorbed features ship with `--json`, `--select`, `--csv`, `--dry-run`, typed exit codes (0/2/3/4/5/7/10), and local-store backing where applicable.**

## Transcendence Features (novel — only possible with our approach)

| # | Feature | Command | Score | Buildability | Why only we can do this |
|---|---|---|---|---|---|
| 1 | Suppression sync | `sendgrid-pp-cli suppression sync --from <csv> --apply --dry-run` | 10 | hand-code | Collapses the API's per-type format inconsistency (bounces vs blocks vs spam_reports vs invalid_emails) into one local table; supports bidirectional dry-run/apply with diff preview. No competitor handles cross-type sync. |
| 2 | Template variable lint | `sendgrid-pp-cli templates lint <id> --against <contact-id|json>` | 10 | hand-code | Statically extracts `{{handlebars}}` from version HTML, cross-checks against contact custom fields or a JSON payload, flags missing/typo'd vars BEFORE send. SendGrid silently drops missing vars; only local parse catches it. |
| 3 | Suppression diff | `sendgrid-pp-cli suppression diff <type> --against <file|url>` | 9 | hand-code | Three-way diff between local SQLite mirror, live API, and external CSV (e.g., CRM export); shows adds/drops/drift. Needs offline mirror + FTS over reason strings. |
| 4 | Stats time-series rollup | `sendgrid-pp-cli stats rollup --by day|week|month --metric opens,clicks --window 90d` | 9 | hand-code | API returns flat buckets; we compute proper rollups + WoW/MoM deltas locally from the mirror. |
| 5 | Bounce investigate | `sendgrid-pp-cli bounce why <email>` | 9 | hand-code | Joins suppressions + activity + stats locally to produce a "why is this address bouncing" narrative. Cross-table join impossible without local mirror. |
| 6 | Template version diff | `sendgrid-pp-cli templates diff <id> <vA> <vB>` | 8 | hand-code | Side-by-side semantic HTML/plain/subject diff of two template versions. Requires fetching, parsing, normalizing HTML locally. |
| 7 | Activity tail | `sendgrid-pp-cli activity tail --rate-aware` | 8 | hand-code | Polls Email Activity API respecting the 6/min limit, streams to terminal + SQLite, supports `--filter from:`, `--filter status:bounce`. Rate-limit-aware backoff + local FTS. |
| 8 | Subuser rollup | `sendgrid-pp-cli subusers rollup --metric reputation,bounces --window 30d` | 8 | hand-code | Per-subuser stats fan-out + local aggregation for ESP operators. API forces one-call-per-subuser; we parallelize and cache. |

**Hand-code commitment: 8 novel features, all hand-written Go (none are spec-emits). Estimated ~50-150 LoC per feature plus `root.go` wiring.**

**Stubs**: None planned. All 8 novel features will ship full or return to this gate.

## Killed Candidates (audit trail)
- **template-preview** — Re-implementing Handlebars locally is a maintenance tar pit; overlaps template-vars-lint.
- **webhook-verify** — Useful but narrow (one-shot crypto check), doesn't lean on SQLite, `openssl dgst` covers it.
