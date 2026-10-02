# Local verification proof

Verification date: 2026-10-02. All API interactions used synthetic data and mock HTTP servers. No Loops credential, paid API, event send, or email send was used.

| Check | Result |
| --- | --- |
| `go test ./...` | PASS |
| `go vet ./...` | PASS |
| `go run ./cmd/loops-pp-cli --version` | `loops-pp-cli 0.0.0-dev` |
| `go run ./cmd/loops-pp-cli doctor --dry-run --json` | Offline preview, PASS |
| Printing Press 4.33.0 `shipcheck --no-live-check --no-fix --json` | PASS, 7/7 legs |
| Printing Press mock endpoint verification | PASS verdict, 85/86 individual checks in final shipcheck manifest; the bare `events` command lacks a payload and intentionally errors instead of executing |
| Printing Press dogfood | PASS, 0 dead flags/functions, 10/10 examples |
| Printing Press skill verification | PASS |

The generated workflow verifier skipped its optional workflow manifest. The package contains a full command effect matrix in [COMMANDS.md](../../../COMMANDS.md). A later read-only live check is documented in [read-only-live.md](read-only-live.md). A controlled test message remains **unverified** until the user explicitly authorizes the send.
