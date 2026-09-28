# serply-pp-cli build log

Manifest transcendence rows: 3 planned, 3 built.

## What was built

- Priority 0/1: generator output for 9 Serply verticals (web, news, scholar, images, videos, bing, job-search, products, maps). All use the query-string form (`/v1/search?q=...`); maps keeps the documented path segment (`/v1/maps/search/{query}`).
- Priority 1 review gate: `news`, `scholar`, `maps` checked with --help, --dry-run and --dry-run --json. Hand-added Example lines to the 7 promoted commands the generator left without one (scholar, images, videos, bing, job-search, products, maps).
- Priority 2 (hand-built, Go, tests in `internal/cli/serp_novel_test.go`):
  - `rank <domain> --q ...` (internal/cli/rank.go): first organic position of a domain or its subdomains, with location/device/gl/hl. Not found is exit 0 with a note.
  - `serp diff --q ...` (internal/cli/serp_diff.go): stores up to 10 snapshots per query+location+device+gl+hl under the data dir (0600 files, atomic writes); reports entered/left/moved/unchanged. `--offline` compares the last two stored runs with no credit spent; `--no-save` skips storing.
  - `research <topic>` (internal/cli/research.go): concurrent web+news+scholar fan-out, URL dedup, numbered sources, markdown brief or JSON. Partial vertical failures land in `fetch_failures`.
- Shared helpers: internal/cli/serp_common.go (params/headers, response parsing per vertical key, domain and link normalization).

## Completion gate

- Per-row Cobra resolution: rank, serp diff, research each print `serply-pp-cli <leaf> ... [flags]`.
- `dogfood --research-dir`: novel_features_check planned 3 found 3, no missing, not skipped.
- `go vet ./...` and `go test ./...` clean.

## Intentionally deferred

- Brainstorm kills: bulk rank, geo compare, credit meter, PAA harvester (see research brainstorm).
- scrape_url and Reddit endpoints are not in the public docs spec surface used here; not absorbed.

## Skipped body fields

- None: every endpoint is GET with query or path parameters.

## Generator limitations found

- `rank --dry-run` initially printed help: the scaffold's "no local flags" help check ignores persistent flags, so `--dry-run` alone looked like a bare call. Switched the check to `cmd.Flags().NFlag() == 0`.
- Generator emits an em dash in the root Short ("Serply CLI — ..."); left as generated.
- 4 dead generated helpers reported by dogfood (handleBinaryResponseDelivery, retainCLIQueryParams, retainExplicitQueryParams, successfulNoop); generator-owned, left in place.
- `defaultSyncResources` is empty: the API is stateless search, so there is nothing to sync; no novel command depends on the store.
