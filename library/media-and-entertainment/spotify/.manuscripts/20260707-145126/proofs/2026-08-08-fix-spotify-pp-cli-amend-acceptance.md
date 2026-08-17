# spotify-pp-cli Amend Acceptance Report — fix/spotify-extraction-search-select (PR #1625)

Scope: this is the verification evidence for the response-handling amend at head `6944c8d5b`
(PR mvanhorn/printing-press-library#1625), not a reprint. The base print evidence for this CLI
lives alongside in `phase5-acceptance.json` / `publish-live-gate.json` (run `20260707-145126`,
press 4.27.1). The amend changes no dependencies and no MCP surface.

```
Amend Acceptance: spotify
  Level: full module test suite + live read-only smoke (real credentials)
  Tests: go test ./... — 9 packages PASS, 0 failed
  Auth: user OAuth token via credentials.toml (doctor: auth configured, api reachable)
  Gate: PASS
```

## Finding-by-finding verification

| ID | Contract | Evidence |
|----|----------|----------|
| F1 | live `/search` sends the required `type` parameter | live smoke: `search "radiohead" --data-source live` returns HTTP 200 with populated results (published binary returns HTTP 400); pinned by `search_live_type_test.go` |
| F2 | provenance envelope added exactly once, real source | live smoke: top-level keys `{meta, results}`, `meta.source=live`, no nested `{meta,results}` wrapper inside `results` |
| F3 | single-object endpoints return the whole resource | live smoke: `artists get-an --json` returns the artist object the API serves (dict, not the pre-fix image-array fragment); pinned by `single_object_response_path_test.go` |
| F4 | `--select` descends nested object envelopes | `--select name` projects through the envelope on a live response; descent through nested envelopes pinned by `root_test.go` |
| F5 | degraded multi-type search keeps partial results | pinned by `search_live_type_test.go` (per-type 400 counted as market rejection, `meta.reason` reports the incomplete set) |
| F6 | `--data-source live` fails fast (exit 2) on local-only types | pinned by `search_live_type_test.go` ("no live Spotify search" path) |
| F7 | mutating commands report `source=live` | pinned by `mutation_provenance_test.go` |

Live-probe caveat, disclosed: the `/v1/artists/{id}` payload for this app tier omits
`followers`/`genres`/`popularity` (Spotify metadata restrictions for newer apps), so F3 is
proven by response shape (object vs. fragment) and by the unit suite, not by field count.

## Verification legs (all PASS)

- `go build ./...` — exit 0
- `go vet ./...` — exit 0
- `go test ./...` — 9 packages ok, 0 failures
- `verify_skill.py --dir library/media-and-entertainment/spotify/` — 0 errors (2 pre-existing
  `[likely false positive]` positional-arg notes, byte-identical on `main`)
- `verify-supply-chain/scan.py --base-ref upstream/main` — no findings (covers the go.mod
  require-block consolidation in `6944c8d5b`)
- `govulncheck ./...` — no vulnerabilities found

Raw command transcripts: [`2026-08-08-fix-spotify-pp-cli-amend-build-log.md`](2026-08-08-fix-spotify-pp-cli-amend-build-log.md)

## Fixtures used

- Live read-only GETs only: `search "radiohead"` and `artists get-an 4Z8W4fKeB5YxbusRsdQVPb`
  (Radiohead's public artist ID). No mutating command was executed against the live API;
  mutation provenance (F7) is covered by the unit suite.
