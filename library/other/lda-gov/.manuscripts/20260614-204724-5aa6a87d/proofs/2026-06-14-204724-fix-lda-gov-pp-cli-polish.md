Mid-pipeline polish result: ship.

Fixes applied during polish:

- Added `graph export --limit` and edge de-duplication after live dogfood showed unbounded repeated edges.
- Made `graph export --format csv` force CSV output even under `--agent` and when the local mirror is missing.
- Added `mcp-descriptions.json` overrides for eight constants endpoint MCP tools.
- Ran `mcp-sync` to apply MCP description overrides to the runtime MCP surface.
- Corrected `reports quarter` examples in `research.json`, README, SKILL, and run artifacts from positional args to `--year` / `--period` flags.

Diagnostics after polish:

- `go test ./...`: PASS
- `go build -o ./lda-gov-pp-cli ./cmd/lda-gov-pp-cli`: PASS
- `go build -o ./lda-gov-pp-mcp ./cmd/lda-gov-pp-mcp`: PASS
- `go vet ./...`: PASS
- `tools-audit`: PASS, no findings
- `pii-audit`: PASS, no findings
- `shipcheck`: PASS, 6/6 legs
- scorecard: 95/100, Grade A
- output review: SKIP, pass samples were empty local-mirror outputs; no findings

Security scan note:

- `gosec` reported findings in generated framework files (`internal/client`, `internal/config`, `internal/store`, MCP shellout/cache/profile/import/feedback). No findings were in the hand-authored novel feature files. Treat these as Printing Press generator-retro candidates, not blockers for this generated CLI.
