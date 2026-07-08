# Salesforce Headless 360 Absorb Manifest

## Tools Surveyed
1. `sf` / `@salesforce/cli` (forcedotcom/cli) — official CLI, dominant incumbent
2. `jsforce` (jsforce/jsforce) — top Node.js SDK
3. `simple-salesforce` (simple-salesforce/simple-salesforce) — top Python SDK
4. Salesforce DX MCP Server (Developer Preview) — local MCP for coding agents
5. Agentforce MCP (Beta) — tool-calling MCP for Agentforce
6. Agentforce Vibes — AI coding assistant in DX MCP ecosystem
7. Fivetran Salesforce connector — Bulk-API sync pattern
8. Airbyte Salesforce source — open-source sync source, Bulk+REST switching
9. jsforce-mcp / sf-mcp community MCPs — various community MCP wrappers

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | OAuth2 Web Server login | `sf org login web` | `auth login --web` with PKCE S256, 127.0.0.1 bind, state validation | Mandatory PKCE closes loopback-race attack path RFC 8252 flags |
| 2 | JWT Bearer login | `sf org login jwt` | `auth login --jwt` | Structured profile + `--run-as-user` FLS guard |
| 3 | `sf` auth reuse | N/A (each tool reauths) | `auth login --sf <alias>` | Import existing `sf` auth, no re-dance |
| 4 | List authed orgs | `sf org list` | `auth list-orgs` | Portable `~/.config/pp/<cli>/profiles/` layout |
| 5 | Switch active org | `sf config set target-org` | `auth switch-org <alias>` | Atomic swap, FLS metadata preserved |
| 6 | Org display | `sf org display` | `doctor --org <alias>` | Per-source rows with fix hints |
| 7 | SOQL query | `sf data query` | `soql query <soql>` | FLS-applied results when configured |
| 8 | Describe sobject | `sf sobject describe` | `describe <sobject>` | Cached 1h, surfaces compliance tags |
| 9 | Record get | `sf data get record` | `record get <sobject> <id>` | UI API (FLS-enforced), optional field filter |
| 10 | List records | `sf data query` | `<sobject> list` | Local FTS5 search after sync |
| 11 | Bulk query | `sf data query --bulk` | `bulk query <soql>` | Blocked without Apex companion unless `--allow-bulk-fls-unsafe` |
| 12 | Sync objects | Airbyte / Fivetran | `sync` (Composite Graph primary) | One-call account assembly; per-account mode |
| 13 | Metadata retrieve | `sf project retrieve` | (deferred to future CLI) | (stub — out of v1 scope per plan) |
| 14 | Apex test run | `sf apex run test` | (deferred) | (stub — out of v1 scope per plan) |
| 15 | Agentforce invoke | Agentforce MCP | (deferred) | (stub — competing-tool detection in `doctor` offers Agentforce MCP path) |
| 16 | Data Cloud profile | Postman collection | `datacloud profile <dmo> <id>` | Offcore token, graceful degradation |
| 17 | Slack linkage query | SOQL on `SlackConversationRelation` | `slack linkage <account>` | Returns linked channels + workspace IDs |
| 18 | Rate-limit status | `/services/data/v63.0/limits` | `limits` + `doctor` row | Live `Sforce-Limit-Info` parse, 80% budget guard |
| 19 | Doctor / health check | `sf doctor` | `doctor` | Per-source rows, Apex companion check, `sf` version pin, competing-tool detection |
| 20 | Output modes (--json / --csv) | `sf --json` | `--json --compact --csv --plain --agent --quiet` | `--agent` = compact stable-ordered JSON |
| 21 | Dry-run on mutations | Limited | `--dry-run` on all mutating verbs | Consistent default, provenance records it |
| 22 | Structured errors | `sf` exit codes | D9 envelope `{code, http_status, stage, org, trace_id, cause, hint}` | Same shape across CLI, MCP, bundle provenance |
| 23 | MCP tools | Agentforce MCP (6+) | 6 tools (`agent_context`, `agent_brief`, `agent_decay`, `agent_verify`, `agent_refresh`, `agent_doctor`) | Read/compute/emit verbs only; config mutations stay CLI-only |
| 24 | SOQL FTS-local | None | `search "<term>"` | SQLite FTS5 across accounts/contacts/activities |
| 25 | Limits monitor | `sf limits` | `limits` + runtime budget guard | 80% cap, graceful degrade |

## Transcendence (only possible with our approach)

Scoring: 10 = absolutely unique, world-first. 8-9 = exists elsewhere only as a workflow built from many tools. 5-7 = partial precedent but materially better. Below 5 excluded.

| # | Score | Feature | Command | Why Only We Can Do This |
|---|---|---------|---------|------------------------|
| 1 | 10 | Signed cross-surface context bundle | `agent context <account>` | Nobody ships a single signed JSON that joins REST + UI API + Data Cloud + Slack linkage for one account, with FLS + compliance redactions applied. DX MCP and Agentforce MCP expose tools, not portable bundles. |
| 2 | 10 | Offline bundle verification with org-anchored key | `agent verify <bundle>` | JWS with a key registered as a Salesforce Certificate (or hardened CMDT) - no industry precedent. Any agent, any host, verifies without calling the org. |
| 3 | 9 | Channel-audience FLS intersection on Slack inject | `agent inject --slack <channel>` | Enumerates Slack channel members, maps to SF users by email, intersects FLS, drops fields outside intersection before posting. Stops re-exposure path no existing tool addresses. |
| 4 | 9 | File-byte attestation | `agent verify --deep` | Bundle manifest carries `{sha256, sf_content_version_id}`; re-hashes ContentVersion bytes on verify. Detects tamper between bundle emit and agent consumption. |
| 5 | 9 | FLS-safe Bulk API via Apex REST companion | `sync --bulk` with `WITH USER_MODE` wrapper | Bulk API 2.0 does not enforce FLS. Our Apex companion wraps Bulk queries with `WITH USER_MODE`, closing the leak path Fivetran / Airbyte simply document as "integration user sees all." |
| 6 | 8 | Customer 360 freshness scoring | `agent decay --account <id>` | 0-100 freshness score weighted across activity staleness, opp stage drift, case silence, chatter quiet. Requires local SQLite of cross-cloud events no single Salesforce endpoint returns. |
| 7 | 8 | Opp-centric narrative brief | `agent brief --opp <id>` | Deterministic template joining Opp + linked contacts + most-recent activities + FeedItem posts into markdown+JSON. Salesforce's UI shows this visually; no first-party JSON-first "brief" exists. |
| 8 | 8 | Competing-tool yield in doctor | `doctor` | Detects DX MCP / Agentforce MCP presence and tells the user "you may not need this CLI" rather than asserting dominance. No CLI does graceful yield today. |
| 9 | 8 | Multi-device signing collection | `trust list-keys` / `trust rotate` | Keys collected per `(host, user, org)`; laptop and CI runner each register their own; rotation grace periods handled. Salesforce's own CLI has no bundle-signing concept. |
| 10 | 7 | GDPR Article 30 in-org audit | `SF360_Bundle_Audit__c` writes on every bundle | Sync in HIPAA mode, async otherwise. Creates the record-of-processing the compliance team needs. Salesforce's Einstein Trust Layer covers in-org agent use; nothing covers agent bundles leaving the org. |
| 11 | 7 | Hash-chained CMDT registration receipts | `trust register` fallback path | Each `SF360_Bundle_Key__mdt` record includes a hash chain over `{kid, pem, user_id, ts, previous}`. Verifiers detect admin overwrite attacks via chain inconsistency + Setup Audit Trail cross-check. Unique defensive pattern. |
| 12 | 7 | Portable multi-profile config | `~/.config/pp/<cli>/profiles/<alias>.json` | Clean layout ready to extract to `cliutil.MultiProfile` for the PP library. Other tools bury this inside CLI-specific configs. |
| 13 | 6 | `agent refresh` MCP tool | `agent_refresh` | Forces a fresh sync before bundle assembly. Agents get a deterministic "ensure current" verb. DX MCP does not expose sync control; Agentforce MCP is tool-call-oriented. |
| 14 | 6 | Compliance-group content scan | Content-scan pass | Regex layer on long-text / rich-text fields catches email, SSN, phone, Luhn CC that `ComplianceGroup = PII` missed because the field itself wasn't tagged. Explicit best-effort with provenance records. |
| 15 | 6 | `--dry-run` on bundle | `agent context --dry-run` | Compliance teams preview what would leave the org without writing anything to disk. Residual-data-free introspection. |
| 16 | 5 | Polymorphic target redaction | Implicit in `security.Filter` | `WhatId` / `WhoId` / `OwnerId` redacts to `polymorphic_target_unreadable` when the target sobject isn't visible to the user. Prevents "Id of Case exists" leaking existence of unauthorized parents. |

## User-First Feature Discovery

### Persona 1: Sales engineer preparing for a customer meeting tomorrow
- Ritual: pull recent emails, cases, opps for the account; skim Slack conversations; Data Cloud for recent signals.
- Frustration: Salesforce + Slack + email are three tabs. Agentforce chat is browser-only. Pasting context into Claude Code is manual.
- Feature: `agent context acme --since 30d` → feed into any agent for meeting prep. Covered by NOI command.

### Persona 2: RevOps analyst preparing a weekly pipeline review
- Ritual: pull all opps closing this quarter; cross-reference with rep activity; flag stale deals.
- Frustration: Opp reports don't surface activity silence at the account level.
- Feature: batch `agent decay --all-accounts --tier enterprise` (v1 is per-account; batch is v1.1 flagged in plan Risks).

### Persona 3: AI engineer integrating Salesforce into an internal agent
- Ritual: read docs, write SOQL, handle OAuth, sanitize PII, build JSON for LLM prompt.
- Frustration: every integration re-solves the same auth + FLS + redaction problem.
- Feature: `agent context --json --compact` is the one-shot. MCP server plugs into any MCP host.

### Persona 4: CISO / compliance officer
- Ritual: approve or reject every data export out of Salesforce.
- Frustration: agents touching CRM are an audit risk with no trail.
- Feature: bundle provenance with redaction counts + `SF360_Bundle_Audit__c` rows + `agent verify --strict` + `--dry-run` for pre-approval review.

## Total
- Absorbed: 25 (13 shipping, 3 stubs out-of-scope for v1)
- Transcendence: 16 scoring >= 5
- Grand total: 41 capabilities, targeting scorecard grade A

## Source Priority
Single source (salesforce-headless-360). No inversion risk.
