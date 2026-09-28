# Validation

Verified locally on 2026-09-27 UTC / 2026-09-28 Japan time. Anonymous, read-only accommodation reads require network access but no API key, login, browser runtime or paid API subscription. Source interfaces are undocumented.

## Evidence

- Go tests, fresh builds and vet passed; four independent-parser Python tests passed. Deterministic assertions cover source points prices, fractional rates, occupancy/date normalization, baths, meal/cancellation equivalence, partial failures, rate limits, bounds, exact integer JSON, observed monetary differences, per-plan budget/meal filtering, and unpriced search candidates with strict filter handling.
- Printing Press shipcheck: all legs pass, verification 100%, score 80/A. [Shipcheck proof](../.manuscripts/20260927-221505-c26573aa/proofs/2026-09-27-fix-ikyu-pp-cli-shipcheck.md).
- Full live matrix: 62/62 executed checks pass, zero failures; 38 framework/fixture/access probes are skipped or unverified. All seven stay commands passed live happy-path, JSON fidelity and dry-run checks. [Binary-owned acceptance](../.manuscripts/20260927-221505-c26573aa/proofs/phase5-acceptance.json).
- Independent public HTML/SSR checks cover hotel 八ヶ岳高原ロッジ and ryokan 蓼科 親湯温泉: exact room/plan names, meals, every ordered cancellation field, ten price fields, visible earn-mode headline/points and source stay echoes. [Live evidence](../.manuscripts/20260927-221505-c26573aa/proofs/publish-account-search-live-acceptance.json).

- Review fixes also passed a fresh live comparison and a 30,000 JPY room-plan budget check. [Regression evidence](../.manuscripts/20260927-221505-c26573aa/proofs/publish-review-regression-live.json).

- An empty source plan window preserves room identity without plan filters and requires a matching plan with a budget filter. [Live pagination evidence](../.manuscripts/20260927-221505-c26573aa/proofs/publish-empty-plan-window-live.json).

## Efficiency

Measured on this macOS host with separate empty caches, then identical immediate warm repeats. Latency includes process startup; RSS is per-process peak from `/usr/bin/time -l`. Requests include redirects/retries; response bytes in the JSON report are decoded bytes. These are observations, not latency guarantees.

| Command | Requests cold/warm | Latency ms cold/warm | Peak RSS MiB cold/warm | Output bytes cold/warm |
|---|---:|---:|---:|---:|---:|
| `destinations` | 1/0 | 66.4/20.1 | 32.8/26.3 | 628/623 |
| `search` | 2/0 | 539.2/29.7 | 38.6/30.2 | 5615/5611 |
| `property` | 1/0 | 185.0/12.3 | 24.2/18.7 | 3426/3424 |
| `rooms` | 1/0 | 207.4/12.8 | 24.4/18.8 | 7758/7756 |
| `offer` | 1/0 | 301.7/11.9 | 24.2/19.2 | 8992/8990 |
| `compare` | 2/0 | 762.5/12.2 | 25.1/20.7 | 18451/18449 |
| `dates` | 2/0 | 987.9/13.4 | 24.8/20.9 | 17311/17309 |
| `offer_projection` | 1/0 | 297.8/11.1 | 24.3/18.9 | 191/188 |

[Full measurements and exact arguments](../.manuscripts/20260927-221505-c26573aa/proofs/publish-account-search-efficiency-metrics.json). Field projection reduced the measured offer from 8,992 to 191 bytes. Search uses heavier SSR pages; room/offer details use small JSON queries and are fetched lazily.

## Reproduce

```sh
go test -count=1 ./...
go build -o ikyu-pp-cli ./cmd/ikyu-pp-cli
python3 -m unittest discover -s scripts -p 'test_*.py'
python3 scripts/verify-live.py --output live-acceptance.json
python3 scripts/measure.py --output efficiency-metrics.json
```

Scripts default to Japan today +21 days and discover current offers. Supply `--check-in`, `--check-out`, `--hotel-offer` and `--ryokan-offer` to reproduce an available specimen. `verify-live.py --write-workflow workflow_verify.yaml` refreshes dated workflow fixtures for later runs. Scripts use Python 3/curl; measurements support macOS/Linux time tooling.

## Limits

Availability caches last five minutes; static details 24 hours. Refresh selected offers before booking handoff. Bulk room summaries omit nightly date echoes, so use exact offer lookup for a verified date quote. Destination search uses the public catalog and filters one bounded page/preview window. Multiple rooms require equal occupancy. Exact inspection uses the default points variant. Account coupons/member inventory, final eligibility and extra checkout taxes remain unverified.

The generated raw `properties` HTML helper returned HTTP403 in two full-matrix probes; it is outside the supported seven-command stay workflow. The CLI does not interact with login challenges or make reservations. Restaurant/spa and overseas products are excluded. See [source contract](source-contract.md) for official access/price/occupancy references.
