# Polish (Phase 5.5) — immoweb-pp-cli

Ship recommendation: ship (further polish not recommended).

| Metric | Before | After |
|---|---|---|
| Scorecard (mock-mode verify inside polish) | 87 | 87 |
| Verify | 100% | 100% |
| Live matrix | exercised | exercised (5/5 features) |
| Tools-audit | 4 pending | 0 pending |
| gosec (hand-authored) | 25 | 0 |
| PII (CLI dir) | 1 | 0 |
| Dogfood | WARN (description drift + 3 dead helpers) | WARN (3 generated dead helpers only) |

Fixes: gosec cleanup in immoweb_store.go/show.go/immoweb_common.go (explicit rollback/close, 0750 photo dir, filepath.Clean, one justified #nosec G202 on a constant-only query builder); root/agent-context/manifest/tools-manifest/goreleaser descriptions synced to the research.json headline; MCP command_mirror_capabilities resynced; Brussels commune label casing (Saint-Gilles) with unit test; sales-rep PII scrubbed from internal/immo/testdata/classified.json; show Short names its ID-or-URL input; 4 tools-audit findings accepted with rationale.

Skipped (generator-owned, retro): dead helpers in generated helpers.go/deliver.go; 32 gosec findings in generated files (incl. header-less platform/migration.go G202, cliutil/testenv/testenv.go G302); thin Shorts in DO-NOT-EDIT learnings/profile files; MCP token efficiency 4/10 is structural for a 6-endpoint API.

Follow-up done by the orchestrator after polish: the same sales-rep contact (emails, phone numbers, name) was redacted from discovery/*.har/html/json and proofs/*dogfood-results.json (44 replacements); HAR has no unredacted cookie/auth values.
