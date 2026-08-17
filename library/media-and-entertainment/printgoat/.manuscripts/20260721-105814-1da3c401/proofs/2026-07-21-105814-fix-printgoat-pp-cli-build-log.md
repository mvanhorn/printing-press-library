Manifest transcendence rows: 10 planned, 10 built. Phase 3 will not pass until all 10 ship.

## Build summary

- Generated scaffold from 3 merged internal YAML specs (Printables `kind: synthetic` primary, Thingiverse REST, Cults3D GraphQL) via `cli-printing-press generate`.
- Fixed a real generator limitation: multi-spec merge silently drops the primary spec's `tier_routing` block (confirmed in `internal/cli/root.go`'s `mergeSpecsWithOptions`), so per-source auth (Thingiverse Bearer, Cults3D Basic) would otherwise never apply to generated commands. Fixed via a new hand-written `internal/client/host_auth.go` dispatching auth by target URL host, plus a 1-line edit to `client.go`'s `doInternal` to call it.
- Built all 19 absorbed-feature rows and all 10 novel transcendence commands from the absorb manifest, plus 2 commands (`browse category`/`browse user`, `search --interactive`) added after an initial Phase 3 Completion Gate check found them missing against the approved manifest.
- Two real bugs found and fixed via live API testing (not assumed from docs):
  1. Printables' `getDownloadLink` mutation needed `ids: [ID]!` grouped per lowercase file type, not the singular/uppercase shape from research.
  2. Printables' CDN 403s Go's default User-Agent on signed download links; fixed with a realistic browser UA.
  3. Printables' `ratingAvg` field returns a JSON string despite being schema-typed as Float; added a `flexNumber` type tolerating both encodings.
- Real HTTP byte-range resumable downloads implemented (not the "skip if filename exists" pattern common in competing tools) — verified against live Printables (GCS-backed, honors Range/206) and Thingiverse (CDN ignores Range, safe-restart fallback discards stale partials rather than corrupting).

## Phase 3 Completion Gate results

- Per-row Cobra resolution check: 22/22 approved commands resolve via `<leaf> --help` (exit 0, correct `Usage:` line).
- Deterministic dogfood backstop: `novel_features_check: {"planned": 10, "found": 10}`.
- `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...` all clean.

## Known gaps (documented, not silent)

- Cults3D file downloads are unsupported by upstream API design (search/metadata only) — permanent, not a bug.
- `job download`/`job resume` can only fetch real bytes for Thingiverse files today; Printables/Cults3D files in a job are recorded but marked `unsupported_source` (Printables needs a separate `getDownloadLink` resolution step not yet wired into the job engine).
- Printables/Cults3D category and user browsing are best-effort where no confirmed query existed at research time; `browse category printables` and `browse user printables` return an honest "not supported" message rather than a guess.
- Cults3D live testing is limited: only the API key was available, not the account handle (`CULTS3D_USERNAME`) needed to complete HTTP Basic Auth, so Cults3D code paths are exercised via error-path/graceful-degradation testing rather than full live confirmation.
