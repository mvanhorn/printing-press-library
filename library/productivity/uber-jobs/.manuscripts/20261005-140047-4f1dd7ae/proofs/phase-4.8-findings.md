# Phase 4.8 agentic SKILL review (2026-10-05)

Reviewer: one Opus general-purpose agent; it ran --help only, made no edits, sent no traffic. Result: 15 findings (6 errors, 9 warnings); check 2 (verified-set alignment) passed.

## Fixed (14)
- Errors: 4a fallback limits stated where the fallback is described (value_prop); 4b screen under the Oracle fallback (all unscreened, empty default keep; check meta.source); 4c stats churn columns null until 30 days of history, always null live; 4e sync added to the Command Reference and a "Build the local history" recipe; 4g exit-code table: 6 added, 7 = refused and never retried; 6b teach example gains --resource-type.
- Warnings: 1a trigger phrase "find uber jobs in london" -> "find uber jobs in the uk"; 1b "what uber postings are new this week" -> "what uber jobs were posted this week", plus a postings --posted-within 7d recipe; 1c and 5a Discovery Signals (Google Maps HAR noise, bogus api_key signal) removed from SKILL and README; 3a the "read-only" claim now says "never writes to Uber; sync, save, searches --delete and new write only the local store"; 4d new needs save first, the first run takes the baseline, keyword searches are skipped under the fallback; 4f screen and stats read the last local sync whatever its age (meta.note gives the date); 5b the credentials and auth wording is removed from Paths and state; 6a screen examples use --verdict all and say the match is literal.
- Source of truth: research.json (narrative.trigger_phrases, value_prop, when_to_use, recipes, troubleshoots; novel_features[].why_it_matters and the screen example), re-synced by dogfood. Template sections were edited in place. Backup: research.json.bak-pre-phase48.
- Also fixed (README): a generic "Not found" tip pointed at a nonexistent list command.
- Checks after the fixes: verify-skill all PASS; validate-narrative --strict --full-examples OK (12 commands); dogfood exit 0 (WARN: dead generated helpers only); go test ./... 14/14 ok.

## Accepted warning (owner decision, 2026-10-05: "Proceed + retro")
- 7a "These capabilities aren't available in any other tool for this API" is hardcoded in the press (internal/pipeline/docsync.go:832) and rewritten on every dogfood run; it is untrue for postings, get, facets, save and searches (absorbed from headstart, fetchaller-mcp, openings-mcp and amazon-jobs). RETRO CANDIDATE: the template claims uniqueness for every novel_features entry, including absorbed features declared there so they register as hand-written leaves.
