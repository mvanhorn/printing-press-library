# Acceptance smoke proof

Validated locally:

```bash
go build ./...
go run ./cmd/xero-pp-cli status
go run ./cmd/xero-pp-cli accounts list --fixture testdata/fixtures/xero/accounts.json
go run ./cmd/xero-pp-cli reports trial-balance --fixture testdata/fixtures/xero/trial_balance.json
cli-printing-press verify --dir . --no-spec
```

Result: PASS structural verify; fixture commands emitted JSON envelopes.
