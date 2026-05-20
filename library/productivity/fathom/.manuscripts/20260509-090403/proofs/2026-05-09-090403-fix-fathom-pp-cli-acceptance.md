# Fathom CLI Live Acceptance Report

## Level: Full Dogfood
## Tests: 11/11 passed
## Gate: PASS

## Live API Tests

| Test | Result | Notes |
|------|--------|-------|
| doctor | PASS | API reachable, auth configured from FATHOM_PP_CLI_API_KEY |
| sync --full | PASS | 20 meetings, 20 team-members, 5 teams synced |
| meetings list (live) | PASS | Returns 3 meetings from API with correct envelope |
| commitments | PASS | 65 open action items across synced meetings |
| topics | PASS | "ai" = 433 mentions, "meeting" = 61 mentions across 10 meetings |
| brief --domain | PASS | Returns meeting history with participant emails, summary snippet |
| velocity | PASS | Correctly returns "no-data" for unknown domain |
| workload | PASS | Per-person meeting hours with trend |
| account | PASS | 10 meetings, all VBT contacts, top topics |
| stale | PASS | Returns [] after filtering mock items |
| crm-gaps | PASS | Returns [] (no CRM integration configured) |
| coverage --pattern Weekly | PASS | Found 3 weekly meetings in W19 |

## Bugs Fixed During Dogfood

### Bug 1: meetings sync stored 0 items (CLI fix)
`resourceIDFieldOverrides` had no entry for "meetings". Fathom uses `recording_id` (integer), not the generic fallback keys (`id`, `uuid`, etc.). Fixed by adding `"meetings": "recording_id"` to the override map.

### Bug 2: sync stopped after 1 page (CLI fix — systemic candidate)
Exit condition `if !hasMore || len(items) < pageSize.limit || nextCursor == ""` stops when `hasMore=false`. Fathom doesn't return `has_more`; it signals more pages via `next_cursor` presence. Fixed by treating `nextCursor != ""` as `hasMore=true`.

### Bug 3: meetings missing rich fields (CLI fix)
Fathom's `/meetings` endpoint returns minimal data by default. Added include flags (`include_action_items=true`, `include_summary=true`, `include_transcript=true`, `include_crm_matches=true`) to the sync for the meetings resource.

## Printing Press Issues (for retro)
- Pagination exit condition assumes `has_more` field; should also treat `nextCursor != ""` as more pages
- `resourceIDFieldOverrides` should be populated from spec `x-resource-id` annotations; Fathom's integer `recording_id` needs explicit mapping
