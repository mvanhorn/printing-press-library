# Shipcheck — bing-webmaster reprint with generator 4.31.7 (2026-09-05)

Scope: `library/developer-tools/bing-webmaster` at the 4.31.7 reprint commit
(`Reprint with generator 4.31.7 (verify-press-version gate)`), on top of the
4.31.1 reconciliation. Fresh tree generated with `printing-press 4.31.7` from
`bing-webmaster-spec.base.yaml` (run `20260905-104918-633b1430`), reconciled
against the preserved branch. Results below are from real runs on the exact
committed tree, not from the generator's self-report.

## Build

`go build ./...` — clean, no output.

## Tests

`go test -count=1 ./...` — all packages ok:

- cmd/bing-webmaster-pp-mcp
- internal/cli
- internal/client
- internal/cliutil
- internal/config
- internal/learn (+ entities, lookups, patterns)
- internal/mcp (+ bound, cobratree)
- internal/platform
- internal/snapshots
- internal/store

No FAIL lines.

## Vet / format

- `go vet ./...` — clean.
- `gofmt -l internal/ cmd/` — clean.

## CLI surface

`go run ./cmd/bing-webmaster-pp-cli --help` renders the full command tree,
including the hand-built intelligence commands: review, drift, publish,
triage, quota, gap, feed-health, watch.

## Preserved behavior (reconciliation)

- Bing WCF `{"d": ...}` envelope unwrapping + Microsoft `/Date(...)/`
  normalization in the client transport (`internal/client/bing_envelope.go`,
  hooked in `client.go`).
- All 8 novel commands registered in `root.go` and covered by the suite.
- `.printing-press-patches/` index updated for the 4.31.7 tree (retired
  write-lock and profile-bind shims removed from the record).
- `SKILL.md` install section restored to the 4.31.7 canonical text;
  hand-written usage documentation elsewhere in the file kept.
- `go mod tidy` applied; module path unchanged
  (`github.com/mvanhorn/printing-press-library/library/developer-tools/bing-webmaster`).
- `cli-skills/pp-bing-webmaster/` mirror stays out of the PR (regenerated
  post-merge by the library workflow).

## Research inputs

See `../research/`: print brief, absorb manifest, `research.json`, and the
base spec from run `20260905-104918-633b1430`. The regen-merge report
(`regen-merge-report.json` in this directory) documents the reconciliation
of the fresh 4.31.1 tree with the preserved branch.
