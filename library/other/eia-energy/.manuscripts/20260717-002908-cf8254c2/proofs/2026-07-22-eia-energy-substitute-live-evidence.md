# EIA Energy substitute live evidence — 2026-07-22

## Credential boundary

`EIA_API_KEY` was not present in the validation environment. EIA's official API documentation requires a free individual key for normal use, so this report does not claim credentialed Phase 5 acceptance. The documented registration and API contract are at <https://www.eia.gov/opendata/documentation.php>.

The non-secret `DEMO_KEY` endpoint behavior was used only as substitute read-only evidence. A direct metadata request to `https://api.eia.gov/v2/electricity/retail-sales/?api_key=DEMO_KEY` returned HTTP 200 with response ID `retail-sales`; no response payload or credential was archived.

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
go test ./internal/cli ./internal/client ./internal/config: PASS
go vet ./...: PASS
go build ./...: PASS
all six novel-command help surfaces contain Examples: PASS
```

No secret value, private account identifier, or full live API payload is included in this proof.
