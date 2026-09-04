# GitHub CLI Brief

> Scope (user-confirmed): prioritize **Issues & PRs** and **Repos & Code**. De-prioritize Actions/CI, Orgs/Enterprise, Users/Notifications/Gists. The official spec is 788 paths / 9.1 MB; the generator truncates significantly, so scoped prioritization is load-bearing.

## API Identity
- **Domain:** Software-development collaboration platform (issues, pull requests, repositories, code, search).
- **Users:** Developers, AI coding agents, release/triage automation, OSS maintainers.
- **Data profile:** Repos → (issues, pulls, branches, commits, contents, labels, milestones, releases). High-gravity entities: **issue, pull_request, repository, commit, branch, label, review, comment**. Search is cross-entity (code/commits/issues/repos/users/topics/labels).
- **Auth:** Bearer PAT. Env vars `GITHUB_TOKEN` / `GH_TOKEN`. `gh auth token` available on this machine. Fine-grained or classic PAT; scopes per workflow. (`auth.type: bearer_token`, `Authorization: Bearer`.)

## Reachability Risk
- **None.** `api.github.com` is a first-party, documented, rate-limited-but-open REST API. No bot protection. 401 expected when unauthenticated; 200 with a token. Phase 1.9 probe expected PASS.
- Rate limit: 5,000 req/hr authenticated (REST); search has its own 30 req/min limit. Secondary rate limits on bursts. Relevant for any fan-out/scan novel command → must use the per-source limiter + surface `RateLimitError`.

## Scoped Endpoint Coverage (ground truth from the 2022-11-28 spec)
| Surface | Paths | Notable |
|---|---|---|
| Issues | 28 | list/create/get/update, comments, events, assignees, labels, lock, reactions, sub-issues, dependencies, timeline |
| Pull Requests | 19 | list/create/get/update, reviews, review comments, requested-reviewers, files, commits, merge, update-branch |
| Branches | 13 | list/get, protection, required-status-checks |
| Commits | 9 | list/get, compare, statuses, check-runs |
| Contents | 1 | GET/PUT/DELETE `/contents/{path}` (read file, create/update, delete) |
| Labels | 2+ | repo labels + per-issue labels |
| Milestones | 3 | list/create/get/update |
| Releases | 9 | list/get/latest/by-tag/create/update, assets |
| Search | 7 | code, commits, issues, labels, repositories, topics, users |

Truncation will keep this dense scoped core and drop the long tail (orgs 229, enterprises 30, gists 10, marketplace, classrooms, codespaces, packages, security advisories).

## Top Workflows
1. **Triage:** list/filter issues by label/assignee/state, comment, label, assign, close.
2. **PR review loop:** list open PRs, read files/diff + reviews, request reviewers, approve/comment, merge.
3. **Code search:** find a symbol/string across a repo or org, jump to file + line.
4. **Repo intelligence:** recent commits, branch divergence, who-touched-what, release notes.
5. **Read a file at a ref** without cloning (`contents` + ref).

## Table Stakes (from competitors — must match)
- `gh` (official): `issue list/create/view/edit/close/comment`, `pr list/view/create/checkout/review/merge/diff`, `repo view/clone/list`, `search code/issues/prs/repos`, `release list/view/create`, `api` passthrough. JSON output via `--json field,...` + `--jq`.
- `hub`: git-centric PR/issue shortcuts (legacy, lower priority).
- SDKs to mirror method coverage: **octokit/rest.js** (JS), **google/go-github** (Go), **PyGithub** (Python). Each exposes issues/pulls/repos/search method families.
- **github/github-mcp-server** (official MCP): tools like `list_issues`, `get_issue`, `create_issue`, `add_issue_comment`, `list_pull_requests`, `get_pull_request`, `create_pull_request`, `get_pull_request_files`, `search_code`, `search_issues`, `get_file_contents`, `list_commits`. These are the agent-native parity targets.

## Data Layer
- **Primary entities (SQLite):** repositories, issues, pull_requests, commits, branches, labels, milestones, releases, reviews, comments.
- **Sync cursor:** `updated_at` / `since` for issues+PRs; SHA pagination for commits.
- **FTS/search:** local FTS5 over issue/PR title+body+comments and commit messages → offline search that GitHub's API rate-limits hard online.

## Why install this instead of `gh`
`gh` is excellent but online-only, no local persistence, no SQL. This CLI's edge:
- **Offline, SQL-composable** issue/PR/commit history (sync once, query forever; survives the 30 req/min search limit).
- **Agent-native:** `--json`, `--select` dotted paths, `--compact`, typed exit codes, `--dry-run` on every mutation — built for tool-calling agents, not humans typing.
- **Local joins** GitHub's API can't do in one call (cross-issue/PR/commit correlation).

## Product Thesis
- **Name:** `github-pp-cli` ("GitHub GOAT") — every issue/PR/repo/code workflow `gh` has, plus a local SQLite mirror, offline FTS, and cross-entity joins no GitHub tool offers.
- **Why it should exist:** turn GitHub's rate-limited, online-only, single-call API into a local, queryable, agent-native datastore for the collaboration core.

## Build Priorities
1. **P0 foundation:** data layer for all scoped entities + sync + FTS + SQL path.
2. **P1 absorb:** match the full `gh` + github-mcp-server surface for issues/PRs/repos/contents/commits/branches/search.
3. **P2 transcend:** local-join/offline commands `gh` can't do (see absorb manifest transcendence table).

## User Vision
- User scoped explicitly to Issues&PRs + Repos&Code. Build those to depth; drop org/enterprise/actions/notifications. Do not spend the truncation budget on the long tail.
