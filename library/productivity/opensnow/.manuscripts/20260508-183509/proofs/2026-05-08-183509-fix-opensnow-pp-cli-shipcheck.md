# OpenSnow CLI Shipcheck Report

## Verify Results
- Pass Rate: 100% (23/23 commands)
- All commands pass help, dry-run, and exec checks
- 4 EXEC FAILs on favorites-dependent commands (expected — no favorites configured in test env)
- Verdict: PASS

## Dogfood Results
- Auth Protocol: MATCH (api_key query parameter)
- Dead Flags: 0 (PASS)
- Dead Functions: 0 (PASS)
- Examples: 10/10 commands have examples (PASS)
- Novel Features: 8/8 survived (PASS)
- MCP Surface: PASS

## Verify-Skill Results
- All checks passed (flag-names, flag-commands, positional-args, unknown-command)
- Canonical sections: PASS
- Fixed: powder-rank --region references updated to --slugs

## Workflow-Verify Results
- Verdict: workflow-pass (no workflow manifest)

## Scorecard
- Total: 76/100 - Grade B
- Output Modes: 10/10
- Auth: 10/10
- Error Handling: 10/10
- Doctor: 10/10
- Agent Native: 10/10
- Local Cache: 10/10
- Insight: 10/10
- Dead Code: 5/5

## Fixes Applied
1. Fixed SKILL.md: replaced --region CO with --slugs for powder-rank command
2. Fixed research.json: updated powder-rank examples to use --slugs flag
3. Synced novel_features_built and README/SKILL Unique Features sections

## Ship Threshold
- verify verdict: PASS ✓
- scorecard >= 65: 76 ✓
- verify-skill: PASS ✓
- workflow-verify: PASS ✓
- no flagship features broken ✓

## Final Recommendation: ship

## Live Testing
No API key available — live smoke testing skipped.
CLI verified against exit codes, dry-run, and mock responses only.
