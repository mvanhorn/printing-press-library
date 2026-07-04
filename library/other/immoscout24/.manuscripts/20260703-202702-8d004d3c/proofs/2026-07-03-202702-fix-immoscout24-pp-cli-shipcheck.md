# ImmoScout24 CLI Shipcheck

Run: `20260703-202702-8d004d3c`
Updated: `2026-07-03T18:49:29Z`

## Result
- Verdict: `PASS`
- Legs: 7/7 passed
- Scorecard: 85/100, Grade A
- Live sample probe: 4/4 passed

## Leg Summary
- `verify`: PASS, 100% pass rate, 0 critical failures.
- `validate-narrative`: PASS.
- `dogfood`: PASS, path validity 3/3, dead flags 0, dead functions 0, novel features 4/4 survived.
- `workflow-verify`: PASS.
- `apify-audit`: PASS, no actor references.
- `verify-skill`: PASS.
- `scorecard`: PASS.

## Notes
- The generator renamed resource `search` to `immoscout24-mobile-search` to avoid shadowing the framework `search` command.
- Remaining scorecard gap: MCP token efficiency is low for this small endpoint mirror, but MCP readiness is `full` and all public tools are available.

Final recommendation: `ship`.
