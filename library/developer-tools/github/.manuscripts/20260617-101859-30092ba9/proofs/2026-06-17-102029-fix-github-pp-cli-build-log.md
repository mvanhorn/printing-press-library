# github-pp-cli — Phase 3 Build Log

Manifest transcendence rows: 8 planned, 8 built. Phase 3 will not pass until all 8 ship.

## Built (all 8 transcendence rows, hand-code, 0 stubs)

| Command | File | Data source | Logic |
|---|---|---|---|
| `issues dupes <term>` | issues_dupes.go | local | FTS5 (resources_fts) over synced issues, BM25 rank |
| `mentions <term>` | mentions.go | local | per-type FTS over issues/pulls/commits + LIKE scan of issues_comments, tagged by entity |
| `issues context <number>` | issues_context.go | local | issue + comments (issues_comments) + mentioning commits (LIKE `#N`) → one JSON envelope |
| `pulls review-load` | pulls_review_load.go | local | parse requested_reviewers per open PR, GROUP BY login |
| `pulls stale` | pulls_stale.go | local | MAX(updated_at) staleness vs now, sorted |
| `repos changelog` | repos_changelog.go | live | compare API commits, group by author; owner/repo inferred from mirror |
| `repos who-touched <path>` | repos_who_touched.go | live | commits?path= scan-and-filter (--max-scan-pages cap, scanned count), author ranking |
| `labels coverage` | labels_coverage.go | local | per-label open/closed counts + unused labels + unlabeled issues |

Shared pure logic + store/JSON helpers: `internal/cli/novel_common.go` (unit-tested in `novel_common_test.go`, 7 table-driven tests, all PASS).

## Wiring
All 8 were generator-scaffolded as stubs AND wired (AddCommand in issues.go/pulls.go/repos.go/labels.go/root.go). Phase 3 replaced the TODO bodies only; generated headers removed so regen-merge treats them as hand-authored.

## Generated-surface notes
- Scoped via spec pre-filter (788→107 paths) → 53 resources / 183 endpoints (issues 29, pulls 21, releases 13, commits 9, github-search 7, etc.). Dropped long tail: orgs/enterprises/actions/codespaces/gists/marketplace.
- Auth: GitHub's bundled OpenAPI declares no securityScheme; enriched the spec with a bearer scheme (GITHUB_TOKEN/GH_TOKEN) and regenerated so config/client/doctor/README all carry auth.
- MCP: Cloudflare pattern auto-applied (183 endpoints > 50): thin search+execute, endpoint tools hidden, stdio+http transport.

## Conventions honored
- Verify-friendly RunE: help-only (no args+no flags), dryRunOK short-circuit, required-input → usageErr (exit 2), missing-mirror guard (stderr hint + `[]` for json, rc 0).
- `mcp:read-only: true` on all (all are reads; changelog/who-touched are live reads).
- Live commands use boundCtx(cmd.Context(), flags) + classifyAPIError; who-touched honors IsDogfoodEnv (caps scan to 1 page) + scan-and-filter caps (--max-scan-pages, scanned_commits, note).
- `// pp:data-source local|live` annotation on every novel command file.

## Deferred / intentional
- mentions covers comment matches via LIKE on issues_comments (comments are in a typed table, not the FTS index).
- 9th survivor `pulls linked` held in reserve (not built; user declined the swap).
