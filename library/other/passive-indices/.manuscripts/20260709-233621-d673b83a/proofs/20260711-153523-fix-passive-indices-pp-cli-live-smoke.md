# Phase 5 Live Dogfood — passive-indices-pp-cli

`cli-printing-press dogfood --dir "$CLI_WORK_DIR" --live --level full --write-acceptance "$RUN/proofs/phase5-acceptance.json"`

## Result: PASS — 107/107, 0 failed (`phase5-acceptance.json` status: pass)

No auth required for either source (niftyindices.com and indiapassivefunds.com
are both no-auth), so this was a full mandatory live run, no skip.

## Iteration history

First run: 7/111 failed. Fixed 3 real bugs, resolved 1 design question, and
removed 1 command (by explicit user decision) to reach 0 failures.

1. **`index history` / `index tri` / `index valuation`** — an unknown NSE
   index name returned exit 0 with an empty array instead of erroring,
   because niftyindices' BackPage historical endpoints return HTTP 200 +
   `[]` for both an unknown name and a valid name with no data in range.
   **Fixed** in `internal/niftyindices/client.go`: added
   `rejectIfUnknownIndex()`, called only on a zero-row result, cross-checking
   the name against the live `LiveWatch()` index list (real API data, not a
   hardcoded list). Verified: bogus name → exit 5 with a clear message;
   valid name unaffected.

2. **`fund timeseries`** — an invalid schemeId got a `{"status":false,
   "message":"Internal Error"}` envelope from indiapassivefunds but the CLI
   printed it as success (exit 0). Root cause: `Client.TimeSeries` in
   `internal/indiapassivefunds/client.go` was the one envelope-returning
   method in that file that didn't check `status` first (every sibling
   method does). **Fixed:** added the same check while still returning the
   full raw envelope on success (the command prints header/types/period
   metadata, not just the unwrapped `response`).

3. **`fund nfo tracking` error_path** — assessed as correct-as-is: this
   command does a fund-name substring match (the NFO listing carries no
   underlying-index field), so an unknown index and a known index with zero
   upcoming NFOs are genuinely indistinguishable; exit 0 + empty array is
   the honest answer either way. Annotated
   `cmd.Annotations["pp:no-error-path-probe"] = "true"` (the sanctioned
   dogfood opt-out for this exact HTTP-200-empty-envelope shape) so the live
   matrix stops flagging it. Separately fixed a doc-accuracy bug:
   `research.json`'s rationale claimed a field-level join that contradicts
   the actual substring-match implementation — corrected the wording (2
   occurrences).

4. **`fund rankings`** — indiapassivefunds' `marketrankings` endpoint
   requires a `--command` enum value it never documents anywhere (confirmed
   unfindable across two sessions of live probing and JS-bundle mining).
   No code fix can supply a value that doesn't exist, and the live-dogfood
   harness has no escape hatch for "no example exists at all" on the
   happy_path check. Tried softening the command's `Example:` text to a
   `#`-prefixed comment (stops sending a literal `<placeholder>` token to
   the live API) — cut the failure count but the harness's own "missing
   runnable example" check still failed happy_path. Presented the tradeoff
   (hand-write a pass marker vs. drop the feature vs. stop for review) to
   the user directly. **User chose to drop the feature.** Removed
   `fund_rankings.go`, its `fund.go` registration, and the unused
   `MarketRankings`/`MarketRankingsParams` client code. `go build`/`go vet`/
   `go test ./...` clean after removal.

## Final state

- Manifest: 21 features shipped (12 absorbed + 9 novel), down from the
  originally approved 22 — `fund rankings` dropped for the reason above.
- `go test ./...`: all packages pass.
- Live dogfood: 107/107 passed, 78 skipped (expected — mutating/destructive
  guards, missing-fixture cases, file-fixture requirements), 0 failed.
