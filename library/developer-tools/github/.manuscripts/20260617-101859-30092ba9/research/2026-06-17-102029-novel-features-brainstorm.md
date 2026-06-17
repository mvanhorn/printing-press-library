# Novel Features Brainstorm — github-pp-cli (audit trail)

> Subagent output, persisted per references/novel-features-subagent.md output-handling step 3. Survivors flow into the manifest transcendence table + research.json novel_features. Customer model + killed candidates retained for retro/dogfood debugging.

## Customer model

**Priya — OSS maintainer of a mid-traffic Go library (~40 open issues, ~12 open PRs).** Today: lives in web UI + `gh`, re-runs filtered `issue list`/`pr list` every morning, throttled by 30 req/min search ceiling when sweeping issues+PRs+commits. Weekly ritual: Saturday triage — re-label, find duplicates, decide which stale PRs to nudge/close. Frustration: can't ask "which open issues mention the same stack-trace symbol as PR #214?" in one command; duplicate detection is manual.

**Devon — staff engineer / release captain (30-person team, bi-weekly release).** Today: `gh pr list --search "is:open review:required"` + web view + CODEOWNERS by eye. Weekly ritual: pre-release sweep — what merged since last tag, who hasn't reviewed assigned PRs, which PRs risk slipping. Frustration: "what changed between v2.3.0 and main grouped by who touched it" = 3 `gh` calls + a spreadsheet; review-backlog ownership invisible (`gh` never aggregates requested reviewers across PRs).

**Sam — AI coding agent tasked with "fix the failing-tests issue and open a PR."** Today: one online MCP call at a time, burns rate-limit budget, re-fetches the same issue list every reasoning step (nothing cached). Per-task ritual: locate issue → find referenced code → read file at HEAD → correlate recent commits. Frustration: every call online + rate-limited + verbose JSON; no way to join issue→referenced file→last commit→PR without N round-trips.

**Ana — incoming engineer onboarding onto an unfamiliar 2,000-commit repo.** Today: clicks through commits page + blame; asks teammates "who knows this file?" Weekly ritual: when assigned a file/area, figure out history + frequent committers + touching issues/PRs. Frustration: GitHub shows per-file blame but no "who has the most commits touching parser/" rollup.

## Survivors (→ transcendence table)

| # | Feature | Command | Build | Score | Persona | Proof |
|---|---------|---------|-------|-------|---------|-------|
| 1 | Duplicate-issue finder | `issues dupes "<term>"` | hand-code | 8 | Priya | local FTS5 over synced issue title+body, BM25-ranked |
| 2 | Cross-entity term hunter | `mentions "<symbol>" --since 30d` | hand-code | 8 | Priya, Sam | UNION over FTS5 tables (issues+PR comments+commit msgs) |
| 3 | Review-backlog by reviewer | `pulls review-load --state open` | hand-code | 8 | Devon | local join pull_requests⋈requested_reviewers GROUP BY login |
| 4 | Stale-PR detector | `pulls stale --older-than 14d` | hand-code | 7 | Devon | MAX(commit,review,comment,update) per PR vs now |
| 5 | Release diff by author | `repos changelog --base <tag> --head <ref>` | hand-code | 7 | Devon | live compare for SHA range + local commits⋈pulls⋈author join |
| 6 | File ownership / churn | `repos who-touched <path> --since 90d` | hand-code | 7 | Ana, Devon | GROUP BY author over synced commits filtered by path |
| 7 | Agent context bundle | `issues context <number> --json` | hand-code | 7 | Sam | one JSON envelope from local issue+comments+mentioning-commits |
| 8 | Label coverage report | `labels coverage` | hand-code | 6 | Priya | GROUP BY local issue_labels; flag unused labels + unlabeled issues |

9th survivor held below table: C9 `pulls linked <number>` (issue⋈PR closes-ref graph, 7/10) — swap for C8 if 8-ceiling is hard.

## Killed candidates

| Feature | Kill reason | Closest survivor |
|---------|-------------|------------------|
| `repos path-prs` (C7) | sub-weekly use + expensive new sync surface (per-PR changed-files repo-wide) | `repos who-touched` |
| `issues triage` (C10) | thin wrapper over absorbed `issues list` filters + date predicate; no cross-entity leverage | `mentions` |
| `issues hot` (C12) | single-table ORDER BY, no join/leverage over `issues list --select reactions` | `issues dupes` |
| `branches divergence` (C13) | live per-branch compare fan-out (no local SHA graph); re-exposes `repos compare` in a rate-limited loop | `pulls review-load` |
| `commits log-grouped` (C14) | weaker sibling of `repos changelog` — string-prefix bucket, no PR/author join | `repos changelog` |
