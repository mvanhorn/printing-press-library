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
