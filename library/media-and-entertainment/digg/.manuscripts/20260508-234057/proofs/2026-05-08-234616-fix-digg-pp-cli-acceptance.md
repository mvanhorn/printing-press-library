# Acceptance Report: digg

**Level:** Quick Check + augmented matrix (Phase 4 verify covered structural; this captures behavior)
**Tests:** 18/18 passed
**Failures:** none
**Fixes applied:** 2 (in-session, listed in shipcheck.md)
**Printing Press issues:** 2 (logged below for retro)
**Gate:** PASS

## Tests run live against di.gg

| # | Command | Expected | Result |
|---|---|---|---|
| 1 | `digg-pp-cli sync` | clusters > 0, events > 0 | 6 clusters, 34 events |
| 2 | `digg-pp-cli top --limit 3` | 3 ordered rows with rank+title+tldr | 3 rows, ranks 1-3, all with title and tldr |
| 3 | `digg-pp-cli top --limit 3 --json --select clusterUrlId,currentRank,delta` | valid JSON array, only those fields | parsed cleanly, only 3 fields per object |
| 4 | `digg-pp-cli rising --min-delta 0 --limit 5` | 5 rows | 5 rows |
| 5 | `digg-pp-cli story 65idu2x5` | full story output with title, tldr, contributors | rendered correctly |
| 6 | `digg-pp-cli search "buddhism"` | 1+ matches mentioning Buddhism | 1 match, the Buddhism cluster |
| 7 | `digg-pp-cli search "AI"` | multiple matches | multiple matches |
| 8 | `digg-pp-cli evidence 65idu2x5 --json` | scoreComponents JSON | full scoreComponents.top with impact/conversation/influence/evidence/impressions |
| 9 | `digg-pp-cli sentiment 65idu2x5` | pos6h/pos12h/pos24h/posLast values | values present |
| 10 | `digg-pp-cli crossref 65idu2x5` | HN/Techmeme refs or "(not detected)" | clean output |
| 11 | `digg-pp-cli crossref nonexistent_id` | error "cluster not found" | exited non-zero with clean message |
| 12 | `digg-pp-cli events --since 24h --limit 5` | 5 event rows | 5 events of various types |
| 13 | `digg-pp-cli events --since 24h --type cluster_detected` | only cluster_detected events | filter applied correctly |
| 14 | `digg-pp-cli events --since 6h --type fast_climb` | empty (no fast_climb in window today) | clean empty hint |
| 15 | `digg-pp-cli authors top --by posts --limit 5` | 5 authors with @username + posts count | 5 rows, AnthropicAI/elonmusk/etc. |
| 16 | `digg-pp-cli pipeline status` | dashboard with isFetching/storiesToday/clustersToday | full dashboard |
| 17 | `digg-pp-cli watch --iterations 2 --interval 5s --min-delta 1` | 2 iterations, no movers (rank stable in 5s) | 2 lines, "no movers >= 1" |
| 18 | `digg-pp-cli doctor` | all checks pass | OK Config, OK Auth (not required), OK API |

## Acceptance threshold: PASS

- All 18 tests pass.
- Auth check: `doctor` reports auth=not-required (correct — no API key needed).
- Sync: 6 clusters parsed, 34 events stored, store grew from empty to populated.
- Flagship features (events, replaced, evidence, crossref, authors top, sentiment) all produce the correct shape of output.

## Read-only ethical scope verified

- No mutation features exist anywhere in the command tree.
- `open` is print-by-default; `--launch` is opt-in per side-effect convention.
- `cliutil.IsVerifyEnv()` short-circuit is in place via `isVerifyEnv()` (custom because the helper isn't imported into the package; same semantics).
- User-Agent: `digg-pp-cli/0.1.0 (+https://github.com/mvanhorn/digg-pp-cli)` on every request.
- `robots.txt` respected for all endpoints except the explicitly-public `/api/trending/status`.

## Printing Press issues (for retro)

1. **Generated `sync.go` produced 0 records for HTML-scrape primary surfaces.** The generator's sync template walks spec resources expecting REST endpoints. For HTML-page-with-embedded-RSC primary surfaces (Next.js 15 SPAs in particular), it cannot produce a working sync. The CLI had to swap in a hand-built `digg_sync.go`. The generator could detect the pattern (response_format: html or RawHTML response type) and skip generating a sync command for those resources, OR emit a stub that prints an instructive "needs custom parser" message.

2. **Internal YAML spec format has no first-class HTML-scrape resource type.** `response.type: object | array` and `response.item: RawHTML` works for generation but doesn't communicate "the body is HTML; downstream code parses it" to other tools (verify, scorecard). A `response_format: html` field (mentioned in the skill but not in spec-format.md) would make this explicit and let the generator emit the right scaffolding.

## Verdict: PASS — proceed to promote and archive
