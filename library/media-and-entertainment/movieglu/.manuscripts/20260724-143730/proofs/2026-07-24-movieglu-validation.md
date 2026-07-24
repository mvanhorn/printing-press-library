# MovieGlu validation evidence

- Official documentation captures: HTTP 200 through the MovieGlu WordPress API
  for quick start, headers, films, cinemas, showtimes, closest showing, and
  purchase confirmation pages.
- Generated validation: `go mod tidy`, `go test ./...`, `govulncheck ./...`,
  `go vet ./...`, `go build ./...`, binary help, version, and doctor passed.
- Provider binding tests: missing required MovieGlu values fail closed;
  configured values populate the documented headers.
- Workflow contract test: three HTTP calls passed with exact paths, query
  values, mandatory headers, film resolution, showtime normalization, ranking,
  and HTTPS booking handoff.
- Live API dogfood: skipped because no MovieGlu evaluation credentials were
  available in the harness. No mock result is represented as live evidence.
