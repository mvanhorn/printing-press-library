# Verification

Verified 2026-09-28 UTC using anonymous Rakuten Travel requests, without API credentials or a browser. Go build, tests and vet pass. Scoped reachable-symbol govulncheck passes.

After the cache-write and receipt-option fixes, the full publish-time matrix passed 82 checks with zero failures; 53 auxiliary cases are explicitly skipped, including framework mutations and inapplicable positional/error probes. All six focused command happy paths passed. The separate semantic suite passed 56 assertions covering actual dated offers, exact identity, children, multiple rooms, pagination, comparisons, empty results and failure meanings.

Deterministic blocked-cache regressions cover all six focused commands, stderr-only warnings, silent `--no-cache`, and preservation of source errors.

Receipt regressions verify usage errors before source construction, with empty stdout and no artifacts, for all six focused commands. Dry runs and profile-applied options are covered.

## Measurements

Hotel 51870, 2026-11-08 to 2026-11-10, one room and two adults; five inventory results. One observation per scenario, not a statistical benchmark.

| Scenario | Output bytes | Requests | Wall seconds | Peak RSS MiB |
|---|---:|---:|---:|---:|
| Property, empty cache | 4,380 | 2 | 1.247 | 36.0 |
| Property, cache hit | 4,385 | 0 | 0.027 | 31.6 |
| Inventory, live | 9,950 | 1 | 1.061 | 30.0 |
| Inventory, explicit cache hit | 9,946 | 0 | 0.054 | 25.6 |

macOS `/usr/bin/time -l` measured individual-process RSS in bytes. Wall time includes startup/parsing. Request counts include attempts; response bytes count bodies delivered by Go HTTP rather than compressed wire traffic. Metadata caching and explicit inventory caching preserve the original observation timestamp; live inventory caching is off by default.

## Reproduce

```sh
go build -o bin/rakuten-travel-pp-cli ./cmd/rakuten-travel-pp-cli
go test -count=1 ./...
python3 scripts/live_verify.py ./bin/rakuten-travel-pp-cli /tmp/rakuten-live-proof --checkin 2026-11-08
python3 scripts/measure.py ./bin/rakuten-travel-pp-cli /tmp/rakuten-measure-proof --checkin 2026-11-08
```

Use future dates. Actual-offer assertions require nonempty inventory; sold-out dates cannot pass those assertions. Deterministic tests use synthetic responses; optional local capture tests require TRAVEL_CAPTURE_DIR and are skipped otherwise.

Evidence: [semantic assertions](evidence/live-semantic.json), [measurements](evidence/measurements.json), [acceptance receipt](evidence/phase5-acceptance.json), and [publication verification](evidence/publish-verification.md). Raw diagnostic transcripts and downloaded reference mirrors are intentionally excluded from the public package.

Gosec v2.26.1 has zero findings in hand-authored Travel code. Twenty generated-framework findings were reviewed and recorded in the [security review](evidence/security-review.md); this is not a claim of a globally clean scanner report.
