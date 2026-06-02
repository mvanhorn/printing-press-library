# Sybill CLI — Build Log

## Generated (Priority 0 + 1)
- Generator produced data layer (SQLite, modernc, FTS5) for accounts, conversations, deals, documents, messages, object_types, rows, sources + sync_state.
- All GET endpoints as typed commands (conversations/deals/accounts/messages/rows/documents/sources/object-types list+get), ingest (POST/PATCH/DELETE), doctor, health, sync, search, analytics.
- Auth: Bearer SYBILL_API_KEY wired via spec x-auth-env-vars enrichment. Confirmed in internal/config/config.go and auth.go.
- Build/vet/govulncheck/doctor all PASS at generation.

## Hand-built (Priority 2 — transcendence, 6/6)
All read from local SQLite (offline = the leverage). New shared helpers in internal/cli/novel_sybill.go (loaders, time parsing, conversation<->deal/account linkage via crm.{id,name,type}=opportunity/account).

1. `deals dark --days N [--include-uncovered] [--owner] [--stage]` — open deals whose newest linked call is older than N days; MAX(conversation.start) per deal joined locally.
2. `digest --since 7d [--type] [--owner]` — conversations in window grouped by linked deal; extracts nextSteps/keyTakeaways from summary when detail synced; unlinked calls grouped separately.
3. `crm-autofill [--deal ID]` — surfaces deal crmAutofill suggestions as field/suggested/current diff; fetches deal detail live when --deal not in store (crmAutofill is detail-only).
4. `account rollup <account>` — joins accounts+deals+conversations+contacts: call count, open deals by stage, open value, contacts.
5. `activity --by owner --since 7d [--dark-days]` — per-owner open/closed deals, open value, calls in window, deals gone dark.
6. `patterns --term "a|b" [--since]` — case-insensitive alternation scan over cached conversation content (title + summary + transcript when synced), grouped by deal+stage.

## Tests
- internal/cli/novel_sybill_test.go: 6 behavioral acceptance tests against a synthetic seeded store. Assert content (counts, group membership, diff values, negative-match), not just exit codes. All PASS.

## Intentionally deferred / not built
- ask_sybill NLP query: MCP-only (OAuth), not available via REST API key. Deliberately excluded.
- deal momentum/velocity: no stage-change history in payload (un-buildable). Dropped at absorb gate.
- Webhook Svix verify helper (absorb #14): not yet wired as a standalone command; webhooks are configured server-side and signature verify is a thin helper. Candidate for polish if scorecard flags coverage.

## Generator notes
- Warning: novel feature command "deals" maps to existing deals.go — expected; `deals dark` registered as subcommand.
- Warning: missing workflows/comm_health.go.tmpl — cosmetic, no impact.
- spec.OwnerName empty → author fallback to slug. Cosmetic.
