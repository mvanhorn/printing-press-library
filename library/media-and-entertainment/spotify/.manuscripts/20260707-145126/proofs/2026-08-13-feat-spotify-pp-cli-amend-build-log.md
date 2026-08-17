# spotify-pp-cli Amend Build Log — amend/spotify-20260813

Head `28cccd8d41`. Finding-by-finding mapping:
[`2026-08-13-feat-spotify-pp-cli-amend-acceptance.md`](2026-08-13-feat-spotify-pp-cli-amend-acceptance.md).

## Build and vet

```
$ go build ./...
(no output — PASS)

$ go vet ./...
(no output — PASS)
```

## Tests

```
$ go test ./...
ok    .../internal/cli
ok    .../internal/client
ok    .../internal/cliutil
ok    .../internal/config
ok    .../internal/mcp
ok    .../internal/mcp/bound
ok    .../internal/mcp/cobratree
ok    .../internal/store
?     .../cmd/spotify-pp-cli       [no test files]
?     .../cmd/spotify-pp-mcp       [no test files]
?     .../internal/cache           [no test files]
?     .../internal/types           [no test files]
```

Three existing `filterFields` cases asserted the old silent-empty contract for a total `--select`
miss. They were updated to the new one, each carrying a note saying the change is deliberate.
`root_test.go` adds coverage for the miss diagnostic, the capped `available_fields` list, and the
untouched partial-match path; `transcendence_tracks_resolve_test.go` covers the ranking ladder and
stdin batching.

## Vulnerabilities

```
$ govulncheck ./...
No vulnerabilities found.
```

## Skill verification

```
$ python3 .github/scripts/verify-skill/verify_skill.py --dir library/media-and-entertainment/spotify/
0 error(s), 2 likely false-positive(s)
  [positional-args] spotify-pp-cli auth: got 2 positional args  [likely false positive]
    evidence: SKILL.md: set-token YOUR_TOKEN_HERE
  [positional-args] spotify-pp-cli auth: got 2 positional args  [likely false positive]
    evidence: README.md: set-token YOUR_TOKEN_HERE
```

Both false positives predate this branch: `auth set-token` is a subcommand the checker reads as
positional arguments to `auth`.

## Dependencies

`go mod tidy` produces no diff against `main`. The amend adds no imports beyond the standard library
and packages the CLI already depends on, so `go.mod` and `go.sum` are deliberately absent from this
PR rather than missing from it.

## Publish validation

`publish validate` reports `phase5 marker missing source_fingerprint` and `install section drift`.
Both reproduce identically on pristine `upstream/main` for this CLI, so neither is introduced here.
