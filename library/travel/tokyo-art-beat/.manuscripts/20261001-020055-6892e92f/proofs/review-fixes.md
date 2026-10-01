Independent review fixes

- Apply nearby artist predicate locally.
- Preserve legacy edition year separately from schedule start year.
- Normalize invalid dates to null and keep source text.
- Namespace cache ownership; retain unrelated JSON/hash files during eviction.
- Emit structured/actionable pre-RunE parse errors.
- Preserve all-failed compare inputs/reasons and useful exit codes.
- Expose bounded domain CLI through MCP; exclude raw transport/unused mirror tools.
- Project --select before encoding; definite misses emit a structured error.

Regression tests, final live domain assertions and source-bound same-instance review verify these changes.
