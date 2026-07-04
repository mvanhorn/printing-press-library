# Acceptance / shipcheck

- `go test ./...`: PASS
- `go build ./...`: PASS
- `go vet ./...`: PASS
- `verify-skill`: PASS
- `cli-printing-press verify --fix --cleanup`: PASS, 38/38
- `cli-printing-press dogfood`: PASS
- `cli-printing-press scorecard`: Grade A, 89
- `cli-printing-press pii-audit`: null findings
- Safety smoke: `send-preflight`, `safe-reply-draft`, `link-audit`, `needs-human`, and `consent-audit` executed locally without live Linq side effects.

Sandbox live smoke remains pending a Linq sandbox key; no real patient numbers were used.
