# Phase 4.85 output review — status: WARN (warnings only, Wave B non-blocking)
1. audit/coverage/opportunities live-check samples were empty (no site/credentials in the harness), so ranking/relevance was not assessable there. A direct audit run against example.com produced 27 issues, spot-checked correct against raw HTML (/all-moved-on/: no meta description, no JSON-LD, 67-char title).
2. coverage returned an empty success under live-check while a direct run without RANKMATH_SITE errors; audit reports meta.source "live" even when only reading the local store (--no-fetch). Suggestion: honest empty-state notes and data-origin-accurate meta.source.
Disposition: (2) addressed in Phase 4.95 where a small fix applies; ranking of coverage/opportunities remains unverified without credentials.
