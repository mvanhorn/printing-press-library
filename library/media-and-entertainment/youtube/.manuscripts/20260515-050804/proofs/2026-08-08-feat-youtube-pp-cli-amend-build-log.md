# youtube-pp-cli Amend Build & Verification Log — head d911bbf8c (2026-08-08)

Raw transcripts of every verification leg run against the PR branch. Summary and
finding-by-finding mapping: [`2026-08-08-feat-youtube-pp-cli-amend-acceptance.md`](2026-08-08-feat-youtube-pp-cli-amend-acceptance.md).

## Build, vet, gofmt
```
$ go build ./...
exit=0

$ go vet ./...
exit=0

$ gofmt -l .
exit=0
```

## Test suite
```
$ go test ./...
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/cmd/youtube-pp-cli	[no test files]
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/cmd/youtube-pp-mcp	[no test files]
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/cache	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/cli	0.566s
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/client	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/cliutil	11.575s
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/config	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/mcp	1.816s
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/mcp/cobratree	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/store	1.110s
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/types	[no test files]
exit=0
```

## verify_skill
```
$ python3 .github/scripts/verify-skill/verify_skill.py --dir library/media-and-entertainment/youtube/
=== youtube ===
  ✓ All checks passed (flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command)
exit=0
```

## govulncheck
```
$ govulncheck ./...
This scan also found 0 vulnerabilities in packages you import and 1
vulnerability in modules you require, but your code doesn't appear to call these
vulnerabilities.
Use '-show verbose' for more details.
```

## Keyless live smoke (isolated HOME)
```
$ youtube search-bulk "test query" (isolated HOME, no YOUTUBE_API_KEY)   # F1: preflight, exit 4, remediation
Error: no YouTube Data API key configured — search calls the official search.list API and needs one.
Set YOUTUBE_API_KEY, or run: youtube-pp-cli auth set-token <key>
(create a key at https://console.cloud.google.com/apis/credentials with "YouTube Data API v3" enabled)
no YouTube Data API key configured — search calls the official search.list API and needs one.
exit=4

$ youtube-pp-cli search --help   # F4: top-level alias resolves
Search YouTube for one or more terms (alias for `youtube search-bulk`)

$ search --help | grep -E -- '--limit|--top'   # F5: --limit alias for --top
      --limit int       Alias for --top (top N results per term); --top wins when both are set (default 5)
      --top int         Top N results per term (default 5)

$ youtube videos-transcript jNQXAC9IVRw --format markdown (keyless)   # F3: timestamped markdown
# Transcript — jNQXAC9IVRw

_language: en (manual)_

**[00:01]** All right, so here we are, in front of the
elephants
**[00:05]** the cool thing about these guys is that they
have really...
```
