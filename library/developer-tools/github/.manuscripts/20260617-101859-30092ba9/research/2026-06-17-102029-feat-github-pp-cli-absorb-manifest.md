## Absorb Manifest — Absorbed (match or beat everything that exists)

Scope: Issues & PRs + Repos & Code. Sources: `gh` (official CLI), `github/github-mcp-server` (official MCP), `octokit/rest.js`, `google/go-github`. Every row works offline-after-sync where applicable, with `--json`, `--select`, `--dry-run`, typed exit codes, SQLite persistence.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List issues (filter state/label/assignee/since) | gh issue list / mcp list_issues | (generated endpoint) issues list | Offline after sync, SQL-composable, `--select` |
| 2 | Get issue | gh issue view / mcp get_issue | (generated endpoint) issues get | `--compact` high-gravity fields |
| 3 | Create issue | gh issue create / mcp create_issue | (generated endpoint) issues create | `--dry-run`, `--stdin` body, idempotent |
| 4 | Edit/close/reopen issue | gh issue edit/close/reopen | (generated endpoint) issues update | `--dry-run`, typed exit codes |
| 5 | Lock/unlock issue | mcp + REST lock | (generated endpoint) issues lock | scriptable |
| 6 | List issue comments | gh issue view --comments | (generated endpoint) issues comments-list | offline, FTS-searchable |
| 7 | Comment on issue | gh issue comment / mcp add_issue_comment | (generated endpoint) issues comment | `--dry-run`, `--stdin` |
| 8 | Add/remove labels | gh issue edit --add-label / mcp | (generated endpoint) issues labels | idempotent set semantics |
| 9 | Add/remove assignees | gh issue edit --add-assignee | (generated endpoint) issues assignees | idempotent |
| 10 | Issue events / timeline | mcp + REST timeline | (generated endpoint) issues events | offline join with issue |
| 11 | Issue reactions | REST reactions | (generated endpoint) issues reactions | scriptable |
| 12 | Sub-issues / dependencies | REST sub_issue/dependencies | (generated endpoint) issues sub-issues | local graph join |
| 13 | List pull requests | gh pr list / mcp list_pull_requests | (generated endpoint) pulls list | offline, `--select`, SQL |
| 14 | Get pull request | gh pr view / mcp get_pull_request | (generated endpoint) pulls get | `--compact` |
| 15 | Create pull request | gh pr create / mcp create_pull_request | (generated endpoint) pulls create | `--dry-run`, `--stdin` |
| 16 | Edit/close/reopen PR | gh pr edit/close/reopen | (generated endpoint) pulls update | `--dry-run` |
| 17 | List PR files (changed) | gh pr diff / mcp get_pull_request_files | (generated endpoint) pulls files | `--select` filename/status/additions |
| 18 | List PR commits | gh pr view / mcp | (generated endpoint) pulls commits | offline join |
| 19 | List/get PR reviews | mcp get_pull_request_reviews | (generated endpoint) pulls reviews | offline |
| 20 | Create review (approve/request-changes/comment) | gh pr review / mcp create_and_submit_pull_request_review | (generated endpoint) pulls create-review | `--dry-run`, typed exit |
| 21 | Request reviewers | gh pr edit --add-reviewer / mcp request_copilot_review | (generated endpoint) pulls request-reviewers | idempotent |
| 22 | Merge PR | gh pr merge / mcp merge_pull_request | (generated endpoint) pulls merge | `--dry-run`, merge-method flag |
| 23 | Update PR branch | gh + REST update-branch | (generated endpoint) pulls update-branch | `--dry-run` |
| 24 | Get repository | gh repo view / mcp | (generated endpoint) repos get | `--compact` |
| 25 | List branches | gh + REST branches | (generated endpoint) branches list | offline |
| 26 | Get branch + protection | REST branch protection | (generated endpoint) branches get | offline |
| 27 | List commits | gh + mcp list_commits | (generated endpoint) commits list | offline, FTS over messages |
| 28 | Get commit | mcp get_commit | (generated endpoint) commits get | `--select` |
| 29 | Compare commits / branches | REST compare | (generated endpoint) repos compare | ahead/behind counts |
| 30 | Read file contents at ref | gh + mcp get_file_contents | (generated endpoint) contents get | no clone, `--ref` |
| 31 | Create/update file | mcp create_or_update_file | (generated endpoint) contents put | `--dry-run`, `--stdin` |
| 32 | Delete file | mcp delete_file | (generated endpoint) contents delete | `--dry-run` |
| 33 | List/get releases (+latest, by-tag) | gh release list/view / mcp | (generated endpoint) releases list | offline |
| 34 | Create/update release | gh release create | (generated endpoint) releases create | `--dry-run` |
| 35 | Repo labels (list/create) | gh label list | (generated endpoint) labels list | offline |
| 36 | Milestones (list/create/update) | REST milestones | (generated endpoint) milestones list | offline |
| 37 | Search code | gh search code / mcp search_code | (generated endpoint) search code | offline FTS fallback |
| 38 | Search issues/PRs | gh search issues/prs / mcp search_issues | (generated endpoint) search issues | offline FTS fallback |
| 39 | Search repositories | gh search repos | (generated endpoint) search repositories | `--select` |
| 40 | Search commits | gh search commits | (generated endpoint) search commits | offline |
| 41 | Search users/topics/labels | gh search | (generated endpoint) search users | — |
| 42 | Offline full-text search (synced) | (none — gh is online-only) | (behavior in github-pp-cli search) FTS5 over issue/PR title+body+comments+commit msgs | survives the 30 req/min search limit, fully offline |
| 43 | Sync to local store | (none) | framework sync (--resources, --since, --full) | one mirror, query forever |
| 44 | Raw SQL over synced data | (none) | framework sql | joins the API can't express |
| 45 | `--json`/`--select`/`--compact`/`--csv` everywhere | gh --json/--jq (partial) | (behavior in github-pp-cli <any command>) global output flags | uniform across every command, not just some |
| 46 | Health/auth check | gh auth status | framework doctor | reports token scopes + rate-limit budget |

## Transcendence (only possible with our approach) — from the novel-features subagent (8 survivors, all hand-code, all >=5/10)

| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|-------------------------|------------------|
| 1 | Duplicate-issue finder | issues dupes "<term>" --limit N | hand-code | 8 | Local FTS5 over synced issue corpus, BM25-ranked, offline — survives the 30 req/min search limit | none |
| 2 | Cross-entity term hunter | mentions "<symbol>" --since 30d | hand-code | 8 | UNION over FTS5 tables (issues + PR comments + commit messages); no single GitHub API call expresses it | Use this to find a string across all synced text entities at once. To match one entity type only, use `search --type <resource>`. To find open issues duplicating each other, use `issues dupes`. |
| 3 | Review-backlog by reviewer | pulls review-load --state open | hand-code | 8 | Local join pull_requests ⋈ requested_reviewers GROUP BY login — gh never aggregates reviewers across PRs | Use this for the reviewer-side rollup (who owes reviews). For the author-side stale-PR list, use `pulls stale`. |
| 4 | Stale-PR detector | pulls stale --older-than 14d | hand-code | 7 | MAX(last commit, review, comment, update) per PR vs now from local tables | Use this for the PR-age view of what is slipping. For who-owes-reviews, use `pulls review-load`. |
| 5 | Release diff by author | repos changelog --base <tag> --head <ref> | hand-code | 7 | Live compare for the SHA range + local join commits ⋈ pulls ⋈ author, grouped by author | none |
| 6 | File ownership / churn | repos who-touched <path> --since 90d | hand-code | 7 | GROUP BY author over synced commits filtered by path; per-path committer ranking gh lacks | none |
| 7 | Agent context bundle | issues context <number> --json | hand-code | 7 | One JSON envelope from local issue + comments + mentioning-commits; replaces N online round-trips with one offline read | Use this to assemble an agent read-set for one issue from the local mirror. For a raw single issue use `issues get`; for a string sweep use `mentions`. |
| 8 | Label coverage report | labels coverage | hand-code | 6 | GROUP BY local issue_labels; flags unused labels + unlabeled issues — no single API call | none |

> 9th survivor held in reserve (brainstorm audit): `pulls linked <number>` (issue↔PR closes-ref graph, 7/10). Swap for #8 only if the user wants it.

## Compound use cases (transcendence rationale)
- **Maintainer Saturday triage:** `issues dupes` + `labels coverage` + `pulls stale` — find duplicates, prune labels, decide which PRs to nudge — all offline after one sync.
- **Release-captain pre-release sweep:** `repos changelog` + `pulls review-load` + `pulls stale` — what merged, who's the review bottleneck, what's slipping.
- **Agent fix-the-issue loop:** `issues context` (one read) → `mentions` (locate symbol) → act — instead of N rate-limited online MCP calls.
- **Onboarding:** `repos who-touched <path>` — who knows this area.

## Phase 3 hand-code commitment
8 transcendence rows, all `hand-code` (~50-150 LoC each + root.go wiring). 0 stubs. The absorbed surface (~46 rows) is generator-emitted (typed endpoint commands) + framework (sync/search/sql/doctor).
