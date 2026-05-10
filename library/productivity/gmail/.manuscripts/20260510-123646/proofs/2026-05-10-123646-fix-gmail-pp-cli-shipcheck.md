# Gmail CLI Shipcheck Report

## Run Info
- CLI: gmail-pp-cli
- Date: 2026-05-10
- Spec: research/gmail-openapi.yaml (49 paths, official Google OpenAPI)

## Shipcheck Results (Pass 2)

| Leg              | Result | Notes |
|------------------|--------|-------|
| dogfood          | PASS   | 8/8 novel features survived, 0 dead flags/functions |
| verify           | PASS   | 29/29 commands 3/3 each (help, dry-run, exec) |
| workflow-verify  | PASS   | No workflow manifest; pass |
| verify-skill     | PASS   | All flag/command checks passed |
| validate-narrative | PASS | 10/10 narrative commands resolved + full examples |
| scorecard        | PASS   | 85/100 Grade A |

## Scorecard Details
- Total: 85/100 Grade A
- Sample Output Probe: 8/8 (100%)
- All novel features built and verified

## Fixes Applied (Pass 1 → Pass 2)
- `auth login --dry-run` now short-circuits before checking `--client-id`
  (fixes validate-narrative --full-examples failure on the quickstart command)

## Known Gaps (non-blocking)
- `mcp_token_efficiency` 4/10 — 79 endpoint tools; Cloudflare pattern would improve
  this but requires spec enrichment + regen. Not blocking ship.
- `mcp_remote_transport` 5/10, `mcp_tool_design` 5/10 — similarly addressable in polish
- `type_fidelity` 3/5 — some userId params default to UUID placeholder in examples

## Final Verdict: SHIP
All 6 shipcheck legs pass. 85/100 Grade A. 8/8 novel features built and verified.
No known functional bugs in shipping-scope features.
