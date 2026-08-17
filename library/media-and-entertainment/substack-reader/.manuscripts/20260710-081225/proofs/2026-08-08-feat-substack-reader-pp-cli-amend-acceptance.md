# substack-reader-pp-cli Amend Acceptance Report — amend/substack-reader-compact-exhausted-batch-read (PR #1634)

Scope: verification evidence for the compact/exhaustion/batch-read amend at head `aa581c4da`
(PR mvanhorn/printing-press-library#1634), not a reprint. The base print evidence lives
alongside in this run (run `20260710-081225`). The amend changes no dependencies and no MCP
surface. The smoke runs against an isolated HOME (the user-level corpus is never touched)
on free public posts, with no credentials.

```
Amend Acceptance: substack-reader
  Level: full module test suite + keyless live smoke (isolated HOME)
  Tests: go test ./... — 12 packages PASS, 0 failed
  Auth: none (free posts are keyless by design)
  Gate: PASS
```

## Finding-by-finding verification

| ID | Contract | Evidence |
|----|----------|----------|
| F1 | compact list projections admit only short scalars, drop all-null keys, derive a bounded `snippet`, never `_pp_body_text` | smoke: `search --data-source local --agent` over a freshly archived 2-post corpus — no `_pp_body_text`, snippet 201 chars, no field over 250 chars |
| F1b | `read` stamps honest `meta.source: "live"` (it fetches the publication) | smoke: `read <post> --agent` reports `meta.source: live`; full body fields present by design (`read` is the full-text command) |
| F2 | `archive` reports why the walk ended: `exhausted` in JSON, hint in text, `--limit 0` walks everything | smoke: `archive astralcodexten --limit 2 --json` returns `archived: 2, exhausted: false`; the hint text and `--limit 0` path are pinned by the amend test suite |
| F3 | `read <post>...` is variadic: array of envelopes, per-post `{post, error}` entries survive, errors only when every post fails | smoke: one good + one bogus post → array of 2, one `{error, post}` entry, exit 0 |

## Verification legs (all PASS)

- `go build ./...` — exit 0
- `go vet ./...` — exit 0
- `gofmt -l .` — clean
- `go test ./...` — 12 packages ok, 0 failures
- `verify_skill.py --dir library/media-and-entertainment/substack-reader/` — all checks passed
- `govulncheck ./...` — no vulnerabilities found

Raw command transcripts: [`2026-08-08-feat-substack-reader-pp-cli-amend-build-log.md`](2026-08-08-feat-substack-reader-pp-cli-amend-build-log.md)

## Fixtures used

- Isolated `HOME` under a session scratchpad — the archive/search probes built a throwaway
  2-post corpus (`astralcodexten`, free public posts) and never touched the user's store.
- No credentials anywhere: free posts are keyless by design; the bogus post in the F3 probe
  is `no-such-pub/no-such-post`.
