# Rightbrain novel-features brainstorm (audit trail)

Subagent output from Phase 1.5 Step 1.5c.5. Customer model and killed candidates are
preserved here for retro/dogfood debugging; only `### Survivors` flows into the manifest.

## Customer model

**Priya — platform engineer at a 40-person B2B consultancy, owns the Rightbrain deployment for four client projects.**

*Today (without this CLI):* She has the Rightbrain dashboard open in four browser tabs, one per client project, because every path is `/org/{org_id}/project/{project_id}/…` and there is no way to see two projects at once. When she ships a prompt change she authors a revision in the dashboard, clicks into the eval set, runs it, eyeballs the pass rate against a number she wrote in a Slack thread last sprint, then goes back to the task settings and edits the traffic weights by hand. She cannot answer: "is the 20% canary revision actually receiving 20% of runs?" or "was last week's pass rate 84% or 91%?" — the eval history is a list of runs she has to page through one at a time.

*Weekly ritual:* Ship one or two task revisions per client. For each: author → run the eval set → compare pass rate to whatever is in production → set weights or a `revision_tag` → watch for a day → roll forward or back.

*Frustration:* There is no gate. The comparison between "candidate revision" and "what's live right now" is a manual, remembered, un-versioned act, and the traffic weights she sets are a *declaration* with no feedback loop telling her what traffic actually happened.

**Marco — AI engineer, iterates on one agent and three tasks daily, runs Claude Code in the same repo.**

*Today (without this CLI):* `curl` piped to `jq` in a scratch shell, plus `rb-api.mjs --sse` to watch an agent run scroll past. When an agent misbehaves he re-runs it and stares at the raw SSE firehose — `text`, `tool_call`, `tool_result` interleaved with no timing — trying to spot which tool call hung. When latency or credits move, he has no baseline: `charged_credits` and `llm_call_timing` come back per run, one page at a time, and he'd have to page through hundreds of runs across four tasks to know whether p95 moved. He cannot answer: "did this get slower after Tuesday?" or "which of the twelve steps in that agent run took nine seconds?"

*Weekly ritual:* Tweak a prompt or an agent's tool set, fire a dozen runs, compare outputs and metrics, chase the one run that failed weirdly.

*Frustration:* Debugging is archaeology. The event stream is ephemeral and unstructured, and the metrics needed to prove a regression exist per-run but never in aggregate.

**Dana — support/ops engineer on rotation, babysits the customer-facing agents.**

*Today (without this CLI):* Checks each agent one at a time for runs parked at `waiting_for_human`, because approval requests are per-agent (`/task-agent/{id}/approval-request`) and there is no project-wide pending queue anywhere — not in the dashboard, not in the API. A run that hits a gated tool stops dead and emits nothing further; nobody is paged. She finds out when a customer complains. She cannot answer: "what is parked right now, and how long has it been parked?"

*Weekly ritual:* Several times a week, sweep for stalled agent runs, read what tool the agent wants to call, approve/reject/resume.

*Frustration:* The sweep is O(number of agents) HTTP calls performed by hand, and staleness — the thing that actually matters — is nowhere in the response; she has to subtract timestamps herself.

**Sam — delivery lead, bills four clients monthly, answers their security questionnaires.**

*Today (without this CLI):* Pulls `usage_report`, `timing_report` and `credit_report` per task, one task at a time, and re-adds them in a spreadsheet, because the report endpoints are task-scoped and clients are project-scoped. For compliance he screenshots the audit log; the events reference resources by bare UUID, so he keeps a side-lookup of which UUID is which task. He cannot answer: "what changed in this client's project this week, in English?" or "is our audit log provably untampered?" without stitching several endpoints together.

*Weekly ritual:* Friday status per client — what changed, what it cost, what broke.

*Frustration:* Everything he needs is UUID-keyed and task-scoped, while everything he reports is name-keyed and project-scoped.

## Candidates (pre-cut)

Source (f) DeepWiki/Codebase-Intelligence active. No `## User Vision` section, so source (e) unused. First print, so source (d) skipped.

- **C1 — Eval release gate.** `gate` — run eval set against candidate revision, compare to production baseline, exit non-zero on regression. Persona: Priya. Source (a)+(b). **keep**
- **C2 — Rollout reality check.** `rollout` — configured weight vs observed traffic share, failure rate, credits, p50/p95 latency per revision. Persona: Priya, Marco. Source (b)+(c). **keep**
- **C3 — Stalled approval triage.** `approvals` — project-wide fan-out for runs parked at `waiting_for_human`, sorted by computed parked age. Persona: Dana. Source (a)+(b). **keep**
- **C4 — Agent run trace.** `agent-trace` — pair `tool_call`/`tool_result`, derive per-step elapsed time, tool histogram. Persona: Marco, Dana. Source (b)+(f). **keep (wrapper risk flagged)**
- **C5 — Cost/latency drift.** `drift` — windowed self-comparison across tasks/agents with revision correlation. Persona: Marco, Sam. Source (c). **keep**
- **C6 — Named, verified changelog.** `changelog` — audit events with UUIDs resolved to names + integrity verdict. Persona: Sam, Priya. Source (b)+(c). **keep**
- **C7 — Eval flake ranking.** `eval-flake` — rank test cases by failure frequency, separate flake from real defect. Persona: Priya, Marco. Source (c). **keep**
- **C8 — Revision diff.** `revision-diff`. **kill** (wrapper)
- **C9 — Guarded promote/rollback.** `promote`/`rollback`. **kill** (wrapper + fragile local snapshot)
- **C10 — Cross-task credit attribution.** `spend`. **kill** (framework `analytics` covers it)
- **C11 — Global cross-entity search.** `find`. **kill** (framework `search` covers it)
- **C12 — Effective agent toolset.** `agent-tools`. **kill** (not weekly)
- **C13 — Unused-resource sweep.** `orphans`. **kill** (monthly at best)
- **C14 — Agent session list/resume.** `agent-sessions`. **kill** (scope creep toward chat client)
- **C15 — Run comparison.** `run-compare`. **kill** (two GETs and a diff)

## Survivors and kills

Pass 3 force-answers:

- **C1 gate** — Weekly: yes (1-2 revisions per client per week). Wrapper: no; API has no compare-to-baseline call. Transcendence: local eval-run baseline + eval-as-release-gate pattern. Sibling killed: C9. Buildability: `hand-code`.
- **C2 rollout** — Weekly: yes. Wrapper: no; combines live weights with mirrored run aggregates. Transcendence: SQLite join + weighted-A/B pattern. Sibling killed: C8. Buildability: `hand-code`.
- **C3 approvals** — Weekly: yes, several times. Wrapper: no; per-agent fan-out plus computed parked age. Transcendence: cross-entity fan-out + `waiting_for_human` pattern. Sibling killed: C14. Buildability: `hand-code`.
- **C4 agent-trace** — Weekly: yes, daily. Wrapper: survives — pairs call/result events, derives elapsed times the API never returns. Transcendence: SSE event-stream pattern + agent-shaped output. Sibling killed: C12. Buildability: `hand-code`.
- **C5 drift** — Weekly: yes, Friday. Wrapper: no; report endpoints are per-task and un-windowed. Transcendence: local windowed aggregation + revision correlation. Sibling killed: C10. Buildability: `hand-code`.
- **C6 changelog** — Weekly: yes. Wrapper: no; joins audit UUIDs to names + integrity verdict. Transcendence: audit-integrity pattern + name resolution. Sibling killed: C11. Buildability: `hand-code`.
- **C7 eval-flake** — Weekly: yes, whenever the gate fails. Wrapper: no; requires N mirrored eval runs. Transcendence: local SQLite across pages. Sibling killed: C15. Buildability: `hand-code`.

Data-source directives: `gate` `auto`, `rollout` `auto`, `approvals` `live`, `agent-trace` `auto`, `drift` `local`, `changelog` `auto`, `eval-flake` `local`.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Revision diff (`revision-diff`) | Config half is two GETs the user can diff themselves; metric half is `rollout` restricted to two rows. | `rollout` |
| Guarded promote/rollback (`promote` / `rollback`) | Thin wrapper over the revision-weight PATCH whose only novelty is a local snapshot file that may not exist when rollback is needed. | `gate` |
| Cross-task credit attribution (`spend`) | The framework already emits `analytics --type task_runs --group-by <field>` over the same local table; the time-comparison value lives in `drift`. | `drift` |
| Global cross-entity search (`find`) | Reimplements the framework's generated `search` command, which already spans local resources. | `changelog` |
| Effective agent toolset (`agent-tools`) | Consulted when building or debugging an agent rather than on a weekly cadence — soft kill on weekly use. | `agent-trace` |
| Unused-resource sweep (`orphans`) | Housekeeping ritual that runs monthly at best. | `drift` |
| Agent session list/resume (`agent-sessions`) | Drifts into an interactive chat client (scope creep); `session_id` passthrough already ships inside the absorbed `agent run --stream`. | `agent-trace` |
| Run comparison (`run-compare`) | Two GETs and a diff, with no local-store leverage. | `rollout` |
