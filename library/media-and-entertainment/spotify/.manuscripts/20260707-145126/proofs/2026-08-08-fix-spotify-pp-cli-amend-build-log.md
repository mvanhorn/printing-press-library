# spotify-pp-cli Amend Build & Verification Log — head 6944c8d5b (2026-08-08)

Raw transcripts of every verification leg run against the PR branch. Summary and
finding-by-finding mapping: [`2026-08-08-fix-spotify-pp-cli-amend-acceptance.md`](2026-08-08-fix-spotify-pp-cli-amend-acceptance.md).

## Build and vet
```
$ go build ./...
exit=0

$ go vet ./...
exit=0
```

## Test suite
```
$ go test ./...
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/cmd/spotify-pp-cli	[no test files]
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/cmd/spotify-pp-mcp	[no test files]
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cache	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cli	1.430s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/client	(cached)
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cliutil	(cached)
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/config	(cached)
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/mcp	0.773s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/mcp/bound	(cached)
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/mcp/cobratree	3.042s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/store	(cached)
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/types	[no test files]
exit=0
```

## verify_skill and supply-chain scan
```
$ python3 .github/scripts/verify-skill/verify_skill.py --dir library/media-and-entertainment/spotify/
=== spotify ===
  ✘ 0 error(s), 2 likely false-positive(s)
    [positional-args] spotify-pp-cli auth: got 2 positional args; Use: "auth" expects 0–0  [likely false positive]
      evidence: SKILL.md: set-token YOUR_TOKEN_HERE
    [positional-args] spotify-pp-cli auth: got 2 positional args; Use: "auth" expects 0–0  [likely false positive]
      evidence: README.md: set-token YOUR_TOKEN_HERE
exit=0

$ python3 .github/scripts/verify-supply-chain/scan.py --base-ref upstream/main
supply-chain scan: no findings.
exit=0
```

## govulncheck
```
$ govulncheck ./...
No vulnerabilities found.
```

## Live read-only smoke (real credentials)
```
$ spotify-pp-cli search "radiohead" --data-source live --agent --limit 3   # F1/F2/F5: one envelope, real source
top-level keys: ['meta', 'results']
meta.source: live
double-wrapped envelope: False

$ spotify-pp-cli artists get-an 4Z8W4fKeB5YxbusRsdQVPb --json   # F3: whole resource, not an image-array fragment
meta.source: live
results is a dict (artist object), not a list: True
artist fields present: ['id', 'name', 'type', 'uri']

$ spotify-pp-cli artists get-an ... --agent --select name,followers.total   # F4: --select descends nested envelope
{
  "meta": {
    "source": "live"
  },
  "results": {
    "name": "Radiohead"
  }
}

# Note: the live /v1/artists/{id} payload for this app tier omits followers/genres/popularity
# (Spotify metadata restrictions for newer apps). F3's claim is proven by the shape: the CLI
# returns the artist *object* the API serves, where the pre-fix binary returned its image array.
# F4's nested-envelope descent is pinned by the unit suite (root_test.go, single_object_response_path_test.go).
```
