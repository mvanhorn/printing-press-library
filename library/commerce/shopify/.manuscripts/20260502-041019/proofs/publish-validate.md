# Publish Validation

Command:

```bash
go run ./cmd/printing-press publish validate \
  --dir library/commerce/shopify \
  --json
```

Result: PASS

| Check | Result |
| --- | --- |
| manifest | PASS |
| transcendence | PASS |
| go mod tidy | PASS |
| go vet | PASS |
| go build | PASS |
| --help | PASS |
| --version | PASS |
| verify-skill | PASS |
| manuscripts | PASS |

Additional local checks:

- `go build ./...` in `library/commerce/shopify`
- `go test ./...` in `library/commerce/shopify`
- quiet read-only live smoke probes against configured env: `products list`, `orders list`, and `bulk-operations current`

Live smoke stdout was discarded and no store data was saved in the repo.
