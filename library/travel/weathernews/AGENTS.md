# Weathernews CLI agent guide

For focused weather/seasonal evidence behavior and recipes, read SKILL.md. Read README.md for source semantics and runtime bounds. Read evidence/final-report.md before claiming live coverage or readiness.

Hand-authored provider logic lives in internal/evidence/; command wiring lives in internal/cli/travel_evidence.go. Preserve the generated framework unless a source correctness fix requires a narrow documented edit. Public domain commands use fixed first-party origins, bounded requests and explicit missing values. Add tests for consequential parsing/date/state/criteria changes, then run the relevant live read-only source check.

Runtime state during development and verification belongs under evidence/runtime-home via --home; Go cache belongs under .cache/go-build. Keep shared/global config unchanged. Keep secrets and raw personal report streams out of evidence. Exactly one independent reviewer is required by this build's user authorization; reuse that reviewer for verification, with no edits delegated.
