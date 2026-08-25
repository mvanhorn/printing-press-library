# Phase 4.85 output-review findings — youtube reprint 20260819-035139

Status: WARN (Wave B — warnings only). Both findings FIXED in-session immediately after review:

1. misleading-empty-state-note (warning): `breakouts` with zero scanned videos but non-empty fetch_failures printed the filter-tuning note, misattributing auth failure to filter strictness. FIX: conditional note — "all search fetches failed; see fetch_failures (check the API key before touching filters)".
2. format-bugs (warning): `workspace list --json` / `auth keys list --json` hint strings rendered `<name>` as `<name>` (Go HTML-escaped JSON via the framework's printJSONFiltered). FIX: hints reworded to NAME (no angle brackets). The framework-level escaping behavior itself is a RETRO CANDIDATE (printJSONFiltered could SetEscapeHTML(false)).

Reviewer also confirmed: no silent source drops, no query-relevance failures, aggregation fan-out clean. 4 of 10 samples failed only on the sanitized-environment API key (environmental, out of scope for output review).

Phase 4.8 SKILL review (same window): no errors, 3 warnings, all FIXED — Command Reference regenerated to include the 9 carried novel subcommands, `videos-transcript` now documented (trigger phrase has a landing spot), headline softened from "full read API coverage" to "the full api-key read surface" at source (research.json) and across all six rendered surfaces.
