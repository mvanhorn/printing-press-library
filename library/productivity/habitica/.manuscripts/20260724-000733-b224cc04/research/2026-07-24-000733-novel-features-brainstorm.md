## Customer model

### Maya — ritual-driven solo player

Maya opens Habitica, scans dailies and due to-dos, and separately checks character state. Her daily ritual needs one agent-safe answer to what is actionable this morning; task lists and game state are currently separate.

### Theo — fitness-focused player

Theo scores a workout habit or daily immediately after exercise. Fast scoring matters, but that is already absorbed task-score behavior rather than a differentiated analysis workflow.

### Rina — budget-conscious reward planner

Before a weekend treat, Rina compares current gold, custom rewards, and purchasable items in separate screens. She needs to preserve a chosen gold reserve without doing that arithmetic by hand.

### Devon — power user managing chores

Devon manages long task lists with tags and checklists, then regularly rebalances workload and checks whether overdue work is growing. CRUD and search do not provide tag-level workload or snapshot-based weekly health.

## Candidates (pre-cut)

| Feature | Command | Description | Persona served | Source | Long Description | Verdict |
|---|---|---|---|---|---|---|
| Daily quest briefing | `today` | Ordered queue of due, overdue, and relevant daily tasks with character state. | Maya | persona + daily/task loop | none | keep |
| Structured morning quest plan | `plan chores --file chores.yaml --dry-run` | Validate a chore plan, preview exact creates, and require `--apply`. | Maya | persona + task/checklist/tag workflow | none | keep |
| Reward runway | `reward afford <reward-or-item> --reserve-gp 20` | Compare real gold and reward costs while preserving a reserve. | Rina | persona + reward economy + cross-entity join | none | keep |
| Tag workload report | `tag load` | Group active, due-today, overdue, and incomplete-checklist work by tag. | Devon | persona + local join | Use this command to compare workload across tags. Do NOT use it for today’s ordered action queue; use `today` instead. | keep |
| Weekly task health | `week review` | Compare synced snapshots for growing overdue work, stalled tasks, and completions. | Devon | persona + local snapshot join | Use this command for seven-day task changes from synced snapshots. Do NOT use it for today’s actionable queue; use `today` instead. | keep |
| Checklist blocker list | `checklist blockers` | Find incomplete checklist items blocking due work. | Devon | persona + local join | none | cut: contained by `today` and `tag load` |
| Purchase checkout preflight | `buy preview <key>` | Preview a purchase. | Rina | persona + purchase flow | none | cut: explicit preview is baseline mutation behavior; `reward afford` is higher leverage |
| Workout check-off | `workout done <task>` | Resolve a workout and score it. | Theo | persona | none | cut: thin wrapper over absorbed task scoring |
| Score impact preview | `task score-preview <task>` | Preview a score mutation. | Theo | mutation safety | none | cut: preview belongs in every mutation |
| Stable next-action helper | `stable next` | Recommend pet/mount actions. | Maya | gameplay loop | none | cut: would invent gameplay policy and duplicate absorbed operations |
| Exact duplicate chore scan | `task duplicates` | Find duplicate active task titles. | Devon | local join | none | cut: weak recurring pain |
| Orphan-tag cleanup | `tag orphaned` | Find unused tags. | Devon | local join | none | cut: low-pain tag CRUD derivative |
| Priority drift report | `task drift` | Find high-priority tasks that remain incomplete. | Devon | snapshot join | none | cut: narrower `week review` slice |

## Survivors and kills

### Survivors

| Feature | Command | Score | Persona Served | Buildability | Buildability Proof | Why Only We Can Do This | Long Description |
|---|---|---:|---|---|---|---|---|
| Daily quest briefing | `today` | 9/10 | Maya | hand-code | Reads the real tasks and user profile/stats endpoints, optionally enriches from synced task/tag data, and emits a read-only queue. | It joins due dates, daily state, to-dos, tags, and character state into one recurring action ritual rather than mirroring an endpoint. | none |
| Structured morning quest plan | `plan chores --file chores.yaml --dry-run` | 9/10 | Maya | hand-code | Parses a user-authored local file, previews all planned task-create requests, and calls the real task-create endpoint only with `--apply` plus explicit confirmation. | It turns a real-world chore batch into controlled, reviewable Habitica task creation with no NLP or hidden mutation. | none |
| Reward runway | `reward afford <reward-or-item> --reserve-gp 20` | 10/10 | Rina | hand-code | Reads the real user profile/stats plus task-reward, in-app-reward, and buy-list data to calculate affordability. | It joins balance and account-specific reward prices into a safety decision that neither task CRUD nor purchase endpoints provide. | none |
| Tag workload report | `tag load` | 9/10 | Devon | hand-code | Queries synced task/tag SQLite resources to calculate active, due-today, overdue, and incomplete-checklist counts per tag. | It provides a cross-entity workload view unavailable from a single task or tag endpoint. | Use this command to compare workload across tags. Do NOT use it for today’s ordered action queue; use `today` instead. |
| Weekly task health | `week review` | 6/10 | Devon | hand-code | Compares timestamped synced task snapshots in SQLite. | It derives auditable seven-day trends from actual local snapshots rather than inventing completion history. | Use this command for seven-day task changes from synced snapshots. Do NOT use it for today’s actionable queue; use `today` instead. |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| Checklist blocker list | Narrower signal already represented by `today` and `tag load`. | `tag load` |
| Purchase checkout preflight | Preview is mandatory mutation behavior; reward affordability is the substantive workflow. | `reward afford` |
| Workout check-off | Direct scoring wrapper already absorbed. | `today` |
| Score impact preview | Preview/confirmation is global mutation safety, not a novel command. | `plan chores` |
| Stable next-action helper | Duplicates absorbed gameplay calls and lacks an evidence-backed policy. | `reward afford` |
| Exact duplicate chore scan | Lower recurring value than workload review. | `tag load` |
| Orphan-tag cleanup | Low-pain derivative of tag CRUD. | `tag load` |
| Priority drift report | A narrower report than `week review`. | `week review` |
