# SendFox CLI product reprint brief

## API Identity
Official contract: https://sendfox.com/openapi.yaml, fetched 2026-09-21 HTTP 200, OpenAPI 3.0.3 / API 1.4.0, 60 operations. Base https://api.sendfox.com. Paid REST API with OAuth2 or personal Bearer token. Preserve SENDFOX_API_TOKEN and SENDFOX_BEARER_AUTH compatibility. Vendor has no canonical env-var declaration; the first is established prior CLI convention.

## Users
- Newsletter creator publishing weekly: composes drafts, checks audiences/exclusions and sender readiness, reviews delivered engagement, then decides whether a resend is justified.
- Audience operations assistant doing weekly hygiene: reviews new/imported subscribers, unsubscribe/suppression discrepancies, duplicate emails, field/list/tag mappings, and support dossiers.
- Creator-business automation maintainer: checks welcome-sequence configuration and builds/revises inactive automations from versioned local files without accidental sends.
- Cathryn as platform evaluator: repeats export comparisons and late-change audits while evaluating a possible Kit migration; wants evidence that normal SendFox operations are excellent before changing platforms.

## Top Workflows
1. Compose and preflight a newsletter, verify sender and inclusion/exclusion choices, create a draft, review stats/links/cohorts, plan a resend without scheduling.
2. Export inventory, sync local resources, search and join contacts/lists/tags/fields for audience health and support dossiers.
3. Build/review an inactive welcome automation with valid trigger IDs and ordered email delays; explicitly report missing enrollment/progress APIs.
4. Preview filtered bulk audience changes, budget calls, follow async job states, and preserve failure/partial-completion evidence.
5. Compare local Kit exports to SendFox snapshots: suppression gate, resource mapping, validation, delta, cutover and rollback artifacts. No account migration.

## Reachability Risk
Public official spec available. Authenticated behavior unverified by explicit instruction. 402 paid-plan requirement in FAQ; 403 account_restricted structured body. Spec says Lifetime/Empire while current official help includes Pro: document discrepancy instead of silently treating plan labels as verified runtime behavior. No credential discovery or live authenticated probe.

## Table Stakes
All 60 operations, typed body/query/path inputs, stable JSON, selection, pagination without truncation, exact filter bracket/array encoding, 60 requests/minute per user, safe GET retries, no replay after ambiguous writes, previews/confirmations. Contact PATCH lists replaces memberships. Custom-field types use documented immutable UUIDs. New contact-tags are not legacy lists. Bulk engagement-only filters cannot be sent to GET /contacts. Campaign stats link breakdown is opt-in; resend omits schedule by default. Domain validation is a mutation.

## Competitors and evidence
- Official hosted MCP https://sendfox.com/developer/mcp covers REST families; requires Pro/Empire whereas REST supports Lifetime. Complements this CLI, lacks advertised deterministic local export/diff/audit workflows.
- Prior published CLI: 11 archived operations, 12 custom workflows, Printing Press 4.20.1, run 20260611-165423. Imported baseline builds under locally available Go 1.26.8. No prior research.json; prior brief and provenance supply reconciliation evidence (degraded machine-readable history, not discarded workflows).
- Pipedream, Activepieces and Composio integrations discovered; verify their source before crediting individual actions. npm/PyPI searches found no authoritative comprehensive SendFox SDK. Search absence is not exhaustive absence.
- Official FAQ https://help.sendfox.com/article/278-endpoints and https://help.sendfox.com/article/322-how-do-i-organize-and-analyze-my-contacts document targeting and troubleshooting friction.

## Concrete pain points
Audience filters can affect more contacts than intended (spec explicitly calls for bulk dry run); membership updates replace rather than append; scheduled campaigns require active lists and account configuration. Weekly campaign decisions otherwise require separate stats/link/cohort and suppression reads. Prior CLI falsely treats campaign writes as absent; obsolete handoffs must be replaced.

## Data Layer
Contacts, lists, contact-tags, contact-fields, campaigns, forms, automations and domains deserve local SQLite storage and bounded exports. Detail/activity/engagement are scoped child resources; never pretend root sync fully hydrates them. No documented change-feed cursor; full page traversal with caps and completeness metadata. Preserve raw typed fields, tombstone limitations, timestamps and source provenance. Local joins must drain/close rows before subsequent SQLite queries.

## Prior patch watch-list
Ambiguous writes must not replay; rolled-back batches report zero committed rows; path IDs percent-encode as individual segments; keep secure Go floor; preserve worthwhile compound workflows and auth aliases. Public patch directory has five records; legacy index returned 404. Imported clone snapshot has same generation ID; refreshed patch records supersede it as regression watch-list.

## User Vision
Build a general excellent SendFox agent CLI first, migration readiness second. Entire current API is in scope. Strong campaign, audience, automations, forms, domains, async actions and offline surfaces. Thin MCP search/execute plus justified intent tools; hide endpoints for large-surface mode. Every listed mutation can be implemented and mocked, but this build run may not use SendFox/Kit credentials or touch account state. No push, PR, catalog edits or publication. Research, generation and recommended enrichments are pre-approved in the user request. All work stays in Printing Press runstate/library/manuscripts. Global skill updater intentionally omitted to respect these boundaries and the just-updated installed pair.

## Product Thesis
SendFox CLI: full current REST operations with auditable plans, dependable local data, campaign decisions and audience operations. Install it for repeatable agent jobs with explicit safety and completeness evidence, including environments that cannot use the premium hosted MCP.

## Build Priorities
1. Complete typed endpoint contract and safe transport semantics.
2. Large-surface MCP and command discovery with canonical auth compatibility.
3. Approved high-scoring cross-resource workflows from mandatory adversarial brainstorm.
4. Mock verification across all operations, offline dogfood, scorecard and polish with explicit live-unverified hold.

## Hard API gaps versus Kit
No webhook management/event delivery contract, purchases/orders, reusable snippet/template API, public post/feed control, Kit-equivalent growth/aggregate analytics, richer behavioral logic than documented AND filters, richer timing/enrollment inspection, or sequence-progress import. Local artifacts may expose these gaps; they cannot invent parity.

## Ecosystem source audit
Activepieces source has create contact, create list, unsubscribe and custom REST call; no triggers. Pipedream source has one create-contact-with-lists action. Both send Bearer tokens. Activepieces issue search returned list-assignment fix PR #2431, no demonstrated widespread access outage in returned results. Official plugins listing contains no SendFox entry. DeepWiki monorepo output has no SendFox-specific added facts; no architectural claims absorbed. Primary sources: https://github.com/activepieces/activepieces/tree/main/packages/pieces/community/sendfox and https://github.com/PipedreamHQ/pipedream/tree/master/components/sendfox_oauth.
