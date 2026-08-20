# USGS Earthquakes — Phase 5.5 Polish Report

|                  | Before    | After     | Delta              |
|------------------|-----------|-----------|--------------------|
| Scorecard        | 81/100 A  | 84/100 A  | +3                 |
| Verify pass rate | 96.875%   | 100%      | +3.125%            |
| Dogfood verdict  | WARN      | PASS      | cleared 3 dead items |
| go vet           | 0         | 0         | 0                  |
| Tools-audit      | 0 pending | 0 pending | unchanged          |
| PII-audit        | 0 pending | 0 pending | unchanged          |

**Fixes applied by polish:**
- `feeds`: defer feed-name validation when `--dry-run` is set so verifiers can confirm dry-run support with synthetic args
- `root.go`: removed dead `allowPartialFailure` flag (declared, never read)
- `helpers.go`: removed dead `partialFailureErr`, `partialFailureReport`, `detectPartialFailure` (no call sites)
- `client.go`: HTML-unescape `APIError` body so echoed request URLs stop leaking `&amp;`

**Ship recommendation:** `ship`. `further_polish_recommended: no`.

**Skipped findings** (spec/generator-level, not addressable in a polish pass):
- `mcp_token_efficiency` 4/10, `mcp_tool_design` 5/10, `mcp_surface_strategy` (unscored) — spec-level enrichment; surface is only 7 typed endpoints (well below the ~50-tool collapse threshold) so the gain would be marginal
- `mcp_description_quality` (unscored) — would need a `mcp-descriptions.json` override
- `cache_freshness` 5/10 — generator-level (cache freshness helper not emitted)
- `type_fidelity` 3/10, `breadth` 7/10 — spec-shape dimensions; small synthetic spec for a public no-auth FDSN service
