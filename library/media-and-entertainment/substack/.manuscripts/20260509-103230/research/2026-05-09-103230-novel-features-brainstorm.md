# Novel Features Brainstorm — Substack CLI (Run 20260509-103230)

> Audit trail of Phase 1.5c.5 subagent output. Personas + full pre-cut candidate list + survivors + killed candidates with reasons. Persisted for retro/dogfood debugging.

## Customer model

**Persona 1 — Maya, the solo niche newsletter owner.** Writes a productivity-for-creatives newsletter (~2,400 subs, ~110 paid). Posts 3 Notes/day, batch-writes long-form on Sundays, ships Tuesday/Friday. Lives in the Substack web app on her phone between meetings; checks the heatmap every Friday afternoon to decide next week's posting times.

- **Today (without this CLI):** Five Substack tabs open — drafts, Notes composer, her own profile, the niche feed, and the activity inbox. She drafts Notes in Apple Notes, copies them in one at a time, and eyeballs the spacing. To know "is my Tuesday 9am note actually working?" she screenshots the post-stats page, dumps numbers into a Google Sheet, and tries to remember which Notes preceded which paid conversions. She cannot answer: which of the 21 Notes she posted last week drove paid subs, which writers she engaged with reciprocated, or whether her recommendation list is actually returning subs.
- **Weekly ritual:** Sunday batch-writes 15-20 Notes; Monday-Friday fires 3-5/day on a 09:00/13:00/18:00 cadence; comments on 5-10 niche-writer Notes daily; Friday reviews heatmap + top-performing Notes + activity inbox; weekly emails one swap partner.
- **Frustration:** No way to connect "the Notes I posted" to "the paid subs that arrived" — every conversion-attribution decision is vibes.

**Persona 2 — Devon, the ghostwriter running 4 client publications.** Switches between four browser profiles to keep cookies separate. Each client has a different voice, a different posting cadence, a different niche, a different list of swap partners. Bills hourly; context-switching is the loss-leader.

- **Today (without this CLI):** Four Chrome profiles, four Notion docs of "client voice samples", four spreadsheets of "client swap partners". Drafts Notes in client-specific voice docs, copies them across, fires from the wrong profile twice a month and has to delete-and-repost. Cannot answer in <30s: "across all clients, who's overdue on their daily 3?"
- **Weekly ritual:** Per-client Monday review (last week's stats), Tuesday-Thursday batch-fire across all four, Friday cross-promo outreach drafted per client.
- **Frustration:** Per-client context switch eats 20 min × 4 clients × 5 days a week. There is no command that says "show me what Maya's Tuesday looks like" without logging out and back in.

**Persona 3 — Priya, the cross-promo network organizer.** Runs a 12-person mutual-aid pod that swaps recommendations and restacks-with-comment. She tracks who-restacked-whom in a spreadsheet she updates Sunday nights.

- **Today (without this CLI):** Spreadsheet of pod members; manually scrolls each member's profile feed weekly to count "did they restack me, did I restack them, last time we swapped." Decides who to drop and who to add. Cannot answer: "is the pod net-positive for me — am I getting back what I'm putting in?"
- **Weekly ritual:** Sunday reconciliation; Monday morning sends "you're up this week" nudge; Wednesday rebalances if a partner is silent.
- **Frustration:** Reciprocity is invisible until she manually counts it. By the time she notices a free-rider, she's restacked them 8 times for 1 in return.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Inline verdict |
|---|------|---------|-------------|---------|--------|----------------|
| C1 | Note→Sub attribution ledger | `analytics attribution [--days 30]` | Joins local `notes.posted_at` × `analytics_snapshots` deltas × per-day reach windows; ranks Notes by "subs acquired within 24h posted-window" | Maya | (a) frustration, (c) cross-entity join | KEEP |
| C2 | Reciprocity ledger | `engage reciprocity [--handle <h>] [--days 30]` | Joins `engagements where by_self=true` vs incoming engagements per target_handle; emits net-give/net-take with drift alerts | Priya, Maya | (a), (b), (c) | KEEP |
| C3 | Cadence guard | `notes schedule --guard` (gates schedule + new) | Reads pending queue + posted_at; blocks/warns when next Note is <30 min from previous own-Note or violates time-of-day rotation | Maya | (b) Noting rituals | KEEP |
| C4 | Best-time recommender | `analytics best-time [--days 90] [--for-goal=subs\|likes\|restacks]` | Aggregates own Notes × engagement events × time-bins into `reach_windows`; returns top 3 day×hour cells | Maya, Devon | (b), (c) | KEEP |
| C5 | Pod overdue-board | `client overdue [--threshold-hours 26]` | Across clients (or pod members), shows last own-Note timestamp; flags accounts past cadence | Devon, Priya | (a), (e) | KILL — wrapper-shaped (single SELECT MAX) |
| C6 | Hook-pattern miner | `discover patterns --niche <slug> [--sort restacks]` | Mechanical extraction over `inspiration_notes`: colon-hook, sentence-count, starts-with-question, em-dash-reframe ratios | Maya, Devon | (b) format patterns, (c) FTS | KEEP |
| C7 | Swap-partner finder | `recs find-partners --my-pub <slug> [--top 20]` | Joins `recommendations` × my `subscriptions`/followees × `profiles`; scores candidates by mutual-overlap density | Priya, Maya | (b), (c) | KEEP |
| C8 | Restack-with-comment template firer | `engage restack-with-comment <url> --pattern=...` | Fills templates, drafts; --send fires endpoint | Maya, Devon | (b) | KILL — covered by absorb #40 |
| C9 | Pod restack scoreboard | `analytics pod --members <h>...` | Given handles, joins last-30d engagements into member×member matrix | Priya | (a), (c) | KEEP |
| C10 | Streak + queue health check | `analytics today` | Composes streak + queue + cadence-guard + inbox unread | Maya, Devon | (b), (a) | KILL — composes other commands without unique join |
| C11 | LinkedIn / X cross-post bridge | `crosspost linkedin\|x <note-id>` | Translate ProseMirror to platform variant | Maya | (e) | KILL — auth gap |
| C12 | A/B headline picker | `posts ab-test <id> --variants <file>` | Variant send + measure | Maya | (e) | KILL — scope creep + auth gap |
| C13 | DM/inbox tooling | `dms list` / `dms reply` | Substack Chat | Maya | (e) | KILL — endpoints unmapped |
| C14 | Behavioral re-engagement | `subs reengage --inactive 30d` | Per-segment win-back drafting | Maya | (e) | KILL — auth gap |
| C15 | Voice-fingerprint | `voice fingerprint [--handle <h>]` | Mechanical fingerprint: sentence length, em-dash rate, colon-hook rate, vocabulary uniqueness | Devon, Maya | (b), (c) | KEEP |
| C16 | Inbox auto-triage | `inbox triage --auto-like-replies --auto-archive-thanks` | Rule-based triage on cached inbox | Maya, Devon | (b) | KILL — covered by absorb #65/66 |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence | Persona |
|---|---------|---------|-------|--------------|----------|---------|
| 1 | Note→Sub attribution ledger | `analytics attribution [--days 30]` | 9/10 | Joins local `notes` × `analytics_snapshots` daily deltas × `engagements` in SQLite; computes "subs acquired within 24h window after this Note" rank. Pure local-store query, no API call at run time. | Brief Top Workflow #6; Maya frustration explicit; not in WriteStack | Maya |
| 2 | Reciprocity ledger | `engage reciprocity [--handle <h>] [--days 30]` | 9/10 | Joins `engagements` rows where `by_self=true` against incoming engagements grouped by `target_handle`; emits net-give/net-take per writer. Pure SQLite join over data populated by `inbox sync` + `notes engage`. | Priya frustration explicit; brief Priority 2 seed; gap in every wrapper | Priya, Maya |
| 3 | Cadence guard | `notes schedule --guard` / pre-flight gate | 8/10 | Reads `notes_queue` + `notes` (own posts) timestamps, rejects schedules with <30 min spacing or violating user-set time-of-day rotation; typed exit code 2 on violation, JSON diagnosis. | Brief Top Workflow #2 (≥30 min spacing); Substack noting ritual; WriteStack has no equivalent guard | Maya, Devon |
| 4 | Best-time recommender | `analytics best-time [--days 90] [--for-goal=subs\|likes\|restacks]` | 8/10 | Aggregates own Notes × engagement events into `reach_windows`; ranks top cells per chosen goal. Brief schemas this table. | Brief Priority 2; schemas `reach_windows`; WriteStack absorbed at heatmap level — we add per-goal optimization | Maya, Devon |
| 5 | Hook-pattern miner | `discover patterns --niche <slug> [--sort restacks]` | 7/10 | Pulls `inspiration_notes`; mechanically extracts: colon-hook presence, sentence-count, starts-with-question, em-dash-reframe; aggregates ratios. Pure regex/tokenize on cached FTS rows. | Brief Priority 2 names format-pattern extraction; WriteStack does AI-gen but not pattern aggregation | Maya, Devon |
| 6 | Swap-partner finder | `recs find-partners --my-pub <slug> [--top 20]` | 7/10 | Joins `recommendations` (from-pub I follow) × my `subscriptions`/followee set × `profiles`; scores candidate pubs by overlap density with my niche. | Brief Top Workflow #5; brief Priority 2 names "niche-overlap-aware follow-list curator"; gap in every wrapper | Priya, Maya |
| 7 | Pod restack scoreboard | `analytics pod --members <h>...` | 7/10 | Given handles, joins `engagements` last-30d into a member×member matrix; emits markdown table + JSON. Replaces Priya's spreadsheet. | Priya frustration explicit; brief Top Workflow #5 (cross-promo); not in WriteStack | Priya |
| 8 | Voice fingerprint | `voice fingerprint [--handle <h>]` | 6/10 | Mechanical extraction over a handle's cached Notes/posts: avg sentence length, em-dash rate, colon-hook rate, hook-line ratios, vocabulary uniqueness vs corpus baseline. JSON fingerprint; `--diff <other-handle>` shows delta. | Brief Priority 2 names "voice-corpus builder"; WriteStack does voice-matched generation but exposes no measurable fingerprint; Devon's per-client voice frustration | Devon, Maya |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C5 Pod overdue-board | Thin wrapper around `SELECT MAX(posted_at) GROUP BY client`; collapses into per-client view | (folded into C10's morning dashboard, then C10 itself dropped) |
| C8 Restack-with-comment template firer | Thin client-call wrapper over absorbed restack endpoint with a string-template helper; fits inside absorbed feature #40 | absorbed #40 |
| C10 Streak + queue health check | Composes existing absorbed commands without a cross-source join or service-specific pattern; "morning dashboard" scope creep without unique data | Survivor #3 (cadence guard) |
| C16 Inbox auto-triage classifier | "Auto-like on reply" already absorbed (#65); rule-based archive piece is a thin filter | absorbed #65/#66 |
| C11 LinkedIn / X cross-post bridge | Auth gap — user has no confirmed LinkedIn/X credentials; would force endpoint stubs | none — defer to follow-up run |
| C12 A/B headline picker | Scope creep + auth gap — community surface has no per-segment send endpoint | none |
| C13 DM/inbox tooling | Brief explicitly defers Substack Chat ("unmapped, defer or browser-sniff in retry") | none — flagged for retry |
| C14 Behavioral re-engagement | Auth gap — needs per-subscriber engagement which is gated to Publisher API user doesn't have | none |
