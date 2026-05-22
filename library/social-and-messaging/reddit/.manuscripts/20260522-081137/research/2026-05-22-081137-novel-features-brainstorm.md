# Novel Features Brainstorm — reddit-pp-cli

Run ID: 20260522-081137
First print: yes
Subagent run: single Task invocation (general-purpose), 3 passes (customer → candidates → cut).

## Customer model

**Mara — modqueue triage moderator, r/programming-sized sub (~500K subs, 4-mod team)**

*Today:* Opens reddit.com every morning before standup, clicks Mod Queue, scrolls through reports tab in the web UI. Personal spreadsheet of banned usernames so she doesn't get confused with repeat offenders. Uses Toolbox extension for shortcuts but Toolbox can't tell her "which of these reporters has the highest false-report rate." Can't answer "what's been sitting in the queue for >24h" without manually clicking each item to check timestamp.

*Weekly ritual:* Monday morning queue clear (40-60 items), nightly 10-minute swipe-through, removalreason-template-driven removals with a customized note, ban escalation for repeat offenders. Sunday review: opens modlog and tries to remember if /u/JaneDoe has been distinguished-warned before, but the modlog UI paginates 25-at-a-time and search-within-modlog doesn't work.

*Frustration:* The modqueue UI has no concept of age. Reports from 30 hours ago look identical to reports from 30 seconds ago. No way to see "of my last 200 removals, how many came from the same 5 reporters."

---

**Devi — content creator / submitter, posts blog articles + dev tweets to 6 subs**

*Today:* Drafts in Notion, opens the new-post page on Reddit per sub, pastes title, sets flair (clicks through a dropdown), unchecks "send replies to inbox," marks OC if it's her own work, clicks submit. Repeats 6 times. Sometimes forgets a sub-specific rule and gets removed. Tracks engagement by manually visiting each post's URL the next day.

*Weekly ritual:* Tuesday/Thursday publish cycle: one technical post → 4-6 subs. Friday review: which subs gave good engagement, which removed her post.

*Frustration:* No way to crosspost-with-different-titles in one command. No way to see "of my last 20 posts to r/programming, what's the median score and what hour was the top one published."

---

**Riko — researcher / archivist, tracks 12 subreddits for academic project**

*Today:* Pushshift died in 2023, so they fell back to manually saving HTML pages and running praw scripts. Wants to ask "every comment by /u/X across the 12 subs" and "every submission tagged with flair 'Research' across r/AskScience and r/science in the last month." Reddit's native search misses ~50% of expected hits.

*Weekly ritual:* Sunday incremental fetch of new submissions + comments per tracked sub. Monday queries the archive. Scripts break every few months when Reddit changes a response shape.

*Frustration:* Reddit's own search returns ~5 results for queries that should match 50. There is no first-class "incremental sync this subreddit into local FTS" tool. Pushshift was the workaround and it's gone.

---

**Syauqi — agency operator (Creativism), monitors brand + competitor mentions**

*Today:* Make.com scenarios polling Reddit search for "creativism" + competitor names hourly, dumping hits into Slack. Scenarios miss context (no thread, no parent comment), false-positive a lot (matches inside other words), don't dedupe.

*Weekly ritual:* Monday brand-mention digest from Make.com. Decide which threads to reply to vs ignore.

*Frustration:* Make.com's polling is wasteful (no cursor state), expensive (counts toward ops), and noisy (no co-occurrence with comment context). The "reply with brand voice from terminal without opening web app" loop doesn't exist.

---

## Candidates (pre-cut)

| # | Name | Command | One-liner | Persona | Source | Verdict |
|---|------|---------|-----------|---------|--------|---------|
| C1 | Modqueue age & stale ranking | `mod queue <sub> --sort age --older-than 24h` | Sort modqueue by how long items have been sitting. | Mara | b, e | KEEP |
| C2 | Reporter reputation ledger | `mod reporters <sub> --window 30d --min-reports 3` | Per-reporter (filed, removed%, approved%, no-action%) over window. | Mara | b, c, e | KEEP |
| C3 | Ghost mod-action diff | `mod ghost-actions <sub> --since 7d` | Detect approve→remove chains by different mods. | Mara | c, f | KEEP |
| C4 | Cross-sub user dossier | `user dossier <username> --in sub1,sub2,...` | One user's activity + karma per-sub across N subs. | Mara, Riko | b, e | KEEP |
| C5 | Local FTS5 over me-history | `me search "<q>" --scope saved,submitted,upvoted,comments` | FTS5 search after sync over Reddit's broken native search. | Riko, Devi | b, e | KEEP |
| C6 | Sub-scoped local FTS5 | `search-local "<q>" --sub <name>` | FTS5 over synced sub corpus. | Riko, Syauqi | b, e | KEEP |
| C7 | Multi-sub brand watch with context | `watch "<term>" --in sub1,sub2 --since 24h` | Fan-out search + dedup + context enrichment. | Syauqi | a, c | KEEP |
| C8 | Best-time-to-post analyzer | `me posting-stats --sub <name>` | Per-sub median score by (dow × hour) from own history. | Devi | b, e | KEEP |
| C9 | Crosspost batch with overrides | `crosspost-batch <id> --plan plan.yaml` | YAML-driven multi-sub crosspost with per-sub title/flair. | Devi | a, b | KEEP |
| C10 | MoreComments full-tree expander | `post comments <id> --expand-all` | Recursive MoreComments resolution. | Riko | b, f | KILL (already absorbed) |
| C11 | Comment-velocity radar | `post velocity <id>` | Comments/minute over first 60 min with sub-median percentile. | Devi, Syauqi | b, c | KEEP (gated) |
| C12 | Karma-by-sub trend | `me karma-trend --window 90d` | Weekly karma per sub. | Devi | b, e | KILL (monthly, redundant) |
| C13 | Removal-batch with plan | `mod remove-batch --plan plan.csv` | CSV-driven mass removal with templates. | Mara | a, b | KEEP |
| C14 | Mod-team split detector | `mod splits <sub>` | Items where mods disagreed. | Mara | c, f | KILL (merge into C3) |
| C15 | OPSEC self-scan | `me opsec --patterns city,employer` | Regex own history for sensitive patterns. | Riko subjects | b | KILL (niche) |
| C16 | Inbox digest by sub + score | `inbox digest --window 24h` | Group inbox by source-sub, enrich with thread score. | Mara | b | KEEP |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Why Only We Can Do This | Persona |
|---|---------|---------|-------|------------------------|---------|
| 1 | Stale modqueue ranker | `mod queue <sub> --sort age [--older-than 24h]` | 9/10 | Local timestamp math; modqueue endpoint has no age sort | Mara |
| 2 | Reporter reputation ledger | `mod reporters <sub> [--window 30d] [--min-reports 3]` | 9/10 | Joins synced modlog + reports payload across rolling window; no API endpoint returns this | Mara |
| 3 | Ghost mod-action / split detector | `mod ghost-actions <sub> --since 7d` | 7/10 | Local temporal join on `(target_id, action, mod)` chains in synced modlog | Mara |
| 4 | Cross-sub user dossier | `user dossier <username> [--in sub1,...]` | 9/10 | Aggregates 4 endpoints + per-sub karma calc, filterable by sub | Mara, Riko |
| 5 | Personal FTS5 search | `me search "<q>" [--scope saved,submitted,upvoted,comments]` | 10/10 | SQLite FTS5 over synced own-history; Reddit's native search misses ~50% | Devi, Riko |
| 6 | Sub-scoped local FTS5 search | `search-local "<q>" --sub <name> [--type submissions,comments]` | 9/10 | Same FTS5 engine over synced sub corpus; replaces broken native search | Riko, Syauqi |
| 7 | Multi-sub brand-mention watch | `watch "<term>" --in sub1,sub2,... [--since 24h] [--enrich-karma]` | 8/10 | Fan-out + dedupe + context enrichment (parent comment + OP karma) | Syauqi |
| 8 | Best-time-to-post analyzer | `me posting-stats [--sub <name>] [--by hour\|dow]` | 7/10 | Local aggregation over synced own-submissions by (sub × dow × hour) → median + p75 | Devi |
| 9 | Plan-driven crosspost batch | `crosspost-batch <id> --plan plan.yaml [--dry-run]` | 8/10 | YAML with per-sub title/flair overrides; idempotent via plan-row hash | Devi |
| 10 | Modqueue remove-batch with templates | `mod remove-batch <sub> --plan plan.csv [--dry-run]` | 8/10 | CSV-driven mass removal + removal-reason template + ban orchestration | Mara |
| 11 | Comment-velocity radar | `post velocity <id> [--baseline-sample 50]` | 6/10 | Comments/minute computation + sub-median percentile comparison | Devi, Syauqi |
| 12 | Inbox digest by source-sub + score | `inbox digest [--window 24h]` | 7/10 | Group inbox items by source-sub, enrich with current thread score via /api/info | Mara |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| MoreComments expander (C10) | Already absorbed into `post comments <id> --expand-all` | absorb table row 5 |
| Karma-by-sub trend (C12) | Monthly use only; redundant with #8 + #4 | `me posting-stats` (#8) |
| Mod-team split detector (C14) | Same data join as ghost-actions; folded as a flag | `mod ghost-actions` (#3) |
| OPSEC self-scan (C15) | Niche, no weekly use, no service-specific transcendence | `me search` (#5) |
