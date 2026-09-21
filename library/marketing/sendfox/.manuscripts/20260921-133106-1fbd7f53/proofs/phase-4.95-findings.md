# Local code review
Review path: sequential main-thread correctness, security, maintainability, API-contract, reliability and data-integrity review under the user's Task mapping. Source scope: CLI/MCP/client/store, contract and evidence packages. Reviewed all own workflow handlers and shared safety boundaries; generated helpers checked at relevant call sites. No subagents or PR-based review used.

Autofix summary: local output-body retention, bounded evidence input, early row-close error handling, ambiguous CSV acknowledgements, source-suppression union and incomplete delta evidence fixed; see source, patch records and final-tests.log. Findings cleared on final rereview. No scope reduction or unresolved novel-workflow finding.

Timeout boundary: request-bearing helpers use the generated client timeout; bulk polling is bounded; local SQLite queries use boundCtx and read-only handles. Mutation approval is enforced in the common client, not only command wrappers; write retries are disabled. MCP execute defaults to preview and validates the public schema before execution.

Template follow-ups (not published): generated HTTP MCP cmd/sendfox-pp-mcp/main.go:86 lacks ReadHeaderTimeout (medium, prefer stdio). Generator template is internal/generator/templates/main_mcp.go.tmpl; retained per template escape hatch. Generated cleanup/diagnostic ignored-error sites are recorded individually in gosec-triage.json (low reliability). Two precise but short framework tool descriptions remain accepted in the tools ledger. PII scan has no findings. Reserved packages were not hand-patched.

Machine follow-ups: inline multiline endpoint descriptions broke generation; derived spec normalization preserves immutable source. Verification incorrectly counted SQL JSON metadata/stderr as table names; local verifier patch and regression tests fix this. Full copied-source upstream test/golden runs are not green: module archive omitted fixtures, generator suite timed out and Keychain tests failed/stalled. Targeted verifier tests and actual SendFox shipcheck pass; no upstream release readiness claim.

Workflow harness limitation: WorkflowStep.Args is parsed but ignored by executeStep in v4.32.4. Keep executable inline args; structural dogfood warns about unmapped flag/value tokens while workflow-verify executes all seven steps successfully. No tool verdict or result is edited.
