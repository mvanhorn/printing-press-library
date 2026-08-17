# youtube-pp-cli Amend Acceptance Report — feat/youtube-search-transcript-usability (PR #1631)

Scope: verification evidence for the search/transcript usability amend at head `d911bbf8c`
(PR mvanhorn/printing-press-library#1631), not a reprint. The base print evidence lives
alongside in this run (run `20260515-050804`). The amend changes no dependencies and keeps
the MCP tool surface unchanged (the `search` alias is annotated `mcp:hidden`).

```
Amend Acceptance: youtube
  Level: full module test suite + keyless live smoke (isolated HOME)
  Tests: go test ./... — 4 packages PASS, 0 failed
  Auth: none required for the smoke (preflight probe runs deliberately keyless;
        transcript fetch is keyless by design)
  Gate: PASS
```

## Finding-by-finding verification

| ID | Contract | Evidence |
|----|----------|----------|
| F1 | `search-bulk` preflights the API key: exit 4 + remediation instead of exit 0 with buried 403s | smoke: keyless run exits 4 naming `YOUTUBE_API_KEY`, `auth set-token`, and the console URL; pinned by the amend test suite |
| F2 | `--lang` default falls back to the only available track (manual preferred) with a stderr note; explicit `--lang` keeps the hard error | pinned by the amend test suite; the fallback note is documented in `--lang`'s help text (visible in the smoke transcript) |
| F3 | `--format markdown\|text` renders transcripts, including on cache hits | smoke: live keyless fetch of `jNQXAC9IVRw` renders timestamped markdown with the language/track header |
| F4 | top-level `search` alias for `youtube search-bulk`, `mcp:hidden` | smoke: `search --help` resolves and names itself an alias |
| F5 | `--limit` aliases `--top`; `--top` wins when both are set | smoke: both flags in `search --help` with the precedence documented; pinned by the amend test suite |

## Verification legs (all PASS)

- `go build ./...` — exit 0
- `go vet ./...` — exit 0
- `gofmt -l .` — clean
- `go test ./...` — 4 packages ok, 0 failures
- `verify_skill.py --dir library/media-and-entertainment/youtube/` — all checks passed
- `govulncheck ./...` — no vulnerabilities in called code

Raw command transcripts: [`2026-08-08-feat-youtube-pp-cli-amend-build-log.md`](2026-08-08-feat-youtube-pp-cli-amend-build-log.md)

## Fixtures used

- Isolated `HOME` under a session scratchpad with `YOUTUBE_API_KEY` unset — the F1 probe is
  deliberately credential-free, so it exercises the preflight path with zero quota spend.
- One keyless live transcript fetch (`jNQXAC9IVRw`, the public "Me at the zoo" video) for the
  F3 markdown rendering. No API key was used anywhere in the smoke.
