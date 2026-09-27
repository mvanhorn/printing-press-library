# Local code review

Review path: root gpt-6-astra directly, as explicitly requested by the user; concrete fixes and tests delegated to two gpt-6-sol workers at max effort. No coordination children.

Scope: hand-authored `internal/travel`, public commands, test harnesses and their generated integration points. Reviewed identity association, price evidence, query echoes, booking links, empty/error interpretation, bounded scans, request deadlines/budgets, redirects, cache paths, retries, output provenance and partial comparisons.

Autofix summary: review corrections were delegated in place; this fresh checkout has no prior Git commit baseline. Regression coverage includes property identity vs related links, actionable offers vs unavailable catalog forms, scoped meal labels, Retry-After delta semantics, persistent-cache bypass, same-invocation snapshot reuse and bounded-scan notes.

Template/static-analysis candidates: Printing Press workflow verification executes flags embedded in command text but its structural mapper expects path-only strings. A read-only offline probe confirmed args-map values are ignored by the installed runner. The shipped workflow uses executable commands; acceptance checks every actual step and independently asserts semantics with the Python harness. Shared Printing Press code remains unchanged.

Convergence: findings cleared in the final root review. Public-source live semantics pass 56/56; all five approved feature samples pass; both workflow chains pass all five actual steps without skips. Explicit command registration fixed static ambiguity. Security scan has zero hand-authored findings; 20 generated framework candidates and their context are retained in security-review.md. No unresolved product defect or scope tradeoff remains.

No PR or upstream issue was opened: local-only delivery was requested. Codex skips the Claude-only simplify step.

Live matrix follow-up: generated raw HTML clients used a JSON Accept default, causing Rakuten keyword HTTP500. Controlled A/B proved the representation requirement. The source spec now declares HTML Accept; a preserved client hook and actual-wire regression provide the local correction without shared-tool edits. Focused public commands were already correct.
