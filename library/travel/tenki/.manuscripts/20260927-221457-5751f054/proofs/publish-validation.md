# Publication validation — 2026-09-28 Asia/Tokyo

Validated the packaged checkout with module `github.com/mvanhorn/printing-press-library/library/travel/tenki` after publication source maintenance and module rewriting.

- Canonical `publish validate`: PASS for all thirteen checks, including manifest, source-bound Phase 5 evidence, module path, tidy, reachable vulnerability scan, vet, build, help/version, skill recipes, patches and manuscripts.
- Repository-owned skill verifier: all five checks passed; 21 recipes; zero findings.
- `go test -count=1 -json ./...`: 677 passing test/subtest events (366 top-level tests), 11 packages passed, zero failures. Two intentional test skips cover rollback-journal behavior under the WAL profile and a Windows-only permission retry; five additional packages have no tests.
- `go build ./...` and `go vet ./...`: PASS.
- `govulncheck@v1.3.0 ./...`: no vulnerabilities found.
- Fresh full live dogfood: 58 executed checks passed, zero failed, 47 inapplicable or guarded framework probes skipped. Run at 2026-09-27T16:21:00Z (2026-09-28T01:21:00+09:00). The tool wrote the source-bound `phase5-acceptance.json`; no acceptance fields were manually authored.
- Publication gosec scan: 21 generated-framework findings, zero custom weather/planning findings. The original missing MCP HTTP header timeout is fixed; no G112 remains. The dated 22-finding baseline remains historical evidence, not the final scan count.
- Mandatory package secret scan and contextual privacy review passed. Only source, dated parser fixtures and curated research/proof files are published; runtime captures and private paths remain local.

The original semantic matrix and efficiency measurements remain separately dated. This record does not relabel fixture assertions as live checks or claim provider authorization for undocumented HTML access.
