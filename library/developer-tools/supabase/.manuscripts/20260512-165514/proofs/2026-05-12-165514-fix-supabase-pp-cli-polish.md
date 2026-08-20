# Supabase CLI — Phase 5.5 Polish Result

## Delta

| Metric | Before | After | Δ |
|---|---|---|---|
| Scorecard | 93/100 | 94/100 | +1 |
| Verify | 100% | 100% | 0 |
| Dogfood | WARN | PASS | cleared 1 reimplementation flag |
| Go vet | 0 | 0 | 0 |
| Tools-audit pending | 0 | 0 | (7 surfaced and resolved via overrides) |
| Publish-validate | FAIL | PASS | cleared manifest + phase5 |

## Fixes applied

1. Added `fga_permissions` and `oauth2` security schemes to `spec.json` in the CLI dir (referenced 169 and 3 times respectively but undefined in `components.securitySchemes`).
2. Added `// pp:client-call` markers to `storage_top.go` on the two real Storage HTTP calls; cleared dogfood's lone reimplementation flag.
3. Ran `printing-press mcp-sync` to regenerate `tools-manifest.json` + `manifest.json` + `internal/mcp/tools.go`.
4. Copied `phase5-acceptance.json` from runstate proofs/ into `<cli-dir>/.manuscripts/<run-id>/proofs/` — the location `publish-validate` checks first.
5. Wrote `mcp-descriptions.json` with 7 agent-grade overrides for thin-description MCP tools (projects claim-token/restore/pause/etc.); re-ran mcp-sync to apply.
6. Accepted 3 pii-audit findings (all `<your-user-email>` synthetic placeholders per RFC 2606 in README/SKILL cookbook examples) with distinct `evidence_context` entries.

## Skipped findings (with reason)

- 4 scorecard live-check failures (auth-admin lookup, pgrst schema, storage usage, auth-admin recent) returning exit 10 "SUPABASE_URL not set" — environmental, scorecard's live-check sub-process doesn't inherit the user's env. The commands themselves work (proven in Phase 5).
- Type Fidelity 3/5, Cache Freshness 5/10, MCP Quality 8/10, Vision 8/10 — structural scorecard dimensions polish doesn't address; would require generator changes (Cache Freshness scaffolding is retro #1131).
- mcp_description_quality / mcp_token_efficiency reported N/A by scorecard (post-mcp-sync the dimensions could score, but scorecard ran before sync added the manifest).

## Ship verdict

```
ship_recommendation: ship
further_polish_recommended: no
further_polish_reasoning: All hard ship-gates pass cleanly; scorecard is 94/A,
  dogfood PASS, verify 100%, publish-validate PASS; remaining sub-max
  scorecard dimensions are structural and would require generator improvements
  or scaffolding to lift further.
remaining_issues: []
```

Polish converged without verdict downgrade. Main SKILL proceeds to Phase 5.6 promote with confidence.

## Retro candidates surfaced by polish

- **Two upstream-Supabase-spec defects had to be patched in the CLI's spec.json copy** (`fga_permissions` + `oauth2` referenced but undefined). The generator's spec ingestor accepts the spec; downstream scorecard rejects it. Possible fix: auto-define placeholder schemes for unresolved references at ingest time, or warn loudly at generate-time.
- **`storage_top.go` was flagged as reimplementation but `auth_admin.go` was not**, despite both using the same `ps.do(...)` helper for real HTTP calls. Possible dogfood heuristic inconsistency. The `// pp:client-call` marker is the canonical opt-out and is now applied.
- **`tools-manifest.json` and `phase5-acceptance.json` placement** in the working/ vs `.manuscripts/` paths: at the time Phase 5.5 polish runs, those artifacts should already be where publish-validate looks, but they aren't. Sequencing issue between Phase 5 marker write and Phase 5.5 polish's publish-validate (related to Datadog retro #1206).
