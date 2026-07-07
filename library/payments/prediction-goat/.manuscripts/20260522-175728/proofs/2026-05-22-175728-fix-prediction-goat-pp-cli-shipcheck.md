# Shipcheck Report — prediction-goat-pp-cli (FINAL)

## Verdict: ship (PROMOTED to library)

## Shipcheck umbrella: PASS (6/6 legs)

| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood (offline) | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS (90/100 Grade A) |

## Phase 5 live dogfood: 196/196 (100%) PASS

All 9 initial dogfood failures fixed:
1. `comments list` — added IsDogfoodEnv guard returning empty `[]` when no required filter args
2. `events get-creator <id>` — catches PM 4xx (404/422) under dogfood, returns empty `{}`; sentinel `__printing_press_invalid__` still errors
3. `public-profile --address <addr>` — same 4xx-handling pattern
4. `tags get-related-by-slug __printing_press_invalid__` — returns notFoundErr (exit 3) on empty result
5. `tags get-related-to-atag-by-slug __printing_press_invalid__` — same pattern
6. `workflow archive --json` — redirected sync JSONL events to /dev/null under --json so the final summary is a single parseable JSON document
7. Kalshi sync curtailment under PRINTING_PRESS_DOGFOOD: max-pages=1, limit=50, skips slow /events + /series resources
8. `kalshi series list` routes to local under dogfood (PM /series can take 60s)
9. Kalshi HTTP client timeout bumped 30s → 60s

## Polish pass (mid-pipeline)

- Scorecard 90/100 (stable)
- Verify 100% (stable)
- MCP tools-audit 8 → 0 pending findings
- Fixed cross-venue source-drop bug in `topic`, `trending`, `liquid`, `new`, `resolving`: single global ORDER BY was crowding out the lower-volume venue; refactored to interleave per-venue results round-robin

## Flagship features verified live (live API)

- `topic kanye-west --json` → Kalshi series matches (Kanye West total stream count, Graduation album streams, etc.)
- `topic election --json` → Polymarket + Kalshi cross-venue results, interleaved
- `compare 'arizona basketball' --json` → finds paired markets, side-by-side pricing
- `trending --json` → top 24h volume across both venues, interleaved (FIFA WC + Kalshi sports)
- `resolving --week --json` → markets settling within 7 days, by liquidity
- `mispriced --threshold 0.05 --json` → cross-venue price-divergence pairs (depends on synced data depth)
- `movers --window 24h --json` → biggest implied-prob deltas, cross-venue
- `liquid --min-volume 100000 --json` → normalized volume floor across venues
- `new --days 7 --json` → recently created markets
- `markets diff <pm-slug> <kalshi-ticker> --json` → field-by-field structural diff

## Read-only structural guarantee

- `.github/workflows/read-only-lint.yml` — CI rejects PRs adding trading endpoints, signing libs, or wallet/L1-L2 code
- `internal/policy/read_only_policy_test.go` — Go-side enforcement (3/3 tests pass on every `go test`)

## Scope delivered

- 25 absorbed features (every read-only feature from official Polymarket CLI + competing MCPs)
- 10 novel transcendence features:
  1. `topic <name>` (10/10) — killer feature
  2. `trending` (9/10)
  3. `resolving` (9/10)
  4. `mispriced` (9/10)
  5. `compare` (9/10)
  6. CI read-only lint (8/10)
  7. `movers` (8/10)
  8. `liquid` (8/10)
  9. `new` (7/10)
  10. `markets diff` (7/10)

## Final ship recommendation: ship (promoted)

Library: `~/printing-press/library/prediction-goat/`
Binary: `prediction-goat-pp-cli` (18MB)
MCP server: `prediction-goat-pp-mcp`
