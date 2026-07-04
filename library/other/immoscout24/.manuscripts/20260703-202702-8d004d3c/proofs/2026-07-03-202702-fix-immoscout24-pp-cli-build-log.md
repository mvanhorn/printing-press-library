# ImmoScout24 CLI Build Log

Run: `20260703-202702-8d004d3c`
Updated: `2026-07-03T18:49:29Z`

## Built
- Generated a Go CLI for the replayable ImmoScout24 mobile JSON API.
- Endpoints: `immoscout24-mobile-search total`, `immoscout24-mobile-search list`, `immoscout24-mobile-search map`, and `expose`.
- Added mobile API default headers: `Accept: application/json` and `User-Agent: ImmoScout_27.12_26.2_._`.
- Replaced the generated Surf transport with the standard Go HTTP client because the mobile host works over direct HTTP and Surf returned empty bodies for this API.
- Made `immoscout24-mobile-search` visible in root help.
- Changed `list` JSON output from mutation-style `action: post` to read-style `meta` + `results`, and removed the misleading `create` alias.
- Changed GET command JSON selection for `total`, `map`, and `expose` so `--select results.*` applies after the provenance envelope is added.
- Added live-check fixtures for Berlin mobile searches and a real expose ID.
- Added sync defaults for the mobile map endpoint and normalized marker IDs from `objects[0].id` so the local store can sync live data.

## Deferred
- Web URL translation, saved search watching, expose digesting, and search diffing remain follow-up workflow commands. The shipped CLI covers the confirmed mobile endpoint surface first.

## Verification
- `go test ./...`: PASS.
- `go build -o ./immoscout24-pp-cli ./cmd/immoscout24-pp-cli`: PASS.
