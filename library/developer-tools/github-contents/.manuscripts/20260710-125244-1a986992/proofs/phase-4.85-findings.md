# Phase 4.85 output review findings (Wave B: warnings only)

status: WARN (1 finding)

1. format-bugs (warning): `stats <target> --agent --select folders` (the CLI's own documented example) emitted by_folder/by_extension/largest as arrays of empty {} objects. Two layers:
   - OUR fix (applied): example used selector `folders`; real JSON key is `by_folder`. Fixed in research.json (both novel_features and novel_features_built); re-synced to README/SKILL via dogfood.
   - GENERATOR retro candidate (not patched in place): framework `--select` silently strips all fields from nested array objects on unknown selector names instead of erroring with valid keys. Lives in generator-emitted output plumbing (internal/cli/helpers.go filterFields) — machine bug class, route to /printing-press-retro.

Other checks: semantic-query-match PASS, aggregation-coverage N/A, ranking-plausibility PASS.
