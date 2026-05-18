# Acceptance Report: trustpilot

Level: Quick Check
Tests: 6/6 passed

| # | Test | Result | Evidence |
|---|------|--------|----------|
| 1 | auth login --chrome | PASS | Harvested aws-waf-token + both buildIds in ~10s |
| 2 | info www.thriftbooks.com | PASS | TrustScore 4.7, 2.78M reviews, AI summary 471k reviews ref'd |
| 3 | top-recent --good 3 --bad 3 | PASS | Returned 3 5-star + 3 1-star recent reviews, both buckets full |
| 4 | agent-bundle | PASS | Composed company/aiSummary/topRecent/histogram in single JSON |
| 5 | search thriftbooks | PASS | 5 hits, top hit www.thriftbooks.com TrustScore 4.7 |
| 6 | sync-trustpilot --max-pages 3 + search-reviews "shipping" | PASS | 60 reviews persisted, FTS5 returned 3 matches for "shipping" |

Failures: none
Fixes applied: 1 (omit ?page=1 query param to avoid Next.js soft-redirect to canonical URL)
Printing Press issues: none surfaced

Gate: PASS
