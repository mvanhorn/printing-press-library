# Polish (printing-press-polish, forked) - 2026-11-17

Scorecard 89 -> 92; verify 100% -> 100%; dogfood WARN -> PASS; gosec (hand-written) 15 -> 0; tools-audit 0 pending.

Fixes: typed exit codes on the products and payment parent groups; four dead generated helpers removed from helpers.go (patch edeka-helpers-dead-code); JSON writer prints & < > literally (test json_unescape_test.go); gosec fixes in hand-written code (filepath.Clean on user file reads, explicit close-error discards, narrow #nosec G124 notes for client-side cookies); stage binaries rebuilt.

Skipped: deals check loose matches (ranked by relevance, sweets that contain the word stay listed); path_validity 4/10 (query-string routes); cache_freshness 5/10 (no freshness helper emitted); 81 gosec findings in generator-emitted files (retro candidates); mutating samples need --allow-destructive (not used).

ship_recommendation: ship
