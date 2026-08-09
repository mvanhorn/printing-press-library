# Shipcheck

- Seven of seven shipcheck legs passed.
- Verify: 100% with zero critical failures.
- Scorecard: 93/100, Grade A.
- Auth protocol: exact `subscription-Key` header match using `FANDANGO_SUBSCRIPTION_KEY`.
- Novel features: 6/6 implemented and resolved.
- Live dogfood was skipped because Fabric Origin requires a paid subscription and explicit Fandango approval; no licensed credential was available.

## Maintainer-requested substitute dogfood

Revalidated on 2026-08-09 with Printing Press 4.29.0 while the licensed
credential remained unavailable:

- `cli-printing-press dogfood --dir library/media-and-entertainment/fandango --spec library/media-and-entertainment/fandango/spec.json --json`: PASS, 100% path validity, exact `subscription-Key` auth match, 70/70 Cobra commands registered, zero dead flags, zero dead functions, and a populated default sync plan.
- `cli-printing-press verify --dir library/media-and-entertainment/fandango --spec library/media-and-entertainment/fandango/spec.json --threshold 80 --cleanup --json`: PASS in spec-derived mock mode, 34/34 checks, zero failures, zero critical failures, and the sync data-pipeline check passed.
- `go test ./...`, `go vet ./...`, and `go build ./...`: PASS.
- `TestFandangoPlanningCommandsAgainstContractServer`: PASS for all six novel workflows against an HTTP contract server. The test asserts the licensed `subscription-Key` header, official `/Fandango/Showtimes` path and query contract, date-window filtering, format grouping, theater comparison, availability grouping, watchlist matching, and licensed checkout-link delivery.
- The behavior suite exposed and fixed an actual defect: timezone-less local showtimes were parsed as UTC and could be filtered out of `movie-plan` incorrectly.

This is substitute evidence, not a live-API claim. A real Fabric Origin run is
still required when a paid, Fandango-approved `FANDANGO_SUBSCRIPTION_KEY` is
available.
