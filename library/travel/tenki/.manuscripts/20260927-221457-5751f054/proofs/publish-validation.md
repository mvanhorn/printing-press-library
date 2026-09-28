# Publication validation — 2026-09-28 Asia/Tokyo

Validated the packaged checkout with module `github.com/mvanhorn/printing-press-library/library/travel/tenki` after publication source maintenance and module rewriting.

- Canonical `publish validate`: PASS for all thirteen checks, including manifest, source-bound Phase 5 evidence, module path, tidy, reachable vulnerability scan, vet, build, help/version, skill recipes, patches and manuscripts.
- Repository-owned skill verifier: all five checks passed; 21 recipes; zero findings.
- `go test -count=1 -json ./...`: 725 passing test/subtest events (369 top-level tests), 11 packages passed, zero failures. Two intentional test skips cover rollback-journal behavior under the WAL profile and a Windows-only permission retry; five additional packages have no tests.
- `go build ./...` and `go vet ./...`: PASS.
- `govulncheck@v1.3.0 ./...`: no vulnerabilities found.
- Fresh full live dogfood: 58 executed checks passed, zero failed, 47 inapplicable or guarded framework probes skipped. Reran after the review fixes on 2026-09-28 Asia/Tokyo. The tool wrote the source-bound `phase5-acceptance.json`; no acceptance fields were manually authored.
- Pre-review publication gosec scan: 21 generated-framework findings, zero custom weather/planning findings. The original missing MCP HTTP header timeout is fixed; no G112 remains. The dated 22-finding baseline remains historical evidence, not the final scan count.
- Mandatory package secret scan and contextual privacy review passed. Only source, dated parser fixtures and curated research/proof files are published; runtime captures and private paths remain local.

The original semantic matrix and efficiency measurements remain separately dated. This record does not relabel fixture assertions as live checks or claim provider authorization for undocumented HTML access.

## Review corrections

The first review identified inconsistent seasonal-year bounds and ambiguous request-rate scope. The provider, product commands and planner now share 1900–2200 validation. Dated fixtures cover both seasonal products and real-provider command list/show/compare at 1900, 1999, 2000, 2100, 2101 and 2200; accepted years return `year_unavailable` with the actual source year. Invalid 1899/2201 requests fail before acquisition. The regressions reproduced the old acquisition errors before the fix and pass afterward.

Help, provider comments, README, agent skill and provider contract now explicitly describe per-client pacing. Separate CLI/MCP invocations do not share a limiter; callers should serialize source calls. The accepted resolution clarifies the existing scope rather than introducing a cross-process coordinator.

Focused live boundary checks also passed after the fix: `seasonal list --year 1900`, `seasonal show --year 2200`, and seasonal `compare --year 2101`. The first two returned `year_unavailable`; comparison had no fetch failures, `insufficient_data`, and seasonal verdict `unknown`. Each preserved source year 2026 separately from the requested year. Three serial commands used a shared isolated cache and at least one-second pauses; each made one HTTP request. Private raw captures are excluded from this public archive.

## Publishing identity correction

At the creator's request, publication attribution and fork ownership were corrected to `zjsng`. At the identity-only replacement commit, the implementation and its source fingerprint were unchanged from the reviewed source (`701d0845247cc80c06b69e290bf439b35500a22ffde0e0d6a26a5e54b89ff3ff`); the 717-test result described those same Go files. The later selector correction and its validation are recorded below. All thirteen canonical publication checks and the full 58-check live gate passed again, completing at 2026-09-28T02:07:00.873597+00:00. The previous PR retains its review history; the replacement PR uses the intended publishing account.

## Selector review correction

The replacement PR review found that preliminary validation could accept an unanchored metadata selector that the renderer rejected, writing a full JSON response alongside the error. The preliminary guard now uses the renderer's own matching rules; invalid selectors exit 2 before stdout. Fully qualified metadata paths, supported list-envelope shorthand and anchored empty arrays remain supported; unrelated empty arrays cannot hide total misses. The regression reproduced the old full-output error and passes with the fix.

That source passed 723 test/subtest events (369 top-level tests, 11 tested packages), build and vet, all thirteen canonical publication checks, and a fresh full live gate (58 executed checks passed, zero failed, 47 guarded/inapplicable skips), completing at 2026-09-28T02:20:32Z. The current source-bound acceptance marker is authoritative.

A subsequent review caught nonempty selector lists containing only whitespace/commas being treated as no selection. The guard again rejects them with exit 2 and empty stdout; two regression cases cover this restoration. The final source passed 725 test/subtest events (369 top-level, 11 packages), build/vet, all thirteen publication checks and a fresh full live gate (58 passed, zero failed, 47 guarded/inapplicable skips), completed at 2026-09-28T02:30:12.059532+00:00.
