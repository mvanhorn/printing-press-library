# scrape-creators-pp-cli Amend Build & Verification Log — head 9a97c3a9d (2026-08-08)

Raw transcripts of every verification leg run against the PR branch. Summary and
contract-by-contract mapping: [`2026-08-08-feat-scrape-creators-pp-cli-amend-acceptance.md`](2026-08-08-feat-scrape-creators-pp-cli-amend-acceptance.md).

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
?   	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-cli	[no test files]
?   	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-mcp	[no test files]
?   	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cache	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cli	(cached)
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/client	(cached)
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/cliutil	(cached)
?   	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/config	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/mcp	0.557s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/mcp/bound	(cached)
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/mcp/cobratree	2.588s
ok  	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/store	(cached)
?   	github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/types	[no test files]
exit=0
```

## verify_skill and supply-chain scan
```
$ python3 .github/scripts/verify-skill/verify_skill.py --dir library/developer-tools/scrape-creators/
=== scrape-creators ===
  ✓ All checks passed (flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command)
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

## Live A/B smoke (real credentials, read-only)
```
$ scrape-creators-pp-cli youtube list-video-2 --url <video> --agent   # branch binary (9a97c3a9d)
meta.source: live
comments key present: True | count: 20

$ (published pre-fix binary, same video)   # regression baseline
comments key present: False | keys left: ['continuationToken', 'credits_charged', 'credits_remaining', 'success']

$ scrape-creators-pp-cli instagram posts my_handle --dry-run --agent   # alias + positional adoption, no spend
GET https://api.scrapecreators.com/v2/instagram/user/posts
  ?handle=my_handle
  x-api-key: ****

(dry run - no request sent)
```
