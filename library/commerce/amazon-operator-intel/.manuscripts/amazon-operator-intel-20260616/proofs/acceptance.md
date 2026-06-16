# Amazon Operator Intel Acceptance Proof

Validation performed from `library/commerce/amazon-operator-intel`.

## Static And Unit Validation

```bash
gofmt -w ./cmd ./internal
go vet ./...
go test ./...
go build -o /tmp/amazon-operator-intel-pp-cli ./cmd/amazon-operator-intel-pp-cli
```

Result: PASS.

`go test ./...` covers:

- `agent-context` descriptor shape.
- default fixture sync without child exec.
- `sync --import` without child exec.
- `sync --source all` missing-config preflight before child exec.
- single-source Seller and Ads sync merge behavior.
- stubbed child CLI args and JSON parser behavior.
- orphan Ads rows preserved.
- child non-zero exit stderr propagation.
- representative Seller and Ads parser shapes.
- COGS, purchase order, vendor deduction, and keyword/listing local-file parsing.
- every required command over fixture data.
- advanced workflow assertions for `operator-plan`, `cash-calendar`, `launch-readiness`, `rank-defense`, `bundle-opportunities`, and `vendor-ops`.

## Command Smoke Validation

The following commands were run with a temporary `AMAZON_OPERATOR_INTEL_HOME`:

```bash
go run ./cmd/amazon-operator-intel-pp-cli --agent agent-context
go run ./cmd/amazon-operator-intel-pp-cli --agent doctor
go run ./cmd/amazon-operator-intel-pp-cli --agent sources doctor
go run ./cmd/amazon-operator-intel-pp-cli sync
go run ./cmd/amazon-operator-intel-pp-cli --agent war-room
go run ./cmd/amazon-operator-intel-pp-cli --agent restock-or-kill
go run ./cmd/amazon-operator-intel-pp-cli --agent ad-spend-guardrail
go run ./cmd/amazon-operator-intel-pp-cli --agent sku-profit-truth
go run ./cmd/amazon-operator-intel-pp-cli --agent listing-triage
go run ./cmd/amazon-operator-intel-pp-cli --agent cash-leaks
go run ./cmd/amazon-operator-intel-pp-cli --agent search-term-actions
go run ./cmd/amazon-operator-intel-pp-cli --agent digest daily
go run ./cmd/amazon-operator-intel-pp-cli --agent digest weekly
go run ./cmd/amazon-operator-intel-pp-cli --agent operator-plan
go run ./cmd/amazon-operator-intel-pp-cli --agent cash-calendar
go run ./cmd/amazon-operator-intel-pp-cli --agent launch-readiness --asin B00FIXTURE --sku FIXTURE-SKU --target-acos 0.25 --launch-budget 500 --inventory-units 250 --cogs 12.50
go run ./cmd/amazon-operator-intel-pp-cli --agent rank-defense
go run ./cmd/amazon-operator-intel-pp-cli --agent bundle-opportunities
go run ./cmd/amazon-operator-intel-pp-cli --agent vendor-ops readiness
go run ./cmd/amazon-operator-intel-pp-cli --agent vendor-ops deductions --fixture
go run ./cmd/amazon-operator-intel-pp-cli --agent vendor-ops po-watch --fixture
go run ./cmd/amazon-operator-intel-pp-cli --agent vendor-ops scorecard --fixture
```

Result: PASS.

Expected failure:

```bash
go run ./cmd/amazon-operator-intel-pp-cli sync --source all
```

Result: PASS. The command fails before running child CLIs when Ads profile config is missing.

## Existing Child CLI Validation

Local child CLI checks were run without printing secret values:

```bash
amazon-seller-pp-cli --agent doctor
amazon-ads-pp-cli --agent doctor
amazon-seller-pp-cli sellers --agent
amazon-ads-pp-cli profiles list --agent
amazon-operator-intel-pp-cli --agent doctor --deep
```

Result: PASS. Both child CLIs reported configured auth, Seller account discovery returned JSON, Ads profile discovery returned JSON, and composite deep doctor reported both child doctors as successful.

Live composite sync notes:

- `sync --source seller --marketplace-id ATVPDKIKX0DER` was started against the configured child CLI and interrupted after about 90 seconds because report-backed Seller intel commands were still running.
- `sync --source ads --ads-profile-id <profile> --ads-report-dir <empty-temp-dir>` now fails before child analytics run with a clear missing normalized report file error.
