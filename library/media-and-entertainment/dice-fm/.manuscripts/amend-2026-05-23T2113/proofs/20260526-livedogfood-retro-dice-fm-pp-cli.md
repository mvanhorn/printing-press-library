# Printing Press Retro: dice-fm (live-dogfood driven)

## Session Stats
- API: dice-fm (DICE Partners GraphQL)
- Trigger: first live verification with a real `DICE_FM_TOKEN` (generation skipped Phase 5 live — no token)
- Live dogfood: 86/96 PASS, 10 FAIL, 45 skip (after fixes)
- Printed-CLI fixes this session (landed on library PR #789): 2
  - `genres`: nested Relay connection missing `first`/`last` pagination arg
  - `sync --json`: result written to stderr instead of stdout

## Findings

### 1. Live dogfood false-fails read commands whose example carries a resource ID via a flag (scorer)
- **What happened:** 10 of the 10 remaining live-dogfood failures were `door list`, `extras`, `orders`, `tickets`, `transfers` invoked with a fabricated `--event` ID (`RXExampleEventID` / `RXThisExampleEventID`) lifted verbatim from each command's `Example:` string. The live DICE API correctly rejects the fake global ID (`graphql: ... Could not extract value from decoded ID`). The commands work with real event IDs (verified live this session).
- **Scorer correct?** No — false positive. The CLI is correct; the harness fabricated the flag value.
- **Root cause:** `internal/pipeline/live_dogfood.go` resolves **positional** placeholders by harvesting a real id from a prior list/sync call (`resolveCommandPositionals` + `extractFirstID` Path 6/7 `edges[0].node.id`), but flag-carried example values (`--event <id>`) are passed through verbatim by `liveDogfoodHappyArgs`. There is no flag-value resolution and no skip. Additionally, the existing required-param skip (`liveDogfoodRequiredParamFixtureReason`) keys on HTTP 400/422; the DICE rejection is a GraphQL `200` with an `errors` array, so it isn't caught.
- **Cross-API check (Step B):** dice-fm (`--event`, strong: 10 false-fails this run); pcgs (`--pcgs-no 1106065`, flag-carried ID — partial). numista's `--client-secret <secret>` is an auth flag, not a resource selector. **Only ~1.5 APIs with real evidence → P3 max per Step B.**
- **Counter-check (Step C):** harvesting a real id for a flag selector, or skipping when none is available, doesn't hurt APIs without the pattern (it mirrors the positional path already in place). No guard needed.
- **Step G (case against):** "dice-fm-specific GraphQL filter shape; just one API." Why it survives anyway: it's the same scorer territory as the open issue #2120 (classify commands the matrix can't safely fixture as skip, not fail), with a concrete new mechanism (flag-carried IDs + GraphQL-200 errors). Routed as a comment on #2120, not a new issue — so it adds evidence without spending a fresh issue.
- **Durable fix:** extend #2120's ask #1 (`BLOCKED_FIXTURE`/skip for unfixturable commands) to (a) resource IDs carried by a flag, not only positionals, and (b) GraphQL-style `200 + errors` rejections, not only HTTP 4xx. Cheapest increment: when a happy-path example's flag value is an opaque/placeholder id and no harvested id is substituted, skip rather than fail.
- **Evidence:** live dogfood v3 — `failed: 10`, every failure output `graphql: ... Could not extract value from decoded ID`. Same commands return real data when given a real `RXZl…`-shaped event id.
- **Related prior retros / issues:** #2120 (`extends` — same scorer area, write-side + sync; this is the read-flag-id manifestation), #2098 (read required-param skip), #1859.

## Prioritized Improvements

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|----------------------|------------|--------|
| 1 | Live dogfood false-fails flag-carried resource-ID examples | scorer | subclass: flag-id-filtered reads | low (looks like a real failure) | small–medium | none needed |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| sync→stderr | `sync --json` wrote events to stderr; stdout empty | printed-CLI — dice-fm's hand-rewritten sync diverged from the generated convention (numista/pcgs emit to stdout); fixed in PR #789 |
| transient API 500s | orders/tickets/transfers occasionally 500, retry recovers | API-quirk — upstream; built-in retry already handles it |

## What the Printing Press Got Right
- The positional-placeholder id-harvesting (Path 6/7) is the right pattern — the gap is only that it doesn't extend to flag-carried ids.
- `validLiveDogfoodJSONOutput` already accepts NDJSON, so the genuine fix (sync result → stdout) needed no scorer change.

## Outcome
One finding, P3, routed as a **comment on cli-printing-press#2120** (dedup match in the same scorer area). No new issue filed. Both printed-CLI bugs fixed on library PR #789.
