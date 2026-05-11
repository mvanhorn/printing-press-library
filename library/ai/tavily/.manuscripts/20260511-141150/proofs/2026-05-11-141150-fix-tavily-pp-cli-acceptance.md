# Acceptance Report: tavily

Level: Quick Check
Tests: 6/6 passed

## Results
1. doctor --json: PASS — API reachable, auth configured from env:TAVILY_API_KEY
2. web-search --query "Tavily API" --max-results 3 --json: PASS — 3 results with titles returned
3. extract --urls "https://docs.tavily.com/welcome" --json: PASS — 8,890 chars extracted
4. usage --json: PASS — Researcher plan, 0/1000 credits
5. usage history --days 7 --json: PASS — Snapshot stored, history returned
6. stale --days 1 --json: PASS — Empty result (correct, no stale content)

Failures: 0
Fixes applied: 4
  1. Fixed empty POST body in web-search (generator bug — flags not populated into body map)
  2. Fixed empty POST body in extract, crawl, map, research (same generator bug)
  3. Fixed SKILL/README command naming: search → web-search
  4. Fixed positional arg examples → --query/--input flag format

Printing Press issues: 1
  - Generator does not populate POST body from flag values (all POST endpoints shipped with empty body map)

Gate: PASS
