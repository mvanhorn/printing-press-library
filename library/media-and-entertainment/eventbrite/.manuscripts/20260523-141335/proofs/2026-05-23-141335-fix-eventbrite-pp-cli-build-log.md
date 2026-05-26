# Eventbrite CLI Build Log

## What was built

**Generator (Priority 0 + 1):**
- Data layer: generic `resources` table + FTS5 + typed tables for ~46 resource_type variants; `sync` (continuation-token pagination), `search`, `sql`, `analytics`, `tail`, `import`, `workflow`.
- ~100 endpoint-mirror commands across 33 resource groups (events, orders, attendees, ticket classes, discounts, inventory tiers, reports, webhooks, organizations, venues, categories, etc.).
- MCP: Cloudflare pattern (code orchestration + hidden endpoint tools + stdio/http transport) via `x-mcp` enrichment, since the raw surface is 100 endpoint tools (>50).
- Auth: Bearer private token, `EVENTBRITE_API_KEY` (spec enriched — converted oauth2 scheme was malformed).

**Hand-authored (Priority 2 — 9 transcendence commands, Codex-delegated):**
- `internal/cli/eventbrite_store.go` (Claude): shared store readers (events/orders/attendees/ticket_classes/discounts) over the generic `resources` table with `resource_type IN (...)` (flat + hierarchical naming), SQL COALESCE NULL-safety, dedup by id, email normalization, minor→major money helpers.
- 9 command files (Codex `gpt-5.3-codex`, one delegation, build/vet/gofmt clean first try): `sales-velocity`, `repeat-attendees`, `discount-performance`, `org-rollup`, `roster`, `capacity`, `refund-rate`, `top-buyers`, `fan-export`. All verify-friendly RunE (dryRunOK guard, no required-arg gates), `mcp:read-only`, `printJSONFiltered`, empty-slice init.
- `internal/cli/eventbrite_store_test.go` (Claude): 3 table-driven tests for the readers (dedup, NULL-safety, check-in bool, name fallback, nested name.text extraction). Pass.

**User-requested cross-pollination (Phase 1.5 brainstorm):** `top-buyers` (← DICE `fans top`) and `fan-export` (← DICE `fans optin`) added. Eventbrite+DICE cookbook recipe authored into research.json narrative (cross-CLI, documented — not a baked-in dependency). DICE amend candidates surfaced to user: `discount-performance`, `capacity`.

## Intentionally deferred / excluded
- `/events/search/` (public Event Search) — removed by Eventbrite in 2020; dropped from the spec, not shipped. The transcendence layer restores cross-event search over the organizer's own synced data.
- Balance-based revenue reconciliation — killed in brainstorm (Balance gross-vs-payout semantics ambiguous, unverifiable).

## Generator limitations found (retro candidates)
1. **Backtick in JSON struct tags → uncompilable `types.go`.** apib2swagger mis-parsed Blueprint MSON, producing property keys containing backticks; the type emitter copied them verbatim into backtick-quoted Go struct tags, terminating the literal. The generator should sanitize struct-tag contents. Worked around by sanitizing 12 polluted schema keys pre-generate. (Carried over from this run; will file in end-of-run retro.)
2. **Converted-spec oauth2 scheme malformed** (`authorizationUrl: "/"`). Expected for Blueprint→OpenAPI conversion; handled by spec enrichment to a clean Bearer scheme. Not a generator bug per se, but a note for any future Apiary-source path.

## Phase 3 Completion Gate
- Per-command Cobra resolution: 9/9 resolve with correct `<cmd> [flags]` Usage line, exit 0.
- Dry-run exit 0 + JSON `[]` (no DB) for all 9; roster handles positional + `--event`.
- Deterministic `novel_features_check`: planned 9, found 9, missing none, skipped false. PASS.
- Reader tests pass.
