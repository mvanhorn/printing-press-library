# AGENTS.md — ecommerce-intel

This is a private conventional Printing Press library module for ecommerce intelligence.

## Agent guidance

- Keep the module buildable as a standalone Go module.
- Keep tests offline and credential-free.
- Use `--agent` for automation; it enables JSON and non-interactive flags.
- Do not print secrets; only report environment variable presence.
- Keep `.printing-press.json`, README/SKILL docs, and `agent-context` schema details in sync when commands or source plans change.
- Maintain local-first fixture/import behavior for Shopify, Klaviyo, GA4, GSC, Ahrefs, inventory, email, and GEO workflows.

## Verification

```bash
gofmt -w ./cmd ./internal
go test ./...
go build ./cmd/ecommerce-intel-pp-cli
make smoke
```
