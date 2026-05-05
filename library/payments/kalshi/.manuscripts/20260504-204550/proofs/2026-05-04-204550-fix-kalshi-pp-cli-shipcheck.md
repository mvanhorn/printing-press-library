# Kalshi Phase 4 Shipcheck Proof

## Verdict
- **PASS** (5/5 legs)

## Per-leg results (pass 2; pass 1 had verify-skill failing on 13 SKILL→CLI flag mismatches)

| Leg | Result | Notes |
|---|---|---|
| dogfood | PASS | structural checks ok |
| verify | PASS | 92% pass rate WARN, 0 critical failures, 1 fix-loop iteration |
| workflow-verify | PASS | no manifest, skipped |
| verify-skill | PASS | first pass had 13 errors (--since/--window/--category/--warn-threshold referenced in SKILL but not declared); fixed by adding flags to portfolio attribution/winrate/exposure and markets movers/correlate |
| scorecard | 82/100 Grade A | |

## Scorecard (full)
- Output Modes 10, Auth 10, Error Handling 10, Terminal UX 9
- README 8, Doctor 10, Agent Native 10, MCP Quality 10
- MCP Desc Quality N/A, **MCP Token Efficiency 7, MCP Remote Transport 5, MCP Tool Design 5, MCP Surface Strategy 2** (machine-level gap; see below)
- Local Cache 10, Cache Freshness 5
- Breadth 10, Vision 9, Workflows 10, Insight 10, Agent Workflow 9
- Path Validity 10, **Auth Protocol 2** (false positive; see below), Data Pipeline Integrity 10, Sync Correctness 10
- Type Fidelity 4/5, Dead Code 5/5
- Total: 82/100

## Known weaknesses

### Auth Protocol 2/10 (false positive)
The OpenAPI spec declares three `securitySchemes` (kalshiAccessKey, kalshiAccessSignature, kalshiAccessTimestamp) but only the first appears in any operation's `security: []` block. The scorecard interprets this as "two declared securitySchemes are unused." The implementation IS correct: client.go's `signKalshiRequest()` emits all three KALSHI-ACCESS-* headers per request via RSA-PSS signing. Live verified: `portfolio get-balance` returns real account JSON.

Fix path requires generator-level support for "composed signature" auth — the generator's `composed` type targets cookie-template formats only. **Retro candidate.**

### MCP Surface Strategy 2/10, MCP Tool Design 5, MCP Remote Transport 5
Documented in memory (`feedback_pre_generation_mcp_enrichment`) and surfaced again in this run: OpenAPI-source specs cannot carry the `mcp:` enrichment block (transport/orchestration/endpoint_tools) — only the internal YAML format does. With 96 MCP tools generated, the surface defaults to endpoint-mirror+stdio which scores poorly on these architectural dimensions. Not fixable in polish (per memory feedback_polish_mcp_misclassify). **Retro candidate: parser support for `x-mcp-*` extensions or root-level `mcp:` YAML in OpenAPI.**

### Cache Freshness 5/10
Generator-default cache is 5 minutes. Most Kalshi reads are price-sensitive; consider --no-cache as default for orderbook/markets endpoints. Not blocking.

## Pass 2 fixes applied (between iterations)
1. Added `--since` to portfolio attribution and portfolio winrate
2. Added `--category` to portfolio winrate and markets movers
3. Added `--window` to markets movers and markets correlate
4. Added `--by` and `--warn-threshold` to portfolio exposure
5. Removed duplicate "Safe paper-trading session" recipe from research.json
6. Fixed bad recipe flag `--price 50` → `--yes-price 50 --action buy` (create-order schema)

## Ship recommendation
**ship** — meets ship threshold (shipcheck PASS, verify PASS, scorecard ≥ 65, no flagship feature broken in structural checks). Phase 5 live dogfood will validate behavioral correctness against real Kalshi.

## Generator retro candidates (for printing-press itself, not this CLI)
1. OpenAPI parser doesn't read `x-mcp-*` extensions — pre-gen MCP enrichment unreachable for OpenAPI inputs (96-tool Kalshi spec generated as endpoint-mirror)
2. Composed RSA-PSS signature auth requires hand-port; generator's `composed` type targets cookie templates only
3. Spec-title pollution in env-var names (`KALSHI_TRADE_MANUAL_KALSHI_ACCESS_KEY` from spec title "Kalshi Trade API Manual Endpoints")
4. Resource-name framework collisions prepend the spec-title prefix instead of using the path's last segment (`/search/filters_by_sport` → `kalshi-trade-manual-search` instead of `search-filters`)
5. Auth_protocol scorecard dim flags multi-securityScheme specs as "unused schemes" even when implementation correctly composes them
