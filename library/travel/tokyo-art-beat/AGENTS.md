# Tokyo Art Beat development

This is a focused public read-only provider CLI. Domain requests and parsing live in `internal/tab`; durable command wiring is registered by `internal/cli/tab_commands.go`. Generated scaffold files remain available but unrelated write workflows are excluded from the ordinary runtime tree.

Preserve exhibition edition IDs, venue IDs, raw archive date values, Japanese/English omissions and source links. Event fees/hours stay separate from venue defaults. Date spans and publication status never establish actual opening or ticket inventory. Unsupported source operators require bounded local filtering and explicit coverage.

Before changing transport or filters, verify their behavior on the public feed: it can silently ignore query keys. Keep logical/wire requests, timeout, response/cache size and scan limits bounded. Network diagnostics go to stderr; domain JSON and errors go to stdout.

Run `go test -count=1 ./...` and `go vet ./...` after consequential changes. Real provider evidence and fixtures are distinct. Source overrides/httptest are test-only; production URLs stay restricted to the published CDN. A source error must not become an empty successful result.

Hand-authored extension files and `.printing-press-patches/` are the reprint contract. Preserve receipts, research and generated scaffolding. README and SKILL describe the shipped tree; `agent-context` reports runtime commands. Release versions remain unstamped until a separate publication workflow.
