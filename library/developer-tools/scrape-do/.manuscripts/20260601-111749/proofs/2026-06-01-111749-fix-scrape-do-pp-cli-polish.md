# Scrape.do CLI — Phase 5.5 Polish

Mid-pipeline polish (forked). publish-validate skipped (mid-pipeline; main SKILL owns Phase 6).

## Delta
```
                    Before    After
  Scorecard:        83/100    83/100
  Verify:           100%      100%
  Verify-skill:     0         0  (clean)
  Tools-audit:      1         0  pending
  PII-audit:        0         0  (clean)
  Go vet:           0         0  (clean)
  Output-review:    PASS      PASS
```

## Fixes applied
- Rewrote `version` Short ("Print version" → "Print the scrape-do-pp-cli version string") to clear the tools-audit `thin-short` finding.
- Added `mcp:read-only: true` to 5 pure-read leaf commands the audit heuristic missed: `which`, `feedback list`, `profile use`, `profile list`, `profile show` (verified read-only at runtime via agent-context).

## Skipped findings (structural / by-design — not defects)
- `mcp_token_efficiency 4/10` + "1 tool": static manifest reflects the lone `/info` spec endpoint; the **runtime** MCP surface mirrors the full Cobra tree (23 tools; dogfood MCP Surface PASS). endpoint_tools/orchestration extensions would be no-op scaffolding with one typed endpoint.
- `path_validity 2/10`: single-path spec (dogfood SKIPs path validity for internal-yaml).
- `cache_freshness 5/10`: intentionally disabled (paid quota-metered API).
- `breadth 7/10`: genuinely small API surface.
- dogfood WARN "cost looks reimplemented": `cost` is a by-design pure-local zero-credit estimator.
- dogfood WARN "defaultSyncResources empty": no bulk-list endpoint; `sync` emits a population-path hint.

## Verdict: ship (further_polish_recommended: no)
All hard gates pass; sub-max scorecard dims are structural to a single-typed-endpoint spec.

## Retro candidate
Generator emits pure-read leaf commands (`which`, `feedback list`, `profile use/list/show`) without `mcp:read-only` in DO-NOT-EDIT files; templates should annotate read-only leaves by default.
