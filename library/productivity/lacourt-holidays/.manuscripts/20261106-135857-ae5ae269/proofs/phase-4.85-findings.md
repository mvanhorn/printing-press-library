# Phase 4.85 output review: WARN (2 warnings)
1. --agent compact stripped holiday `description` in nested lists -> fixed (printJSONFilteredKeep "description"/"reason").
2. `holidays --year` prints upstream MM/DD/YYYY while novel commands use ISO -> left as-is: generated endpoint mirrors the court's raw payload; novel commands normalize. Noted for user.
Date math hand-verified by reviewer (incl. 2027-03-01..04-15 = 32 court days).
