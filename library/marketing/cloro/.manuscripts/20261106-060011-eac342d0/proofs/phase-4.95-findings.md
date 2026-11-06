# Phase 4.95: local code review

Scope: hand-written files `internal/cli/cited.go` and `internal/cli/cited_test.go`. Generated files are out of scope. Reviewed inline by the printing agent for correctness, security and maintainability.

- Timeout boundary: `cited` uses `flags.newClient()` and wraps the context with `boundCtx`. PASS.
- Replay safety: each scrape sends `X-Printing-Press-Replay-Safe: false`, so the client does not resend a paid request after an ambiguous failure. PASS.
- Partial failures: a failed engine goes to `fetch_failures` with a stderr warning and is not counted as "not cited"; when every engine fails, the first API error is returned with its typed exit code. Covered by `TestCitedAgainstStubServer`. PASS.
- Domain match: host equality or a dot-bounded suffix, so `notnike.com` and `nike.com.evil.example` do not match `nike.com`. Covered by `TestCitedHostMatches`. PASS.
- Input validation runs after the `--dry-run` short circuit and returns usage errors (exit 2) for a missing domain, prompt, country or an unknown engine. PASS.
- No secrets, no file writes, no shell-outs.

Findings cleared at round 1 (0 findings).
