# Polish — pushpress-pp-cli

| Metric | Before | After | Delta |
|---|---|---|---|
| Scorecard | 89/100 | 89/100 | 0 |
| Verify | 100% | 100% | 0 |
| Dogfood | PASS | PASS | — |
| Tools-audit | 0 pending | 0 pending | — |
| **PII-audit** | **3 raw** | **0** | **−3** |
| Output-review | 1 WARN | 0 | −1 |
| Go vet | 0 | 0 | — |

## Fixes
1. **PII catch.** A real customer email had leaked into the `research.json` example field. Replaced with the RFC-2606-reserved `<redacted-placeholder>`. Propagated to README, SKILL, and the `member` Cobra `Example:` field.
2. **Name-as-dict rendering bug fixed** in 4 commands (going-dark, roster, recency, member). The real /v3 Customer.name is `{first, last, nickname}` not a string; added `nameDisplay()` + `jsonOrString()` helpers in `internal/cli/transcendence.go`. Now renders "First Last" in human output and a real nested object in JSON.
3. **Example: fields added** to `classes roster` and `notes list` stub subcommands.
4. **3 user@example.com finds** accepted as `synthetic_placeholder` (RFC 2606 reserved TLD) in `.printing-press-pii-polish.json`.

## Skipped (structural)
- `mcp_token_efficiency: 0/10` — runtime cobratree walker inflates every tool's inputSchema with global flags (~587 tokens/tool). Generator-level, not per-CLI fixable.
- `insight: 4/10` — file-count heuristic; novel features all in `transcendence.go` by design.
- `mcp_quality: 8/10` — DO-NOT-EDIT `internal/mcp/tools.go` lacks hint strings; generator concern.

## Ship recommendation
`ship`. Further polish not recommended — remaining scorecard gap is structural.
