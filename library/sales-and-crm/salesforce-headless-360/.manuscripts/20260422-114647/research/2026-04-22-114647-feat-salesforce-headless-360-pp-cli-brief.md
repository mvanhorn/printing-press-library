# Salesforce Headless 360 CLI Brief

## API Identity
- Domain: Agent-ready customer context assembly. Salesforce Headless 360 (announced April 2026 at TDX 2026) exposes the entire Salesforce platform as unified APIs + MCP tools + CLI. "Headless 360" is a positioning umbrella over REST, SOAP, Bulk 2.0, Metadata, Tooling, Connect (Chatter), UI API, Agentforce, and Data 360 (Data Cloud) APIs.
- Users: RevOps / sales engineers / solutions architects who live in a terminal, AI engineers building agents that need trusted Salesforce context, Salesforce admins running automation pipelines.
- Data profile: Large. Accounts, Contacts, Opportunities, Cases, Tasks, Events, ContentDocumentLinks, FeedItems, plus optional Data Cloud unified profiles and Slack channel linkage via Slack Sales Elevate.

## Reachability Risk
- None. Salesforce is a public, well-documented API. REST v63.0 OpenAPI specs are available (resources.docs.salesforce.com). `sf` CLI has been around for years. Auth is OAuth2 + JWT + client credentials.

## Top Workflows
1. Package customer context for an agent: `agent context acme-corp --since 90d` produces a signed JSON bundle of Account + Contacts + Opps + Cases + 90d activity + chatter + linked Slack channels + Data Cloud unified profile.
2. Opportunity brief: `agent brief --opp O-1234` renders a human-plus-structured summary of a single deal for handoff to an LLM.
3. Freshness assessment: `agent decay --account <id>` scores how stale the CRM data is on an account (no activity in 60d, open cases without reps, chatter silence).
4. Verify a bundle: `agent verify <bundle.json>` confirms a previously-exported bundle is still signed by the org, has not been tampered with, and has not expired.
5. Inject to Slack: `agent inject --slack #acme-deal --bundle <file>` posts a rendered summary to a linked Slack channel with channel-audience FLS intersection applied.

## Table Stakes (absorbed from the Salesforce + agent ecosystem)
- Multi-org auth (web OAuth, JWT Bearer, `sf` CLI fall-through)
- SQLite local mirror of Customer 360 with FTS5 search
- Full `--json` / `--compact` / `--csv` / `--agent` output modes
- `doctor` self-diagnostic with per-source status rows
- MCP server exposing agent-useful tools
- Incremental sync via cursor-based pagination
- `dry-run` on any mutating command

## Data Layer
- Primary entities: accounts, contacts, opportunities, cases, tasks, events, feed_items, content_document_links, slack_relations, compliance_field_map, bundle_audit, sync_cursors.
- Sync cursor: `Sforce-Limit-Info` header for rate-limit budgeting; per-table `last_modified_cursor`; Composite Graph as the primary assembler (one call for Account + all children, up to 500 nodes).
- FTS/search: account_search (name + description), contact_search (name + email + title), activity_search (subject + description).

## Codebase Intelligence
- Source: official Salesforce developer docs + existing `sf` CLI at github.com/forcedotcom/cli
- Auth: OAuth2 Web Server Flow (PKCE S256 mandatory on loopback), JWT Bearer for CI, `sf` CLI fall-through for local reuse. Device Flow is being deprecated (forcedotcom/cli issue #3368). `sf org display --json` returns accessToken + instanceUrl for import.
- Data model: Account -> (Contacts, Opportunities, Cases, Tasks, Events, FeedItems, ContentDocumentLinks). Data Cloud is separate: `UnifiedAccount__dlm` / `UnifiedIndividual__dlm` DMOs on a separate host (`<cdpOrgId>.c360a.salesforce.com`) with an offcore token exchanged via `/services/a360/token`. Slack linkage via `SlackConversationRelation` SOQL when Slack Sales Elevate is installed.
- Rate limiting: 100k API calls / rolling 24h on Enterprise Edition; Composite Graph bundles a full account in 1-3 calls. `Sforce-Limit-Info` response header gives real-time posture.
- Architecture insight: FLS is NOT enforced by REST for integration users. UI API enforces FLS + sharing. `WITH USER_MODE` in Apex is the only full guarantee. Bulk API 2.0 does not enforce FLS. This dictates a decorator-layer design in the CLI.

## User Vision
The plan at `/Users/mvanhorn/code/cli-printing-press/docs/plans/2026-04-22-001-feat-pp-salesforce-360-plan.md` (Deep, deepened 2026-04-22) is authoritative. Key framing:

- NOI: "Salesforce isn't just a CRM. It's the trust graph an agent needs to answer 'who, when, why.' pp-salesforce-360 turns that graph into a portable, verifiable context bundle any agent can consume, not just Agentforce."
- The gap: Headless 360 shipped unified APIs + DX MCP + Agentforce MCP. None of them produce a signed, FLS-safe, cross-surface bundle any agent can consume. That is the gap.
- v1 scope: Sales + Service (core REST) + Data Cloud (optional) + Slack linkage (optional). Marketing Cloud / Commerce / MuleSoft deferred.
- Trust layer: JWS-signed bundles with an org-registered Certificate (or hardened CMDT fallback). `agent verify` runs offline with key-cache; `--live`, `--deep`, `--strict` escalate.
- No live testing available: user has no Salesforce account or API key. Phase 5 live smoke testing will skip.
- User has Slack but it does not unlock the Slack linkage path (Slack Sales Elevate lives inside Salesforce, not Slack).

## Source Priority
Single source (salesforce-headless-360). Multi-source priority gate does not apply.

## Product Thesis
- Name: salesforce-headless-360-pp-cli
- Why it should exist: Salesforce shipped the raw surfaces. They did not ship the cross-surface agent-bundle producer. The ecosystem needs a tool that takes an Account, pulls Customer 360 across REST + UI API + Data Cloud + Slack linkage, honors FLS (including in JWT and Bulk paths via an Apex REST companion with `WITH USER_MODE`), redacts compliance-tagged fields, signs the result with an org-anchored key, and hands the bundle to any agent (Claude Code, Cursor, Codex, Windsurf, Agentforce, self-hosted).

## Build Priorities
1. Synthetic spec + catalog entry. Multi-org auth (sf fall-through + JWT first, OAuth second). SQLite schema with Composite Graph sync.
2. `security.Filter` decorator (FLS + PII + polymorphic + Shield + content scan + log scrubber). Apex REST companion for `WITH USER_MODE`.
3. `agent *` family (context, brief, decay, verify, inject) with `--dry-run` and file-byte attestation.
4. `trust.Signer` interface + Certificate-preferred key registration + rotation + audit trail.
5. Data Cloud + Slack linkage enrichment with graceful degradation.
6. MCP server (6 tools: context, brief, decay, verify, refresh, doctor).
7. Doctor + README + SKILL + scorecard hardening to grade A.
