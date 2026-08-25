# spotify-pp-cli Amend Acceptance Report — amend/spotify-20260813

Scope: verification evidence for the `--select` and `tracks resolve` amend at head `28cccd8d41`,
not a reprint. Base print evidence for this CLI lives alongside in `phase5-acceptance.json` /
`publish-live-gate.json` (run `20260707-145126`, press 4.27.1). The amend adds no dependencies —
`go mod tidy` is a no-op against `main`, so `go.mod` and `go.sum` are unchanged — and touches no
MCP surface.

```
Amend Acceptance: spotify
  Level: full module test suite + live smoke against real credentials
  Tests: go test ./... — 8 packages ok, 0 failed
  Auth: user OAuth token via credentials.toml (doctor: auth configured, api reachable)
  Gate: PASS
```

## Finding-by-finding verification

| ID | Contract | Evidence |
|----|----------|----------|
| F1 | a `--select` matching nothing is an error, not a full payload | live: `tracks get 2txSWiEp1BDIIb6irXJIcf --agent --select pippo.pluto` exits 2 and returns a 602-byte diagnostic; the published binary returns the whole 2336-byte track at exit 0 |
| F1 | the diagnostic reaches a human, not only a parser | same run writes 177 bytes to stderr naming the spec and pointing at `available_fields`; stdout keeps the machine-readable envelope |
| F1 | partial and valid selections are untouched | `--select name,uri` still projects two fields at exit 0 with an empty stderr; pinned by `root_test.go` |
| F3 | artist + title resolves to exactly one URI | live: `tracks resolve --artist Northlane "4D"` returns one row, `match_kind=exact`, `candidates=7` |
| F3 | ranking beats relevance order on title exactness | live: `Cut it` resolves to `CUT_it` at `match_kind=normalized` — an exact-string matcher misses this row, which is what the originating session hand-wrote in `jq` and got wrong |
| F3 | a whole tracklist resolves in one invocation | live: 14-line setlist on stdin → `resolved=14 missed=0` in 3.3s wall clock |
| F3 | progress is visible to a human and invisible to a pipe | live: same batch under a pty writes `resolving N/14: <title>` to stderr; through a pipe stderr is 0 bytes |
| F2 | playlist rows carry the track under `item` | live: `playlists items get-playlists <id> --agent` → `has_track=false`, `has_item=true`; documented in SKILL.md and README.md |
| F2 | the legacy `/tracks` subtree is unusable | live: `DELETE /playlists/{id}/tracks` answers HTTP 403 for current app credentials |

## Findings dropped after re-validation against `main`

Two findings from the originating capture no longer reproduce and ship no code:

- `--compact` nulling playlist track objects — fixed by #1625's keep-rule for keys present in most rows.
- `--public=false` not reading back on `change-details` — fixed by #1674's scoped cache invalidation. The
  original symptom was a stale read, not a lost write; `--no-cache` did not defeat it, which is why it
  survived several sessions as a suspected write bug.

One finding is deferred: `spotify-web-search` still logs a `no extractable ID field` cache warning on
every call. It is noise in an otherwise correct path, and folding it into this PR would mix a store-layer
change into a CLI-layer amend.

## Live smoke transcript

```
$ spotify-pp-cli tracks get 2txSWiEp1BDIIb6irXJIcf --agent --select "pippo.pluto"
exit=2  stdout=602B  stderr=177B
error: --select "pippo.pluto" matched no fields; available_fields on stdout lists what this
payload offers (paths are relative to the payload, not the {meta, results} envelope)

$ spotify-pp-cli tracks get 2txSWiEp1BDIIb6irXJIcf --agent --select name,uri
exit=0  stderr=0B
{"meta":{"source":"live"},"results":{"name":"4D","uri":"spotify:track:2txSWiEp1BDIIb6irXJIcf"}}

$ printf '4D\nEvian\n...\nWelcome to the Industry\n' | spotify-pp-cli tracks resolve --artist Northlane --agent
resolved=14 missed=0            # 3.342s total

$ printf 'Cut it\n' | spotify-pp-cli tracks resolve --artist Northlane --agent
normalized  CUT_it  CUT_it  spotify:track:20UAoMZQ3XFRrc1U7eGDgB
```
