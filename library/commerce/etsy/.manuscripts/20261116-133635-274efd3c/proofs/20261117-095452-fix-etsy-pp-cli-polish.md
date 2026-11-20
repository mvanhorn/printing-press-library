# Phase 5.5 polish — etsy-pp-cli

**Ship recommendation: ship.** Polish ran printing-press-polish in forked mode, mid-pipeline. Its fixes are commit 2474ba0, and its ledgers were committed afterwards.

```
---POLISH-RESULT---
scorecard_before: 96
scorecard_after: 96
verify_before: 100
verify_after: 100
dogfood_before: PASS
dogfood_after: PASS
dogfood_live_matrix_before: exercised
dogfood_live_matrix_after: exercised
govet_before: 0
govet_after: 0
gosec_before: 24
gosec_after: 0
tools_audit_before: 0 pending
tools_audit_after: 0 pending
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
fixes_applied:
- gosec G104: explicit discard of sql.Rows/DB Close errors on 13 hand-authored read paths
- gosec G101/G202/G304: narrow reasoned #nosec on 11 verified-safe hand-authored sites
- market shops: new_shop_definition in plain words
- market price: pluralized group notes; zero-review groups say "no listings have review data"
- added mkt_labels_test.go
- PII: accepted Etsy's published developer-support address (Etsy published support address) as api_provider_data
skipped_findings:
- 76 gosec findings in generator-owned files (G112 auth callback server, G119 redirect policy, G201/G202 store.go, ...): generator retro candidates
- JSON output HTML-escapes & and < (generated printJSON): retro candidate
- market price raw taxonomy IDs as group labels: feature add, deferred
- scorecard MCP Quality 8 / Cache Freshness 5 / Path Validity 9 / Agent Workflow 9: structural
- non-hermetic generated tests (ETSY_API_KEY in env; GNU chmod on PATH): retro candidate
remaining_issues:
ship_recommendation: ship
further_polish_recommended: no
---END-POLISH-RESULT---
```

**Orchestrator verification:**
- The polish diff (6e4aaea..2474ba0) touches 15 files, and none of them is generator-owned.
- The hermetic `go test ./...` passes 15/15.
- `TestEveryEtsyExtFileHasPatchRecord` and internal/mcpconformance pass.

**Deviation, recorded:** the polish diff was not put through another peer-review round. It holds lint discards, `#nosec` annotations with reasons, and output wording with a new test. The user chose the narrow final confirmation at round 9.
