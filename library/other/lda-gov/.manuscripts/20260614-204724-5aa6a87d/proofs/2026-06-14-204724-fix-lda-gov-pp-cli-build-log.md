Manifest transcendence rows: 7 planned, 7 built.

Implemented local SQLite-backed analyses for:

- audit filings
- entities resolve
- audit spend
- graph export
- contributions totals
- reports quarter
- lobbyists covered-positions

Verification so far:

- `go test ./...` passed.
- `go build ./cmd/lda-gov-pp-cli ./cmd/lda-gov-pp-mcp` passed.
