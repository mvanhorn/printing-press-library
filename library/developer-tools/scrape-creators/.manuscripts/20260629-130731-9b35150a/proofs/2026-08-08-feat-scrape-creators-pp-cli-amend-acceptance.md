# scrape-creators-pp-cli Amend Acceptance Report — feat/scrape-creators-pagination-usability (PR #1624)

Scope: verification evidence for the pagination/usability amend and the compact-comments fix
at head `9a97c3a9d` (PR mvanhorn/printing-press-library#1624), not a reprint. The base print
evidence for this CLI lives alongside in this run's proofs (run `20260629-130731-9b35150a`,
press 4.27.0). The amend changes no dependencies (go.mod's only delta documents the existing
x/sys floor) and no MCP surface.

```
Amend Acceptance: scrape-creators
  Level: full module test suite + live A/B smoke (real credentials, read-only)
  Tests: go test ./... — 7 packages PASS, 0 failed
  Auth: user API key via config (doctor: auth configured, api reachable)
  Gate: PASS
```

## Contract-by-contract verification

| Contract | Evidence |
|----------|----------|
| `--all` follows response cursors instead of refetching page one | pinned by `amend_pagination_test.go` (cursor traversal, termination, 100-page cap) |
| Feed commands accept aliases and positional handles/URLs, reject surplus positionals | live dry-run: `instagram posts my_handle` resolves to `GET /v2/instagram/user/posts?handle=my_handle` (alias + positional adoption, zero spend); bounds pinned by `amend_pagination_test.go` |
| `--agent`/`--compact` no longer strips the sole payload array (comments fix) | live A/B on the same YouTube video: branch binary returns `comments` with 20 entries; the published pre-fix binary returns `success:true` with the `comments` key absent (`credits_charged` but zero data); pinned by `amend_compact_comments_test.go` |
| Local-store sync guidance and 403 hints | pinned by `amend_pagination_test.go` (sync hint assertions) |

## Verification legs (all PASS)

- `go build ./...` — exit 0
- `go vet ./...` — exit 0
- `go test ./...` — 7 packages ok, 0 failures
- `verify_skill.py --dir library/developer-tools/scrape-creators/` — all checks passed, 0 notes
- `verify-supply-chain/scan.py --base-ref upstream/main` — no findings (covers the go.mod
  x/sys floor-block move in `9a97c3a9d`)
- `govulncheck ./...` — no vulnerabilities found

Raw command transcripts: [`2026-08-08-feat-scrape-creators-pp-cli-amend-build-log.md`](2026-08-08-feat-scrape-creators-pp-cli-amend-build-log.md)

## Fixtures used

- Live read-only GETs only: two `youtube list-video-2` calls (branch and published binary)
  against the same public video for the A/B, ~2 credits total. The alias/positional probe ran
  with `--dry-run` (no request sent). No mutating command exists in this CLI's amended surface.
