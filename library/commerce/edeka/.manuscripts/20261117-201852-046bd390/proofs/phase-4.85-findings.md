# Phase 4.85 output review (2026-11-17)

status: WARN (2 warnings, nothing blocking). Samples: slots earliest, deals check, price history, compare markets (basket add, order place, list add skipped as mutating).

1. deals check "butter" listed peanut-butter sweets first (no relevance ranking). FIXED: offers per term are now sorted by edeka.Relevance (title words + category path words, non-grocery penalty), shop order breaks ties; "Landliebe Butter" now leads. Sweets that contain the word remain listed after it (they are real offers matching the word).
2. price history repeated one price per source. FIXED: one point per article, market and day (lowest price, offer flag, sources joined).

Checked OK: slots earliest timestamp/fee, compare markets keeps failing markets with an error field, deals check URLs and encoding.
