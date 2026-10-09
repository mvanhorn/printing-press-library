# Build log notes
- Generate (2026-10-09): code written; post-merge gate govulncheck failed because stdlib go1.26.6 (pinned go directive) has GO-2026-66xx net/http, mime/multipart, crypto/tls vulns fixed in go1.26.9. Fix: go.mod `go 1.26.9`; `GOTOOLCHAIN=go1.26.9 govulncheck ./...` -> No vulnerabilities found. Press-level finding for retro: the generator's go directive floor (1.26.6) is below the current patched release.
- Spec: internal YAML with raw Queue-Times `source parks` and `source queue-times`; TDR and ThemeParks.wiki accessed by hand-written clients (TDR needs Chrome TLS; spec transport is standard).
- Phase 12 (2026-10-09): shipcheck scorecard leg held with 'manifest evidence unavailable' because the failed-validation generate run (govulncheck exit 3) returned before WriteManifestForGenerate, so .printing-press.json, spec.yaml, manifest.json, CHANGELOG.md and .printing-press-patches/ were never written. Fix: regenerated the same spec with --validate=false into a scratch dir and copied only those five files. Press-level finding for retro: write the manifest (or a clear 'manifest missing' error) even when post-generate validation fails.
- Phase 4.95 (2026-10-09): go.mod now `go 1.26.9` + `toolchain go1.27.2`, golang.org/x/net v0.60.0; `GOTOOLCHAIN=auto govulncheck ./...` -> 0 called vulnerabilities. Generated internal/client body read bounded with io.LimitReader. surf client uses SecureTLS (surf defaults to InsecureSkipVerify=true).

## Phase 18 dogfood finding (Printing Press issue)

- Generated GET handlers (source parks, source queue-times) wrapped the client
  dry-run sentinel `{"dry_run": true}` in the results/meta envelope. Live
  dogfood requires top-level `dry_run:true` and a non-empty `action`. Fixed in
  the handlers with `writeDryRun(..., "GET "+path)`. The generator template
  should do this.
