# Bento CLI Brief

## API Identity
- **Domain:** Email marketing, marketing automation, transactional email, list hygiene for SMB/ecom. Bento competes with Klaviyo, Customer.io, Mailchimp, Kit (ConvertKit). Strong ecom emphasis (Shopify, BigCommerce, Magento native; Vendure via webhook bridge).
- **Users:** Solo founders, lean ecom shops, indie SaaS, agencies. Power users live in subscriber tagging, automations, and transactional ops.
- **Data profile:** Subscribers, Tags, Custom Fields, Events, Broadcasts, Sequences, Workflows (read-only), Email Templates, Segments, Stats (site + segment + report), plus an "experimental" set for email validation / spam / geolocation / blacklist / content moderation / gender guess. ~25 distinct endpoints across these resources.

## Reachability Risk
- **Low** for the documented API surface. Single open issue across all official wrappers is a Socket.dev false-positive flag (bento-mcp #9). No 403/blocked/deprecated reports.
- **Footgun:** Cloudflare WAF returns 403 if `User-Agent` header is missing or generic (`Go-http-client/2.0`, `python-urllib`). Hard-set a descriptive UA -- documented in Bento's own quickstart.
- **Footgun:** `site_uuid` is required on every call. Goes in the query string for GET, in the JSON body for POST/PATCH. Missing it returns 400, not 401.
- **Footgun:** `/batch/emails` batch size is 100 in the Node SDK source but 60 in the public docs. Take the docs (60) as authoritative.
- **Footgun:** `/stats/segment?segment_id=` (docs, query param) vs `/stats/segments/:id` (Node SDK, path param) -- docs form wins.
- **Footgun:** `from` email on `/batch/emails` and broadcasts must be a pre-authorized Author or 403.

## Top Workflows (the things power users actually do)
1. **CSV -> validate -> tag -> trigger automation in one shot.** Today: 3-4 separate commands or a manual script. Bento's own CLI README leads with this as the canonical CI/CD example.
2. **Post-purchase 6-email sequence with new-vs-returning split.** Multi-step UI workflow today (Trigger -> Conditional Split -> Sequence). Strong CLI angle: scaffold the sequence + Bento.js purchase event payload in one command.
3. **Abandoned-cart popup + email + tag decay.** Workflow + JS snippet + custom event. CLI could generate the JS snippet and the corresponding Workflow scaffold.
4. **High-intent visitor capture.** Page-view depth + custom event -> tag -> Slack ping. CLI could verify the event firing end-to-end.
5. **Win-back / reactivation segment.** Built on tag decay + `last_engaged_at` field. Segment then broadcast. CLI could materialize segment criteria offline against synced subscribers.
6. **Pre-import list hygiene.** Run `experimental/validation` + `experimental/jesses_ruleset` against a CSV before `subscribers import` to avoid deliverability tank. Today: ad-hoc Ruby script.
7. **Transactional send from ecom backend.** Order confirmations, shipping notifications, password resets via `/batch/emails`. Ruby/PHP SDKs support it; CLI/MCP do NOT today.
8. **Vendure -> Bento webhook bridge.** Vendure is NOT a native integration; shops bridge via webhook -> custom event, mirroring the Magento pattern. Strong CLI angle for Ozark specifically.

## Table Stakes (must match these or get embarrassed)
From `@bentonow/bento-cli` v0.1.5 (~22 leaf commands):
- `auth` (set/check/clear), `profile`, `subscribers` (search/import/tag/un/subscribe), `tags` (list/create/delete-stub), `fields` (list/create), `events` (track), `broadcasts` (list/create), `sequences` (list/create-email/update-email), `stats site`, `dashboard`.

From `@bentonow/bento-mcp` (14 tools):
- `get_subscriber`, `batch_import_subscribers`, `list_tags`, `create_tag`, `list_fields`, `create_field`, `track_event`, `get_site_stats`, `list_broadcasts`, `create_broadcast`, `list_automations`, `list_workflows`, `list_sequences`, `create_sequence_email`, `get_email_template`, `update_email_template`.

From Ruby + PHP SDKs (the CLI/MCP miss):
- `send_transactional`, `send` (Ruby) -- transactional email send
- `Bento::Spam.valid?` / `Bento::Spam.risky?` -- pre-import validator
- `execute_commands` (Python) -- batch tag/field ops via `/fetch/commands`
- `get_segments` / `get_segment` -- segment list (docs-only endpoint not in any official SDK)

## Data Layer
- **Primary entities:** subscriber, tag, field, event, broadcast, sequence, workflow, email_template, segment.
- **Sync cursor:** Bento has no `since`/`updated_at` filter on `/fetch/subscribers` or `/fetch/search`. Walk pages with `?page=N` until empty. Snapshot full state + diff is the only path.
- **FTS/search:** subscribers, broadcasts, sequences, templates all benefit from FTS5 over name/subject/body. Local search is a strict win because Bento's `/fetch/search` is paginated, slow, and has no fuzzy match.
- **Why local store wins:** ecom shops want to JOIN subscribers x tags x events to answer "who are my top spenders this quarter, segmented by acquisition source" -- impossible in one API call.

## Codebase Intelligence (from SDK source crawl)
- **Auth:** HTTP Basic, base64(publishable_key + ":" + secret_key). User-Agent is REQUIRED -- generic UAs get 403'd by Cloudflare.
- **Data model:** Path constants in `bento-node-sdk/src/sdk/<resource>/index.ts` are ground truth. The OpenAPI spec at `github.com/bentonow/api/blob/main/bento-api.yaml` covers ~20 ops across 17 paths; SDK source covers ~30 ops total.
- **Rate limiting:** 100 req/min on /fetch/, /batch/, /experimental/. 60 req/hour on /stats/. 60 emails/min on /batch/emails (queue throttle separate from HTTP). 1000/day on /data_deletion_requests. Spam-flagged sends drop to 1/hour. NO `Retry-After` header -- client-side backoff required.
- **Pagination:** page-based `?page=N`, starts at 1, walk until empty. No cursor.
- **Special semantics:**
  - `/batch/events` with `type: $subscribe`/`$unsubscribe`/`$tag` TRIGGERS automations
  - `/fetch/commands` with `command: subscribe`/`unsubscribe`/`add_tag` DOES NOT trigger automations
  - `$purchase` events need `details.unique.key` for dedupe, `details.value.amount` in CENTS, currency as ISO 4217
- **No DELETE endpoints in SDKs** except docs-claimed `DELETE /fetch/tags`. No subscriber delete -- use `/data_deletion_requests` (GDPR queue, form-encoded body).

## Source Priority
Single-source CLI -- Bento is the only API. Primary spec: `github.com/bentonow/api/blob/main/bento-api.yaml` (already saved at `$RESEARCH_DIR/bento-openapi.yaml`). SDK source crawl fills the ~10 endpoints the spec misses (sequences, workflows, templates, forms, data-deletion, commands batch op, jesses_ruleset, gender, segments list).

## Product Thesis
- **Name:** `bento-pp-cli` (binary), `bento` (slug in printing-press-library/marketing/)
- **Why it should exist:** Bento has an official Node CLI and an official MCP, but both:
  1. Cover only ~60% of the actual API surface (no transactional send, no segments list, no `/fetch/commands` batch op, no spam/validator surface, no `data_deletion_requests`).
  2. Require Node runtime -- friction for CI/CD, Go shops, single-binary deployments.
  3. Have no offline/local-store mode -- every list call hits the API.
- **Differentiator:** Go binary + SQLite-backed local store + agent-native --json --select --dry-run on every command + MCP tools auto-derived from the Cobra tree. Add the ~10 endpoints the official tools miss. Layer ecom-specific commands (Vendure bridge, transactional sends, list-hygiene scrub) that nobody else has.
- **Pitch line:** "Every Bento feature, plus the ones their own CLI doesn't have, plus a local SQLite store that makes 'who are my top spenders by tag' a one-liner."

## Build Priorities
1. **Priority 0 (foundation):** SQLite store for subscribers, tags, fields, broadcasts, sequences, workflows, templates, segments. `sync` command that walks all paginated endpoints into the store. FTS5 over searchable text fields.
2. **Priority 1 (absorb everything official):** Every command from `@bentonow/bento-cli` (22 leaves) + every tool from `@bentonow/bento-mcp` (14 tools) + the Ruby SDK's transactional send + spam-check + commands-API.
3. **Priority 2 (transcend with novel commands):** See absorb manifest.
4. **Priority 3 (polish):** README cookbook with Ozark-flavored ecom recipes, exit-code conventions, MCP read-only annotations for safe agent use.
