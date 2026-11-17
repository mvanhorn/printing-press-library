# Polish (Phase 5.5), forked printing-press-polish run

Recommendation: ship. Docs-only changes; no .go file changed (pre/post hash snapshot identical), so the live acceptance marker (153/153) stays valid.

| | Before | After |
|---|---|---|
| Scorecard | 92/100 (A) | 92/100 |
| Verify (mock) | 100% (57/57) | 100% |
| Dogfood | WARN (sync is a no-op) | WARN |
| verify-skill / workflow-verify | 0 / pass | same |
| tools-audit / pii-audit | 0 / 0 | 0 / 0 |
| go vet | clean | clean |
| gosec (hand-written) | 2 (G304, false positives) | counted resolved, not silenced to keep the fingerprint |

Fixes applied:
- research.json narrative.troubleshoots: rate-limit entry states exit 7; new "sync does nothing and search costs credits" entry.
- README.md API-specific troubleshooting synced with both.

Skipped (with reasons): gosec G304 x2 in autom_common.go (ledger path is the CLI data dir or user-set AUTOM_LEDGER_PATH; #nosec would invalidate the marker); 61 gosec findings in generated files (generator retro); dogfood WARN / Data Pipeline PARTIAL / Cache Freshness 5/10 (every Autom endpoint needs a query); MCP Token Efficiency 7/10 (needs a spec mcp block and a regen); thin parent group Shorts (cosmetic .go edits); mock verify exec=false on bing/duckduckgo/yahoo/ground/web-search (no --query in the mock harness); live-check 401 (no key in the polish env; the parent's live run owns the gate); gofmt drift in internal/cli/which.go (written by dogfood's sync; not reformatted to keep the fingerprint).

ship_recommendation: ship
