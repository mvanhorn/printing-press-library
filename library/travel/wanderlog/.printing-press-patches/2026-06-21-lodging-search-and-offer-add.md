# 2026-06-21 Lodging Search And Offer Add

## Intent

Preserve support for Wanderlog's itinerary-page Lodging button workflow across future reprints:

- Expose `/api/lodging/searchLodgings` as `lodging search` using the read-only POST client path.
- Return compact lodging candidate summaries by default, with `--raw-response` for the full browser payload.
- Let `plan reservation add --kind lodging` accept `--lodging-offer-json` so Airbnb/Kayak/internal lodging search offers without Google place ids can still be inserted as native lodging itinerary blocks.
- Preserve provider id, coordinates, images, rating, price, and booking URL as `lodgingOffer` metadata on the lodging block.

## Touched Surface

- `internal/cli/lodging.go`: top-level lodging search command and compact response summaries.
- `internal/cli/root.go`: command registration.
- `internal/cli/which.go`: discovery entry for `lodging search` and hotel base workflows.
- `internal/cli/plan_reservation.go`: `--lodging-offer-json` support for lodging reservations.
- `internal/cli/plan_edit.go`: dry-run block summaries include `lodging_offer`.
- `internal/cli/lodging_test.go` and `internal/cli/plan_reservation_test.go`: request/summary/offer insertion coverage.
- `SKILL.md` and `references/reservations-attachments.md`: agent workflow docs.

## Verification

- `go test ./...`
- `go build -o ./wanderlog-pp-cli ./cmd/wanderlog-pp-cli`
- `go build -o ./wanderlog-pp-mcp ./cmd/wanderlog-pp-mcp`
- `lodging search --geo-id 50 --bounds 127.63045,26.17561,127.73895,26.24614 --start-date 2026-08-30 --end-date 2026-09-06 --adult-count 2 --room-count 1 --min-guest-rating 8 --limit 3 --agent` returned compact lodging candidates.
- `plan reservation add --kind lodging --lodging-offer-json "$(jq -c '.data.offers[0]' /tmp/wanderlog_lodging_search.json)" ... --agent --select block,operation,op_paths` dry-ran an itinerary lodging block with `hotel` and `lodging_offer` metadata.
