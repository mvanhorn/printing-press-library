# Rightbrain CLI — Absorb Manifest

Sources catalogued: `rightbrain` npm CLI v0.3.0, `@rightbrain/sdk` v0.3.0,
`@rightbrain/brain-api-client`, `rightbrain-agent-skill` (`rb-api.mjs`),
`RightbrainAI/terraform-provider-tasks`, `RightbrainAI/github-action-tasks`,
`RightbrainAI/bruno`, `RightbrainAI/cursor-plugin`, and the official OpenAPI 3.0.3
spec (312 user-reachable operations).

No third-party tooling exists for this API. Every source above is first-party.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Browser OAuth login | rightbrain npm CLI `login` | **NOT BUILT** — the CLI consumes an already-minted token via `rightbrain-pp-cli auth set-token` or `RB_API_KEY`; it performs no browser OAuth exchange | Parity gap vs the incumbent |
| 2 | Logout / clear creds | rightbrain npm CLI `logout` | `rightbrain-pp-cli auth logout` | Typed exit codes |
| 3 | Show current identity | rightbrain npm CLI `whoami` | `rightbrain-pp-cli whoami` | `--json`, `--select`, cached offline |
| 4 | Print access token | rightbrain npm CLI `token --json` | **NOT BUILT** — `rightbrain-pp-cli auth status` reports the credential source without printing the secret | Parity gap vs the incumbent (deliberate: printing tokens to stdout is a leak vector) |
| 5 | Bootstrap project credentials | rightbrain npm CLI `init` | `rightbrain-pp-cli auth setup` | Writes CLI config; no `.env` mutation required |
| 6 | Switch active project | rightbrain npm CLI `switch-project` | `rightbrain-pp-cli scope use` | Sets org and project together, and every generated command then omits both UUIDs |
| 7 | Environment select | rightbrain npm CLI `env list` / `env use` | **NOT BUILT** — a single environment is addressed via `RIGHTBRAIN_BASE_URL` / config `base_url` | Parity gap vs the incumbent |
| 8 | TypeScript codegen | rightbrain npm CLI `generate` | (out of scope — TypeScript-specific, not a CLI capability) | n/a |
| 9 | List organizations | rightbrain npm CLI `organization list` | `(generated endpoint) org list` | Offline mirror, `--json` |
| 10 | List projects | rightbrain npm CLI `project list` | `(generated endpoint) project list` | Offline mirror |
| 11 | List tasks | rightbrain npm CLI `task list` | `(generated endpoint) task list` | Offline, FTS-searchable |
| 12 | Get task | rightbrain npm CLI `task get` | `(generated endpoint) task get` | `--select` field filtering |
| 13 | Create task | rightbrain npm CLI `task create` | `(generated endpoint) task create` | `--dry-run`, stdin body |
| 14 | Update task | rightbrain npm CLI `task update` | `(generated endpoint) task update` | `--dry-run` |
| 15 | Run task | rightbrain npm CLI `task run` / SDK `runTask` / GitHub Action | `(generated endpoint) task run` | Revision pin, file attach, fallback model, `--json` |
| 16 | List models | rightbrain npm CLI `model list` | `(generated endpoint) model list` | Offline mirror |
| 17 | Attach files to a run | `@rightbrain/sdk` `runTask({files})` | `(behavior in rightbrain-pp-cli task run) --file` | Base64 and multipart both supported |
| 18 | Pin revision at run time | `@rightbrain/sdk` `runTask({revisionId})` | `(behavior in rightbrain-pp-cli task run) --revision-id / --revision-tag` | Both id and tag |
| 19 | Force fallback model | `@rightbrain/sdk` `runTask({useFallbackModel})` | `(behavior in rightbrain-pp-cli task run) --use-fallback-model` | — |
| 20 | Generic authenticated HTTP | rightbrain-agent-skill `rb-api.mjs` | `rightbrain-pp-cli api` | Typed exit codes, automatic org/project scoping |
| 21 | Auto-paginate everything | `rb-api.mjs --all` | `(behavior in rightbrain-pp-cli sync) --max-pages` | Cursor-aware and persisted, not just streamed |
| 22 | SSE streaming passthrough | `rb-api.mjs --sse` | `(behavior in rightbrain-pp-cli task-agent run) --stream` | Event-typed rendering, not raw firehose |
| 23 | Dry-run any call | `rb-api.mjs --dry-run` | `(behavior in every mutating command) --dry-run` | Framework-wide, not one script |
| 24 | Declarative task management | Terraform provider `rightbrain_task` | `(generated endpoint) task create` / `task update` | Also covers agents, skills, collections, triggers — which the provider cannot |
| 25 | Agents CRUD + run | (none — API only) | `(generated endpoint) task-agent list` | First CLI to expose it |
| 26 | Skills CRUD + global catalog | (none — API only) | `(generated endpoint) skill list` | First CLI to expose it |
| 27 | Collections / RAG query | (none — API only) | `(generated endpoint) collection list` | First CLI to expose it |
| 28 | Documents | (none — API only) | `(generated endpoint) document list` | First CLI to expose it |
| 29 | Triggers (webhook/schedule/gmail) | (none — API only) | `(generated endpoint) trigger list` | First CLI to expose it |
| 30 | Evals (sets + runs) | (none — API only) | `(generated endpoint) task eval-set list` | First CLI to expose it |
| 31 | Approval requests | (none — API only) | `(generated endpoint) task-agent approval-request list` | First CLI to expose it |
| 32 | MCP servers (inbound + outbound) | Bruno collection (manual) | `(generated endpoint) mcp-server list` | First CLI to expose it |
| 33 | Integrations | (none — API only) | `(generated endpoint) integration list` | First CLI to expose it |
| 34 | Audit events + integrity verify | (none — API only) | `(generated endpoint) audit-event list` | First CLI to expose it |
| 35 | API keys (incl. revoke) | Bruno collection | `(generated endpoint) api-key list` | Revoke included |
| 36 | OAuth clients | Bruno collection | `(generated endpoint) oauth-client list` | — |
| 37 | Task forwarders (webhook + email) | Bruno collection | `(generated endpoint) task-forwarder list` | — |
| 38 | Input processors | Bruno collection | `(generated endpoint) input-processor list` | — |
| 39 | Guardrails | Bruno collection | `(generated endpoint) guardrail list` | — |
| 40 | Tags | Bruno collection | `(generated endpoint) tags list` | — |
| 41 | Reporting groups | (none — API only) | `(generated endpoint) reporting-group list` | — |
| 42 | Usage / timing / credit reports | (none — API only) | `(generated endpoint) task usage-report` | — |
| 43 | Sharing + cloning | (none — API only) | `(generated endpoint) task share` | — |
| 44 | IAM members / permission test | (none — API only) | `(generated endpoint) iam members` | — |
| 45 | Health / readiness rollup | (none) | `rightbrain-pp-cli doctor` | Rolls up auth + reachability + cache state |
| 46 | Scrape | (none — API only) | `(generated endpoint) scrape list` | — |
| 47 | Compose | (none — API only) | `(generated endpoint) compose list` | — |
| 48 | Slack integration | (none — API only) | `(generated endpoint) slack list` | — |
| 49 | Local mirror + incremental sync | (none anywhere) | `rightbrain-pp-cli sync --resources tasks,task_runs --since 7d` | No Rightbrain tool keeps any local state |
| 50 | Offline cross-entity search | (none anywhere) | `rightbrain-pp-cli search "term" --type tasks` | FTS over tasks, agents, runs, skills, audit events |
| 51 | Local SQL over synced data | (none anywhere) | **NOT BUILT** — this generator did not emit a `sql` command for this spec; `search` and `analytics` cover the local-query surface | Claimed in error at Phase 1.5 |
| 52 | Grouped analytics | (none anywhere) | `rightbrain-pp-cli analytics --type task_runs --group-by status` | — |

No stubs. Every row above is shipping scope.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Eval release gate | `gate <task-id> --revision-id <id> --min-pass-rate 0.9` | 10/10 | hand-code | Calls the eval-run endpoint for a candidate revision, then compares its pass rate against the last stored eval run for the currently-weighted production revision in local SQLite, exiting non-zero on regression. The API has no compare-to-baseline call and keeps no baseline. | Use this command to decide whether one candidate revision is safe to promote. Do NOT use this command to inspect live traffic split or observed latency of already-promoted revisions; use 'rollout' instead. Do NOT use it for per-test-case failure history across many eval runs; use 'eval-flake' instead. |
| 2 | Rollout reality check | `rollout <task-id> --since 7d` | 9/10 | hand-code | Joins live revision weights and tags against locally-mirrored run rows to compute observed traffic share, failure rate, mean credits, and p50/p95 latency per revision. The API exposes configured weights but never observed traffic. | Use this command to see how a task's configured revision weights compare with the traffic and latency actually observed. Do NOT use it to decide whether a revision passes its eval set; use 'gate' instead. Do NOT use it for a project-wide, cross-task regression sweep; use 'drift' instead. |
| 3 | Stalled approval triage | `approvals --older-than 1h` | 7/10 | hand-code | Reads the project-scoped approval list and computes what the API does not return: parked age, time-to-expiry, and the split between still-actionable requests and those whose approval window already lapsed, sorted by urgency. | Use this command to sweep every agent for runs parked awaiting human approval. Do NOT use it to inspect what a single run did step by step; use 'agent-trace' instead. |
| 4 | Agent run trace | `agent-trace <run-id> --tools` | 8/10 | hand-code | Reads the run's raw event array, pairs `tool_call` with `tool_result`, derives per-step elapsed time and a tool-duration histogram, and marks approval pauses and terminal status inline. Existing tooling offers raw SSE passthrough with no structure or timing. | Use this command to reconstruct one agent run's event timeline by run id. Do NOT use it to find which runs are currently parked awaiting approval; use 'approvals' instead. |
| 5 | Cost and latency drift | `drift --since 7d --group-by task` | 9/10 | hand-code | Runs a windowed self-comparison over mirrored run rows against the preceding window, annotating each mover with the revision that became dominant in that window. The API's report endpoints are per-task and un-windowed. | Use this command for project-wide, week-over-week movement in cost, latency, and failure rate across all tasks and agents. Do NOT use it to inspect one task's revision weight split; use 'rollout' instead. |
| 6 | Named, verified changelog | `changelog --since 7d --verify` | 7/10 | hand-code | Lists mirrored audit events for a window, resolves each event's resource UUID to a task/agent/skill/collection name from local SQLite, and attaches the cryptographic integrity verdict. Audit events reference bare UUIDs only. | Use this command for what changed in the project's configuration over a window, with UUIDs resolved to names. Do NOT use it for changes in cost or latency metrics; use 'drift' instead. |
| 7 | Eval flake ranking | `eval-flake <task-id> --last 10` | 7/10 | hand-code | Aggregates mirrored eval-run results across the last N runs to rank test cases by failure frequency and separate cases that flip on an unchanged revision from cases that fail consistently. Requires N pages the API only serves one at a time. | Use this command for per-test-case failure history across many eval runs. Do NOT use it for a single pass/fail promotion decision on one candidate revision; use 'gate' instead. |

**Hand-code commitment:** 7 of 7 transcendence rows are tagged `hand-code`. All
require post-generate Go (SQLite joins, endpoint fan-out, custom output shapes)
plus `root.go` wiring.

**Data-source directives:** `gate` `auto`, `rollout` `auto`, `approvals` `live`,
`agent-trace` `auto`, `drift` `local`, `changelog` `auto`, `eval-flake` `local`.

Killed candidates and the full customer model are preserved in
`2026-07-31-212537-novel-features-brainstorm.md`.

## Corrections applied after spec verification

Before building, every transcendence premise was re-checked against the OpenAPI
schemas rather than trusted from the brainstorm. One was wrong:

- **`approvals`** — the brainstorm asserted that approval requests are per-agent
  only and that a project-wide queue "exists nowhere", requiring an O(N) fan-out.
  That is false. `GET /org/{org_id}/project/{project_id}/task-agent/approval-request`
  (`listProjectTaskAgentApprovalRequests`) is project-scoped and supports
  `status`, `task_agent_id`, `run_id`, `session_id`, `tool_type`, `created_after`,
  and `created_before` filters, returning `task_agent_name`, `tool_name`,
  `status`, `created`, and `expires_at`. The command still ships under the same
  name and serves the same user need, but it is implemented as a single call
  plus computation, not a fan-out, and its rationale and score were corrected
  (10/10 -> 7/10) rather than left overstating the case.

The other six premises verified clean against the schemas:
`EvalRunResponse.pass_rate` / `pass_count` / `fail_count` (gate),
`TaskActiveRevision.weight` (rollout), `TaskRun.charged_credits` /
`llm_call_timing` / `is_error` / `task_revision_id` (rollout, drift),
`EvalRunDetailResponse.results[].verdict` (eval-flake),
`AuditEventListItem.resource_id` / `resource_type` plus
`/audit_event/integrity/verify` (changelog), and the agent run events endpoint
(agent-trace).

## Generator defects encountered (retro candidates)

1. **Composed resource-name collision.** A top-level resource `task_run` and a
   sub-resource `task/{task_id}/run` both normalize to `newProjectTaskRunCmd`,
   producing a redeclaration that fails compilation. The same class affects
   `task_share` vs `task/{id}/share` and `task_agent_share` vs
   `task-agent/{id}/share`. Worked around with `x-pp-resource` on the operations.
2. **`x-pp-resource` is not threaded into the sync profiler.** After the command
   layer honored the override, `internal/cli/sync.go` still emitted a duplicate
   `case "project_task_run"` because the syncable-resource profiler derives its
   key from the path rather than the override. Worked around with
   `x-pp-syncable: false` on the two redundant run paths.

3. **`x-pp-syncable: false` is not honored on an operation.** Setting it on the
   two redundant run-list paths left all three `dependentResourceDef` entries
   named `project_task_run` in place, so `syncResourceSinceParam` still emitted a
   duplicate `case`. Resolved only by removing the colliding operations from the
   spec.

### Endpoints removed to work around defect 3 (no capability lost)

- `GET /org/{org_id}/project/{project_id}/task/{task_id}/run` (`getTaskRuns`) —
  a strict subset of `listProjectTaskRuns`, which accepts the same
  `status`, `task_revision_id`, `start_date`, `end_date`, `page_limit`, and
  `cursor` params **plus** `task_id` as a query filter. `runs list --task-id <id>`
  returns identical rows. The `POST` on the same path — the task-execution
  command and the API's single most important operation — is untouched.
- `GET /org/{org_id}/project/{project_id}/task_run/recent_by_task`
  (`listProjectRecentTaskRunsByTask`) — a convenience ordering of the same rows
  `runs list` returns.

Generated surface: **310 operations**.

## Absorbed-row verification (run at the Phase 3 gate)

Every absorbed row declaring a clean command path was resolved against the built
binary. Five rows were found to claim commands that do not exist and have been
corrected above rather than left overstating parity:

- `auth login` (browser OAuth) — not built
- `auth token` (print access token) — not built, and deliberately so
- `env list` / `env use` (environment switching) — not built
- `sql` (arbitrary local SELECT) — not built
- `project use` — superseded by the hand-built `scope use`, which sets org and project together

None of these appear in the shipped README or SKILL, so no user-facing document
claims them; the overstatement was confined to this manifest. The remaining
absorbed rows (`whoami`, `auth setup`, `auth logout`, `api`, `doctor`, `sync`,
`search`, `analytics`, `scope use`, and the generated endpoint surface) all
resolve.
