## Customer model

### Maya — Agent tooling engineer
**Today (without this CLI):** She wires Parallel Search MCP and Task MCP into a LangGraph/Hermes agent, keeps a scratch `session_id` in a shell var or Redis, and pastes run IDs between IDE chat and `parallel-cli research status`. She cannot answer “what did this agent already fetch for this objective?” without grepping logs.

**Weekly ritual:** Ship and dogfood a tool-calling loop: objective search → extract top URLs → deep research follow-up with `--previous-interaction-id` → emit `--json` for the next agent turn.

**Frustration:** Cross-step continuity is manual — search, extract, and task runs live in separate tool transcripts with no local stitch, so agents re-fetch and burn credits on work already done.

### Jordan — Research ops / enrichment operator
**Today (without this CLI):** He runs `parallel-cli enrich` / Task Groups from cron or a Makefile, scrapes CSV of FindAll hits into a second enrichment job, and tracks unfinished groups in a spreadsheet of run IDs. He watches prepaid balance in the Parallel dashboard when a batch “feels expensive.”

**Weekly ritual:** Entity discovery (FindAll or entity-search) for a lead list, then batch enrichment Task Groups, poll until done, export results for the CRM sync.

**Frustration:** FindAll candidates and enrichment runs are disconnected — promoting discoveries into a Task Group and knowing which local runs are still open requires copy-paste across CLIs and tabs.

### Priya — Web-change monitor owner
**Today (without this CLI):** She creates monitors in `parallel-cli monitor`, points webhooks at a Slack/HTTP sink, and occasionally polls events when the webhook drops. Missed changes mean scrolling raw event JSON with no local digest of “what moved since last check.”

**Weekly ritual:** Review monitor health Monday morning — which monitors fired, which went quiet, pull events since last review into a triage note.

**Frustration:** No offline digest of monitor events by monitor/query window; stale or silent monitors only show up when a stakeholder asks why nothing alerted.

### Sam — Platform admin (credits & keys)
**Today (without this CLI):** He uses Account API docs / dashboard with device OAuth for balance top-ups, app CRUD, and key create/delete, while engineers use `PARALLEL_API_KEY` for Product calls. He has no local history of balance over the week vs how many research runs the team fired.

**Weekly ritual:** Check prepaid balance, top up if low, rotate a compromised key, confirm apps still map to the right team keys.

**Frustration:** Product spend and Account balance live in different auth worlds — he cannot gate expensive Task runs against a recent local balance snapshot or see burn rate without opening the dashboard.

## Candidates (pre-cut)

### 1. Research session stitch
- **Command:** `session stitch`
- **Description:** Bind recent search, extract, and task/findall run IDs under one local session and print the chain for agent resume.
- **Persona:** Maya
- **Source:** (a) Persona-driven; (e) User Vision / Build Priorities “research-session stitch”
- **Long Description:** Use this command to attach already-completed Product calls into one local session. Do NOT use it to start a new deep research run; use `tasks runs create` instead. Do NOT use it to walk a single task follow-up chain; use `tasks lineage` instead.
- **Kill/keep:** Keep. Mechanical local SQLite graph; no LLM. `// pp:data-source local`. Minor data-model addition (session_members).

### 2. Session continue (last agent loop)
- **Command:** `session continue`
- **Description:** Replay the latest session’s session_id / interaction_id hints as flags for the next Product call.
- **Persona:** Maya
- **Source:** (a) Persona-driven
- **Long Description:** none
- **Kill/keep:** Soft keep for Pass 3. Thin sibling of stitch; risk of wrapper-only UX. Prefer stitch if only one ships.

### 3. Balance-aware run cost guard
- **Command:** `tasks guard`
- **Description:** Before creating a Task/TaskGroup, require a fresh Account balance snapshot (OAuth) and refuse if below `--min-balance` or if projected run count exceeds `--max-runs`.
- **Persona:** Sam; Jordan
- **Source:** (a) Persona-driven; (e) User Vision / Build Priorities “balance-aware run cost guard”
- **Long Description:** Use this command to gate expensive Task creates on prepaid balance. Do NOT use it to inspect historical burn; use `balance burn` instead.
- **Kill/keep:** Keep gated on Account OAuth (auth check; feature useless without JWT — gate, don’t cut). Not a fake API — calls Account balance + reads local run intent. `// pp:data-source auto`.

### 4. Prepaid burn rate from snapshots
- **Command:** `balance burn`
- **Description:** Diff local balance_snapshots over `--since` and correlate with local task_runs/findall_runs counts.
- **Persona:** Sam
- **Source:** (c) Cross-entity local queries
- **Long Description:** Use this command for historical credit burn vs local run volume. Do NOT use it to block a new run; use `tasks guard` instead.
- **Kill/keep:** Keep. Pure local join of balance_snapshots + run tables. `// pp:data-source local`. Framework note: if exposed via analytics-style UX, use `--since 7d` duration form only.

### 5. Stale monitor event digest
- **Command:** `monitors digest`
- **Description:** From local monitor_events, emit per-monitor counts and latest event titles/URLs since `--since` (mechanical top-N, no summarization).
- **Persona:** Priya
- **Source:** (a) Persona-driven; (e) User Vision / Build Priorities “stale-monitor digests”; (c) Cross-entity
- **Long Description:** Use this command for a mechanical local digest of synced monitor events. Do NOT use it to create or list monitors; use `monitors` endpoint commands instead.
- **Kill/keep:** Keep after reframe — forbid LLM “summary”; output counts + top event fields only. `// pp:data-source local`.

### 6. Quiet monitors
- **Command:** `monitors quiet`
- **Description:** List monitors with zero local events in the last N hours.
- **Persona:** Priya
- **Source:** (a) Persona-driven; (c) Cross-entity
- **Long Description:** none
- **Kill/keep:** Soft keep. Subsumable by `monitors digest --quiet-only`; likely sibling-kill in Pass 3.

### 7. Cross-run research recall
- **Command:** `research recall`
- **Description:** FTS join across local searches, extracts, and task run input/output summaries for one query string; return typed hits with source table + IDs.
- **Persona:** Maya; Jordan
- **Source:** (c) Cross-entity local queries; (a) Persona-driven
- **Long Description:** Use this command to find prior local research across searches/extracts/runs. Do NOT use it for live web search; use `search` instead. Do NOT use framework `search --local` when you need a cross-type join with run IDs; use this command.
- **Kill/keep:** Keep. Distinct from framework `search --type` (single resource) — this is multi-table join. `// pp:data-source local`. Verifiable via seeded SQLite fixtures.

### 8. FindAll → Task Group promote
- **Command:** `findall promote`
- **Description:** Read local findall run candidates and create a Task Group enrichment job from selected entity URLs/names (`--limit`, optional stdin IDs).
- **Persona:** Jordan
- **Source:** (b) Service-specific content patterns (FindAll → enrich pipeline); (a) Persona-driven
- **Long Description:** Use this command to turn FindAll candidates into a Task Group. Do NOT use it for one-off deep research on a free-text objective; use `tasks runs create` instead.
- **Kill/keep:** Keep. Live Task Group create + local findall cache read. `// pp:data-source auto`. Not reimplementation — creates real Task Group via Product API.

### 9. Interaction lineage
- **Command:** `tasks lineage`
- **Description:** Walk local previous_interaction_id / run parent links and print the follow-up chain for a run ID.
- **Persona:** Maya
- **Source:** (c) Cross-entity local queries
- **Long Description:** Use this command to show the local follow-up chain for a task run. Do NOT use it to poll live status; use `tasks status` / `tasks result` instead.
- **Kill/keep:** Keep. Local graph only. `// pp:data-source local`. Absorbed #18 is continuity flag behavior; this is the offline graph view.

### 10. Agent provenance receipt
- **Command:** `session receipt`
- **Description:** Emit a machine-readable receipt of all local Product calls in a session (IDs, timestamps, endpoint family) for audit/debug.
- **Persona:** Maya
- **Source:** (a) Persona-driven; (e) User Vision (agent-native JSON / verifyability)
- **Long Description:** Use this command for an audit trail of a stitched session. Do NOT use it to stitch new members; use `session stitch` instead.
- **Kill/keep:** Keep tentatively. Overlaps stitch output; may fail weekly-use / sibling-kill if stitch already prints the chain.

### 11. Open enrichment coverage
- **Command:** `enrich coverage`
- **Description:** Percent complete/failed/pending across local task_groups for `--since`.
- **Persona:** Jordan
- **Source:** (c) Cross-entity local queries
- **Long Description:** none
- **Kill/keep:** Soft kill lean — close to generic `analytics --type task_groups --group-by status`; weak transcendence vs framework analytics.

### 12. Dual-auth context map
- **Command:** `auth which`
- **Description:** Print which auth (API key vs Account JWT) each command family requires and what is currently configured.
- **Persona:** Sam
- **Source:** (e) User Vision (dual-auth clarity)
- **Long Description:** none
- **Kill/keep:** Kill lean — reimplementation/thin wrapper of absorbed `doctor` dual-auth clarity (#10/#11). No new local leverage.

### 13. Orphan run sweeper
- **Command:** `tasks orphans`
- **Description:** List local task/findall runs missing a stored terminal result.
- **Persona:** Jordan
- **Source:** (c) Cross-entity local queries
- **Long Description:** none
- **Kill/keep:** Soft keep. Useful but “depends” weekly-use; likely Pass 3 kill vs lineage/guard.

### 14. Search→extract deepen
- **Command:** `research deepen`
- **Description:** Take URLs from a local search row / session and batch `extract` under the same session_id.
- **Persona:** Maya
- **Source:** (b) Service-specific (search→extract continuity); (a) Persona-driven
- **Long Description:** Use this command to extract URLs from a prior local search into the same session. Do NOT use it to stitch arbitrary run IDs; use `session stitch` instead.
- **Kill/keep:** Keep for Pass 3. Live extract + local search read. `// pp:data-source auto`. Overlaps session stitch + absorbed extract --stdin; sibling pressure high.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Research session stitch | session stitch | 9/10 | hand-code | Joins local searches, extracts, task_runs, and findall_runs into a session_members table and prints the chain with IDs for agent resume (`// pp:data-source local`). | Brief Build Priorities item 5 “research-session stitch”; Top Workflows 1–3 (search→extract→tasks); absorb #2/#18 session continuity gaps for offline stitch | Use this command to attach already-completed Product calls into one local session. Do NOT use it to start a new deep research run; use `tasks runs create` instead. Do NOT use it to walk a single task follow-up chain; use `tasks lineage` instead. |
| 2 | Balance-aware run cost guard | tasks guard | 8/10 | hand-code | GETs Account `/balance` when OAuth JWT present, compares to `--min-balance` / local pending run intent, exits non-zero before Task/TaskGroup create (`// pp:data-source auto`). | Brief Build Priorities “balance-aware run cost guard”; Auth Notes dual-auth split; Top Workflow 6 admin + Workflow 3 expensive tasks | Use this command to gate expensive Task creates on prepaid balance. Do NOT use it to inspect historical burn; use `balance burn` instead. |
| 3 | Stale monitor event digest | monitors digest | 8/10 | hand-code | Aggregates local monitor_events by monitor_id since `--since` into counts plus top-N event title/URL fields with no NLP (`// pp:data-source local`). | Brief Build Priorities “stale-monitor digests”; Top Workflow 5 continuous monitoring; Data Layer monitor_events entity | Use this command for a mechanical local digest of synced monitor events. Do NOT use it to create or list monitors; use generated `monitors` commands instead. |
| 4 | Cross-run research recall | research recall | 8/10 | hand-code | FTS5 multi-table query over cached search excerpts, extract bodies, and task input/output summaries returning typed hits with source + run IDs (`// pp:data-source local`). | Product Thesis “offline SQLite recall of past researches”; Data Layer FTS over titles/URLs/excerpts and task summaries; Maya/Jordan weekly re-fetch pain | Use this command to find prior local research across searches/extracts/runs. Do NOT use it for live web search; use `search` instead. |
| 5 | FindAll → Task Group promote | findall promote | 7/10 | hand-code | Reads local findall_runs candidates then POSTs Task Group create on Product API with selected entities (`// pp:data-source auto`). | Top Workflows 3–4 (enrichment + entity discovery); table-stakes parallel-cli findall/enrich gap as one pipeline; Jordan ritual | Use this command to turn FindAll candidates into a Task Group. Do NOT use it for one-off deep research on a free-text objective; use `tasks runs create` instead. |
| 6 | Prepaid burn vs runs | balance burn | 7/10 | hand-code | Diffs balance_snapshots rows over `--since` and joins counts from local task_runs/findall_runs (`// pp:data-source local`). | Absorb #12 local balance snapshots; Account API balance get/add; Sam weekly credit ritual | Use this command for historical credit burn vs local run volume. Do NOT use it to block a new run; use `tasks guard` instead. |
| 7 | Task interaction lineage | tasks lineage | 6/10 | hand-code | Walks local previous_interaction_id / parent run links stored from Task creates and prints the chain for one run ID (`// pp:data-source local`). | Absorb #18 follow-up interaction_id behavior without offline graph; Top Workflow 3 deep research follow-ups; Maya frustration | Use this command to show the local follow-up chain for a task run. Do NOT use it to poll live status; use `tasks status` / `tasks result` instead. |

**Pass 3 force-answers (survivors):**

1. **session stitch** — Weekly use: yes (Maya every agent loop). Wrapper? no (local multi-entity graph). Transcendence: SQLite stitch. Sibling killed: `session continue`, `research deepen`, `session receipt`. Buildability: hand-code. Long Description siblings survive (`tasks runs create`, `tasks lineage`).
2. **tasks guard** — Weekly use: yes (Jordan batch days / Sam before top-up week). Wrapper? no (Account balance + gate policy). Transcendence: cross-auth local+live. Sibling killed: `auth which`. Buildability: hand-code.
3. **monitors digest** — Weekly use: yes (Priya Monday triage). Wrapper? no (local aggregation). Transcendence: local events. Sibling killed: `monitors quiet`. Buildability: hand-code.
4. **research recall** — Weekly use: yes (Maya/Jordan before re-running paid search). Wrapper? no (cross-table FTS vs single-type framework search). Transcendence: SQLite join. Sibling killed: `enrich coverage`. Buildability: hand-code.
5. **findall promote** — Weekly use: yes (Jordan enrichment batches). Wrapper? no (compose FindAll cache → Task Group API). Transcendence: service-specific pipeline. Sibling killed: `tasks orphans`. Buildability: hand-code.
6. **balance burn** — Weekly use: yes (Sam credit check). Wrapper? no (snapshot diff + run counts). Transcendence: cross-entity local. Sibling killed: none; redirects from guard. Buildability: hand-code.
7. **tasks lineage** — Weekly use: yes (Maya follow-up research weeks). Wrapper? no (offline graph). Transcendence: local interaction graph. Sibling killed: `session receipt` (audit dump only). Buildability: hand-code.

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| Session continue (`session continue`) | Thin flag-echo of last session; weekly value collapses into stitch output | session stitch |
| Quiet monitors (`monitors quiet`) | Subsumed by digest with a quiet filter; separate command fails wrapper/leverage bar | monitors digest |
| Agent provenance receipt (`session receipt`) | Audit dump only; stitch already prints the member chain agents need weekly | session stitch |
| Open enrichment coverage (`enrich coverage`) | Generic status rollup better served by framework `analytics --type task_groups`; weak transcendence | findall promote |
| Dual-auth context map (`auth which`) | Reimplements absorbed doctor dual-auth clarity; no new local/API leverage | tasks guard |
| Orphan run sweeper (`tasks orphans`) | Soft weekly use (“depends”); lineage + guard cover the actionable cases | tasks lineage |
| Search→extract deepen (`research deepen`) | Overlaps absorbed extract batch/stdin + session stitch; second pipeline command without unique weekly ritual | session stitch |
