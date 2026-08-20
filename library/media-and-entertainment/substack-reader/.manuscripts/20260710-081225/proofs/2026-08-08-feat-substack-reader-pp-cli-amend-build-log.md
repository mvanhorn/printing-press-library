# substack-reader-pp-cli Amend Build & Verification Log — head aa581c4da (2026-08-08)

Raw transcripts of every verification leg run against the PR branch. Summary and
finding-by-finding mapping: [`2026-08-08-feat-substack-reader-pp-cli-amend-acceptance.md`](2026-08-08-feat-substack-reader-pp-cli-amend-acceptance.md).

## Build, vet, gofmt
```
$ go build ./...
exit=0

$ go vet ./...
exit=0

$ gofmt -l .
internal/substack/session_test.go
exit=0
```

## Test suite
```
$ go test ./...
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/cmd/substack-reader-pp-cli	[no test files]
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/cmd/substack-reader-pp-mcp	[no test files]
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/cache	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/cli	1.274s
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/cli/playbooks	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/client	2.509s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/cliutil	14.333s
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/config	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/learn	1.737s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/learn/entities	1.827s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/learn/lookups	2.479s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/learn/patterns	1.720s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/mcp	3.080s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/mcp/bound	3.960s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/mcp/cobratree	6.265s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/store	3.950s
ok  	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/substack	0.834s
?   	github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/types	[no test files]
exit=0
```

## verify_skill
```
$ python3 .github/scripts/verify-skill/verify_skill.py --dir library/media-and-entertainment/substack-reader/
=== substack-reader ===
  ✓ All checks passed (flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command)
exit=0
```

## govulncheck
```
$ govulncheck ./...
No vulnerabilities found.
```

## Keyless live smoke (isolated HOME, free public posts)
```
# Isolated HOME (user corpus untouched), free public posts, no credentials
$ read astralcodexten/open-thread-441 --agent   # F1b: honest live provenance (read is full-text by design; body fields expected)
meta.source: live
_pp_body_text present: False
fields over 250 chars: ['body_html', 'body_markdown']

$ read astralcodexten/open-thread-441 no-such-pub/no-such-post --json   # F3: variadic, per-post error entry, exit 0 when one succeeds
is array: True | entries: 2
error entries: 1 | error entry keys: ['error', 'post']
exit=0

$ archive astralcodexten --limit 2 --json   # F2: exhaustion honesty + hint
archived: 2 | exhausted: False

$ search 'astral' --data-source local --agent   # F1: compact list projection over the 2-post local corpus
rows: 2
_pp_body_text present: False
snippet bounded: True (201 chars)
fields over 250 chars: none
```
