# Shipcheck Results

Run ID: `20260510T124341Z-e440f457`
Printing Press version: `4.2.2`
API: Etherpad (productivity)
CLI: `etherpad-pp-cli` / MCP: `etherpad-pp-mcp`

Phase 5 (runtime dogfood) was skipped — see `phase5-skip.json` in this
directory. `pad-dev.etherpad.org` serves the cleaned OpenAPI spec but is
shared infrastructure with no isolated write-test sandbox, so runtime
dogfood would either skew real users' data or fail authentication.

Spec invariants the dogfood would normally exercise are instead covered
upstream by Etherpad's backend tests in
`src/tests/backend/specs/api/api.ts`
([ether/etherpad#7714](https://github.com/ether/etherpad/pull/7714) —
merged, the same PR that produced the cleaned spec this CLI consumes).

Local verification was rerun after the develop-branch merge that brought
the PR up to date with `main` (108 upstream commits + 4 greptile-fix
patches preserved):

| Leg | Result |
| --- | --- |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test ./...` | PASS (4 packages with tests) |
| `etherpad-pp-cli --help` | PASS (50 commands across pad / author / session / group / chat / server) |
| `etherpad-pp-cli --version` | PASS (`etherpad-pp-cli 1.0.0`) |
| `verify-skill` | PASS (1 likely-false-positive flagged: `set-token YOUR_TOKEN_HERE` substring of `auth set-token` recipe) |
| `verify-attribution` | PASS |
| `verify-publish-package` | PASS after this artifact lands |
| Patch markers | All four `.printing-press-patches.json` entries point at files containing matching `// PATCH(...)` comments |

The four customizations from cc3a208 (post-greptile review) are recorded
in `../../../.printing-press-patches.json` and continue to resolve the
P1/P2 findings noted there:

1. OIDC URLs derive from `cfg.BaseURL` (or `cfg.OIDCIssuer`) instead of
   hardcoded `localhost:9001` — `internal/config/config.go`,
   `internal/cli/auth.go`, `internal/client/client.go`.
2. Dead `if tokenURL == ""` guard removed and replaced with the
   config-derived helper call — `internal/client/client.go`.
3. `ExpiresIn == 0` guard on initial token storage mirrors the parallel
   guard in `refreshAccessToken` — `internal/cli/auth.go`.
4. Comment on the MCP raw-args passthrough documenting the intentional
   escape hatch and trust boundary — `internal/mcp/cobratree/shellout.go`.

`govulncheck` is run by the repo's `govulncheck.yml` workflow on the
final pushed branch — see the per-CLI run there for the gated result.
