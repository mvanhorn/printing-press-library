# Provider maintenance

This is a locally generated Japan Bus Online CLI. Maintain the five source workflows in `internal/jbo/` and `internal/cli/bus*.go`; use command help for current flags. Keep generated framework changes narrow and record required customizations in `.printing-press-patches/`.

Provider access is anonymous English HTTPS GET with a memory-only CookieJar. AJAX needs `X-Requested-With: XMLHttpRequest`. Reproduce normal public browser steps before changing request shape. Keep reservation, passenger submission, payment, account and cancellation mutations outside the adapter.

Preserve course, direction, service, fare-plan and route-local stop identities. Distinguish published schedules from dated inventory; preserve requested and effective source dates and JST 24+ time semantics. Party evidence must use the selected fare table; numeric positive counts are lower bounds, zero is no seats, and transaction caps are separate. Preserve unknown age fares, seat/gender feasibility, source names, canonical URLs and partial cancellation failures.

Every provider command sets `pp:data-source=live`, rejects local mode, uses `boundCtx` and the generated output helpers, and declares semicolon-separated `pp:happy-args`. Source requests use the adaptive limiter, typed rate errors, bounded bodies and same-provider HTTPS redirects. Keep output lists bounded and nil-free.

After provider changes, run relevant deterministic tests and a read-only live check for the changed workflow. Final checks are `go test ./...`, `go vet ./...`, canonical Printing Press shipcheck and the full live matrix. Fixtures never substitute for real source proof. Preserve the SKILL.md generated `## Prerequisites: Install the CLI` section byte-for-byte; its surrounding local-build note must state publication status accurately.

Read `.printing-press.json` for generated provenance. Releases and GitHub publication require a separate explicit request; personal account is zjsng. Source research and public build proofs are preserved in `.manuscripts/20261002-001752-4770e0c8/`. Disposable run receipts remain outside this public package.

After Printing Press syncs generated surfaces, build both binaries and run `python3 scripts/refresh-provider-artifacts.py` to derive the actual MCP catalog/counts from stdio tools/list and remove unsupported generated exclusivity prose. The original endpoint-generation catalog is retained as `generator-endpoint-provenance.json`, explicitly historical. Keep runtime manifests aligned with the five read-only provider tools, and inspect the refreshed SKILL prerequisites section for exact preservation.
