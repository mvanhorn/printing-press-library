# readwise-pp-cli shipcheck (run 20261118-122243-3e032552)

Command: `cli-printing-press shipcheck --dir $CLI_WORK_DIR --spec research/readwise-spec.yaml --research-dir $API_RUN_DIR`. It ran in a sandbox HOME holding a working copy of the operator store snapshot (2026-08-29 data). The live store was not touched (hash b4a2e05947c270d2 throughout).

## Runs
| Run | Result | Blocker |
|---|---|---|
| 1 | FAIL 6/7, 94/A | verify-skill canonical-sections. SKILL.md was the library's stale copy (install section said Go 1.26.4). |
| 2 | PASS 7/7, 94/A, verify WARN 97% (68/70) | Took fresh README.md and SKILL.md. The library docs are kept at $API_RUN_DIR/library-docs/ to port prose in Phase 4.9. |
| 3 | PASS 7/7, 94/A, verify WARN 97% | Fixed the webhook dryRunOK branches (4 returned nil, so --json printed nothing). The two remaining verify fails were resource-path:export and import: the library's stale tools-manifest.json lacked the 4 new tools. |
| 4 (final) | **PASS 7/7, 94/A, verify PASS 100%** | Non-Go sweep: generator-owned files taken from fresh. Hand edits merged: the .gitignore union, the Makefile ldflags plus fresh -trimpath/BIN_EXT, and the polished tool descriptions overlaid (documents_delete, highlights_delete, review_get). |

Before/after verify pass rate: 97.1% → 100%. Scorecard: 94 → 94 (Grade A). The prior library's 4.24.0 scorecard was also 94.

Final legs: verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill and scorecard all PASS.

## Behavioural sample of every novel feature (store copy, sandbox)
| Command | Result |
|---|---|
| ask "habit" --format json | exit 0, 16 source-attributed blocks |
| backlog --budget 30m | exit 0, totals, by_location, by_age, abandoned, worklist |
| dedupe --by url,title --scope docs,books | exit 0, 618 clusters / 635 duplicates. A sampled cluster is a genuine duplicate: the same SO question saved with and without ?rq=1. |
| digest --since 2026-08-01 --group-by source | exit 0, grouped by source |
| triage stale --older-than 180d --never-opened --zero-highlights | exit 0, 3529 stale docs and 4 orphan sources. Plausible for a library built mostly from imports. The store copy has no Reader highlight/note documents. |
| read-next --have 15m --top 5 | exit 0, 5 real titles |
| connections --library-wide --by tags,author --min-overlap 2 --top 5 | exit 0, 5 pairs from 415 engaged docs |
| watch list / webhook list | exit 0, empty (no saved watches in this store) |

Outputs are saved under proofs/novel-samples/ (private: real titles).

## Ship recommendation: **ship**
All ship-threshold conditions are met. There are no known functional bugs in shipping-scope features. Live write-path behaviour is verified in Phase 5.
