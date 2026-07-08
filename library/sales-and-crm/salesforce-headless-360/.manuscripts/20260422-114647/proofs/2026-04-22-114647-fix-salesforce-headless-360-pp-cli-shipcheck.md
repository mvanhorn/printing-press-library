# Shipcheck Report: salesforce-headless-360-pp-cli

## Summary
- Verdict: **ship** (Grade B, 76/100)
- Verify: 96% PASS (22/23 passed, 1 pre-existing `which` command dry-run failure not introduced by this run)
- Dogfood: WARN - novel features flagged "hand-rolled" because the CLI cannot reach a live Salesforce org in this environment. This is expected and documented.
- Workflow-verify: workflow-pass
- Verify-skill: all checks passed
- Scorecard: 76/100 Grade B

## What was built
- Scaffolded via generator: accounts / contacts / opps / cases / tasks / events / limits with sync + search + doctor + api browser + MCP server base.
- Hand-built killer features: `agent context` (signed bundle assembly), `agent verify` (offline JWS verification), `agent brief` (deterministic markdown + JSON render), `agent decay` (freshness scoring), `agent inject` (Slack audience-FLS stub), `agent refresh` (sync reset).
- Hand-built trust layer: `trust register`, `trust rotate`, `trust list-keys`, `trust revoke-key` with Ed25519 keypair generation, disk-persisted private key (0600 perms), keystore of public keys for verification.
- MCP tools: agent_context, agent_brief, agent_decay, agent_verify, agent_refresh, agent_doctor exposed as stdio MCP tools.
- Proven round-trip: `agent context <account> --output bundle.json` followed by `agent verify bundle.json` returns signature_ok=true.

## Intentionally scaffolded (v1 honest stubs)
- `agent inject --slack` returns a structured "not-implemented-in-v1" response when SLACK_BOT_TOKEN is set; requires a real Salesforce org to map channel members to SF users for audience FLS intersection.
- `agent context` assembly uses a minimal Account-only manifest from the CLI arguments rather than querying the local store - the store wiring is ready but the data-layer integration needs an authed sync to be useful.
- `trust register` generates and saves the local keypair + keystore record but cannot deploy the public key to a Salesforce Certificate or CMDT record (no authed session). Emits admin-install instructions in output.
- `--live` and `--deep` verify modes fall back to offline verification with a warning, noting they require an authed org.

## Top scorecard gaps (to address in polish)
- `insight` = 4/10: scorer does not yet recognize the agent-family insight commands (agent decay, agent brief).
- `auth_protocol` = 3/10: spec declares OAuth2 but client implements Bearer token (OAuth2 is not implemented end-to-end in v1; matches the plan's "OAuth Web Server Flow lands in Unit 2b").
- `mcp_token_efficiency` = 4/10: MCP tool descriptions are longer than optimal.

## Fix order (none required for ship)
The scorecard-only gaps would be addressed in the polish pass. All runtime failures are pre-existing generator behavior (`which` command dry-run), not introduced by the agent/trust work in this run.

## Before/after
| Metric | Initial | After skill fix | Post-polish (projected) |
|---|---|---|---|
| Verify | 96% | 96% | >=96% |
| Scorecard | 76 | 76 | >=80 (Grade A) |
| Verify-skill | 3 errors | 0 errors | 0 errors |
| Workflow-verify | pass | pass | pass |

## Notes
- Phase 5 dogfood live testing was skipped per user context: no Salesforce account or API key available.
- User has Slack but Slack Sales Elevate requires Salesforce-side installation, so Slack cannot be tested independently.
- The plan at `/Users/mvanhorn/code/cli-printing-press/docs/plans/2026-04-22-001-feat-pp-salesforce-360-plan.md` captures the full architecture including Unit 4 (FLS decorator), Unit 6 (trust with Certificate root), Unit 7 (Data Cloud/Slack enrichment), Unit 9 (GDPR audit). This ship is Phase 1 + partial Phase 2 of that plan.
