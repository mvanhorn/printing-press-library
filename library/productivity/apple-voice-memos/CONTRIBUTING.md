# Contributing

Contributions are welcome, especially compatibility reports for new macOS releases and synthetic transcript fixtures.

## Development

Requirements:

- Go version declared in `go.mod`
- macOS for live integration tests

Run the local gates:

```bash
gofmt -w *.go
go test -race ./...
go vet ./...
govulncheck ./...
go build ./...
```

## Tests and privacy

Do not commit real Voice Memos databases, recordings, titles, transcripts, UUIDs, or local filesystem paths. Build fixtures programmatically from synthetic metadata and synthetic ISO-BMFF atoms.

Every behavior change should include a test that fails before the implementation and passes afterward.

## Compatibility changes

Apple does not publish the Voice Memos database or transcript atom formats as a public API. Compatibility changes should:

1. preserve read-only database access
2. add a synthetic regression fixture
3. document the affected macOS version
4. fail clearly when a schema is unsupported

## Pull requests

Keep changes focused. Include the commands used to verify the change and note whether live macOS integration was exercised.
