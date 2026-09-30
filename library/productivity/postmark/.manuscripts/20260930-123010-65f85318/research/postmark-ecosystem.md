<!-- slop-gate: off -->
# Postmark ecosystem research (absorb manifest input)

Researched 2026-09-30. Sources: GitHub API (repo metadata, raw source reads), npm registry, WebSearch, WebFetch.
`(source)` = extracted from code read this session. `(readme)` = from README/docs only.

## 1. Tools table

| Name | URL | Lang | Stars | Last push | Type | Status |
|---|---|---|---|---|---|---|
| postmark-cli (official, npm `postmark-cli` 1.6.19) | https://github.com/ActiveCampaign/postmark-cli | TypeScript | 88 | 2026-07-17 | CLI | active-ish (maintenance concerns, see pains) |
| postmark-mcp (official, npm `@activecampaign/postmark-mcp` 2.1.1) | https://github.com/ActiveCampaign/postmark-mcp | JavaScript | 57 | 2026-09-28 | MCP | active |
| postmark-skills (official, 5 skills) | https://github.com/ActiveCampaign/postmark-skills | Markdown | 46 | 2026-09-24 | skill | active |
| pmx (Postmark Explorer) | https://github.com/nicolasacchi/pmx | Go | 0 | 2026-06-29 | CLI (agent-first) | active |
| agent-postmark | https://github.com/shhac/agent-postmark | Go | 0 | 2026-09-03 | CLI + skill (agent triage) | active |
| postmark-send | https://github.com/btafoya/postmark-send | Go | 2 | 2026-06-24 | CLI (send only, agent JSON) | active |
| marccampbell/postmark-cli | https://github.com/marccampbell/postmark-cli | TypeScript | 2 | 2017-12-03 | CLI (template CI) | stale |
| postmarkit | https://github.com/pforret/postmarkit | Shell | 3 | 2021-02-05 | CLI (send) | stale |
| postmark-template-trans | https://github.com/Stilborg/postmark-template-trans | TypeScript | 0 | 2023-01-04 | CLI (cross-account template copy) | stale, self-described unfinished |
| postmark-reports | https://github.com/vlatan/postmark-reports | Go | 1 | 2024-11-30 | CLI (DMARC raw report analysis) | stale |
| mcp-postmark (Pipeworx gateway) | https://github.com/pipeworx-io/mcp-postmark | TypeScript | 0 | 2026-09-26 | MCP (hosted) | active |
| Lupiita4/postmark-mcp (copy of old Postmark Labs MCP) | https://github.com/Lupiita4/postmark-mcp | n/a | 0 | 2025-06-26 | MCP | stale |
| nirda13/postmark-mcp | https://github.com/nirda13/postmark-mcp | Python | 0 | 2026-08-16 | MCP (send + server info) | active, tiny |
| ParseBounce MCP | https://github.com/ParseBounce/mcp-server | TypeScript | 0 | 2026-08-29 | MCP (multi-ESP, via ParseBounce SaaS) | active |
| mailsandbox | https://github.com/btafoya/mailsandbox | Go | 8 | 2025-09-15 | Postmark API emulator + MCP | stale |
| postmark.js (npm `postmark` 5.1.0) | https://github.com/ActiveCampaign/postmark.js | TypeScript | 361 | 2026-07-10 | SDK | active |
| postmark-gem | https://github.com/ActiveCampaign/postmark-gem | Ruby | 230 | 2024-07-31 | SDK | slow |
| postmark-rails | https://github.com/ActiveCampaign/postmark-rails | Ruby | 396 | 2024-09-20 | SDK (ActionMailer) | slow |
| postmark-php | https://github.com/ActiveCampaign/postmark-php | PHP | 176 | 2026-09-25 | SDK | active |
| postmark-dotnet | https://github.com/ActiveCampaign/postmark-dotnet | C# | 54 | 2026-07-07 | SDK | active |
| postmark-java | https://github.com/ActiveCampaign/postmark-java | Java | 45 | 2026-03-02 | SDK | active |
| postmarker (PyPI `postmarker`) | https://github.com/Stranger6667/postmarker | Python | 149 | 2025-09-04 | SDK | slow |
| python-postmark (PyPI `postmark`) | https://github.com/themartorana/python-postmark | Python | 201 | 2026-08-10 | SDK | active |
| keighl/postmark | https://github.com/keighl/postmark | Go | 71 | 2021-01-25 | SDK | stale |
| mrz1836/postmark (fork of keighl) | https://github.com/mrz1836/postmark | Go | 71 | 2026-09-29 | SDK | active |
| retry-hard-bounces-postmark | https://github.com/dszp/retry-hard-bounces-postmark | Python | 0 | 2026-08-11 | script | active |
| Postmark-Bounce-Removal | https://github.com/arnaud-coral/Postmark-Bounce-Removal | Shell | 1 | 2023-10-02 | script | stale |
| postmark-backup (GH Actions on postmark-cli) | https://github.com/eSolia/postmark-backup | YAML | 6 | 2021-09-30 | script | archived |
| postmark-bounced-email-blocker | https://github.com/djurovicigoor/postmark-bounced-email-blocker | PHP | 7 | 2026-03-31 | script/lib | active |

Not found: no Postmark entry in `anthropics/claude-plugins-official` (external_plugins lists asana, context7, discord, fakechat, firebase, github, gitlab, imessage, laravel-boost, linear, playwright, serena, telegram, terraform). No dedicated Postmark Claude Code plugin found on GitHub.

Historical, removed: the unofficial npm package `postmark-mcp` (not ActiveCampaign) was malicious from v1.0.16, BCCing every email to an attacker domain (Koi Security finding; https://www.bleepingcomputer.com/news/security/unofficial-postmark-mcp-npm-silently-stole-users-emails/ , https://snyk.io/blog/malicious-mcp-server-on-npm-postmark-mcp-harvests-emails/ , HN https://news.ycombinator.com/item?id=45395957).

Count by type: CLI 8 (incl. agent-postmark, pmx), MCP 6, SDK 10, skill 1 (official; agent-postmark also ships a skill but is counted as CLI), script 4. Total 29 rows.

## 2. Feature catalog

### 2.1 ActiveCampaign/postmark-cli (official) (source)
Entry: `src/index.ts` uses yargs `.env('POSTMARK')` + `commandDir('commands')`. Wraps `postmark` npm SDK v4.
- `postmark email raw --from --to --subject [--html] [--text]` : POST /email via `ServerClient.sendEmail`. Prints raw JSON response. No cc/bcc/reply-to/stream/tag/attachments. (source)
- `postmark email template (--id | --alias) --from --to [--model <json string>]` : POST /email/withTemplate via `sendEmailWithTemplate`. (source)
- `postmark servers list [--count] [--offset] [--name] [--json] [--show-tokens]` : GET /servers via `AccountClient.getServers`. Masks ApiTokens unless `--show-tokens`. Table shows Name, ID, ServerLink, SmtpApiActivated, TrackOpens, TrackLinks, InboundHookUrl, Color. (source)
- `postmark templates pull <dir> [--overwrite]` : GET /templates?count=300 then GET /templates/{alias} per template, then POST /templates/validate to capture `SuggestedTemplateModel` as `TestRenderModel`. Writes `<alias>/content.html`, `content.txt`, `meta.json`; layouts go under `_layouts/`. Skips templates without an alias. Hard cap 300 templates. (source)
- `postmark templates push <dir> [--force] [--all]` : builds local manifest, GET /templates?count=300, GET /templates/{id} per remote to diff, prints review table, confirm prompt, pushes layouts first, then POST /templates (create) or PUT /templates/{alias} (edit). No delete. (source)
- `postmark templates preview <dir> [--port]` : local Express server + file watcher; renders via POST /templates/validate for html and text. (source)
- `postmark cheats` : hidden Konami-code easter egg that prints a promo code. No API call. (source)
- Hidden flags on every command: `--server-token` / `--account-token`, `--request-host`. (source)

### 2.2 ActiveCampaign/postmark-mcp (official) (source, index.js 72 KB, v2.1.1)
Native fetch client `postmarkRequest(path)` against `https://api.postmarkapp.com`, 60 s timeout, maps non-2xx to `Postmark API <status> (ErrorCode N): <Message>`. No retry, no 429 backoff. Output is Markdown text, not JSON. Annotation presets READ_ONLY / MUTATING / DESTRUCTIVE.
Sending:
- `sendEmail` (MUTATING) : POST /email. to (1 or up to 50), subject, textBody, htmlBody, from (defaults to DEFAULT_SENDER_EMAIL), cc, bcc, replyTo, tag.
- `sendEmailWithTemplate` (MUTATING) : POST /email/withTemplate. templateId xor templateAlias, templateModel, cc/bcc/replyTo.
- `sendBatch` (MUTATING) : POST /email/batch.
- `sendBatchWithTemplate` (MUTATING) : POST /email/batchWithTemplates.
Templates:
- `listTemplates` (READ_ONLY) : GET /templates?count=100&offset=0 only. Selects Name, TemplateId, Alias, Subject, TemplateType, LayoutTemplate. Prints "pagination not yet supported" when 100 returned.
- `getTemplate` : GET /templates/{idOrAlias}.
- `createTemplate` : POST /templates.
- `editTemplate` : PUT /templates/{idOrAlias}.
- `deleteTemplate` (DESTRUCTIVE) : DELETE /templates/{idOrAlias}.
- `validateTemplate` : POST /templates/validate.
Messages / diagnostics:
- `searchOutboundMessages` : GET /messages/outbound with recipient, fromemail, tag, subject, status (queued|sent|processed), messagestream, fromdate/todate (YYYY-MM-DD only), count<=500, offset. Selects Subject, MessageID, Recipients, From, Status, ReceivedAt, Tag.
- `getMessageDetails` : GET /messages/outbound/{id}/details. Selects MessageEvents[].Type, ReceivedAt, Details.Summary.
- `diagnoseDelivery` (composite, READ_ONLY) : parallel GET /messages/outbound (last 7 days, count 5) or /details, GET /message-streams/{stream}/suppressions/dump?EmailAddress=, GET /bounces?emailFilter=&count=10; promotes latest hit to /details; emits plain-English recommendation (SpamComplaint = do not retry; HardBounce with CanActivate = activateBounce; else deleteSuppressions).
Bounces:
- `searchBounces` : GET /bounces (type, inactive, emailFilter, tag, messageID, messageStream, dates, count<=500, offset). Selects Email, ID, Type, TypeCode, Description, BouncedAt, Inactive.
- `getBounceDump` : GET /bounces/{id}/dump.
- `activateBounce` (MUTATING) : PUT /bounces/{id}/activate.
Suppressions:
- `listSuppressions` : GET /message-streams/{stream}/suppressions/dump (SuppressionReason, Origin, EmailAddress, dates).
- `createSuppressions` (MUTATING) : POST /message-streams/{stream}/suppressions (max 50).
- `deleteSuppressions` (DESTRUCTIVE) : POST /message-streams/{stream}/suppressions/delete (max 50; SpamComplaint cannot be deleted).
Stats / server / webhooks:
- `getDeliveryStats` : one of /stats/outbound, /sends, /bounces, /spam, /tracked, /opens, /opens/platforms, /opens/emailClients, /opens/readTimes, /clicks, /clicks/browserFamilies, /clicks/platforms, /clicks/location (tag, dates, messageStream).
- `getServerInfo` : GET /server.
- `listWebhooks` : GET /webhooks?MessageStream=.
- `createWebhook` (MUTATING) : POST /webhooks, HTTPS only, optional WEBHOOK_URL_ALLOWLIST origin+path check.
- `deleteWebhook` (DESTRUCTIVE) : DELETE /webhooks/{id}.
Gaps (source + CHANGELOG "Planned"): no getWebhook/editWebhook, no message streams, no inbound, no opens/clicks lists, no outbound dump, no bulk API, no account-token surface (servers, domains, sender signatures, DKIM), no template pagination. Open issues #29-#44 request listMessageStreams, per-send stream override, lookupRecipientStatus, renderTemplatePreview, batch dry-run, bulk, allowed-senders allowlist.

### 2.3 postmark.js SDK (source, `src/client/ServerClient.ts`, `AccountClient.ts`)
ServerClient (X-Postmark-Server-Token):
sendEmail POST /email; sendEmailBatch POST /email/batch; sendEmailWithTemplate POST /email/withTemplate; sendEmailBatchWithTemplates POST /email/batchWithTemplates; sendBulkEmail POST /email/bulk; getBulkEmailStatus GET /email/bulk/{id}; getDeliveryStatistics GET /deliveryStats; getBounces GET /bounces; getBounce GET /bounces/{id}; getBounceDump GET /bounces/{id}/dump; activateBounce PUT /bounces/{id}/activate; getTemplates GET /templates; getTemplate GET /templates/{idOrAlias}; deleteTemplate DELETE; createTemplate POST /templates/; editTemplate PUT; validateTemplate POST /templates/validate; getServer GET /server; editServer PUT /server; getOutboundMessages GET /messages/outbound; getOutboundMessageDetails GET .../{id}/details; getOutboundMessageDump GET .../{id}/dump; getInboundMessages GET /messages/inbound; getInboundMessageDetails GET .../{id}/details; bypassBlockedInboundMessage PUT .../{id}/bypass; retryInboundHookForMessage PUT .../{id}/retry; getMessageOpens GET /messages/outbound/opens; getMessageOpensForSingleMessage GET .../opens/{id}; getMessageClicks GET /messages/outbound/clicks; getMessageClicksForSingleMessage GET .../clicks/{id}; getOutboundOverview GET /stats/outbound; getSentCounts /sends; getBounceCounts /bounces; getSpamComplaintsCounts /spam; getTrackedEmailCounts /tracked; getEmailOpenCounts /opens; getEmailOpenPlatformUsage /opens/platforms; getEmailOpenClientUsage /opens/emailClients; getEmailOpenReadTimes /opens/readTimes; getClickCounts /clicks; getClickBrowserUsage /clicks/browserFamilies; getClickPlatformUsage /clicks/platforms; getClickLocation /clicks/location; createInboundRuleTrigger POST /triggers/inboundRules; deleteInboundRuleTrigger DELETE /triggers/inboundRules/{id}; getInboundRuleTriggers GET /triggers/inboundRules; getWebhooks GET /webhooks; getWebhook GET /webhooks/{id}; createWebhook POST; editWebhook PUT; deleteWebhook DELETE; getMessageStreams GET /message-streams; getMessageStream GET /message-streams/{id}; editMessageStream PATCH; createMessageStream POST; archiveMessageStream POST .../archive; unarchiveMessageStream POST .../unarchive; getSuppressions GET /message-streams/{s}/suppressions/dump; createSuppressions POST /message-streams/{s}/suppressions; deleteSuppressions POST .../suppressions/delete.
AccountClient (X-Postmark-Account-Token):
getServers GET /servers; getServer; createServer; editServer; deleteServer; getDomains GET /domains; getDomain; createDomain; editDomain; deleteDomain; verifyDomainDKIM PUT /domains/{id}/verifyDKIM; verifyDomainReturnPath PUT .../verifyReturnPath; verifyDomainSPF POST .../verifySPF; rotateDomainDKIM POST .../rotateDKIM; getSenderSignatures GET /senders; getSenderSignature; createSenderSignature; editSenderSignature; deleteSenderSignature; resendSenderSignatureConfirmation POST /senders/{id}/resend; verifySenderSignatureSPF POST .../verifySpf; requestNewDKIMForSenderSignature POST .../requestNewDkim; pushTemplates PUT /templates/push; requestDataRemoval POST /data-removals; getDataRemovalStatus GET /data-removals/{id}.

### 2.4 postmarker (Python) (source, `src/postmarker/models/*.py`)
One `PostmarkClient(server_token, account_token)` with managers: bounces (all, get, get_dump, activate, deliverystats, tags); domains (all/get/create/edit/delete, verifyspf, verifydkim, verifyreturnpath, rotatedkim); emails (send, send_with_template, send_batch, send_template_batch, from_mime, attach); messages (outbound all/get/get_dump/opens, inbound all/get/bypass/retry, as_mime, attachment save); senders (CRUD, resend, verifyspf, requestnewdkim); server (get, edit); stats (overview, sends, bounces, spam, tracked, opens, opens_platforms, emailclients, readtimes, clicks, browserfamilies, clicks_platforms, location); status (status.postmarkapp.com: incidents, services, availability, delivery); templates (CRUD, validate); triggers (inbound rules; tag triggers). Missing: webhooks, message streams, suppressions, servers (account), bulk, data-removals. Unique: Postmark status-page API.

### 2.5 nicolasacchi/pmx (readme)
messages outbound [filters], outbound get|dump|events; messages inbound, inbound get|dump; bounces list|get|dump|activate|tags|deliverystats; stats outbound|sends|bounces|spam|opens|clicks; suppressions list|create|delete; email send|send-template|send-batch; templates list|get|validate|create|edit|delete; streams list|get|create|edit|archive; server get|edit; webhooks list|get|create|edit|delete; inbound-rules list|create|delete; `overview` (parallel snapshot); `otp <email>` (recipient trace lens). Account: servers CRUD; signatures CRUD + resend + request-dkim; domains CRUD + verify-dkim + verify-returnpath + rotate-dkim; data-removals create|status. Output: TTY table, piped JSON, gjson `--jq`, `CLAUDECODE=1` caps lists at 100 rows. Writes need `--yes`. Exit codes 0-6 incl. 5 = 429, 6 = write refused.

### 2.6 shhac/agent-postmark (readme)
profiles setup|add|check|update, profiles servers add|update|remove (keychain); servers list; streams list; messages search|get|content|dump|inbound-search|opens|clicks|inbound-retry|inbound-bypass; bounces list|dump|activate; suppressions list|check|create|delete (deliberately no /suppressions/dump); webhooks health; domains verify-dkim|verify-spf; `investigate delivery|bounce|domain-health|stream-health|webhook-health`. NDJSON default, redaction of tokens and raw blobs, stderr JSON errors with `fixable_by: agent|human|retry`, `--yes` on writes, bundled `mockpostmark` fixture server.

### 2.7 Smaller tools (readme)
- btafoya/postmark-send: single send via flags or JSON stdin, NDJSON output, `--setup` writes token into ~/.bashrc.
- pipeworx-io/mcp-postmark: send, send_batch, delivery_stats, messages_outbound, message_outbound_detail, message_outbound_dump, bounces, bounce, bounce_activate, server. Hosted gateway, adds ~30 Pipeworx meta-tools.
- nirda13/postmark-mcp: getServerInfo, sendEmail.
- ParseBounce MCP: bounces, complaints, suppressions, delivery stats across SES/SendGrid/Mailgun/SparkPost/Postmark/Mandrill via ParseBounce SaaS, not Postmark API directly.
- ActiveCampaign/postmark-skills: knowledge skills only (postmark-send-email, postmark-inbound, postmark-templates, postmark-webhooks, postmark-email-best-practices); no executable tool; uses POSTMARK_SERVER_TOKEN.
- dszp/retry-hard-bounces-postmark: finds messages blocked by hard-bounce suppression, optionally reactivates, resends original HTML + attachment only to the bounced recipient(s).
- arnaud-coral/Postmark-Bounce-Removal: remove bounces by domain + bounce type, `--dry-run`, CSV result.
- eSolia/postmark-backup: cron GH Action running `postmark templates pull` per env with per-env server-token secrets, commits to repo.
- Stilborg/postmark-template-trans: copy templates between accounts, JSON backup.

## 3. Auth conventions

- Postmark headers: `X-Postmark-Server-Token` (server scope) and `X-Postmark-Account-Token` (account scope). Test value `POSTMARK_API_TEST` in the server-token header validates sends without delivery (https://postmarkapp.com/developer/api/overview).
- Official CLI (source): yargs `.env('POSTMARK')` maps `POSTMARK_SERVER_TOKEN`, `POSTMARK_ACCOUNT_TOKEN`, `POSTMARK_REQUEST_HOST` onto hidden flags `--server-token`, `--account-token`, `--request-host`. Falls back to a masked interactive prompt. No config file, no profiles. Multi-server = swap env var per run (eSolia uses `POSTMARK_SERVER_TOKEN_STG` style CI secrets).
- Official MCP (source): `POSTMARK_SERVER_TOKEN` (required), `DEFAULT_SENDER_EMAIL` (required), `DEFAULT_MESSAGE_STREAM` (required), optional `AGENT_LABEL`, `LOG_FILE`, `LOG_EMAIL_FULL`, `WEBHOOK_URL_ALLOWLIST`, `POSTMARK_SKIP_VERIFY`. Sends `X-Postmark-Client: postmark-mcp`, `X-Postmark-Client-Version`, `X-Postmark-MCP-Client`, `X-Agent-Label`. Startup probe GET /server. Server token only, one server per process. README: neither token type "supports sub-scoped permissions".
- postmark-skills: `POSTMARK_SERVER_TOKEN`.
- postmark.js: constructor args only (`new ServerClient(token)`, `new AccountClient(token)`), no env reading.
- postmarker: `PostmarkClient(server_token=..., account_token=...)`, `from_config(config, prefix="postmark_")` (source signature).
- pmx: flag, then env (`POSTMARK_SERVER_TOKEN`, `POSTMARK_ACCOUNT_TOKEN`), then `~/.config/pmx/config.toml` with `[projects.<name>]` + `default_project`; `pmx config add|use|doctor`.
- agent-postmark: OS keychain profiles; one optional account token plus N named server contexts, each with server ID + default stream; `--server <name>` selects. Mock env: `AGENT_POSTMARK_BASE_URL`, `AGENT_POSTMARK_ACCOUNT_TOKEN`, `AGENT_POSTMARK_SERVER_TOKEN`.
- Outliers: btafoya uses `POSTMARK_API_TOKEN`; Pipeworx uses `PLATFORM_POSTMARK_KEY`; some directory listings cite `POSTMARK_API_TOKEN`.
- De facto standard: `POSTMARK_SERVER_TOKEN` + `POSTMARK_ACCOUNT_TOKEN`. Named multi-server profiles are only in the two small Go CLIs.

## 4. Pain points

1. Template sync caps at 300. "We crossed more than 300 templates recently ... Curious if there are plans to support more than 300 templates via CLI?" https://github.com/ActiveCampaign/postmark-cli/issues/92 (source confirms `client.getTemplates({ count: 300 })` in pull and push).
2. All-or-nothing template ops, no delete. "want to be able to pull and update individual templates instead of all at once." and "our team is going to move off of postmark templates" https://github.com/ActiveCampaign/postmark-cli/issues/102#issuecomment-4489436037 ; pull filter request https://github.com/ActiveCampaign/postmark-cli/issues/87 ; "the only case we can't handle is if we want to delete a template." https://github.com/ActiveCampaign/postmark-cli/issues/75
3. Official CLI looks under-maintained. Issue titled "Is this project dead?" https://github.com/ActiveCampaign/postmark-cli/issues/102 ; TestRenderModel not stored remotely: "A bit disappointing to have an enhancement request outstanding for 5+ years." https://github.com/ActiveCampaign/postmark-cli/issues/29 ; no message-stream choice on send https://github.com/ActiveCampaign/postmark-cli/issues/56 ; batch-from-CSV request https://github.com/ActiveCampaign/postmark-cli/issues/88
4. Templates across environments. "One of the biggest frustrations we keep hearing about templates is how difficult it is to manage changes across your servers/environments." https://postmarkapp.com/blog/feature-announcement-easier-template-management-across-environments (2018; /templates/push exists but no official CLI command wraps it).
5. Search window caps. Count + offset cannot exceed 10,000 on messages, bounces, opens (https://postmarkapp.com/developer/api/messages-api , https://postmarkapp.com/developer/api/bounce-api). Airbyte's connector "paginated forever over the same 500 records" until fixed with daily windowing https://github.com/airbytehq/airbyte/pull/87518 . Official MCP listTemplates stops at 100 with "pagination not yet supported" (source).
6. 45-day retention, then gone from API. "Postmark stores email content, events (e.g. delivery, click, open), and metadata for all messages for 45 days by default" (7-365 with add-on); bounces and spam stats kept forever https://postmarkapp.com/support/article/how-long-are-inbound-and-outbound-messages-stored-in-activity . Script author: Postmark "offers no built-in retry" for suppression-blocked messages https://github.com/dszp/retry-hard-bounces-postmark
7. Date filters are day-granular and body search is missing. "It would be great if the date time frames also allowed time." https://github.com/ActiveCampaign/postmark-mcp/issues/59 ; body search request https://github.com/ActiveCampaign/postmark-mcp/issues/60
8. Agents double-send. "the agent (Claude in our case) sent an email and realized there was an issue with an attachment, so it automatically resent the email. This could be problematic if you're sending one-off OTP emails" https://github.com/ActiveCampaign/postmark-mcp/issues/54
9. No client retry on transient outages. "Today I've experienced what might be a short Postmark API outage, which led to some customers not receiving emails." Maintainer: retries are "better if it's handled on your side." https://github.com/ActiveCampaign/postmark.js/issues/129
10. Trust in third-party Postmark tooling. Malicious `postmark-mcp` npm package BCC'd all mail to an attacker; ~1,643 downloads https://thehackernews.com/2025/09/first-malicious-mcp-server-found.html

## 5. Reachability / risk notes

- 429 exists but is unquantified: "We have detected that you are making requests at a rate that exceeds acceptable use of the API." https://postmarkapp.com/developer/api/overview . Secondary sources say no numeric limit and no rate-limit headers are published (https://mailtester.com/blog/postmark-rate-limits-burst-handling/), unverified.
- Neither official client retries: MCP throws on any non-2xx (source); postmark.js declined built-in retry (#129). pmx maps 429 to exit 5.
- Error semantics worth encoding: ErrorCode 406 = inactive recipient (do not retry), 422 = validation/unverified sender, 413 = payload over 10 MB (50 MB batch). SpamComplaint suppressions cannot be deleted via API; spam-bounce reactivation needs support.
- Tokens are all-powerful per scope (MCP README). Side-effect gating (`--yes`, dry-run, POSTMARK_API_TEST) matters for agents.
- Local official OpenAPI specs in this run (`research/specs/postmark-server.official.yml`, `postmark-account.official.yml`) are missing endpoints the SDK uses: /webhooks (5 ops), /message-streams (6 ops incl. archive/unarchive), /message-streams/{s}/suppressions (3 ops), /email/bulk + status, /data-removals (2 ops), /stats/outbound/opens/readtimes. The generator will need spec augmentation to reach MCP/SDK parity.
- Template list limit is inconsistent across sources: MCP CHANGELOG says the API "caps this endpoint at 100 results with no pagination", while the CLI requests count=300 and issue #33 (fixed) was about >100 templates. Verify live before designing template sync.
- SDK hygiene issues: axios CVE bump https://github.com/ActiveCampaign/postmark.js/issues/191 , `url.parse()` deprecation https://github.com/ActiveCampaign/postmark.js/issues/173 , edge runtime incompatibility https://github.com/ActiveCampaign/postmark.js/issues/130 . CLI: Node 26 support https://github.com/ActiveCampaign/postmark-cli/issues/104 .
- Outage/reputation history on HN: "Tell HN: Postmark Is Down" https://news.ycombinator.com/item?id=43595732 ; Spamhaus listing https://news.ycombinator.com/item?id=32295067 .
- Separate DMARC Digests API at https://dmarc.postmarkapp.com (X-Api-Token header, raw report metadata kept ~2 weeks) and status API used by postmarker; both are distinct hosts from api.postmarkapp.com.

## 6. Workflows (evidence-ranked)

1. "Did my email reach X?" recipient triage: search outbound by recipient, open event timeline, check suppression + bounce history, reactivate. Evidence: MCP `diagnoseDelivery`, pmx `otp` headline use case, agent-postmark `investigate delivery`, Postmark support article https://postmarkapp.com/support/article/1267-why-didn-t-this-recipient-receive-my-message
2. Templates as code across staging and production: pull, diff, preview, push, backup in CI. Evidence: official CLI's core surface, eSolia backup Action, template-trans, CLI issues #92/#87/#75/#102, Postmark 2018 blog.
3. Bounce and suppression hygiene at scale: list inactive by domain/type, bulk reactivate or delete suppressions, resend what was blocked. Evidence: dszp retry script, arnaud-coral removal script, MCP activate/delete tools, bounced-email-blocker.
4. Scripted/agent sending: raw, templated, batch (500/batch), bulk. Evidence: CLI `email raw|template`, postmark-send, every MCP's send tools, CLI issue #88 batch from CSV, MCP issue #42 bulk.
5. Deliverability monitoring and domain health: outbound stats (bounce rate under 10%, spam under 0.1% thresholds), DKIM/SPF/Return-Path verification, webhook health. Evidence: MCP `getDeliveryStats`, pmx `overview` + domains verify, agent-postmark `domain-health` / `stream-health` / `webhooks health`.
