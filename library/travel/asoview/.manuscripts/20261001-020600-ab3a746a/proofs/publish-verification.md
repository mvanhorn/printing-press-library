# Asoview publish verification

Fresh read-only verification ran at 2026-10-01T01:51:20.922223+00:00. Printing Press 4.32.5 produced the original CLI and ran these checks.

- `publish validate`: PASS, including manifest, live marker, tidy, reachable govulncheck, vet, build, help, version, skill and patch checks. The pre-package bare module name is an expected warning; package rewrites it to the canonical public module path.
- `go test -count=1 ./...`, `go vet ./...`, `go build ./...`: PASS.
- Press shipcheck: PASS; all 7 legs exited zero. Structural/mock checks are separate from live correctness.
- Publish-time full live dogfood: 66/66 passed; 39 skipped/unverified framework rows remain explicitly unverified. No live test skip was requested.
- Independent `scripts/live_check.py`: 21 real-source semantic assertions passed at 2026-10-01T01:48:31.821504+00:00, for Japan date 2026-10-02; no fixtures used.
- Source fingerprint recorded by the runner: `d8fbf1d3c6d12a25fe085af5c2c9759a91b2541c0853a868dbfe17114a67cf8b`. Raw full dogfood output was kept in a private temporary directory and deleted after extracting the genuine acceptance proof.

Source IDs, Japanese labels, exact units, nulls and public timestamps remain intact. Only anonymous allowlisted GETs run. Account/history/checkout, confirmed price quotes, purchases, booking and holds are unavailable. Public stock is a snapshot; unknown/request-only never implies sold out. General admission validity never becomes a fabricated reserved slot. Japanese is the verified source language.

This PR requests publication; a maintainer merge and catalog automation remain separate. The older generation final-evidence records local promotion and its original no-publication scope. The current direct user request authorizes this publication.
