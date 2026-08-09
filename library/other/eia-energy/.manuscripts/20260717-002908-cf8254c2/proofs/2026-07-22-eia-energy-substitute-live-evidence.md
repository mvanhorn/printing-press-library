# EIA Energy substitute live evidence — refreshed 2026-08-09

## Credential boundary

`EIA_API_KEY` was not present in the validation environment. EIA's official API documentation requires a free individual key for normal use, so this report does not claim credentialed Phase 5 acceptance. The documented registration and API contract are at <https://www.eia.gov/opendata/documentation.php>.

The non-secret `DEMO_KEY` endpoint behavior was used only as substitute read-only evidence. A direct metadata request to `https://api.eia.gov/v2/electricity/retail-sales/?api_key=DEMO_KEY` returned HTTP 200 with response ID `retail-sales`; no response payload or credential was archived.

## Current-head live read

Validated code commit `c070a81e1c967d13a6b9664f147d9a5a5555c1c5`
was rebuilt and exercised through the shipped CLI with the non-secret demo key:

```text
EIA_ENERGY_API_KEY=DEMO_KEY eia-energy-pp-cli doctor --json --no-cache
PASS: API reachable; canonical env credential recognized

EIA_ENERGY_API_KEY=DEMO_KEY eia-energy-pp-cli electricity --length 1 --json --no-cache --no-learn
PASS: live source returned one hourly EIA-930 balancing-authority row
      with period, respondent, type, value, and value-units
```

The returned row was displayed only in the validation terminal and is not
committed. This verifies current-head auth wiring, request construction,
upstream reachability, response decoding, and JSON output end to end.

## Canonical shipcheck

The current PR tree was built and checked with Printing Press 4.29-compatible tooling:

```text
cli-printing-press shipcheck --dir <eia-energy> --research-dir <run> --no-fix --json
passed: true
exit_code: 0
verify: PASS (35/35, 100%)
validate-narrative: PASS
dogfood: PASS
workflow-verify: PASS
apify-audit: PASS
verify-skill: PASS
scorecard --live-check: PASS (85%, Grade A)
```

The live scorecard leg ran with `EIA_API_KEY=DEMO_KEY`. The generated CLI binary was built before shipcheck so full narrative examples executed against the current source rather than a missing or stale binary.

## Full live-matrix attempt and fixes

The binary-owned full Phase 5 matrix was attempted with the same read-only demo access. It enumerated 83 tests and passed 73 before reporting 10 failures:

- Six failures were missing `Examples:` sections on the novel command help surfaces (`anomaly`, `grid pulse`, `revisions`, `spread`, `state compare`, and `watch run`). Those defects were fixed in this PR, and all six help-contract checks now pass locally.
- Four failures were HTTP 4xx responses after EIA returned `OVER_RATE_LIMIT` for the shared demo key. The CLI surfaced the upstream 429 and retry delay rather than treating it as energy data.

Because the shared demo key was throttled, the failed acceptance marker is intentionally not presented as a Phase 5 pass and is not included in the publication package. This evidence is a transparent substitute pending maintainer or contributor access to an individual EIA key.

## Local validation after fixes

```text
go test ./...: PASS (576 named tests; 8 service-shaped HTTP test servers)
go vet ./...: PASS
go build ./...: PASS
verify-skill --strict: PASS
all six novel-command help surfaces contain Examples: PASS
```

No secret value, private account identifier, or full live API payload is included in this proof.
