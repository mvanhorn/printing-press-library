# Is Agentic Shipcheck

## Results
- Final verdict: `ship`
- Shipcheck: 7/7 legs passed with live verification trigger enabled.
- Verify: live mode, 31/31 checks passed.
- Full live dogfood: 86/86 tests passed; no hollow novel features.
- Narrative validation: 6/6 examples passed.
- Verify-skill: all checks passed.
- Scorecard: 95/100, Grade A.
- Live API sample: 7/7 novel-feature samples passed.

## Fixes applied
- Replaced the generated report response-path behavior with durable hooks so both v1 and legacy report commands return the complete report object, not only their nested issues arrays.
- Corrected nullable report scores and bonus breakdown fields.
- Made retained report snapshots searchable through the local search command, restored full data-pipeline integrity, and removed the final dead helpers.
- Added SQLite snapshot provenance and safe serialized portfolio writes.
- Added first-run diff behavior with an explicit provisional-baseline note.
- Added a missing framework feedback help example for the live matrix.
- Implemented seven approved novel features: history, diff, check, portfolio, issue lifecycle, rate-aware fleet refresh, and evidence.

## Known warnings
- The generated `sync` command has no default bulk resource because the API is a single-target report lookup; report and portfolio commands populate the local mirror instead.
- Two generator helper functions remain reported as dead by the structural audit.
- `agent-browser` is installed as a package; its optional browser cache setup remains unnecessary for this spec-resolved run because Chrome DevTools MCP supplied the discovery evidence.
