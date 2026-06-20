# Manual Validation

- Generated from an internal OpenAPI 3.0 spec based on observed Plaud Web traffic for user-owned recording workflows.
- Live Plaud Web smoke tests were intentionally skipped because publishing should not require or expose a real Plaud bearer token.
- Local validation covered `go test ./...`, `go vet ./...`, generated help output, and publish validation.
- Secret-bearing artifacts such as bearer tokens, cookies, signed URLs, downloaded audio, and HAR files were not added.
