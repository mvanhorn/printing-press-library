# Durianpay CLI — Polish Proof (Phase 5.5)

Mid-pipeline polish, forked context, working copy canonical (not yet promoted).

| | Before | After |
|---|---|---|
| Scorecard | 92/100 (A) | 92/100 (A) |
| Verify | 100% | 100% |
| Go vet | 0 | 0 |
| Gosec (hand-authored) | 8 | 0 |
| Tools-audit | 1 pending | 0 |
| PII-audit | 0 | 0 |
| Verify-skill | 0 | 0 |

Fixes: gosec G304 x5 (narrow #nosec on deliberate user-named file reads), G117 (token-cache marshal, 0600/0700), G306 (public-key write 0644→0600), G104 (unchecked Body.Close), tools-audit `version` thin-short accepted.

Skipped (classified): dogfood WARN on explain/sandbox-simulate (intentional offline `pp:data-source local`), 32 gosec findings in generated files (retro), scorecard insight/mcp_tool_design/cache_freshness/mcp_token_efficiency (spec/structural, need regen).

ship_recommendation: ship. further_polish_recommended: no.

Retro candidates: (1) generated store.go json_extract via fmt.Sprintf (G201) + cobratree shellout G204; (2) generator Cobra Short fallback titlecases operationId ("Patch id" for customers) when spec summary empty.
