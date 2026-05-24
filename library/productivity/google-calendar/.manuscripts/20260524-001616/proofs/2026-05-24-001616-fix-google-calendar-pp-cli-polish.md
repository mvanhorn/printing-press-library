# Polish (Phase 5.5)

Verdict: **ship** (further_polish_recommended: no)

| Metric | Before | After |
|--------|--------|-------|
| Scorecard | 90/100 (A) | 90/100 (A) |
| Verify | 100% | 100% |
| Dogfood | PASS | PASS |
| go vet | 0 | 0 |
| tools-audit pending | 5 | 0 |
| PII-audit pending | 13 | 0 |

Fixes:
- Real bug: `changes` examples used `--calendar` (singular) but the flag is `--calendars`; fixed in research.json + rendered README/SKILL.
- Replaced synthetic emails in test fixtures with non-email calendar identifiers (8 PII matches removed at source).
- Accepted @example.com doc placeholders + 5 generated grouper "Manage X" Shorts with per-finding evidence; both audit ledgers clear.

Skipped (out of scope / environmental):
- verify execute-leg score 2 on book/profile/workflow: no live OAuth credential in sandbox; help+dry-run pass, verdict PASS, pass_rate 100, 0 critical.
- scorecard mcp_token_efficiency 4/10 + insight 4/10: structural for a 37-endpoint read/write surface; not closable without spec-level MCP orchestration changes beyond polish scope.

Retro candidate: generator emits thin-short "Manage X" Shorts on help-shim grouper commands (parentNoSubcommandRunE) — should exempt or enrich.
