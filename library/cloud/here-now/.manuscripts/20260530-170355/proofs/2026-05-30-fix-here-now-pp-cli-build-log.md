# here.now CLI — Phase 3 Build Log

## What was built
The generator produced the data layer + all 56 endpoint commands + MCP (code orchestration, stdio+http) + scaffolded the 7 spec-recognized novel features as TODO stubs. Phase 3 implemented the real logic for all transcendence features and fixed structural issues.

### Novel features implemented (7, all hand-authored, all live-verified against the real API)
| Command | Status | Live evidence |
|---|---|---|
| `publish dir <path>` | DONE | Anon publish → live site (HTTP 200, marker matched), claim token vaulted, finalize completed |
| `publish resume <slug>` | DONE | Reports "already finalized" on a complete publish |
| `claims` | DONE | Vault table with time-remaining |
| `claims expiring --within <dur>` | DONE | count=1 within 48h |
| `claims redeem <slug>` | DONE | "Claimed — now permanent" exit 0 (live, attached to account) |
| `drives sync <path> --drive <id>` | DONE | 1 uploaded → 0 uploaded/1 skipped on rerun (sha256 diff works) |
| `drives diff <path> --drive <id>` | DONE | unchanged:1 (read-only) |
| `usage` | DONE | Real free-plan metrics: drives 1/1 (100%), publishes 3/60, drive_bytes vs 10GB |

### Structural fixes applied
- **root.go registration**: re-parented `sites stale` under `sites`, `publish resume`/`publish dir` under `publish`, `drives sync`/`drives diff` under `drives`; removed the standalone singular `drive` group (`drive.go` deleted). No duplicate registrations.
- **Durability**: all feature logic lives in hand-authored files (`internal/cli/here_now_novel.go`, `internal/store/here_now_store.go`, `internal/cli/publish_dir.go`, `claims_redeem.go`) that survive regen as whole files; the generated `newNovel*Cmd` stubs delegate to them.
- **Store**: added `claim_vault` + `publish_state` tables via hand-authored `internal/store/here_now_store.go` (not the generated migration slice).

## Real API behaviors that differed from the spec
- Publish response uses `siteUrl` (not `url`), with `status: "pending"`, `isLive: false`, `requiresFinalize: true`, a nested `upload.uploads[]` of presigned R2 targets, `upload.finalizeUrl`, `upload.versionId`, plus `claimToken`/`claimUrl`/`expiresAt` for anonymous publishes. The implementation reads these correctly and performs PUT-to-R2 + finalize.
- Anonymous publish requires sending NO Authorization header (the client otherwise always sends the configured key, which the server treats as authenticated and returns no claim token). `runPublishDir` blanks the client auth for `--anon`.

## Intentionally dropped
- **Cross-site Site Data search** (was novel feature #8): the here.now API has **no endpoint to enumerate a site's collections** — you can only read a known `(slug, collection)` pair. "Search across every site's every collection" is therefore not implementable without the user manually naming every collection. Dropped from `novel_features`, narrative recipes, and `when_to_use` to avoid overclaiming. Site Data remains fully covered by the generated `publishes data` CRUD commands (list/create/get/patch/delete records).

## Analytics paid-gate
The generated `classifyAPIError` already maps 401/402/403/404/429/5xx to friendly typed errors, and the novel commands route errors through it. A separate `mapPaidGateError` helper added during the build was redundant (0 callers, generic path already covers it) and was removed as dead code. The README/SKILL troubleshooting still guides free users to `usage` when analytics is paid-gated. NOTE: the dogfood test account is on a PAID plan (`/api/v1/analytics` returns 200), so a real free-tier 402 could not be exercised live.

## Verification
- `go build ./...` exit 0
- `go vet ./...` exit 0
- `gofmt -l internal/` clean
- `go test ./...` — 330 passed in 11 packages
- root.go: 27 AddCommand calls, no duplicates
- No TODO stubs remaining in novel command files
- Full command tree matches plan; no orphan singular `drive` group
