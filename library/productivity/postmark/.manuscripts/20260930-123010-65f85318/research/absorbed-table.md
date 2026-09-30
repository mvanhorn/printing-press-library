<!-- slop-gate: off -->
### Absorbed (match or beat everything that exists)

Command paths for generated endpoints are provisional until the merged spec is generated; resource/endpoint names follow the upstream API families.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Send one email (to/cc/bcc/reply-to/tag/stream/metadata/attachments/headers) | postmark-mcp sendEmail (source); postmark-cli email raw | (generated endpoint) email send | Full field set incl. MessageStream + Metadata (CLI #56), --stdin JSON, --dry-run, write confirmation |
| 2 | Send with template (id or alias + model) | postmark-mcp sendEmailWithTemplate; postmark-cli email template | (generated endpoint) email send-with-template | Model from file or stdin, stream selection, --dry-run |
| 3 | Batch send (up to 500) | postmark-mcp sendBatch; postmark.js | (generated endpoint) email batch | Per-message ErrorCode surfaced as structured rows; nonzero exit on partial failure |
| 4 | Batch send with templates | postmark-mcp sendBatchWithTemplate | (generated endpoint) email batch-with-templates | Same partial-failure reporting |
| 5 | Bulk send + bulk status | postmark.js sendBulkEmail/getBulkEmailStatus | (generated endpoint) email bulk-send / bulk-status / bulk-list | Doc-only endpoints added to spec; approval-required error (ErrorCode 14) explained |
| 6 | Validation-only send via POSTMARK_API_TEST | Postmark docs overview | (behavior in postmark-pp-cli email send) --sandbox swaps in the test token | Proves a payload without delivering; safe default for agents |
| 7 | Search outbound messages (recipient, from, tag, subject, status, stream, dates, metadata) | postmark-mcp searchOutboundMessages; pmx | (generated endpoint) messages search-outbound | Full filter set incl. metadata_* and time-of-day (MCP #59); pagination past 500 |
| 8 | Outbound message details + event timeline | postmark-mcp getMessageDetails; pmx events | (generated endpoint) messages outbound-details | --select for event fields |
| 9 | Outbound message raw dump | postmark.js getOutboundMessageDump; pmx dump | (generated endpoint) messages outbound-dump | |
| 10 | Search inbound messages | postmark.js; pmx | (generated endpoint) messages search-inbound | |
| 11 | Inbound message details | postmark.js | (generated endpoint) messages inbound-details | |
| 12 | Inbound bypass rules / retry webhook | postmark.js; agent-postmark | (generated endpoint) messages inbound-bypass / inbound-retry | Write-gated |
| 13 | Opens: list all / per message | postmark.js; agent-postmark | (generated endpoint) messages opens / message-opens | |
| 14 | Clicks: list all / per message | postmark.js; agent-postmark | (generated endpoint) messages clicks / message-clicks | |
| 15 | List bounces (type, inactive, email, tag, message ID, stream, dates) | postmark-mcp searchBounces; pmx | (generated endpoint) bounces list | messagestream filter restored (drift) |
| 16 | Get bounce | postmark.js | (generated endpoint) bounces get | |
| 17 | Bounce dump (raw SMTP) | postmark-mcp getBounceDump | (generated endpoint) bounces dump | Warns when older than 30-day dump retention |
| 18 | Reactivate bounced address | postmark-mcp activateBounce | (generated endpoint) bounces activate | Write-gated; refuses SpamComplaint with explanation |
| 19 | Delivery stats (bounce counts by type, inactive total) | postmark.js getDeliveryStatistics | (generated endpoint) bounces delivery-stats | |
| 20 | Bulk bounce reactivation by domain + type with dry-run CSV | arnaud-coral/Postmark-Bounce-Removal (script) | postmark-pp-cli bounces reactivate | Dry-run default, filter by domain/type/date, CSV/JSON report, 429 backoff |
| 21 | List stream suppressions | postmark-mcp listSuppressions | (generated endpoint) suppressions list | |
| 22 | Add suppressions (max 50) | postmark-mcp createSuppressions | (generated endpoint) suppressions create | Auto-chunks input over 50 |
| 23 | Remove suppressions (max 50) | postmark-mcp deleteSuppressions | (generated endpoint) suppressions delete | Auto-chunks; skips SpamComplaint with reason |
| 24 | Is this address suppressed on any stream? | agent-postmark suppressions check | postmark-pp-cli suppressions check | Checks every stream on the server in one call |
| 25 | Templates list with full pagination | postmark-mcp listTemplates (caps at 100); postmark-cli (caps at 300) | (generated endpoint) templates list | No cap; TemplateType/LayoutTemplate filters (drift) |
| 26 | Template get / create / edit / delete | postmark-mcp; postmark.js | (generated endpoint) templates get / create / update / delete | Delete exists (CLI #75); write-gated |
| 27 | Validate / test-render a template with a model | postmark-mcp validateTemplate; postmark-cli preview | (generated endpoint) templates validate | Returns rendered subject/html/text + suggested model |
| 28 | Pull templates to a directory (content.html, content.txt, meta.json, _layouts) | postmark-cli templates pull (source) | postmark-pp-cli templates pull | Official on-disk layout kept for drop-in migration; no 300 cap (CLI #92); single-template and filtered pull (CLI #87, #102) |
| 29 | Push templates from a directory with diff review | postmark-cli templates push (source) | postmark-pp-cli templates push | Diff shown before write, --dry-run, layouts first, optional prune of remote-only templates |
| 30 | Local template preview | postmark-cli templates preview (Express server + watcher) | postmark-pp-cli templates render | One-shot render to files/stdout via validate endpoint; no resident server (descoped from live-reload server) |
| 31 | Cross-server template push | postmark.js pushTemplates (/templates/push) | (generated endpoint) templates push-between-servers | PerformChanges:false preview first; account token |
| 32 | Outbound stats: overview, sends, bounces, spam, tracked, opens, open platforms, email clients, read times, clicks, browser families, click platforms, click location | postmark-mcp getDeliveryStats; postmark.js | (generated endpoint) stats <each> | messagestream + tag filters restored; read-times added (doc-only) |
| 33 | Server settings get / edit | postmark-mcp getServerInfo; postmark.js | (generated endpoint) server get / update | Current enum values (TrackLinks, DeliveryType) fixed from docs |
| 34 | Message streams list / get / create / edit / archive / unarchive | postmark.js; pmx streams | (generated endpoint) message-streams <each> | Doc-only endpoints added; archive warns about 45-day purge |
| 35 | Webhooks list / get / create / edit / delete / verify / statistics | postmark-mcp (list/create/delete); postmark.js (get/edit) | (generated endpoint) webhooks <each> | Full set incl. doc-only verify and 24h statistics |
| 36 | Webhook health check | agent-postmark webhooks health | postmark-pp-cli webhooks health | Verifies every webhook on every stream and summarizes failures |
| 37 | Inbound rules list / create / delete | postmark.js; pmx | (generated endpoint) inbound-rules <each> | |
| 38 | Servers list / get / create / edit / delete (tokens masked by default) | postmark-cli servers list; postmark.js AccountClient | (generated endpoint) servers <each> | Token redaction unless explicitly requested |
| 39 | Domains CRUD + verify DKIM / Return-Path / SPF + rotate DKIM | postmark.js; pmx; agent-postmark | (generated endpoint) domains <each> | Deprecated SPF call labeled |
| 40 | Sender signatures CRUD + resend confirmation + SPF + new DKIM | postmark.js; pmx | (generated endpoint) senders <each> | Deprecated calls labeled |
| 41 | Data removal request + status | postmark.js requestDataRemoval | (generated endpoint) data-removals create / get | Irreversible: typed confirmation required |
| 42 | Recipient delivery diagnosis (messages + suppressions + bounces + recommendation) | postmark-mcp diagnoseDelivery (source); pmx otp; agent-postmark investigate delivery | postmark-pp-cli diagnose | Structured verdict JSON with next command; works across servers; reads local history past 45 days |
| 43 | Account snapshot (parallel overview) | pmx overview | postmark-pp-cli overview | Every server in the account in one view |
| 44 | Domain health (DKIM/SPF/Return-Path status) | agent-postmark investigate domain-health | postmark-pp-cli domains health | All domains + signatures, pass/fail per record, fix command per failure |
| 45 | Stream health (bounce/spam rate per stream) | agent-postmark investigate stream-health | postmark-pp-cli streams health | Thresholds (bounce 10%, spam 0.1%) flagged |
| 46 | Named server contexts, select server by name | agent-postmark profiles; pmx projects | (behavior in postmark-pp-cli servers use) global --server <name> resolves the server token through the account token | One account credential drives all servers; no per-server token juggling |
| 47 | Resend messages blocked by hard-bounce suppression | dszp/retry-hard-bounces-postmark (script) | postmark-pp-cli bounces resend-blocked | Dry-run default; reactivate then resend original content to the bounced recipient only |
| 48 | Postmark platform status (incidents, availability) | postmarker status manager (status.postmarkapp.com, public, no auth) | postmark-pp-cli service-status | Answers "is it Postmark or us?" during outages |
| 49 | Agent-safe writes: --yes, dry-run, typed exit codes (429, write refused), structured errors with fixability, token redaction | pmx; agent-postmark | (behavior in postmark-pp-cli email send) global agent contract on every write | Consistent across all 87 operations |
| 50 | Retry with backoff on 429/5xx | none (MCP and postmark.js lack it; #129) | (behavior in postmark-pp-cli messages search-outbound) client-wide read retry | Reads retry automatically; sends never auto-retry |
| 51 | Default sender and stream from config | postmark-mcp DEFAULT_SENDER_EMAIL / DEFAULT_MESSAGE_STREAM | (behavior in postmark-pp-cli email send) config defaults per server | Per-server defaults instead of one global |

Not absorbed: DMARC Digests API (postmark-reports) uses a separate host and token outside this API; ParseBounce MCP proxies a third-party SaaS; Pipeworx gateway meta-tools are unrelated to Postmark.
