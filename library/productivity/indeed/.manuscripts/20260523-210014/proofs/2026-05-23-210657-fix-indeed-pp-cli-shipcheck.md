# Indeed CLI Shipcheck

## Verdict: ship

Shipcheck umbrella: **PASS (6/6 legs)**.

| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS (82/100, Grade A) |

## Scorecard highlights
- Output Modes 10, Auth 10, Error Handling 10, README 10, Doctor 10, Agent Native 10, Local Cache 10, MCP Remote Transport 10, MCP Desc Quality 10, Path Validity 10, Sync Correctness 10.
- Weaker (inherent to a focused read-only HTML-extraction CLI): Insight 4, Cache Freshness 5, Breadth 6, Workflows 6, Type Fidelity 3/5, Dead Code 3/5. Polish pass targets Dead Code + descriptions.

## Live behavioral verification (manual, pre-dogfood)
All headline + novel commands exercised against the live Cloudflare-protected site via Surf:
- `search "software engineer" --location Remote --limit 5 --json` → real jobs, salary parsed (min/max/period), ratings, cleaned snippets.
- `job get <key> --json` → JSON-LD JobPosting: title, company, location, employmentType, dates, full cleaned description.
- `find "java" --json --select ...` → offline FTS over stored jobs.
- `saved save` / `saved list` / `new <name>` → baseline then fresh-only diff.
- `track` / `tracked`, `company <name>`, `apply <key>` (print-default), `related --job-key`.

## Sample Output Probe note
The scorecard live-sample probe reported 3/6 because it ran `new daily-remote`, `find "rust kubernetes"`, and `saved save ...` against a fresh empty store; those legitimately return empty/confirmation output that doesn't echo a query token. Not real failures — verified manually above.

## Reachability
Surf (Chrome TLS fingerprint) clears Cloudflare on homepage and SERP (probe-reachability mode=browser_http, no clearance cookie). Validated by live calls returning real data.
