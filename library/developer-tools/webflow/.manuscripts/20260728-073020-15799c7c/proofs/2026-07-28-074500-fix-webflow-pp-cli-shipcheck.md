# Webflow CLI — Shipcheck

Run: 20260728-073020-15799c7c · Printing Press v4.29.0

## Final verdict: `ship`

## Leg results

| Leg | Result | Exit | Elapsed |
|---|---|---|---|
| verify | PASS | 0 | 7.3s |
| validate-narrative | PASS | 0 | 0.6s |
| dogfood | PASS | 0 | 5.0s |
| workflow-verify | PASS | 0 | 0.02s |
| apify-audit | PASS | 0 | 0.06s |
| verify-skill | PASS | 0 | 8.5s |
| scorecard | PASS | 0 | 1.8s |

`Verdict: PASS (7/7 legs passed)`

## Scorecard: 93/100 — Grade A

| Dimension | Score |
|---|---|
| Output Modes | 10/10 |
| Auth | 10/10 |
| Error Handling | 10/10 |
| Terminal UX | 10/10 |
| README | 10/10 |
| Doctor | 10/10 |
| Agent Native | 10/10 |
| MCP Quality | 8/10 |
| MCP Remote Transport | 10/10 |
| MCP Tool Design | 10/10 |
| MCP Surface Strategy | 10/10 |
| Local Cache | 10/10 |
| Cache Freshness | 5/10 |
| Breadth | 10/10 |
| Vision | 9/10 |
| Workflows | 10/10 |
| Insight | 10/10 |
| Agent Workflow | 9/10 |
| Path Validity | 9/10 |
| Auth Protocol | 10/10 |
| Data Pipeline Integrity | 7/10 |
| Sync Correctness | 10/10 |
| Type Fidelity | 5/5 |
| Dead Code | 5/5 |

Omitted from denominator: `mcp_description_quality`, `mcp_token_efficiency`,
`live_api_verification`.

**Cache Freshness 5/10 is a deliberate choice, not a defect.** Webflow allows 60
requests per minute on Starter and Basic plans. Generator-owned cache
auto-refresh would issue an upstream call before every local read, which is a
real cost against that ceiling. The CLI ships manual `sync` plus the generated
`doctor` freshness report instead. Raising this score would mean making the
product worse.

## Sample Output Probe

`Passed: 7/7 (100% pass rate, 0 skipped)` — every novel-feature command was
invoked and produced output of the expected shape.

## Blockers found and fixed

**1. `validate-narrative` FAIL — troubleshoot referenced a command that does not exist.**
The narrative said `webflow-pp-cli items publish`; the generated path is
`collections items publish`. A second entry said `sites plan <site-id>`; the
generated path is `sites plan get-site <site-id>`. Both fixed at the source in
`research.json`, not in the rendered README/SKILL, so the correction survives
regeneration.

**2. Placeholder args in narrative examples.** After fixing the paths, both
examples still failed because `<site-id>` / `<collection-id>` are not runnable.
Replaced with concrete IDs, matching the convention the quickstart already used.

**3. The quickstart's sync recipe was wrong and would have half-failed.**
`sync --resources sites,pages,collections,items` errors with
`WEBFLOW_COLLECTION_ID not set` on the `items` resource, because CMS items are
scoped to a collection and cannot resolve until collections are synced. Three of
fifteen resources errored. This was caught by running the command, not by any
gate — `validate-narrative` only checks the command path, not framework
resource names.

Fixed to `webflow-pp-cli sync --full`, which resolves the dependency order and
reports 16/16 success. Added a troubleshooting entry naming the failure and both
escapes (`sync --full`, or set `WEBFLOW_COLLECTION_ID`). This mattered because
all seven novel commands read the `items` table.

## Before / after

| Metric | Before | After |
|---|---|---|
| Shipcheck legs passing | 6/7 | 7/7 |
| validate-narrative | FAIL (1 failed example) | PASS (16 ok, 0 failed) |
| Scorecard | 93/100 Grade A | 93/100 Grade A |
| Sample output probe | 7/7 | 7/7 |
| Tests | 1022 pass | 1022 pass |

## Phase 4.7 — Sync Param-Drop Gate

**Skipped, correctly.** The gate compares sync-call parameter cardinality
against a browser-sniff capture. This CLI was generated from the official vendor
OpenAPI spec; there is no `traffic-analysis.json` for this run, and no browser
capture was performed (Phase 1.7 recorded `skip-silent` / `spec-complete`).

## Known gaps

- **Ecommerce commands are generated but not behaviorally verified.** Products,
  SKUs, orders, and inventory ship from the spec. No ecommerce-plan site was
  available to exercise them.
- **Live API verification is not scored.** The token supplied for this run is a
  DevLink workspace token with no Data API scopes; it returns
  `403 OAuthForbidden: missing scopes 'sites:read'` on every data endpoint. A
  site token with read scopes is needed for Phase 5.
