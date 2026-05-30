## Customer model

**Persona A — Wade, the multi-client CPA-and-ops operator**
Today: runs scenarios as glue between SmartSuite, Buzzsprout, WordPress, Descript across 3+ teams. Watches UI live exec panel; finds out about 2am failures the next morning. Weekly ritual: Monday triage — opens Make UI for each team, eyeballs scenarios by last-edit, clicks orange/red icons, opens DLQs one scenario at a time. Frustration: no cross-team view; no "what broke this week"; connection expiry hidden until 401.

**Persona B — Riley, the agent-builder integrator**
Today: wiring a Claude agent that needs to fire a Make scenario and act on the parsed result. POST /scenarios/{id}/run returns immediately with executionId; Riley writes polling + backoff + bundle parsing himself. Weekly ritual: iterate scenario → run from terminal → poll execution → copy bundle. Frustration: synchronous execution is the whole point of using Make for agents, and nothing in the official stack provides it.

**Persona C — Sam, the dev→prod scenario-promoter consultant**
Today: builds in sandbox team, recreates in prod team. Manual blueprint export → paste → walk module list re-pinning connectionId/hookId/dataStoreId. Weekly ritual: end-of-week dev→prod cutover. Frustration: ID-remap problem is undocumented; metadata.expect/restore silently changes UI defaults; no git story; community DIY scenarios push blueprints to GitHub daily because no first-party tool does.

**Persona D — Jordan, the DLQ-triager / on-call operator**
Today: owns ~40 active scenarios. UI has no cross-scenario DLQ view; per-scenario retry/resolve clicks. Weekly ritual: morning DLQ click-through. Frustration: no bulk action; no error-reason grouping; no cross-scenario 24h failure view.

## Candidates (pre-cut)

(16 candidates generated — see Survivors and Kills below for which made it.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Blocking agent run | `scenarios run <id> --wait --timeout 5m --json` | 10/10 | POST /scenarios/{id}/run, poll executions with backoff until terminal, fetch output bundles, emit one envelope | User Vision; make-cli has no --wait; cloud MCP is paid + Anthropic-only |
| 2 | Git-backed blueprint sync | `blueprint sync --repo ./make-blueprints [--all-teams]` | 9/10 | Iterate scenarios per team, GET blueprints, write canonicalized JSON + sidecar files into repo, commit | Brief workflow #3; community DIY scenarios prove demand |
| 3 | Dev→prod promote with auto-remap | `blueprint promote --from-team <dev> --to-team <prod> --scenario <id> [--auto-suggest \| --map remap.yml] [--dry-run]` | 10/10 | Read source blueprint; fetch connections/hooks/data-stores in target; propose name-match map; rewrite IDs; POST to target team | Brief workflow #2 + Codebase Intelligence note ID-remap is "lossy and undocumented" |
| 4 | Cross-scenario DLQ inbox + bulk action | `dlq inbox [--team <id> \| --all-teams] [--age 24h] [--group-by reason] [--retry-all --match-reason <re>]` | 9/10 | SQL over synced dlqs + scenarios, group by error-reason fingerprint, bulk POST retry/resolve filtered by regex | Brief workflow #4; UI per-scenario only |
| 5 | Connections audit | `connections audit [--unused 30d] [--expiring 7d] [--errored 7d]` | 8/10 | Join connections × blueprint references × execution errors | Brief workflow #5; UI has no usage/expiry view |
| 6 | Cross-team scenario list + stale | `scenarios list --all-teams [--active] [--stale 30d] [--folder <path>]` | 8/10 | Union scenarios across all teams, left-join executions for last-run | Brief workflow #6; UI is single-team |
| 7 | Webhook→scenario routing map | `hooks map [--team <id> \| --all-teams] [--orphans] [--shared]` | 7/10 | Walk locally-cached blueprints for gateway:CustomWebHook hookId references, join to hooks table | No Make UI surface for hook→scenario reverse map |
| 8 | Blueprint diff & restore | `blueprint diff <scenarioId> [--from <snap>] [--to current]` + `blueprint restore <scenarioId> --snapshot <id>` | 7/10 | Read versioned snapshots from local store, compute structural diff, restore PUTs snapshot via API | Brief data-layer carves out snapshot table; Make UI history is shallow |

### Killed candidates

| Feature | Kill reason | Closest survivor |
|---------|-------------|------------------|
| remap-suggest standalone | Folded into S3 as --auto-suggest | S3 |
| DLQ patterns standalone | Folded into S4 as --group-by reason --summary | S4 |
| scenarios stale standalone | Folded into S6 as --stale Nd | S6 |
| blueprint packages inventory | Useful but not weekly; reachable via sql passthrough | (absorbed sql) |
| time-travel restore standalone | Restore without diff is dangerous; merged with diff | S8 |
| scenarios tail --follow | Thin polling loop; --wait + executions list --since covers the workflow | S1 |
| run-and-replay standalone | Folded into S1 as --replay <executionId> | S1 |
| team health snapshot | Dashboard reskin of S4 + S5 + S6; adds no new joins | S4/S5/S6 |
