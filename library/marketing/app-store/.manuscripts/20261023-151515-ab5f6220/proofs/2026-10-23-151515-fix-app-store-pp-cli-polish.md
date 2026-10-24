# Polish: app-store-pp-cli

Phase 5.5 (19-polish) ran the printing-press-polish skill in a forked context and finished on 2026-10-24 at 14:57 PKT. Its full response follows.

---

The polish recommendation is **ship**: every hard gate passes and nothing is left that another polish pass could close. A final `shipcheck --no-fix` passes 7/7 and re-saved verify and scorecard to `.printing-press.json` (still 100% and 80/100, A).

**How this run went**
- **Build lock:** the parent held it with phase `polish`, as Phase 5.5 is designed to. Polish worked under it, kept its heartbeat fresh, and did not release it.
- **Divergence check:** the public library is cloned locally (and upstream `mvanhorn/printing-press-library` was checked through the GitHub API), but no app-store CLI is published there. This is a first print, so the internal copy is canonical.
- **Mac slept twice:** the lid was closed on battery (53%, then 52%), from 11:01 to 12:58 and again from about 13:05 to 14:21. The first output-review attempt failed with a DNS error on wake; the retry worked.
- **Apple traffic:** the only requests to Apple were the two governed verify runs (the baseline and the one inside shipcheck). Polish could not see whether either was throttled, because the press deletes the temp home holding the pacing state; both passed 51/51. The output reviewer and all polish checks used only local-reader commands on a scratch copy of the parent's store. The owner's real store at `~/.local/share/app-store-pp-cli` was not touched.

| Check | Before | After |
|---|---|---|
| Scorecard (with `--spec`) | 75/100 | 80/100 (dead_code 1/5 → 5/5) |
| Scorecard (without `--spec`) | 85 | 91 |
| Verify | 100% (51/51, 0 critical) | 100% |
| Dogfood | WARN (9 dead helpers) | PASS |
| Live matrix | exercised (5/5) | exercised (5/5) |
| gosec, hand-written code | 9 | 0 |
| gosec, generated files | 41 | 41 (left for the press) |
| `go vet` / tools-audit / PII | 0 / 0 / 0 | 0 / 0 / 0 |
| verify-skill / workflow-verify | pass / pass | pass / pass |
| Output review | 3 warnings | all 3 fixed |

The two scorecard rows differ only in how path_validity is counted. The internal-yaml spec makes dogfood skip that check, and the scorer counts the skip as 0/10 only when `--spec` is passed.

**What was fixed (5 commits as qazmataz: a8d8e09, 58ac164, 14dd9d0, 2a279db, ace7058)**
1. **Security scan:** three store queries (Captures, Reviews, SearchReviews) now pass LIMIT as a bound parameter. The other 8 findings were safe as written, so each got a narrow suppression with its reason: 6 SQL sites that only join fixed text, the `--db` write-permission probe, and the pacing jitter. A new test covers the bound LIMIT.
2. **Dead code:** removed the 9 unused generated helpers from `internal/cli/helpers.go`, with a patch record in `.printing-press-patches/`.
3. **SKILL.md regression:** every dogfood run was deleting the doctor line, the pacing paragraph and "Other commands read one storefront per run" from SKILL's Auth Setup. Those were hand edits missing from the text it is rebuilt from. They were moved into `research.json` (`narrative.auth_narrative`) so they stick. "The local history commands also take --countries" was replaced with the exact ten commands that accept it; movers, rank-history and keyword-share don't. README's Authentication section carries the same text.
4. **Output review fixes**, with tests:
   - traction and impact say which requested apps and storefronts have no stored data; traction's table used to drop them silently.
   - traction and breakouts capture hints name the requested storefronts instead of a fixed us,gb or gb.
   - changes no longer says "nothing can have changed yet". It counts only captured apps, names never-captured ones, and lists releases from stored version history that fall in the window.
5. **MCP tool text:** the MCP server shows each command's short description to agents. 16 of them were noun phrases that didn't say whether the command calls Apple live or reads stored data. Each now starts with a verb and says which, within the existing 90-character limit.

**Left alone on purpose**
- **Live commands stay marked read-only for MCP**, even though they save snapshot rows to the CLI's own store. A build-checklist test requires it, the alternative label would wrongly say they don't reach outside, and removing it would make hosts prompt on every chart lookup.
- **No MCP description sync:** improving the two typed `categories` tool descriptions needs `mcp-sync`. That would regenerate `tools-manifest.json` and undo the shipcheck's URL fix, breaking a critical verify check.
- **Scorecard gaps that can't be closed honestly:** error_handling, vision, cache_freshness and mcp_quality lose points for features a read-only App Store client doesn't have (409 handling, sync/tail/import, the generated freshness helper, a sync tool). Adding them would only game the score.
- **Known product gap:** keyword-share still leaves competitor titles empty (deferred in Phase 4.85; it needs a schema change), and `keyword-history --serp` shows bare ids for the same reason. A few minor display issues the reviewer didn't file are also untouched.
- **Simplify pass:** the code-review phase deferred `/simplify` to this phase; it was not run.

**Worth filing against the press**
- 41 gosec findings in generator templates.
- path_validity scored 0 when the check is skipped (a 10-point swing).
- Dead helpers emitted for CLIs that have no generated endpoint commands.
- The generated `sql` MCP tool says "Requires sync first", but this CLI has no sync and keeps its data in domain tables.
- `mcp-sync` would undo the hand-patched URLs in `tools-manifest.json`.
- `scorecard --live-check` copies the operator's real store into samples unless `APP_STORE_HOME` is set.
- Dogfood's doc re-sync silently overwrites hand edits.
- Parent and framework commands are generated without examples.

---POLISH-RESULT---
scorecard_before: 75
scorecard_after: 80
verify_before: 100
verify_after: 100
dogfood_before: WARN
dogfood_after: PASS
dogfood_live_matrix_before: exercised
dogfood_live_matrix_after: exercised
govet_before: 0
govet_after: 0
gosec_before: 9
gosec_after: 0
tools_audit_before: 0 pending
tools_audit_after: 0 pending
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
fixes_applied:
- gosec: LIMIT bound as a query parameter in Captures, Reviews and SearchReviews; narrow #nosec with reasons on 6 constant-fragment G202 sites, the --db write-permission probe (G304) and pacing jitter (G404); added TestReviewQueriesBindTheLimit
- Removed 9 unused generated helpers from internal/cli/helpers.go (dogfood dead functions 9 -> 0, dead_code 1/5 -> 5/5); patch record .printing-press-patches/polish-dead-generated-helpers.json
- SKILL Auth Setup kept its doctor line, pacing paragraph and one-storefront sentence by moving them into research.json narrative.auth_narrative (dogfood re-sync had been deleting them); the --countries claim now lists the exact ten commands; README Authentication matches
- Output review: traction and impact name requested apps/storefronts with no stored data (traction's table footer too); traction and breakouts capture hints use the requested storefronts; changes counts only captured apps, names never-captured ones and lists in-window releases from stored version history; tests in scope_notes_test.go
- 16 MCP tool descriptions (Cobra Shorts) made verb-led and labelled live vs stored, within the s2a 90-character contract
- Rebuilt staged binaries; shipcheck --no-fix PASS 7/7; verify and scorecard re-persisted to .printing-press.json (100%, 80/100 A)
skipped_findings:
- 41 gosec findings in generator-emitted files (G304x13, G115x9, G201x4, G302x4, G104x3, G202x3, G101x2, G117, G119, G703): press retro candidates, generated files not hand-edited
- Live App Store commands keep mcp:read-only despite writing snapshot rows to the CLI's own store: the s2a build-checklist test requires it, mcp:local-write would set openWorldHint=false on commands that call Apple, and dropping it would label every lookup destructive
- path_validity 0/10: dogfood SKIP for the internal-yaml spec is scored 0 when --spec is passed (omitted without --spec: 91/100); press scoring issue
- error_handling 8, vision 8, cache_freshness 3, mcp_quality 7: structural (no 409 idempotency on a read-only API, no sync/tail/import, the client keeps its own 15-minute cache, no sync tool)
- Typed categories_list/categories_games descriptions not overridden: mcp-descriptions.json needs mcp-sync, which would revert the tools-manifest.json effective-URL patch and re-break verify's critical resource-path check
- Framework sql MCP tool description ("Requires sync first", synced resources) is hard-coded in the generator template and can't be overridden; press retro
- 12 parent/framework commands lack Examples (categories, completion x5, help, learnings, playbook, profile, version, workflow): generator-owned
- keyword-share empty competitor titles (Phase 4.85 #1) and keyword-history --serp bare ids: deferred, needs titles stored with each keyword capture (schema change)
- Minor unfiled display items: unlabeled last column in impact/keyword-share tables; stale hint on most history commands (generated 30-minute --max-age default); keyword-share stale hint shows a <term> placeholder
- Phase 4.95 deferred /simplify to polish; not run, the code passed three review rounds
remaining_issues:
ship_recommendation: ship
further_polish_recommended: no
further_polish_reasoning: Everything left is press-owned (generator gosec findings, path_validity skip scoring, the sql tool text, the tools-manifest URL inconsistency) or a deliberate product deferral (keyword-share titles need a schema change), and another polish pass cannot close any of them.
---END-POLISH-RESULT---
