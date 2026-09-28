# Walkerplus publication preparation

Status: preparation complete; publication requested and awaiting the root orchestrator's submission/maintainer review. This worker performed no GitHub action, installation, shared configuration change, or root-checkout source edit.

## Durable changes

- Explicit `mcp.transport: [stdio]` in canonical `spec.yaml`; current Printing Press 4.32.5 regenerated the stdio-only MCP main in fresh `.press/publish-prep/stdio-generated`. Copied only that generated main and removed its obsolete generated HTTP-auth tests. No HTTP listener/authentication/TLS hosting flags remain in main.
- Ran supported `mcp-sync` on a verified scratch copy. Other MCP code/metadata stayed byte-identical. The tool does not refresh raw `spec_checksum` on this spec edit, so only that manifest field was copied from the freshly generated manifest; original run, generator version, timestamps and attribution were retained.
- Recorded schema-2 bounded-workflow and stdio-generation customizations with existing file/marker guards. `base_run_id` and version come from the canonical manifest; `applied_at` is the actual current 2026-09-28 date.
- README/SKILL now use durable source-checkout/catalog-availability conditions; current publication status is recorded here rather than baked into user docs. MCP is stdio-only. VERIFICATION describes historical verification/preparation and links embedded research/proofs under `.manuscripts/<run>/`; original pipeline receipts remain local and are excluded from the package.
- Canonical AGENTS now permits user-authorized Printing Press publication while keeping shared global credentials/configuration outside scope.

## Preservation and checks

- 44 custom CLI/core/CLI-entry-point files are byte-identical to the pre-change snapshot. Root checkout was not edited. Verified scratch corrected an initial overly broad binary-name copy exclusion; that incomplete scratch was never applied.
- Focused canonical MCP tests PASS: `internal/mcp` 0.985s, `internal/mcp/bound` 1.303s, `internal/mcp/cobratree` 4.277s; stdio-only command package has no generated HTTP tests.
- Canonical MCP build PASS. `verify-skill --strict` PASS: 19 recipes, zero findings. Both schema-2 patch files, every guarded file/marker, exact raw spec checksum and absence of HTTP hosting symbols were checked.
- Original ignored `build/*.mcpb` artifact predates stdio narrowing; root was notified to rebuild/exclude it before packaging. No stale build artifact was copied from scratch.

Spec checksum: `sha256:c6481cd050a8699dc2f7fc86b6a3140b9ac8ffd54890d0719cd680c9db32ca9d`.

## Changed canonical paths

- `spec.yaml` — modified
- `README.md` — modified
- `VERIFICATION.md` — modified
- `SKILL.md` — modified
- `.printing-press.json` — modified
- `AGENTS.md` — modified
- `cmd/walkerplus-pp-mcp/http_auth_test.go` — removed
- `cmd/walkerplus-pp-mcp/main.go` — modified
- `.printing-press-patches/walkerplus-bounded-workflows.json` — new
- `.printing-press-patches/walkerplus-stdio-only-mcp.json` — new

## Evidence

- `.press/publish-prep/generation.json` and `stdio-generated/cmd/walkerplus-pp-mcp/main.go`
- `.press/publish-prep/mcp-sync-verified.log` and `sync-verified-changes.json`
- `.press/publish-prep/canonical-before.json` and `canonical-changes.json`
- `.press/publish-prep/mcp-tests.log`, `mcp-build.log`, `verify-skill.json`

Root owns remaining fresh live/publish-validation gates, managed clone, GitHub submission and release bookkeeping.
