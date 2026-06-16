# Claude Agent SDK Python Docs CLI Shipcheck

## Results
- `shipcheck`: PASS, 6/6 legs passed.
- `verify`: PASS, 25/25 passed, 0 critical.
- `validate-narrative`: PASS, 9 narrative commands resolved and full examples passed.
- `dogfood`: PASS with warnings for generated dead helper functions and docs-live novel commands that intentionally use the docs Markdown corpus rather than generated endpoint wrappers.
- `workflow-verify`: workflow-pass.
- `verify-skill`: PASS.
- `scorecard`: 84/100, Grade A.
- Live sample probe: 7/7 passed.

## Fixes applied after review
- Bounded docs response reads at 10 MiB.
- Bounded verifier file reads at 5 MiB and capped directory walks by depth and file count.
- Switched docs corpus loading to concurrent fetches with transient 5xx/429 retry.
- Corrected README/SKILL overclaims about offline search, sync snapshots, cached graph validation, and output envelopes.
- Improved `recipe` example ranking and `map --kind options` output.

## Final ship recommendation
- ship
