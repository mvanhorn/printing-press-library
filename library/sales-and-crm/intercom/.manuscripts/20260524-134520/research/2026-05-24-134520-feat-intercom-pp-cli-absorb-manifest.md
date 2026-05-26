# Intercom Absorb Manifest

## Source tools surveyed

| Tool | URL | Lang | Stars | Notes |
|---|---|---|---|---|
| intercom-client (Node SDK) | https://github.com/intercom/intercom-node | TypeScript | 389 | Fern-generated; 131 methods across 35 resource groups |
| python-intercom | https://github.com/intercom/python-intercom | Python | 243 | Mirrors Node SDK exactly; same surface |
| Intercom MCP (official) | https://github.com/intercom/intercom-mcp-server | hosted | 5 | 6 tools; US-region only |
| fast-intercom-mcp | https://github.com/evolsb/fast-intercom-mcp | Python | 0 | 4 tools; SQLite cache (~2 KB/conv); three-phase sync |
| fabian1710/mcp-intercom | https://github.com/fabian1710/mcp-intercom | TypeScript | 8 | 1 tool; conversation search with operator+UNIX-timestamp filters |
| raoulbia-ai/mcp-server-for-intercom | https://github.com/raoulbia-ai/mcp-server-for-intercom | TypeScript | 8 | 4 tools; conv + tickets w/ keyword & exclude filters |
| kaosensei/intercom-mcp | https://github.com/kaosensei/intercom-mcp | JavaScript | 8 | 16 tools; Help Center–focused (multilingual) |
| kyoji2/intercom-cli | https://github.com/kyoji2/intercom-cli | TypeScript | 2 | AI-native CLI; `--format toon`; rich state verbs |
| SamP9999/intercom-cli | https://github.com/SamP9999/intercom-cli | TypeScript | 1 | Agent-optimized CLI; deterministic exit codes; env-var auth |
| 44-pixels/intercom-mcp | https://github.com/44-pixels/intercom-cli | TypeScript | 0 | 27 tools incl. raw `request` escape-hatch; OAuth 2.1 resource server |
| GetintheLoop/intercom-cli | https://github.com/GetintheLoop/intercom-cli | JavaScript | 0 | Archived 2021; CSV bulk ops, dedup, file-driven update |

## Absorbed (match or beat everything that exists)

### Endpoint mirrors — auto-emit from the official 2.13 OpenAPI

Every method in the SDKs maps 1:1 to an Intercom REST endpoint and will be emitted as a typed `pp:endpoint` Cobra command by the generator. The 31 resource groups below cover the full SDK surface.

| Resource | Generator surface | Source |
|---|---|---|
| admins | identify, away, list, find, listAllActivityLogs | Node/Py SDKs |
| ai-content | content-import-sources, external-pages CRUD | Node/Py SDKs |
| articles | full CRUD + search | Node/Py SDKs + kaosensei |
| away-status-reasons | list | Node/Py SDKs |
| calls | list, listWithTranscripts, show, showRecording, showTranscript | Node/Py SDKs |
| companies | retrieve, createOrUpdate, find, update, delete, list, scroll, attach/detach contact, attached contacts/segments | Node/Py SDKs |
| contacts | list, create, find, update, delete, search, archive, unarchive, block, mergeLeadInUser, showByExternalId, attach/detach subscription, attached companies/segments/subscriptions/tags | Node/Py SDKs + Official MCP + 44-pixels |
| conversations | list, find, create, update, search, reply, manage, delete, attach/detach contact as admin, convertToTicket, redactPart, runAssignmentRules | All MCPs + SDKs |
| custom-channel-events | notifyAttributeCollected, notifyNewConversation, notifyNewMessage, notifyQuickReplySelected | Node/Py SDKs |
| custom-object-instances | by id / by external id (create/get/delete) | Node/Py SDKs |
| data-attributes | list, create, update | Node/Py SDKs |
| data-export | create, find, cancel, download; reporting variants | Node/Py SDKs |
| events | list, create, summaries | Node/Py SDKs |
| export | enqueueReportingDataExportJob, listAvailableDatasets | Node/Py SDKs |
| help-centers | list, find | Node/Py SDKs + kaosensei |
| internal-articles | full CRUD + search | Node/Py SDKs |
| ip-allowlist | get, update | Node/Py SDKs |
| jobs | status (polling helper) | Node/Py SDKs |
| messages | create (outbound) | 44-pixels send_message |
| news (items + feeds) | full CRUD | Node/Py SDKs |
| notes | list, find, create | Node/Py SDKs + kaosensei |
| phone-call-redirects | create | Node/Py SDKs |
| segments | list, find | Node/Py SDKs |
| subscription-types | list | Node/Py SDKs |
| tags | list, create, find, delete, tag/untag {contact,conversation,ticket} | All MCPs + SDKs |
| teams | list, find | Node/Py SDKs + SamP9999 |
| tickets | create, get, update, search, reply, delete, enqueueCreate | raoulbia + SamP9999 + SDKs |
| ticket-states / ticket-types | list, full CRUD | Node/Py SDKs |
| visitors | find, update, mergeToContact | Node/Py SDKs |

### Capabilities beyond direct endpoint mirrors

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Bearer-token auth w/ env-var fallback | All CLIs | Press default `auth login/logout/status`; `INTERCOM_ACCESS_TOKEN` recognized | Standard |
| 2 | Region-aware base URL | SamP9999, 44-pixels | `--region us\|eu\|au` flag + `INTERCOM_REGION` env | Official MCP is US-only; this is the wedge |
| 3 | Version pinning via `Intercom-Version` header | Both SDKs | Generator emits required header per spec; configurable | Standard |
| 4 | Cursor pagination + scroll API | Both SDKs | Generator default + `--all` auto-paginate flag | Standard |
| 5 | Raw REST escape hatch | 44-pixels (`request`) | `intercom-pp-cli api <METHOD> <PATH> [--data @file.json]` | Covers the long tail (e.g. `unstable` resources, preview endpoints) |
| 6 | Local SQLite mirror with sync/search/sql | fast-intercom-mcp | Press default; FTS5 on conversation parts, articles, contacts, companies | The whole transcendence layer |
| 7 | Multilingual help-center article CRUD | kaosensei | Endpoint mirror (`articles update --locale fr`) | Generator-emitted; no novelty here |
| 8 | Outbound admin message | 44-pixels (`send_message`) | Endpoint mirror `messages create` | Generator-emitted |
| 9 | Find conversations by body substring | raoulbia (`keywords[]`) | Intercom search predicate + offline FTS | Both online and offline |
| 10 | File-driven bulk operations on contacts/conversations | GetintheLoop, manifest of intent across community | `bulk-tag/assign/close` reading IDs from stdin or `--from <file.jsonl>` | Fills the "spreadsheet user" gap |
| 11 | Async-job polling (Data Export) | Both SDKs | `dataexport create --wait` + `jobs wait <id>` | Standard |
| 12 | Deterministic exit codes | SamP9999 | Press defaults (0/2/3/4/5/7/10) | Standard |
| 13 | TOON / agent-friendly output | kyoji2 (`--format toon`) | `--agent`, `--json`, `--select`, `--compact`, `--csv` cover the agent need | Standard |
| 14 | Schema dump for agent context | kyoji2 (`schema`) | Press default `agent-context` command + MCP context tool | Standard |
| 15 | Rate-limit aware retry | Both SDKs | `cliutil.AdaptiveLimiter` reads `X-RateLimit-Remaining`/`Reset` | Standard |
| 16 | MCP surface (every command as a tool) | Press default | Cobratree walker auto-mirrors user-facing commands | Beats Official MCP's 6-tool surface; spans EU/AU |

## Transcendence (only possible with our approach)

User opted for medium trim at Phase Gate 1.5: ship only features scoring 8/10 or higher. The four below are the approved shipping scope.

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Incident-tag dry-run | `conversations incident-tag --mentions "<phrase>" --since 24h --tag <slug> [--apply]` | 9/10 | hand-code | Calls `/conversations/search` with body-substring + updated_at predicate, prints would-tag diff; on `--apply`, fans out tag mutations through `cliutil.AdaptiveLimiter` respecting `X-RateLimit-Remaining` | Brief Top Workflow #3; kyoji2 + 44-pixels MCPs both surface tag-conversation as primary; no existing CLI offers filter-piped + default-dry-run shape |
| 2 | Articles git round-trip | `articles pull --to ./articles/`, `articles push` | 9/10 | hand-code | Paginates `/articles` + `/help_center/collections`, writes `<id>-<slug>.<lang>.md` with YAML frontmatter; push diffs against last-pull manifest and PATCHes changes per locale | Brief Top Workflow #4 + Priya persona; `kaosensei/intercom-mcp` exists specifically because article CRUD is painful via UI; no tool round-trips to a markdown tree |
| 3 | Contact 360 | `contact 360 <email\|external_id\|id>` | 8/10 | hand-code | Resolves contact, then SQL-joins synced `companies` (via attached_companies), `conversations` (author=contact), `tickets` (contact attached), `notes`, `tags`; single nested JSON payload | Brief Data Layer enumerates all joined entities; cross-entity 360 is uniquely valuable to triage (Mei) and agents (Ash) |
| 4 | SLA analytics | `conversations sla --group-by team --metric first-response,resolution` | 8/10 | hand-code | SQL over local `conversations` join `conversation_parts` (min admin-author `created_at` minus conversation `created_at`) join `teams`; emits JSON/CSV/table | Brief Top Workflow #2 + Mei's Monday ritual; warehouse teams currently replicate to Snowflake to answer this |

**Hand-code commitment: 4 novel features.**

### Trimmed at Phase Gate 1.5 (not shipping this run)

| Feature | Score | Trim reason |
|---|---|---|
| Contact dupe clusters | 7/10 | Medium-trim threshold (keep ≥ 8/10) |
| Filter explain | 7/10 | Medium-trim threshold |
| Stale conversations | 7/10 | Medium-trim threshold |
| Article translation gap | 6/10 | Medium-trim threshold |
| Rate-budget meter | 6/10 | Medium-trim threshold |

## Notes / explicit non-features

- **Fin AI agent replay** was killed: Fin endpoints stream via webhook only; not callable from a CLI.
- **Conversation rebalance** was killed: high blast radius + hard to verify; the spec already exposes `runAssignmentRules` for the same outcome.
- **Workspace inventory** was killed: fans out to existing list commands without adding compute; fails Weekly Use.
- **Tag sprawl audit** was killed: hygiene cadence is quarterly, not weekly; partial overlap with `contacts dupes`.
- **Stubs:** none planned. Every survivor ships full or is removed at gate review.
