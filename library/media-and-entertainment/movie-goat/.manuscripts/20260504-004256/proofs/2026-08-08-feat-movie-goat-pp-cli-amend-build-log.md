# movie-goat-pp-cli Amend Build & Verification Log — head 68cb19d5e (2026-08-08)

Raw transcripts of every verification leg run against the PR branch. Summary and
finding-by-finding mapping: [`2026-08-08-feat-movie-goat-pp-cli-amend-acceptance.md`](2026-08-08-feat-movie-goat-pp-cli-amend-acceptance.md).

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
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/cmd/movie-goat-pp-cli	[no test files]
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/cmd/movie-goat-pp-mcp	[no test files]
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/cache	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/cli	0.463s
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/client	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/cliutil	11.189s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/config	1.367s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/mcp	1.784s
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/mcp/cobratree	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/omdb	0.762s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/store	2.420s
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/types	[no test files]
exit=0
```

## verify_skill
```
$ python3 .github/scripts/verify-skill/verify_skill.py --dir library/media-and-entertainment/movie-goat/
=== movie-goat ===
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

## Credential round-trip smoke (isolated HOME, dummy key, zero network)
```
# Isolated HOME (no real config touched), dummy key, zero network, OMDB_API_KEY unset
$ auth set-omdb-token test-omdb-key-123   # F1/F2: persistent slot + dedicated subcommand
OMDb token saved to <tmp>/mg-home/.config/movie-goat-pp-cli/config.toml

$ auth status   # F3: OMDb credential visible, source=config
  OMDb:   configured (config:omdb_api_key)

$ OMDB_API_KEY=env-wins auth status   # env precedence preserved
  OMDb:   configured (env:OMDB_API_KEY)

$ auth logout; auth status   # round-trip clear
Logged out. Credentials cleared.

$ grep omdb_api_key .config/movie-goat-pp-cli/config.toml   # cleared on disk
omdb_api_key = ''
```
