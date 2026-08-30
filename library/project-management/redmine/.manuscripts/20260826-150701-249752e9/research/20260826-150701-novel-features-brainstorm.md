## Customer model

**Priya Shah — senior developer / issue triager**, one of 12 engineers on a team running self-hosted Redmine for a multi-project codebase.

*Today (without this CLI):* Priya works entirely inside the Redmine web UI. To triage she opens the issue list, re-applies the same project+status+assignee filter URL she can never quite remember, and clicks into each issue individually to read its journal history and check whether it's blocked by something else in flight.

*Weekly ritual:* Monday morning triage — she reviews new issues assigned to her team, reassigns a few, adds notes, and before starting anything she wants to make sure it isn't blocked by an issue that's itself blocked by another issue three hops away. She also tries to catch up on "what happened while I was out" across the 4 projects she watches.

*Frustration:* There's no way to see a full blocker dependency chain without opening each linked issue's page one at a time. Issues that nobody has touched in weeks don't surface anywhere — Redmine has no "stale" concept. And reconstructing "what changed since last Friday" means paging through each project's activity feed by hand.

**Marcus Bellweather — engineering manager / delivery lead**, owns release planning across 3 active projects and their versions.

*Today (without this CLI):* He opens Redmine's Roadmap page per project and manually counts open vs. closed issues under each version to gauge whether it's on track. To check if time logged is blowing past estimates, he exports time entries to a spreadsheet and does the math himself. To see if anyone's overloaded, he eyeballs each engineer's assigned-issue list one by one.

*Weekly ritual:* Friday planning review — for each open version he decides whether it's still hittable or should slip, flags any engineer visibly buried under open issues, and skims which issues are taking far longer to close than their tracker's typical pace.

*Frustration:* Redmine's own Roadmap UI has no REST equivalent — every burndown or workload number is a from-scratch manual count or spreadsheet, rebuilt weekly, because the API has no aggregate/report endpoint at all.

**Devika Rao — Redmine instance admin / ops-adjacent PM**, manages project setup, users, groups, and reference data for the org, and treats the wiki as the living spec doc for each project.

*Today (without this CLI):* She configures new projects by clicking through the memberships/roles screens, and when she wants to know if a project has gone quiet she checks issues, wiki, and news feeds separately since there's no single "last touched" signal.

*Weekly ritual:* Reviews the project list for ones that look dead (no engineering activity) and decides whether to archive them; onboards/offboards users and double-checks role memberships stuck.

*Frustration:* No single place answers "when was this project last touched, across any entity" — she has to check three different tabs per project, every time, for every project.

## Candidates (pre-cut)

| # | Candidate | Command | Description | Persona | Source | Kill/keep check applied |
|---|-----------|---------|-------------|---------|--------|--------------------------|
| 1 | Roadmap burndown | `roadmap burndown <version> [--project X]` | Open/closed issue counts, avg done_ratio, estimated-vs-logged hours for one version; flags whether it's ready to close | Marcus | (a) persona | Reimplementation check: computed entirely from synced `issues`+`versions`+`time_entries` tables, no fake API calls — keep |
| 2 | Assignee workload/overload report | `workload [--project X] [--threshold N]` | Per-assignee open-issue count + estimated-hours sum across a project or globally; flags anyone over threshold | Marcus | (a) persona | Verifiability: counts are checkable against issue list — keep |
| 3 | Stale issue detection | `issues stale [--project X] --days 14` | Open issues whose `updated_on` is older than N days | Priya | (a) persona | Mechanical, no LLM — keep |
| 4 | Since-last-sync digest | `digest --since 7d [--mine] [--watched]` | Issues created/updated/closed in a window, optionally scoped to assigned-to-me or watched | Priya | (a) persona | Mechanical join-and-filter over synced data — keep |
| 5 | Cycle-time / time-to-resolution by tracker | `issues cycle-time --group-by tracker [--project X]` | Avg days between `created_on` and `closed_on`, grouped by tracker/project | Marcus | (a) persona / (c) cross-entity | Not the generic `analytics` count — needs a duration computation, hand-coded — keep |
| 6 | Transitive blocker-chain traversal | `issues blockers <id> [--depth N]` | Full multi-hop "blocks"/"blocked by" dependency chain for an issue | Priya | (b) content pattern | Local recursive query over synced `issue_relations` — keep |
| 7 | Wiki page diff | `wiki diff <project> <page> --from N --to M` | Textual diff between two wiki page versions | Devika | (b) content pattern | Verifiability fine, but weak evidence — flag for cut pass |
| 8 | Time budget / estimate variance report | `time budget --group-by project` | Compares `estimated_hours` vs. summed logged time per issue/project, flags overruns | Marcus | (c) cross-entity | Overlaps candidates 1 & 5 — flag for sibling review |
| 9 | Project health scorecard | `projects health [name]` | Open/overdue issue counts + last-activity date per project, one command | Devika | (c) cross-entity | Overlaps candidates 1, 2, 3 combined — flag for sibling review |
| 10 | Version release readiness check | `versions readiness <version>` | Boolean-style check: any open issues left, any overdue | Marcus | (c) cross-entity | Near-duplicate of candidate 1 — flag for merge/cut |
| 11 | Duplicate / near-duplicate issue detection | `issues duplicates <id>` | Finds issues with similar subject/description to a given issue | Priya | (b) content pattern | LLM dependency check: semantic similarity is classification — reframe to keyword/FTS substring match only, flag low value after reframe |
| 12 | Name→ID smart resolution (tracker/status/priority/project by name, not numeric ID) | n/a — cross-cutting flag behavior on existing commands | Lets every absorbed CRUD command accept human names instead of memorized IDs | all | (a) persona (Build Priorities #3) | Scope-creep check: not a standalone command, it's implementation behavior baked into already-absorbed commands — not eligible as a showcase feature row, cut from candidate pool |
| 13 | Custom field usage audit | `custom-fields audit` | Lists which custom fields are unset across issues in a project | Devika | (c) cross-entity | No brief evidence, speculative — flag for cut |
| 14 | Repository revision → issue commit digest | `repo-issues digest <project>` | Summary of commits linked to issues in a window | (none named) | (c) cross-entity | Reimplementation/overlap check: duplicates already-absorbed manifest item #18 (`repository-issues list/get`) — flag for cut |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Roadmap burndown | `roadmap burndown <version> [--project X]` | 10/10 | hand-code | This uses the locally synced `issues` and `versions` tables to compute open/closed counts, avg `done_ratio`, and estimated-vs-logged hours per version with no external dependencies. | Brief Data Profile: "the API can't natively answer cross-entity questions... without N+1 requests"; Top Workflows #3: "Redmine's own Roadmap UI page has no REST equivalent"; Build Priorities #3 names burndown explicitly. | Use this for version/roadmap progress and close-readiness. Do NOT use it for a single issue's status; use `issues get` instead. |
| 2 | Assignee workload/overload report | `workload [--project X] [--threshold N]` | 9/10 | hand-code | This uses the locally synced `issues` table grouped by `assigned_to_id` to sum open-issue counts and estimated hours per user with no external dependencies. | Build Priorities #3 names "assignee-overload detection" explicitly as a transcend target. | Use this for an aggregate view across all assignees. Do NOT use it to see one person's issue list; use `issues list --assigned-to <user>` instead. |
| 3 | Stale issue detection | `issues stale [--project X] --days 14` | 9/10 | hand-code | This uses the locally synced `issues` table filtered on `updated_on` older than N days and `status` still open, with no external dependencies. | Build Priorities #3 names "stale-issue detection" explicitly as a transcend target. | Use this to find issues that have gone quiet (no recent activity). Do NOT use it for recently changed issues; use `digest` instead — its semantics are the opposite (recent activity, not inactivity). |
| 4 | Since-last-sync digest | `digest --since 7d [--mine] [--watched]` | 10/10 | hand-code | This uses the locally synced `issues` table filtered/joined on `created_on`/`updated_on`/`closed_on` within a window, optionally filtered to the current user's assignments or watches, with no external dependencies. | Top Workflows #5 describes this exact feature verbatim ("personal digest... requires join-and-filter logic no single Redmine endpoint provides"); Build Priorities #3 names "'since' digest" explicitly. | Use this for a personal activity report over a time window. Do NOT confuse with the framework's `sync --since`, which controls what data is pulled from the API, not what is reported from local data. Do NOT use for inactivity detection; use `issues stale` instead. |
| 5 | Cycle-time / time-to-resolution by tracker | `issues cycle-time --group-by tracker [--project X]` | 10/10 | hand-code | This uses the locally synced `issues` table to compute the average number of days between `created_on` and `closed_on`, grouped by tracker or project, with no external dependencies. | Data Profile section gives this as its exemplar cross-entity query verbatim: "average time-to-resolution by tracker across projects." | Use this for duration-based aggregation (average days to close). Do NOT use the generic `analytics` command for this — it only counts rows per group, it does not compute durations. |
| 6 | Transitive blocker-chain traversal | `issues blockers <id> [--depth N]` | 6/10 | hand-code | This uses the locally synced `issue_relations` table with a recursive traversal over "blocks"/"blocked by" edges to build the full multi-hop chain for an issue, with no external dependencies. | Top Workflows #1 names relation-aware triage ("view one issue with its full journal/history"; relations are part of table stakes); depth beyond one hop is not itself evidenced but is a direct mechanical extension of already-absorbed relation data. | Use this for the full transitive dependency chain across multiple issues. Do NOT use it for a single issue's direct relations; use `issues relations list <id>` instead (one hop only). |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Wiki page diff | Speculative persona need with no brief evidence (only "per-page version history exists" is cited, not diffing); wrapper-vs-leverage fails — two `wiki get-version` calls piped to any local `diff` tool gets the same result without a bespoke command | none (fetch both versions via absorbed `wiki get-version`) |
| Time budget / estimate variance report | Sibling-redundant with roadmap burndown and cycle-time — three separate time-based aggregate reports is scope creep for one CLI, and this one has the weakest brief evidence of the three | Roadmap burndown |
| Project health scorecard | Sibling-redundant — its signal (open/overdue counts, last activity) is a re-slice of what burndown, stale, and workload already surface individually; no distinct evidence in the brief | Roadmap burndown / Stale issue detection |
| Version release readiness check | Near-duplicate of roadmap burndown (a subset of the same computation: "any open issues left") — folding a second command in for a boolean the burndown command already reports is scope creep | Roadmap burndown |
| Duplicate / near-duplicate issue detection | LLM dependency check: true duplicate detection requires semantic similarity/classification; the mechanical reframe (keyword/FTS substring match) produces too many false positives to be verifiable or trustworthy, and has zero brief evidence either way | Digest / Stale issue detection (surface candidates for manual review instead) |
| Name→ID smart resolution (tracker/status/priority/project by name) | Scope check: not a standalone command — it's implementation behavior that belongs inside the already-absorbed CRUD commands' flag parsing, not a showcase feature row | (folds into every absorbed CRUD command, not a sibling) |
| Custom field usage audit | No brief evidence (research backing 0), speculative admin-niche use case with no cited pain point | none |
| Repository revision → issue commit digest | Reimplementation/overlap check: duplicates already-absorbed manifest item #18 (`repository-issues list/get`) — a second command surfacing the same join adds no new capability | Repository revisions ↔ issues linking (absorbed manifest #18) |
