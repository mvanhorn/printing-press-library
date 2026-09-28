# Verification

Verified locally on 2026-09-28 (Asia/Tokyo). These are dated results from the original generation run, not current weather advice. Publication preparation preserves this baseline and records subsequent source maintenance separately.

## Results

- Build and vet pass. The uncached Go suite passed 673 test/subtest events across 11 packages.
- Seventeen live semantic cases passed: Chiyoda, Sapporo, Kyoto Sakyo and Naha; Kinkaku-ji identity/directory; Fuji foothill and model levels; active foliage; ended Sakura and unavailable next year; unsupported dates; daily/hourly/seasonal/mountain comparisons.
- Printing Press full live matrix: 58 executed checks passed, zero failed. Its 47 skips are inapplicable positional or guarded framework probes; every approved product command has a real passing happy path and JSON check.
- Shipcheck: all seven installed legs pass, verification 100%, scorecard 80/100. Four planned comparison behaviors are built and sampled live. Root reviewed documentation, outputs and source.
- Dated raw fixtures and explicitly constructed boundary variants live in `internal/tenki/testdata`; they are separate from live verification. No current active Sakura prediction was claimed from September data.

## Publication checks

The packaged public-module checkout passed 725 test/subtest events with zero failures, all thirteen canonical publish checks, and the repository-owned skill verifier (21 recipes). A fresh full live gate passed all 58 executed checks. [Publication validation](.manuscripts/20260927-221457-5751f054/proofs/publish-validation.md) records the final evidence and the remaining generated-framework scanner findings.

## Efficiency

Single serial measurements on this macOS host, using isolated caches. Latency includes process startup; one-second pauses between cases are excluded. Response bytes measure decoded source bodies, not network transfer. RSS comes from `/usr/bin/time -l`.

| Case | Exit | Wall (s) | Stdout bytes | HTTP | Cache hits | Decoded response bytes | Peak RSS bytes |
|---|---:|---:|---:|---:|---:|---:|---:|
| chiyoda_daily_cold | 0 | 0.337 | 3085 | 1 | 0 | 244014 | 27885568 |
| chiyoda_daily_warm | 0 | 0.020 | 3080 | 0 | 1 | 0 | 22413312 |
| chiyoda_daily_refresh | 0 | 0.344 | 3085 | 1 | 0 | 244014 | 27721728 |
| chiyoda_daily_projected_warm | 0 | 0.043 | 313 | 0 | 1 | 0 | 22183936 |
| kyoto_hourly_window | 0 | 0.442 | 5165 | 1 | 0 | 241264 | 28409856 |
| murodo_foliage | 0 | 0.585 | 1724 | 1 | 0 | 754432 | 31260672 |
| tokyo_sapporo_compare | 0 | 0.674 | 8647 | 1 | 1 | 244272 | 29212672 |
| tokyo_sapporo_compare_warm | 0 | 0.043 | 8640 | 0 | 2 | 0 | 24182784 |

The first two-place comparison reused the earlier Tokyo page and fetched Sapporo once; the warm repeat used zero HTTP requests. Projection reduced the warm three-day daily response from 3,080 to 313 bytes. Numbers are individual samples, not statistical guarantees.

## Reproduce

```bash
go build -o bin/tenki-pp-cli ./cmd/tenki-pp-cli
go test -count=1 ./...
go vet ./...
python3 scripts/live-acceptance.py --binary bin/tenki-pp-cli --output-dir verification-output/live-new
python3 scripts/measure.py --binary bin/tenki-pp-cli --output-dir verification-output/measure-new --cache-dir verification-output/page-cache
```

Choose new/empty output directories. The scripts retain captures and never clear an existing cache. Live assertions assume currently active foliage and ended current-year Sakura; seasonal/year changes produce explicit failures requiring review. macOS resource permission is needed for RSS; other platforms report it unmeasured.

## Tooling and limitations

The original installed Printing Press 4.32.5 was left unchanged. A workspace-local `4.32.5+local.verifier-fixes` corrected four proven verification defects: stdout/stderr mixing, hook command resolution, mandatory public-install prose for an unpublished local build, and a false critical error for an explicitly empty polling registry. [Local verifier provenance](.manuscripts/20260927-221457-5751f054/proofs/local-verifier-repairs.md) records the changed verifier files and positive/negative tests. The original run, receipts and source-bound acceptance marker were retained; pass fields were not hand-authored. The public SKILL uses the ordinary published-library prerequisite contract.

The original security scan found zero issues in custom weather/comparison code and twenty-two matches in the generated framework. The dated [scanner triage](.manuscripts/20260927-221457-5751f054/proofs/gosec-triage.json) retains those findings; most concern intended file/directory access, validated SQL identifiers or enum names. Its missing MCP HTTP request-header timeout is resolved for publication with a ten-second `ReadHeaderTimeout` and preservation metadata. The baseline scan is not a claim that all twenty-two findings remain present in the publication source. The CLI does not start the optional HTTP server, and HTTP hosting was outside the CLI live acceptance matrix. Dependency vulnerability validation passed during generation.

No API key or browser is required at runtime. Public HTML is undocumented, and [tenki.jp terms Article 8(6)](https://tenki.jp/docs/rule/) restrict non-browser/RSS acquisition. Technical access and public distribution do not establish provider authorization. Leisure search is bounded to reported directories. Seasonal/model update cadence is unverified; cache/source-age policies are explicit. Mountain municipal weather is foothill evidence; no summit forecast is invented. See [SKILL.md](SKILL.md) for interpretation and [README.md](README.md) for commands.

The curated publication archive contains curated research/proof files under [.manuscripts/20260927-221457-5751f054/](.manuscripts/20260927-221457-5751f054/). See the [live semantic matrix](.manuscripts/20260927-221457-5751f054/proofs/live-semantic-matrix.md), [efficiency evidence](.manuscripts/20260927-221457-5751f054/proofs/efficiency.md), [test summary](.manuscripts/20260927-221457-5751f054/proofs/test-summary-final.json), [publication preparation](.manuscripts/20260927-221457-5751f054/proofs/publication-preparation.md) and [archive sanitization record](.manuscripts/20260927-221457-5751f054/proofs/archive-sanitization.md). Full private originals were retained separately; the runnable scripts above reproduce the bounded checks from the shipped checkout.
