# Acceptance Report: lichess

- Level: Full Dogfood attempted; bearer-auth live matrix skipped because no credential was supplied.
- Structural matrix: PASS (`go test ./...`, `go vet ./...`, `go build ./...`, command help, and JSON dry runs).
- Public live smoke: PASS (`user lichess --json` returned an official public profile).
- Approved command dry runs: PASS for `challenge ten`, `loss-patterns`, and `training-brief`.
- Live matrix: 76/82 checks passed; four bearer-auth endpoint checks returned the expected HTTP 401 without a token, and two missing-help-example findings were fixed afterward.
- Gate: SKIP (`auth_required_no_credential`); no token, personal game data, challenge, or write operation was used during verification.

## Fixes applied

- Added examples for `challenge ten` and `loss-patterns` after live dogfood identified missing help examples.
- Removed generated unconstrained challenge, game-export, raw-API, and standalone puzzle-fetch surfaces after focused safety review; narrowed OAuth scopes and MCP metadata to the approved boundary.
