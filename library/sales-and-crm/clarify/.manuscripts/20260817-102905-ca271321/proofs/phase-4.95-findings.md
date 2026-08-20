# Phase 4.8 / 4.9 / 4.95 findings — clarify

## Review path chosen
Direct subagent dispatch via the Agent tool: one general-purpose doc auditor (Phase 4.8+4.9 combined) and one general-purpose code reviewer over the hand-written files (Phase 4.95, correctness/security/maintainability lenses).

## Autofix summary
16 findings autofixed in-place across 1 round (working tree edits, no git repo in the run dir):
- Doc audit (6): non-existent `sql` command in anti-trigger; dossier `--select` example keys; CLARIFY_CAMPAIGNID env name (README x2); `auth-status` wording; sync-first disclosure on all 7 novel capabilities; velocity accrual caveat. Content fixes went to research.json and were re-rendered by regen; README-only items patched after regen.
- Code review (10): local-time parsing for zone-less date layouts; followup task signal now scoped to the meeting's linked deal/company; dossier meeting dedup via addMeeting closure; prep client/cache error surfacing; rune-safe transcript truncation; stale side-table error warnings; StageHistory unparseable-timestamp count surfaced; RecordStageObservations latest-map update inside loop; url.PathEscape on record/meeting IDs in API paths (prep, dossier); dupes merge-command sources built with json.Marshal.

## Template-shape retro candidates
1. README template renders path-param env vars without underscore normalization (`CLARIFY_CAMPAIGNID`) while internal/client/url.go declares `CLARIFY_CAMPAIGN_ID` — same derivation should feed both surfaces. Hand-fixed in README this run.
2. README agentcookie boilerplate references `auth-status` (hyphenated) where the command tree ships `auth status`. Hand-fixed this run.
3. Undocumented exit code 6 (partial failure) missing from README exit-code table (omission noted by auditor; low priority).

## Deliberately not fixed (accepted)
- Maintainability findings 11-12 (consolidating the guard→load→index preamble across four commands and the meeting-company fallback into shared helpers): behavior-preserving refactor deferred — the duplicated logic differs subtly per command (first-wins vs append-all) and consolidating day-boundary/relationship logic immediately before ship risks behavior drift; the behavioral test suite pins current semantics.

## Convergence outcome
All error-severity findings cleared in round 1; full build + test suite green after fixes.
