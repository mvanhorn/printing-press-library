# Surfline CLI Polish

ship_recommendation: ship | further_polish_recommended: no
- Scorecard 90→90 (A), Verify 100→100, Gosec (hand-auth) 11→0, Output-review 1 WARN→fixed.
- Cleared 11 hand-authored gosec G104 (tw.Flush/db.Close error handling).
- buoy-check: added stale/OFFLINE buoy detection + STATUS column + staleness warning.
- Skipped (generator retro candidates, not hand-edited): replacePathParam dead helper in generated helpers.go; 27 gosec findings in generated files.
