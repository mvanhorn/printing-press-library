# POP acceptance notes

- `printing-press publish validate --dir ~/printing-press/library/pop --json` passed locally after auth-surface cleanup and manuscript proof completion.
- `go build ./...` passed for the generated POP CLI and MCP package.
- `go test ./...` passed for the published library package copy under `library/payments/pop`.
- `pop-pp-cli create-xml --help` showed the expected promoted command surface.
- POP staging smoke checks reached the live staging API for doctor, PEPPOL retrieval, and XML creation paths and returned real domain responses.
