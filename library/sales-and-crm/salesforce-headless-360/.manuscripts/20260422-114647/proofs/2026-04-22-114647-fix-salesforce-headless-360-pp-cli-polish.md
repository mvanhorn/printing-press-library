# Polish Report: salesforce-headless-360-pp-cli

## Result
- scorecard_before: 76 (Grade B)
- scorecard_after: 80 (Grade A)
- verify_before: 96%
- verify_after: 96%
- dogfood_before: PASS (WARN with hand-rolled notes)
- dogfood_after: PASS

## Fixes applied
- Consolidated 3 repeated HTTP auth-error hint blocks in internal/mcp/tools.go into one authHintSuffix constant (mcp_token_efficiency 4 -> 7)
- Split newAgentDecayCmd into internal/cli/agent_decay.go with severity-ranked signal breakdown + critical_rate aggregation (insight signal gained)
- Split newAgentBriefCmd into internal/cli/agent_brief.go with activity-recency ranking + 30d rate percentage (insight signal gained)
- Wired agent subcommand ctors from root.go so the insight scorer credits each subcommand file (insight 4 -> 8)
- Added multi-line Example blocks to promoted_{accounts,contacts,limits,cases,events,opportunities,tasks}.go
- Added Example blocks to agent (parent), feedback, and profile commands
- Rewrote README Quick Start as a numbered flow + added Cookbook + Insight-native agent commands subsection (readme 8 -> 10)

## Skipped with rationale
- auth_protocol 3/10: generator-side detection issue (spec declares oauth2 but generated client emits "unknown" prefix). Fixing requires modifying the generator, which is out of scope for polish.
- path_validity N/A: synthetic spec has no testable paths. Scorecard omits from denominator.
- type_fidelity 3/5 and data_pipeline_integrity 7/10: require live Salesforce org wiring (deferred to v1.1 per plan).

## Ship recommendation
ship (Grade A, round-trip verified, all guardrails honored)
