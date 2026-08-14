# iClassPro CLI — Shipcheck

## Final leg results

| Leg | Result | Exit |
|---|---|---|
| verify | **PASS** | 0 |
| validate-narrative | **PASS** | 0 |
| dogfood | **PASS** | 0 |
| workflow-verify | **PASS** (`workflow-pass`) | 0 |
| apify-audit | **PASS** | 0 |
| verify-skill | **PASS** | 0 |
| scorecard | HOLD | 3 |

**Scorecard: 83/100 — Grade A.** The hold is `unverified dimensions: live_api_verification`.

Go test suite: **899 tests passed across 20 packages.** `go vet` clean. `govulncheck` clean.

## The scorecard hold, explained

`live_api_verification` reports `N/A` and is already excluded from the scoring denominator (alongside `mcp_tool_design`, `mcp_surface_strategy`, and `auth_protocol`). It is keyed on verifying live calls through a configured credential, and this API has none — the iClassPro Open API answers plain HTTP with no key, cookie, or user agent. There is no credential to verify with, so the dimension cannot resolve for this CLI shape.

The underlying property is verified, and by a stronger check: **Phase 5 full live dogfood ran the entire command surface against the real portal and passed 107/107**, recorded in `phase5-acceptance.json` with `status: pass`. The scorecard's own live probe also passed **8/8** sample invocations.

This is a scorecard mechanism limitation for no-auth APIs, not an unverified CLI. Treated as a documented gap rather than a ship blocker.

## Scorecard dimension detail

| Dimension | Score |
|---|---|
| Local Cache | 10/10 |
| Breadth | 10/10 |
| Workflows | 10/10 |
| Path Validity | 10/10 |
| Agent Workflow | 9/10 |
| Sync Correctness | 8/10 |
| Data Pipeline Integrity | 7/10 |
| Vision | 6/10 |
| Type Fidelity | 5/5 |
| Dead Code | 3/5 |
| Cache Freshness | 3/10 |
| Insight | 2/10 |

## Fixes applied during shipcheck

1. **`opens-soon` returned a bare array.** The sample-output probe flagged that its response carried no account context — a fresh install got `[]` with no way to tell which account it answered for. Replaced with a self-describing envelope (`account`, `days`, `entities_scanned`, `findings`, `note`). Probe went 7/8 → 8/8.

2. **`--max-age` was a dead flag.** `dogfood` reported it unused. Rather than delete a generated flag, it was wired to real behavior: `icpStaleHint` now warns on stderr when the local mirror for an account is older than the budget, across all six store-reading commands. Writes only to stderr, so JSON/CSV stdout stays stable; `--max-age 0` disables.

3. **A scoped `watch` poisoned the catalog view.** See the build log — `watch` shared `sync`'s recording path and wrote a filtered one-entity snapshot that became "the newest run" for five other commands. `drift` would have reported 123 phantom removals. Split into `RecordICPOpenings` (history only); only `sync` writes snapshots.

4. **Three wrong examples in README/SKILL.** A required positional omitted, an unsupported `--resources locations` value, and a class id belonging to a different tenant. Fixed at source in `research.json` and regenerated.

## Transient non-issue

One scorecard run reported `Cross-tenant compare: exit 2: unexpected fault address` (SIGBUS). Investigated rather than dismissed: `compare` was run 8 times across three DB states plus a sandboxed HOME with zero reproductions, and two subsequent scorecard runs both passed 8/8. The staged binary's mtime predated the `compare.go` edit, and SIGBUS with an unexpected fault address is the signature of an executable being overwritten in place while running — the probe executed the binary while it was being rebuilt mid-run. Not a CLI defect.

## Known gaps

- **`insight` 2/10.** The CLI has no analytics/aggregation command in the generated sense. The equivalent capability ships as `fill-rate`, `drift`, and `compare`, which the dimension does not recognize.
- **`cache freshness` 3/10.** The spec declares `cache.enabled: true`, but the generator emits its pre-read auto-refresh helpers only alongside a generated syncer, which this multi-tenant spec did not produce. Freshness is instead handled explicitly by `--max-age` stale hints on every store-reading command.
- **`dead code` 3/5.** Three unused helpers (`collectionItemsForOutput`, `hasChangedLocalFlags`, `isDryRunResponseForClient`) live in generated files. Left untouched deliberately: hand-editing generated code to satisfy a lint creates regen churn for no runtime benefit. Retro candidate.
- **`appointments` untested live.** Neither open tenant has the subscription; the route is modeled and its plan-gate message is surfaced, but no successful response was ever observed.
- **`news` article ids unknown.** The route is confirmed to exist; no live article was found on any tested tenant.
- **Read-only `auth login` is unverified end-to-end.** The login exchange is implemented from the community driver's observed shape. No gated-tenant credentials were available, so the token-lifts-the-gate step has not been confirmed against a live sign-in-gated account.

## Verdict

**ship**

All six substantive legs pass, the seventh holds only on a dimension that cannot resolve for a no-auth API and whose underlying property is verified more strongly by Phase 5. Scorecard 83/100 exceeds the 65 threshold. Every flagship feature was sampled against live data and returns correct, relevant output. No known functional defect remains in shipping scope; the gaps above are documented and none affects a headline command.
