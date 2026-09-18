# Garmin CLI — Shipcheck Proof

## Verdict: 6 of 7 legs PASS; scorecard leg HOLD, so the shipcheck envelope is HOLD (exit 3)

- Run: `cli-printing-press shipcheck --no-live-check --no-fix`, press 4.32.0, 2026-09-19 (UTC), on the tree this PR ships.
- Legs: verify PASS, validate-narrative PASS, dogfood PASS, workflow-verify PASS, apify-audit PASS, verify-skill PASS, scorecard HOLD.
- Scorecard: 95/100, grade A, with 1 of 26 dimensions unverified: `live_api_verification`. (scorecard block in the run copy's `.printing-press.json`; not carried into the published one.)
- Go: `go build`, `go vet`, `gofmt` clean; `go test ./...` 15 packages ok.

## Why the scorecard leg holds

The press scores `live_api_verification` from a live-mode `verify` run. Live-mode `verify` executes every read command's plain form, and a plain `history` on an empty home is a full-account archive fill that starts at 2007. A rehearsal against a local stub server showed it walking the step series window by window until `verify` cut it off. `verify` therefore ran against the spec-derived mock server, and the press leaves that dimension unscored.

Live behaviour is covered by the Phase 5 matrix instead: 150 of 150 counted tests passed, 39 of them against the real API (19 happy-path and 19 JSON-fidelity reads plus the one error-path row (`search`)); the rest are help, offline and archive checks. See `phase5-acceptance.json` and the acceptance report beside this file.

## Checks made on the way

- Default requests carry no `DI-Backend` header, and `--di-backend connectapi.garmin.com` adds it. Confirmed by capturing the CLI's requests on a local server: the default run sends Accept, Accept-Encoding, Authorization, Host and User-Agent only, and the flagged run adds Di-Backend: connectapi.garmin.com.
- `sync` prints its pointer to `history` and exits 2.
- The 27 generated endpoint command files match a fresh press render of the committed spec, apart from the patch records' edits to `internal/cli/{sync,activities_list,root,doctor,auth_login}.go`.

## Known gaps

See the PR description.
