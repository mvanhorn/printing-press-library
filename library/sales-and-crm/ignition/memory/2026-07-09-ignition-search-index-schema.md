# Debug report: Ignition invoice and billing-item search schema

- **Symptom:** Invoice and billing-item search queries selected guessed scalar fields that Ignition rejects; analytics also expected those nonexistent scalars.
- **Root cause:** The generated query defaults and shared decoder predated the live-verified `InvoiceResult` and `BillingItemResult` shapes in `~/.openclaw/workspace/projects/ignition-final-invoice/ignition_gql.py`.
- **Fix:** Replaced both selections with the verified fields, modeled nested money/status data, parsed formatted money, and routed outstanding/client-billing/rejected-payment aggregation through the real fields.
- **Evidence:** `go build ./...`, targeted `go vet`, the standalone binary build, `go test ./internal/cli/`, and both requested dry runs exit 0. The invoice dry run prints the verified selection and `pageNumber`/`pageSize` without sending a request.
- **Regression test:** `internal/cli/ignition_analytics_test.go` covers query defaults, nested decoding, money parsing, status precedence, invoice display IDs, all five analytics views, and invoice-side client matching.
- **Related:** `.printing-press-patches/0004-invoice-billing-schema.md` preserves this generated-tree correction across reprints.
- **Status:** DONE
