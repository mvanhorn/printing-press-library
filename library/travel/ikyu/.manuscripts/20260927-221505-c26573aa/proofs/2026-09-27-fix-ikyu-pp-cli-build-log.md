Manifest transcendence rows: 5 planned, 5 built. All five refinements have meaningful content assertions.

Generation passed tidy, fresh tests, vulnerability check, vet and build. Implementation starts from the generated staging module.

## Core slice complete

- Implemented typed `internal/ikyu` model, bounded anonymous HTTP/JSON source, Nuxt destination fallback, read-through TTL/cache eviction, strict stay/occupancy and exact-offer nightly-date checks. Five core `stay` commands now have real handlers: destinations, search, property, rooms and offer.
- Added independent room/plan paging and a 100-summary product cap; joint search filters require one observed room-plan and expose its qualifying headline/link. Bulk room summaries disclose null date echoes; exact offers verify nightly dates. Public point variation is preserved; exact inspection uses the supported default.
- Source and CLI content assertions pass: exact30800/24640/6160 yen/points, source cancellation union rules, source normalization/errors/sold-out, fractional rates, bath unknowns, real Nuxt/schema errors, paging/coverage, split room-plan filter negative case, cache TTL/refresh/stale/bounds/redirect attempt counts, dry-run and field projection. CLI JSON is single-line and preserves int64 values beyond float64 precision.
- Checks: `go test ./internal/ikyu ./internal/cli` passed; `go build ./...` and `go vet ./...` passed. Native live minimal property/rooms/offer: 3 requests, 20,310 decoded bytes, 1.25s; offer source amount30800. Native Tokyo catalog/search paths work; canonical dated search returned source total587 and relevant source IDs.
- Priority1 review checkpoint: pause for root three-command gate. Next slice implements compare/date fan-out, content-equivalence assertions, workflow/live/measurement scripts, patch durability and final full verification.

## Priority 2 complete

- `stay compare` checks exact room/stay, detailed meal inclusions, cancellation rules, point variation, payment/content, check-in/out and eligibility. Unknown or different conditions do not produce savings. `stay dates` retains each date, including source sold-out and error rows. Both use generated FanoutRun with concurrency two; all-failed rate limits retain exit 7.
- All five refinements have deterministic content/failure assertions. Final regressions cover finite minimum size, 100 room-plan summary cap, ambiguous bath attribute36, malformed plan IDs, filtered destination labels and stale catalog dependency, exact original versus earn/use-now price scenarios, bounded text and typed fan-out failures.
- Fresh full `go test ./... -count=1` passed; `go build -o ikyu-pp-cli ./cmd/ikyu-pp-cli` and `go vet ./...` passed. Anonymous live expanded offers for hotel and ryokan each used one request: hotel source57540/headline54664, ryokan source30800/headline30800. Detailed hotel meals remain explicitly unknown where a source menu is absent; point variation0 is preserved.
- `workflow_verify.yaml` performs live Tokyo search → returned property rooms → returned room-plan exact offer. Go edits are frozen for the root completion gate; independent live-display and cold/warm efficiency scripts are the remaining test-delivery slice.

## Root completion-gate finding

All seven runtime leaf-help/dry-run checks passed. Printing Press static discovery reported five missing workflows because shared constructor helpers hid literal Cobra declarations; worker is exposing equivalent literal declarations before retrying. A first restricted invocation could not build the CLI; rerunning with authorized build access resolved compilation and exposed the scanner issue. No receipt or acceptance count was overridden. Description metadata was aligned from research narrative.

The static-discovery fix passed: dogfood now verifies 5 planned / 5 built. All seven leaf commands resolve and dry-run without IO. Full deterministic tests/build/vet passed (priority2-assertions.json). Shared fan-out API calls are proven by live specimens and tests; static reimplementation warnings do not identify a missing source call. Remaining mechanical workflow fixture formatting is being corrected for shipcheck.

## Independent live and efficiency delivery complete

- `scripts/verify-live.py` passed the anonymous seven-command flow for supplied future dates, including independent fresh hotel+ryokan public HTML/SSR comparisons. Every source price field, exact room/plan name, meal, ordered cancellation field and nightly/party echo matched. Visible HTML was checked against the earn-points headline and points; instant prices use SSR evidence. Only safe extracted facts, URLs, status and timestamps were retained in `live-acceptance.json`.
- `scripts/measure.py` passed separate cold/warm command measurements. All seven cold commands used 1–2 actual requests; all immediate warm commands used zero requests and zero decoded source bytes. Warm latency was 10.8–31.5ms. Exact offer projection reduced 8,974 bytes to 191 bytes; cold peak RSS was 24–35MiB. Actual per-command values are in `efficiency-metrics.json`.
- `python3 -m unittest discover -s scripts -p 'test_*.py' -v`: four independent parser/source-bound tests passed. Scripts accept supplied future dates or derive bounded Japan-local dates and current default-variation selections; no expired fixed baseline is required.
- Static discovery compatibility exposes literal Cobra declarations for all seven leaf commands. Offer/dates show three required IDs. Dry-run request-plan envelopes emit compact top-level dry_run/action control metadata directly, bypassing agent wrapping and live-result projection without changing output flags. Relevant CLI content tests and fresh build pass after these focused gate corrections.

- Final control-contract check: all seven fresh-binary `--dry-run --agent --select data` reads passed top-level dry_run/action/planned/zero-IO assertions. Focused zero-IO and normal live field-projection tests passed; fresh binary rebuilt.
