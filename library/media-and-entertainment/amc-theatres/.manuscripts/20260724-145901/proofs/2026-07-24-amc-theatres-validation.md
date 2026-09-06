# AMC Theatres validation evidence

- Official contract capture: the AMC developer portal's browser-rendered RapiDoc
  identified five Showtime API v2 read operations, the production and sandbox
  hosts, required `X-AMC-Vendor-Key`, and optional `X-AMC-Auth-Token`.
- Generated validation: `go test ./...`, `go vet ./...`, and `go build ./...`
  passed after provider binding and the novel workflow were added.
- Provider tests: sandbox selection, optional auth-token injection, explicit
  custom-base preservation, and invalid environment rejection passed.
- Workflow tests: exact current-location path and query, both AMC headers,
  normalization, movie/time/format filtering, distance/time ranking, dry-run,
  invalid location arguments, and a structured HTTP 409 JSON error envelope
  with exit code 5 passed.
- Live API dogfood: skipped because no AMC developer credential was available
  in the harness. No fixture or mock response is represented as live evidence.
