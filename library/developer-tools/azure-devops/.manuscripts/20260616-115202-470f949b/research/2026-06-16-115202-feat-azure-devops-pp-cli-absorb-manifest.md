# Azure DevOps CLI — Absorb Manifest

## Absorb Manifest

### Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List projects | az devops project list | azure-devops-pp-cli projects list | --json, --agent, offline via sync |
| 2 | Create/delete/show project | az devops project create/delete/show | azure-devops-pp-cli projects create/delete/show | Dry-run, structured output |
| 3 | List/create/delete teams | az devops team create/delete/list/show | azure-devops-pp-cli teams list/create/delete/show | --json, member lists |
| 4 | List team members | az devops team list-member | azure-devops-pp-cli teams members | --json, offline |
| 5 | Create work item | az boards work-item create | azure-devops-pp-cli wit create | Stdin JSON, dry-run, bulk via template |
| 6 | Show work item | az boards work-item show | azure-devops-pp-cli wit get | --select for fields, --agent output |
| 7 | Update work item | az boards work-item update | azure-devops-pp-cli wit update | Dry-run, patch JSON |
| 8 | Delete work item | az boards work-item delete | azure-devops-pp-cli wit delete | Dry-run |
| 9 | WIQL query | az boards query | azure-devops-pp-cli wit query | --json, --csv, local FTS offline fallback |
| 10 | Work item relations add/remove | az boards work-item relation add/remove | azure-devops-pp-cli wit relate / wit unrelate | Typed relation types from spec |
| 11 | Batch get work items | microsoft/azure-devops-mcp wit_get_work_items_batch | azure-devops-pp-cli wit batch-get | Up to 200 IDs, --json |
| 12 | Batch update work items | microsoft/azure-devops-mcp wit_update_work_items_batch | azure-devops-pp-cli wit batch-update | Stdin JSONL, dry-run |
| 13 | Work item comments list/add | microsoft/azure-devops-mcp wit_list_work_item_comments | azure-devops-pp-cli wit comments / wit comment-add | --agent friendly |
| 14 | Work item revisions | microsoft/azure-devops-mcp wit_list_work_item_revisions | azure-devops-pp-cli wit revisions | Historical audit trail |
| 15 | My work items | microsoft/azure-devops-mcp wit_my_work_items | azure-devops-pp-cli wit mine | Offline via sync, --agent |
| 16 | Work items for iteration | microsoft/azure-devops-mcp wit_get_work_items_for_iteration | azure-devops-pp-cli wit sprint-items | Includes capacity/state breakdown |
| 17 | List backlogs | microsoft/azure-devops-mcp wit_list_backlogs | azure-devops-pp-cli wit backlogs | --json |
| 18 | Get/run WIQL query | microsoft/azure-devops-mcp wit_get_query_results_by_id | azure-devops-pp-cli wit query --saved <name> | Named query support |
| 19 | Work item attachment get | microsoft/azure-devops-mcp wit_get_work_item_attachment | azure-devops-pp-cli wit attachment | Download to file |
| 20 | Link work item to PR | microsoft/azure-devops-mcp wit_link_work_item_to_pull_request | azure-devops-pp-cli wit link-pr | Bidirectional |
| 21 | Add child work items | microsoft/azure-devops-mcp wit_add_child_work_items | azure-devops-pp-cli wit add-children | Bulk parent-child wiring |
| 22 | Area paths manage (project) | az boards area project create/delete/list | azure-devops-pp-cli areas project list/create/delete | --json |
| 23 | Area paths manage (team) | az boards area team add/list/remove | azure-devops-pp-cli areas team add/remove | |
| 24 | Iteration manage (project) | az boards iteration project create/delete/list | azure-devops-pp-cli iterations project list/create/delete | --json |
| 25 | Iteration manage (team) | az boards iteration team add/list/remove/list-work-items | azure-devops-pp-cli iterations team list/add/remove | |
| 26 | List repos | az repos list | azure-devops-pp-cli repos list | --json, offline sync |
| 27 | Create/delete/show repo | az repos create/delete/show | azure-devops-pp-cli repos create/delete/show | Dry-run |
| 28 | Import repo from external git | az repos import create | azure-devops-pp-cli repos import | |
| 29 | List/create/delete refs | az repos ref list/create/delete | azure-devops-pp-cli refs list/create/delete | |
| 30 | Lock/unlock ref | az repos ref lock/unlock | azure-devops-pp-cli refs lock / refs unlock | |
| 31 | List branches | microsoft/azure-devops-mcp repo_list_branches_by_repo | azure-devops-pp-cli branches list | --json, filter by prefix |
| 32 | List my branches | microsoft/azure-devops-mcp repo_list_my_branches_by_repo | azure-devops-pp-cli branches mine | Offline via sync |
| 33 | Create branch | microsoft/azure-devops-mcp repo_create_branch | azure-devops-pp-cli branches create | --from-work-item auto-names |
| 34 | Create pull request | az repos pr create | azure-devops-pp-cli pr create | --stdin for description, --draft, --auto-complete |
| 35 | List pull requests | az repos pr list | azure-devops-pp-cli pr list | Offline via sync, --mine, --reviewer |
| 36 | Show pull request | az repos pr show | azure-devops-pp-cli pr get | --select fields, --agent |
| 37 | Update pull request | az repos pr update | azure-devops-pp-cli pr update | Dry-run |
| 38 | Checkout PR branch | az repos pr checkout | azure-devops-pp-cli pr checkout | Opens in shell |
| 39 | PR vote | az repos pr set-vote | azure-devops-pp-cli pr vote | approve/reject/reset |
| 40 | PR reviewer add/remove | az repos pr reviewer add/remove | azure-devops-pp-cli pr reviewers add/remove | |
| 41 | PR linked work items | az repos pr work-item add/list/remove | azure-devops-pp-cli pr work-items | |
| 42 | PR policy list/queue | az repos pr policy list/queue | azure-devops-pp-cli pr policies | |
| 43 | Get PR changes (line diffs) | microsoft/azure-devops-mcp repo_get_pull_request_changes | azure-devops-pp-cli pr diff | --json, file-level and line-level |
| 44 | PR threads + comments | microsoft/azure-devops-mcp repo_list_pull_request_threads | azure-devops-pp-cli pr threads / pr comment | |
| 45 | Search commits | microsoft/azure-devops-mcp repo_search_commits | azure-devops-pp-cli commits search | Full-text, date range |
| 46 | List directory/file content | microsoft/azure-devops-mcp repo_list_directory | azure-devops-pp-cli repos browse / repos cat | |
| 47 | Branch policies (generic) | az repos policy create/list | azure-devops-pp-cli policies list/create | |
| 48 | Branch policy types (approver-count, build, merge-strategy, comment-required, etc.) | az repos policy approver-count... | azure-devops-pp-cli policies approver/build/merge/comment-required | Per type commands |
| 49 | List builds | az pipelines build list | azure-devops-pp-cli builds list | Offline via sync, --json |
| 50 | Queue build | az pipelines build queue | azure-devops-pp-cli builds queue | --dry-run, --variables |
| 51 | Cancel build | az pipelines build cancel | azure-devops-pp-cli builds cancel | |
| 52 | Get build | az pipelines build show | azure-devops-pp-cli builds get | --select, --agent |
| 53 | Build definitions list/show | az pipelines build definition list/show | azure-devops-pp-cli build-defs list/get | |
| 54 | Build tags add/delete | az pipelines build tag | azure-devops-pp-cli builds tags | |
| 55 | Get build log | microsoft/azure-devops-mcp pipelines_get_build_log | azure-devops-pp-cli builds log | --tail N lines, stream |
| 56 | Get build changes | microsoft/azure-devops-mcp pipelines_get_build_changes | azure-devops-pp-cli builds changes | |
| 57 | List/download build artifacts | microsoft/azure-devops-mcp pipelines_list_artifacts | azure-devops-pp-cli builds artifacts / builds artifact-download | |
| 58 | List/create/run YAML pipelines | az pipelines run | azure-devops-pp-cli pipelines list/create/run | --variables JSON, dry-run |
| 59 | Get pipeline run | microsoft/azure-devops-mcp pipelines_get_run | azure-devops-pp-cli pipelines runs get | --poll until complete |
| 60 | List pipeline runs | microsoft/azure-devops-mcp pipelines_list_runs | azure-devops-pp-cli pipelines runs list | Offline via sync |
| 61 | Update build stage (retry/cancel) | microsoft/azure-devops-mcp pipelines_update_build_stage | azure-devops-pp-cli builds stage-retry / stage-cancel | |
| 62 | Agent pool/queue list/show | az pipelines pool list/show | azure-devops-pp-cli agents pools / agents queues | |
| 63 | Agent list/show | az pipelines agent list/show | azure-devops-pp-cli agents list | Status, capabilities |
| 64 | Pipeline folders CRUD | az pipelines folder | azure-devops-pp-cli pipelines folders | |
| 65 | Pipeline variables CRUD | az pipelines variable | azure-devops-pp-cli pipelines variables | |
| 66 | Variable groups CRUD | az pipelines variable-group | azure-devops-pp-cli variable-groups | |
| 67 | Create release | az pipelines release create | azure-devops-pp-cli releases create | --dry-run |
| 68 | List/show releases | az pipelines release list/show | azure-devops-pp-cli releases list/get | --json, offline |
| 69 | List release definitions | az pipelines release definition list/show | azure-devops-pp-cli release-defs list/get | |
| 70 | Pipeline runs artifacts | az pipelines runs artifact | azure-devops-pp-cli pipelines runs artifacts | |
| 71 | Search code | microsoft/azure-devops-mcp search_code | azure-devops-pp-cli search code | --json, cross-repo |
| 72 | Search work items | microsoft/azure-devops-mcp search_workitem | azure-devops-pp-cli search wit | Full-text across projects |
| 73 | Search wiki | microsoft/azure-devops-mcp search_wiki | azure-devops-pp-cli search wiki | |
| 74 | List wikis | microsoft/azure-devops-mcp wiki_list_wikis | azure-devops-pp-cli wikis list | |
| 75 | Get/list wiki pages | microsoft/azure-devops-mcp wiki_list_pages | azure-devops-pp-cli wikis pages / wikis page-get | |
| 76 | Create/update wiki pages | microsoft/azure-devops-mcp wiki_create_or_update_page | azure-devops-pp-cli wikis page-create / page-update | --stdin for content |
| 77 | List test plans | microsoft/azure-devops-mcp testplan_list_test_plans | azure-devops-pp-cli test-plans list | |
| 78 | Create test plan | microsoft/azure-devops-mcp testplan_create_test_plan | azure-devops-pp-cli test-plans create | |
| 79 | List/create test suites | microsoft/azure-devops-mcp testplan_list_test_suites | azure-devops-pp-cli test-suites list/create | |
| 80 | Add/list test cases | microsoft/azure-devops-mcp testplan_add_test_cases_to_suite | azure-devops-pp-cli test-cases add/list | |
| 81 | Create/update test case steps | microsoft/azure-devops-mcp testplan_create_test_case | azure-devops-pp-cli test-cases create/update | |
| 82 | Test results from build | microsoft/azure-devops-mcp testplan_show_test_results_from_build_id | azure-devops-pp-cli test-results build | --json, pass/fail counts |
| 83 | List team iterations | microsoft/azure-devops-mcp work_list_team_iterations | azure-devops-pp-cli sprints list | --current, --future |
| 84 | Iteration capacities | microsoft/azure-devops-mcp work_get_iteration_capacities | azure-devops-pp-cli sprints capacity | Per-team-member breakdown |
| 85 | Get/update team capacity | microsoft/azure-devops-mcp work_get_team_capacity | azure-devops-pp-cli sprints capacity update | --dry-run |
| 86 | Team settings get | microsoft/azure-devops-mcp work_get_team_settings | azure-devops-pp-cli teams settings | |
| 87 | Create iterations | microsoft/azure-devops-mcp work_create_iterations | azure-devops-pp-cli iterations create | |
| 88 | Advanced Security alerts | microsoft/azure-devops-mcp advsec_get_alerts | azure-devops-pp-cli security alerts | --json, by severity |
| 89 | User add/list/remove | az devops user add/list/remove/show/update | azure-devops-pp-cli users list/add/remove | License management |
| 90 | Security groups CRUD | az devops security group | azure-devops-pp-cli security groups | |
| 91 | Security group membership | az devops security group membership | azure-devops-pp-cli security members | |
| 92 | Service endpoints list/show | az devops service-endpoint list/show | azure-devops-pp-cli service-connections list/get | |
| 93 | Service endpoint create (Azure RM, GitHub) | az devops service-endpoint azurerm/github create | azure-devops-pp-cli service-connections create | Type-specific options |
| 94 | Extension install/list/search | az devops extension | azure-devops-pp-cli extensions list/install/search | |
| 95 | Admin banners CRUD | az devops admin banner | azure-devops-pp-cli admin banners | |
| 96 | Raw API invoke | az devops invoke | azure-devops-pp-cli api invoke | Escape hatch: any path, method, body |
| 97 | Pipeline run tags | az pipelines runs tag add/delete/list | azure-devops-pp-cli pipelines runs tags | |
| 98 | Get identity IDs | microsoft/azure-devops-mcp core_get_identity_ids | azure-devops-pp-cli identity lookup | |
| 99 | PR create with work item auto-link | RyanCardin15/AzureDevOps-MCP | (behavior in azure-devops-pp-cli pr create) --link-work-items flag | |
| 100 | Board card movement | RyanCardin15/AzureDevOps-MCP moveCardsOnBoards | azure-devops-pp-cli boards move-card | Move between columns with dry-run |
| 101 | Backlog sort/hygiene check | adopt (PyPI) | azure-devops-pp-cli wit hygiene | Flags items missing area, iteration, or story points |

---

### Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| T1 | Sprint velocity trend | velocity | 8/10 | hand-code | Joins sprints + work_items + state_history across N sprints in local SQLite; requires synced revision data unavailable from a single live call | none |
| T2 | PR aging report | pr aging | 8/10 | hand-code | Cross-joins local PR + work_item + timeline tables to find PRs with no activity; no single ADO endpoint aggregates this | Use this command to find stale PRs. Do NOT use it for individual PR status; use 'pr get' instead. |
| T3 | Daily standup digest | standup | 9/10 | hand-code | Aggregates: PRs awaiting my review, my in-progress work items, newly failed builds — cross-entity, from local SQLite in one command | none |
| T4 | Pipeline flakiness score | pipeline flaky | 8/10 | hand-code | Joins build_history by definition, computes intermittent failure rate per stage; requires build history volume infeasible to compute live | none |
| T5 | Work item cycle time | wit cycle-time | 8/10 | hand-code | Joins work_item_revisions to compute Active→Done duration per type; revision history is expensive to paginate live — local store is required | Use this to analyze cycle time by work item type. Do NOT use for sprint velocity; use 'velocity' instead. |
| T6 | Branch stale cleanup plan | branches stale | 7/10 | hand-code | Joins synced branches × PRs × last-commit dates; flags merged-but-not-deleted and branches with no recent commit | none |
| T7 | Build cost trend | builds cost | 7/10 | hand-code | Joins agent_queue + build_history + duration; computes agent minutes per pipeline per week — ADO Analytics has raw data but no tool surfaces it as CLI trend | none |
| T8 | Cross-pipeline pending approvals | release gate-queue | 8/10 | hand-code | Merges classic release approvals API + YAML pipeline approvals API, computes wait time; no existing CLI or MCP unifies both approval types | none |
| T9 | Sprint scope creep detector | work sprint-creep | 8/10 | hand-code | Joins work_item_revisions (IterationPath field change timestamps) against sprint start date to count story points added after sprint started | none |
| T10 | PR review readiness queue | pr review-queue | 9/10 | hand-code | Joins local PRs (reviewer assignments + vote status) × live build status per PR; orders by build-green + unvoted first; no existing tool provides readiness-ordered review queue | none |
| T11 | Work item field diff | work diff | 7/10 | hand-code | Calls GET /wit/workItems/{id}/revisions/{n} for two revisions, diffs field maps locally, renders human-readable changelog | none |
| T12 | Sprint rollover counter | work rollover | 8/10 | hand-code | Scans work_item_revisions for IterationPath changes across sprint boundaries; surfaces items moved to new sprint more than once | none |
| T13 | Area path work item load | work area-load | 7/10 | hand-code | SQLite GROUP BY areaPath over synced work_items; returns open item counts + story point sums per area; WIQL GROUP BY not available in WIT query API | none |
| T14 | Commit-to-build traceability | git commit-builds | 7/10 | hand-code | Joins local builds × build_changes tables on commit SHA; returns all build outcomes for a given commit; no ADO endpoint offers "builds by commit" | none |
| T15 | Cross-repo branch health | git branch-health | 7/10 | hand-code | Four-way SQLite join: repos × branches (last-commit age) × builds (latest status per default branch) × prs (open count per repo) | Use this command for per-repo summary across all repos. Do NOT use it for single-repo branch listing; use 'branches list' instead. |

---

**Notes:**
- All absorbed commands include: `--json` (structured output), `--agent` (token-efficient JSON for AI agents), `--dry-run` (show request without sending), `--select <fields>` (filter output fields)
- All list commands include: `--limit N`, `--json`, offline mode via local SQLite mirror
- Auth: PAT via `AZURE_DEVOPS_TOKEN` (Basic auth), `auth login --chrome` for browser session import
- Configuration: `--org` / `AZURE_DEVOPS_ORG`, `--project` / `AZURE_DEVOPS_PROJECT`, `--api-version` (default 7.1)
