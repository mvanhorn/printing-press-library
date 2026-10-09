# Phase 4.95 local code review: japan-timed-tickets-pp-cli (run 20261009-095850-f6dfc042)

## Autofix summary
About 50 findings were fixed in place across 3 rounds (round 1: correctness 1-9, security 1-5, maintainability 1-19; round 2: 5 correctness, 2 security, 4 maintainability; round 3: 6 low text/consistency items), followed by a /simplify pass. The working dir has no git repo, so there are no commit hashes; the edited files are the authoritative record. Generated-file edits are recorded in `.printing-press-patches/client-body-limit.json` and `go-mod-govulncheck-floor.json`.

## Template-shape retro candidates
- `internal/client/client.go:1416,1748` (medium, security). Template: `internal/generator/templates/client.go.tmpl`. The generated client used `io.ReadAll(resp.Body)` with no limit. Patched here with `io.LimitReader(..., maxDecodedBodyBytes+1)` because the orchestrator asked for it; it is recorded as a patch so it survives regen. The machine should emit the bound.
- `go.mod` (medium, security). Template: the go.mod emit. The generated module failed govulncheck (old `golang.org/x/net`, toolchain below the fixed Go release). Fixed with `golang.org/x/net v0.60.0` and `toolchain go1.27.2`. When the gate failed, generate stopped before it wrote `.printing-press.json`/`manifest.json` (recovered with a scratch regen).
- `internal/cli/helpers.go` `printPlain`/`printCSV` (`formatTabularCell`, `plainCellValue`) (low, security). They do not remove control characters from remote fields in any printed CLI. This CLI cleans remote strings at parse time instead.
- `internal/cli/root.go:288` dead `--max-age` flag and 8 dead helpers in `internal/cli/helpers.go` (low, maintainability) on a CLI with no store or typed endpoints (dogfood WARN).
- `tools-manifest.json` still lists `source_ghibli-calendar` when `mcp.endpoint_tools: hidden` (low). The runtime `RegisterTools` does not add it, so only the manifest is wrong.
- README/SKILL generator-synced header "aren't available in any other tool for this API" (low, wording) on a multi-source CLI.
- `--select results.x` nests when the source is computed; recipes use bare field names (low, docs).
- `deliver.go`/`feedback.go` send requests to URLs the user configures, outside the paced fetcher (informational).

## Out-of-scope retro candidates
- None in `internal/cliutil/` or `internal/mcp/cobratree/`. Informational only: the per-host rate limit and request budget apply to one process, so parallel MCP mirror calls do not share the 2 rps limit (each call still has its own budget).

## Surface-to-user findings
- None. Skipped /simplify suggestions (refactors with no behaviour gain, noted only): one `sd.read(url, note, fetchFn)` for all source reads, one central `SightData.sanitize()` in place of parse-point StripControl calls, a typed `Slot.StartAt` for the past-slot filter, an exported `buildRow` for availability in place of `BuildOnsale`, a parallel museum/Lawson read for Ghibli, reuse of the generated doctor probe response.

## Convergence outcome
Findings cleared at round 3: the round-3 findings were all low (help text, URL constants, comments, warning sanitising, StartJST fallback). They were fixed and checked with tests, a live smoke run and shipcheck (7/7 PASS, 80/100).

## Review path chosen
Direct subagent dispatch (Agent tool): correctness, security and maintainability reviewers in parallel each round; then `/simplify` (reuse, simplification, efficiency, altitude agents).
