# Insurance Finder CLI — Shipcheck

Run: 20260628-093000

## Build / static gates

| Gate | Result |
|------|--------|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `gofmt -l .` | PASS (clean) |
| `go test ./...` | PASS (31 test cases across 2 packages) |
| `govulncheck ./...` | PASS (no reachable findings) |

## Behavioral matrix (CLI smoke)

Every command exercised; `--json` / `--csv` / `--plain` / `--select` output modes
verified; importer and non-importer routing checked against the registry.

| Command | Check | Result |
|---------|-------|--------|
| `intake` | non-interactive write persists profile.json at 0600; effective date defaults to next business day | PASS |
| `profile show` | renders saved profile | PASS |
| `providers list` | lists 15 providers (json/csv) incl. Tivly + Supersure (unverified) | PASS |
| `match` (importer) | specialty markets ranked recommended; mainstream decliners (Hartford/biBerk/Next) in avoid tier | PASS |
| `match` (non-importer retail) | inverse: instant-quote mainstream recommended; specialty demoted | PASS |
| `answersheet <id>` | maps profile -> paste-ready values incl. `$1M/$2M`, decline-marketing, provider hints | PASS |
| `checklist <id>` | CAPTCHA / account / EIN-SSN / payment / two-gate submit / decline-consents present | PASS |
| `warnings` (importer) | foreign-products exclusion (CRITICAL) + Coverage B IP gap present | PASS |
| `guide` | combined warnings + per-provider URL + answer sheet + checklist | PASS |
| `doctor` | registry + profile + path checks OK | PASS |
| `--version` / `--help` | exit 0 | PASS |

## Notes

- This CLI wraps no HTTP API; it is a data-driven guided tool over an embedded,
  editable provider registry. There is therefore no live-API smoke test — the
  behavioral matrix above exercises the deterministic logic and output paths.
- The matching logic is covered by unit tests in both directions (importer ->
  specialty; low-hazard retail -> mainstream instant-quote).
