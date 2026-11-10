# bulk update-deals: send the `updates[]` body the API reads (2026-10-07)

- `spec.yaml` `/bulk/deals/update`, `tools-manifest.json` `bulk_update-deals` and
  `internal/cli/bulk_update-deals.go`: the operation takes ONE required body
  param, `updates` (array of 1..100 items, each `id` + editable deal fields),
  not the old `ids[]` + `data{}` (the live route ignored those). The command
  exposes `--updates <json array>` (no `--ids` / `--data`) and sends exactly
  `{"updates":[...]}`. Array flags pass the parsed JSON through, as
  `availability set --rules` does; the API owns the 400 (non-array/empty) and
  422 (>100) checks.
- The 200 response carries a per-item outcome (`data.summary.failed`,
  `data.results[].success/error`). `detectBulkItemFailures` turns any failed item
  into the repo's partial-failure report (id + error on stderr, exit code 6)
  unless `--allow-partial-failure` is set; an all-success batch exits 0.
- `internal/cli/bulk_update_deals_shape_test.go` pins the flag set, the
  `spec.yaml` required keys, the outbound body (httptest), malformed JSON
  sending nothing, and mixed / all-failed / all-success exit codes.
- A reprint from an older spec must keep this shape; `upstream/openapi.yaml`
  was left stale on purpose (a `make sync-spec` refresh is a separate change).
