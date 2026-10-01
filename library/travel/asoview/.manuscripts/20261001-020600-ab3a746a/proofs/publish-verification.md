# Asoview publish verification

Fresh read-only verification ran at 2026-10-01T01:51:20.922223+00:00. Printing Press 4.32.5 produced the original CLI and ran these checks.

- `publish validate`: PASS, including manifest, live marker, tidy, reachable govulncheck, vet, build, help, version, skill and patch checks. The pre-package bare module name is an expected warning; package rewrites it to the canonical public module path.
- `go test -count=1 ./...`, `go vet ./...`, `go build ./...`: PASS.
- Press shipcheck: PASS; all 7 legs exited zero. Structural/mock checks are separate from live correctness.
- Publish-time full live dogfood: 66/66 passed; 39 skipped/unverified framework rows remain explicitly unverified. No live test skip was requested.
- Independent `scripts/live_check.py`: 23 real-source semantic assertions passed at 2026-10-01T01:48:31.821504+00:00, for Japan date 2026-10-02; no fixtures used.
- Source fingerprint recorded by the runner: `96e4651533bc35aeffd79ceb373ed45b3d6b5a2bea08922236e79417e26c0a2c`. Raw full dogfood output was kept in a private temporary directory and deleted after extracting the genuine acceptance proof.

Source IDs, Japanese labels, exact units, nulls and public timestamps remain intact. Only anonymous allowlisted GETs run. Account/history/checkout, confirmed price quotes, purchases, booking and holds are unavailable. Public stock is a snapshot; unknown/request-only never implies sold out. General admission validity never becomes a fabricated reserved slot. Japanese is the verified source language.

This PR requests publication; a maintainer merge and catalog automation remain separate. The older generation final-evidence records local promotion and its original no-publication scope. The current direct user request authorizes this publication.

After all three review fixes, full Go tests/vet/build and all seven shipcheck legs passed again. Full live dogfood regenerated this source-bound acceptance proof (66/66 pass, 39 skipped/unverified). The 23 independent source assertions reran at 2026-10-01T02:09:32.485835+00:00.
