---
title: "Non-Obvious Insight Review: Dub CLI"
type: fix
status: active
date: 2026-03-27
phase: "3"
api: "dub"
---

# Non-Obvious Insight Review: Dub CLI

## Automated Scorecard Baseline: 68/100 (Grade B)

| Dimension | Score | Gap |
|-----------|-------|-----|
| Output Modes | 10/10 | - |
| Auth | 8/10 | -2 |
| Error Handling | 10/10 | - |
| Terminal UX | 9/10 | -1 |
| README | 5/10 | -5 |
| Doctor | 10/10 | - |
| Agent Native | 8/10 | -2 |
| Local Cache | 10/10 | - |
| Breadth | 8/10 | -2 |
| Vision | 9/10 | -1 |
| Workflows | 4/10 | -6 |
| Insight | 0/10 | -10 |
| Path Validity | 5/10 | -5 |
| Auth Protocol | 5/10 | -5 |
| Data Pipeline Integrity | 10/10 | - |
| Sync Correctness | 8/10 | -2 |
| Type Fidelity | 3/5 | -2 |
| Dead Code | 0/5 | -5 |

## Competitor Feature Matrix

| Feature | dubco (24★) | Ours | Status |
|---------|------------|------|--------|
| Create link (interactive) | ✅ | ❌ (flags-based) | BETTER (flags > prompts) |
| Create link (flags) | ❌ | ✅ | HAVE |
| List links | ❌ | ✅ | HAVE |
| Search links | ❌ | ✅ | HAVE |
| Update/delete links | ❌ | ✅ | HAVE |
| Bulk operations | ❌ | ✅ | HAVE |
| Analytics | ❌ | ✅ | HAVE |
| Domains CRUD | ❌ | ✅ | HAVE |
| Tags/folders CRUD | ❌ | ✅ | HAVE |
| Partners/customers | ❌ | ✅ | HAVE |
| QR codes | ❌ | ✅ | HAVE |
| --json output | ❌ | ✅ | HAVE |
| doctor | ❌ | ✅ | HAVE |
| sync + local DB | ❌ | ✅ (basic) | HAVE |
| search (FTS5) | ❌ | ✅ (basic) | HAVE |
| tail (events) | ❌ | ✅ (basic) | HAVE |
| export/import | ❌ | ✅ (basic) | HAVE |
| snapshot | ❌ | ❌ | MISSING |
| stale | ❌ | ❌ | MISSING |
| health | ❌ | ❌ | MISSING |
| compare | ❌ | ❌ | MISSING |

## GOAT Improvement Plan

### Priority 0: Data Layer (from Phase 0.7)
1. Rewrite store.go with domain-specific tables (links, analytics_snapshots, events, customers, sync_state)
2. Add FTS5 on links (url, key, title, description)
3. Rewrite sync command with proper link/event sync strategies
4. Add `sql` command for raw queries
5. Add `snapshot` command for analytics archival

### Priority 1: Table Stakes (from Phase 0.6)
- All CRUD commands already generated. Need name normalization only.

### Priority 2: Workflow Commands (from Phase 0.5)
1. `snapshot` - Analytics archival with all groupBy dimensions
2. `stale` - Zero-click link detection
3. `health` - Destination URL health check
4. `compare` - Period-over-period analytics comparison
5. `export` already exists but needs enrichment
6. `import` already exists but needs CSV format support
7. `tail` already exists but needs enrichment

### Priority 3: Command Names + Product Name
- Rename `dub-cli` to `dub-pp-cli` everywhere
- Fix module path from `github.com/trevin-chow/dub-cli` to `github.com/tmchow/dub-pp-cli`
- Normalize command names (retrieve -> get, etc.)

### Priority 4: Scorecard Fixes
- Fix dead code (0/5)
- Fix insight commands (0/10) — add health, stale commands
- Fix workflows score (4/10) — add snapshot, compare, import/export enrichment
- Fix README (5/10) — add cookbook section with workflow examples
- Fix path validity (5/10) and auth protocol (5/10)

### Complex Body Field Plan
Top 3 for --stdin examples:
1. `geo` (link geo-targeting): `echo '{"US":"https://us.example.com","UK":"https://uk.example.com"}' | dub-pp-cli links create --url https://example.com --stdin`
2. `metadata` (track lead/sale): `echo '{"plan":"pro","source":"organic"}' | dub-pp-cli track lead --stdin`
3. `data` (bulk update): `echo '[{"id":"link_123","url":"https://new.com"}]' | dub-pp-cli links bulk-update --stdin`
