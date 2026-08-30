# AgentMail Phase 4.95 Review

## Review path chosen
Direct subagent dispatch: correctness, data-integrity, and security reviewers against the generated CLI/MCP source and six hand-authored workflow commands.

## Autofix summary
7 workflow correctness findings fixed in-place across the implementation pass: AgentMail message direction inference, display-name mailbox normalization, composite resource IDs, draft-to-message thread linkage, plural attachment linkage, context-aware SQLite reads, malformed JSON/timestamp handling, outbound counts, schedule deduplication, local data-source validation, extracted content, and order-independent triage reconciliation. MCP HTTP startup now rejects unauthenticated non-loopback binds. API-key mutation responses are no longer persisted to the local mirror. Focused workflow tests and builds pass.

## Template-shape retro candidates
These remain generator/template-owned and were not broadly rewritten in the printed CLI:

- `internal/mcp/code_orch.go:60-65` — high — the generated `agentmail_execute` MCP tool dispatches mutating methods without method-specific read-only/destructive annotations. Generator should emit approval metadata or split read/write tools.
- Generated API-key and signup command output paths — high — upstream create responses legitimately return newly issued secrets, but the generic generated surfaces do not consistently redact secret-shaped fields. The current narrative discloses this behavior; generator should provide explicit secret-output policy and tests.
- Generated client response-error masking — high — per-request Authorization overrides are not consistently included in redaction. Generator should centralize header masking.
- Generated `cmd/<cli>-pp-mcp/main.go` HTTP transport — high — the printed CLI now rejects non-loopback binds locally; generator should make unauthenticated HTTP exposure impossible by default across all CLIs.
- Generated `deliver.go` fixed `.tmp` path — medium — generator should use exclusive temporary files to avoid symlink races.

## Surface-to-user findings
- The CLI must keep MCP HTTP on loopback unless an authenticated reverse proxy is deliberately placed in front of it. This is enforced by the printed binary.
- API-key creation and first-time signup return the newly created secret because AgentMail's contract exposes it once; the CLI does not persist those responses in SQLite. Configured credentials are not printed.

## Convergence outcome
Workflow findings cleared after the implementation and focused verification pass; template-shape security candidates remain for the Printing Press generator.
