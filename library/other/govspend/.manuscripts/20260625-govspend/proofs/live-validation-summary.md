# Govspend Live Validation Summary

Validation is intentionally read-only and uses public data sources.

## Source Research

- USAspending endpoint index confirms endpoints do not currently require authorization and lists `/api/v2/search/spending_by_award/`.
- SAM.gov Opportunities documentation confirms the production endpoint, required public API key, and required `postedFrom`/`postedTo` date parameters.
- Grants.gov public search endpoint was smoke-tested with a one-row `climate` query and returned a successful JSON response.

## Local Validation Plan

- PASS: `go test ./...`
- PASS: `go vet ./...`
- PASS: `go build ./cmd/govspend-pp-cli`
- PASS: `govspend-pp-cli sources --agent`
- PASS: `govspend-pp-cli doctor --agent`
- PASS: `govspend-pp-cli doctor --live --agent`
- PASS: `govspend-pp-cli vendor "Palantir" --from 2025-01-01 --to 2025-12-31 --limit 2 --agent`
- PASS: `govspend-pp-cli agency NASA --from 2025-01-01 --to 2025-12-31 --limit 2 --agent`
- PASS: `govspend-pp-cli awards --query "cloud migration" --from 2025-01-01 --to 2025-12-31 --limit 2 --agent`
- PASS: `govspend-pp-cli grants --query climate --limit 2 --agent`
- PASS: `govspend-pp-cli opportunities --query cybersecurity --dry-run --agent`
- PASS: `govspend-pp-cli opportunities --query cybersecurity --agent`

## Live Smoke Notes

- `doctor --live` reached USAspending and Grants.gov successfully; SAM.gov reported `not_configured` because no `GOVSPEND_SAM_API_KEY` was provided.
- `vendor` returned USAspending contract-award records for the requested vendor and date window.
- `agency NASA` matched the NASA toptier agency reference and returned awards scoped by USAspending's awarding-agency filter.
- `awards --query "cloud migration"` returned awards scoped by USAspending's supported `keywords` filter.
- `grants --query climate` returned public Grants.gov opportunity summaries.
- `opportunities --dry-run` printed a SAM.gov request shape without a key value.
- `opportunities --agent` returned structured setup guidance when `GOVSPEND_SAM_API_KEY` was missing.

The SAM.gov live search path was not run with a real key in proof artifacts. The command returns setup guidance when `GOVSPEND_SAM_API_KEY` is missing and redacts the key in dry-run output when configured.
