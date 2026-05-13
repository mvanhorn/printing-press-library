# Novel Features Brainstorm — monday-pp-cli

## Customer model

**Persona 1 — Priya, the ops/PM lead running 3 product squads on Monday**

*Today (without this CLI):* Priya keeps the Monday web app pinned in two browser tabs (one for the sprint board, one for the bugs board) plus a Google Sheet she pastes status snapshots into every Friday. To answer "what changed since Monday standup?" she scrolls each item's activity log by hand, because the universal Activity view only shows the most recent ~50 events across boards. To answer "who's overloaded?" she filters each board by Person column, eyeballs the dot count, and types it into the sheet. She has a curl script saved in `~/scripts/monday-dump.sh` that hits the GraphQL endpoint with a hand-tuned `items_page` query and pipes through `jq`, and it breaks every 6–10 weeks when she edits a board's column types and the JSON shape changes under her.

*Weekly ritual:* Monday morning sprint review, Wednesday mid-sprint check, Friday status snapshot to leadership. Each one is the same shape: pull the current state of 4–6 boards, diff against last cycle, surface what slipped, paste into a doc.

*Frustration:* The polymorphic `column_values` JSON. Every column type — status, person, dropdown, mirror, formula, timeline — has a different shape, so her `jq` pipelines look like 40-line case statements, and a single column-type swap by another admin breaks the whole thing silently.

**Persona 2 — Devraj, the engineering manager running a Monday-dev sprint board**

*Today (without this CLI):* Devraj uses Monday's built-in sprint widgets for the daily, but for retros and "did we actually finish what we committed to" he exports the board to CSV every other Friday, drops it into a local SQLite via DB Browser, and writes the same three SQL queries he wrote two weeks ago. The CSV export drops mirror columns, and the activity log isn't in the export at all, so he can't see "this item was committed Monday and moved to Done Thursday — was it cherry-picked in?" without clicking each item.

*Weekly ritual:* Sprint commit on Monday, mid-sprint stale check on Wednesday, retro on the Friday the sprint closes. He cares about velocity-by-person, slip rate, and which items got added mid-sprint.

*Frustration:* The activity log is per-item only via the API, and the items_page cursor + complexity-points combination means a naive "give me every change in this sprint" query trips the per-call 5M cap.

**Persona 3 — Sam, the RevOps integrator gluing Monday to Salesforce/Slack/Stripe**

*Today (without this CLI):* Sam owns four Make.com scenarios and a Python script on cron that reads from Monday and pushes to other systems. They write the GraphQL queries by hand, regenerate them against `__schema` whenever someone adds a new column, and constantly fight the complexity-point limit on bulk reads. When sales asks "find every deal-board item that mentions Acme Corp anywhere — text columns, updates, mirrors", Sam has to open the universal search in the web UI because their Python script only searches one board.

*Weekly ritual:* Mid-week reconcile (does Monday-side state match Salesforce?), bulk status-update push from a Salesforce export CSV, ad-hoc cross-board lookups when AEs ask "where is this customer mentioned?".

*Frustration:* No transactional bulk-edit. Every CSV row is a separate mutation. They've built `--dry-run` themselves three times in three different scripts. And complexity-points are unobservable until the call fails — there's no "tell me how expensive this query will be before I run it" affordance.

## Candidates (pre-cut)

(see absorb manifest below for survivors and rationale)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Persona |
|---|---------|---------|-------|-------------|---------|
| 1 | Cross-board activity since window | `monday since <window>` | 9/10 | Joins local `activity_logs` table with API tail-fetch since cached high-water mark; filters by user/column/board in SQLite | Priya, Devraj |
| 2 | Per-person workload across boards | `monday whoami-load` | 8/10 | Joins items + person column_values + boards + status in local SQLite; weights open items by status and overdue | Priya |
| 3 | Status-bottleneck dwell-time analyzer | `monday bottleneck --board <id>` | 8/10 | Reads activity_logs that record column-value changes; computes per-status median + p90 dwell time | Devraj, Priya |
| 4 | Sprint velocity & slip report | `monday velocity --board <id>` | 8/10 | Joins sprint metadata + activity log + items; counts committed/done/slipped/added-mid-sprint per cycle | Devraj |
| 5 | Column-schema drift detector | `monday column-drift` | 8/10 | Diffs cached boards.columns JSON vs fresh get_board_info response; reports added/removed/renamed/retyped | Sam, Priya |
| 6 | Cross-board contextual mentions | `monday mentions "<text>"` | 7/10 | FTS5 over indexed item names, update bodies, doc bodies, and text-typed column values; results hydrated with board+group+owner context | Sam |
| 7 | Pre-flight complexity budget probe | `monday complexity-budget` | 7/10 | Wraps a query with `complexity { before after reset_in_x_seconds }`; returns predicted points + remaining account budget | Sam |
| 8 | Bulk column-value editor with typed dry-run | `monday bulk-edit --from items.csv` | 7/10 | Reads CSV; validates each cell against cached typed-column schema; prints unified diff in dry-run; resume on apply | Sam, Priya |
| 9 | Mirror & formula dependency resolver | `monday resolve --item <id>` | 6/10 | Walks local column_values rows of type mirror/formula; follows board_relation linkage; lists formula column refs | Priya, Sam |
| 10 | External-CSV reconcile | `monday reconcile --against external.csv` | 6/10 | Loads CSV; joins to local items by column-value match; emits only-in-monday/only-in-csv/diff sets | Sam |
| 11 | Per-board health scorecard | `monday boards health <id>` | 6/10 | Aggregates local store: % owner-set, % due-set, % overdue, % updated-7d, count empty-status, count broken-mirror | Priya |
| 12 | Item full-context dump | `monday context <item-id>` | 6/10 | Single SQLite join over items + column_values + updates + replies + docs + assets + activity_logs + mirror sources for one item-id | Devraj, Sam |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Stale items detector | Already in standard PP transcendence | #3 bottleneck |
| Saved query library | Wrapper over absorbed `monday gql` | #60 gql (absorbed) |
| Notetaker action-item extractor | Requires LLM classification of free transcript | #12 context |
| What's-on-my-plate digest | Sibling-kills with #2 whoami-load | #2 whoami-load |
