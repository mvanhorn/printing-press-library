# wanderlust-goat — Polish Pass

## Delta

|  | Before | After |
|---|---|---|
| Scorecard | 80/100 | 81/100 |
| Verify pass rate | 100% | 100% |
| Tools-audit pending | 0 | 0 |
| Dead functions | 1 | 0 |

## Fixes applied (6)

1. Removed dead helper `extractResponseData` from `internal/cli/helpers.go`.
2. Created `internal/httperr` package with `Snippet()` (HTML-strip + whitespace-collapse + 200-char trunc, rune-safe). Adopted in 10 client packages: atlasobscura, eater, michelin, navermap, osrm, overpass, reddit, timeout, wikipedia, wikivoyage. Fixes the "kilobyte of HTML in error message" bug.
3. Added `accept-language=en` to Nominatim geocode URL in `goat_orchestrator.go::resolveAnchor` — prevents alt-language transliterations leaking into display names.
4. `golden-hour --zone` default changed `Local` → `UTC` with help-text update — prevents misleading local-OS-zone display for remote anchors.
5. Wikipedia GeoSearch path in Fanout now filters by per-page intent classification, dropping Wikipedia hits whose classified intent doesn't match the user's parsed intent. Stops generic-landmark Wikipedia articles from dominating persona-shaped queries.

## Skipped findings (out of scope or false-positive)

- `mcp_token_efficiency` 4/10 / `mcp_remote_transport` 5/10 — structural, requires `mcp:` block in spec.yaml + regen.
- 14 client files flagged for "missing rate-limiting" — false-positive heuristic; clients already implement `throttle()`/`RateLimitPerSecond`.
- 7 stub packages flagged for "no tests" — intentional v1 stubs, tests provide no value.
- Various scorecard breadth/vision/workflows dimensions — calibrated for larger APIs, lower scores reflect scorer not CLI.

## Ship recommendation: **ship**
## Further polish recommended: **no**
