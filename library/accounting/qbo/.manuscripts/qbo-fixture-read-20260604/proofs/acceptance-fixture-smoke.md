# Acceptance smoke proof

Validated locally:

```bash
go build ./...
go run ./cmd/qbo-pp-cli status
go run ./cmd/qbo-pp-cli accounts list --fixture testdata/fixtures/qbo/accounts.json
go run ./cmd/qbo-pp-cli reports trial-balance --fixture testdata/fixtures/qbo/trial_balance.json
cli-printing-press verify --dir . --no-spec
```

Result: PASS structural verify; fixture commands emitted JSON envelopes.
