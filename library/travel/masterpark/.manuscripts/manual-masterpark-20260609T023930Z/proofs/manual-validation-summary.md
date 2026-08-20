# MasterPark Manual Validation Summary

Validated on 2026-06-09.

- `go fmt ./...` passed.
- `go test ./...` passed.
- `go vet ./...` passed.
- `go build -o ./masterpark-pp-cli ./cmd/masterpark-pp-cli` passed.
- Live `locations` returned MasterPark Lot B and Lot G.
- Live `quote` succeeded for Lot B and Lot G for the requested parking window.
- `PRINTING_PRESS_VERIFY=1 reserve --submit --yes --json` no-opped before live mutation.
- A real gated reservation submit returned an active reservation; the user independently confirmed a MasterPark email confirmation.
- Follow-up testing found `listReservations` may lag or omit freshly-created reservations, so docs and help text now call that out.

Secrets and personal vehicle/license details are intentionally omitted from this manuscript.
