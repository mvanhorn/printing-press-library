# Instagram CLI — Novel Features Brainstorm (audit trail)

> Command renames applied in manifest: the subagent expressed survivors as
> `analytics --type X`; promoted to clean top-level commands to avoid the
> framework `analytics --type <resource>` collision:
> C1→`compare`, C2→`growth`, C3→`best-time`, C4→`top-posts`,
> C5→`formats`, C7→`rivals`, C6→`hashtag-perf`.

## Customer model

**Maya — Multi-brand social manager (primary).** Runs several owned Business/Creator accounts.
- Today: logs into Meta Business Suite per account, screenshots reach/ER into a weekly Google Sheet by hand; no cross-account view exists.
- Weekly ritual: Monday — pull last-7-day reach + interactions for every brand, decide where to put effort.
- Frustration: Meta shows one account at a time, no history beyond a rolling window, no side-by-side ranking. Rebuilds the same spreadsheet weekly.

**Sara — Brand owner / operator (single brand).** Owns one brand, checks weekly, not a data person.
- Today: opens the app, sees "reach down 12%" with no context for noise vs trend.
- Weekly ritual: Friday "are we growing?" — wants follower growth WoW + best/worst post in one glance.
- Frustration: no follower-growth history (Graph dropped `follower_count` time-series); can't tell a bad week from a bad month.

**Devin — Agency analyst / competitive watcher.** Benchmarks the portfolio vs rivals, chases reach.
- Today: manually checks ~5 competitor public profiles, copies follower counts into a deck, guesses posting times.
- Weekly ritual: pull competitor follower/engagement deltas, find under-served hashtags, recommend a posting calendar.
- Frustration: nothing tracks competitor deltas over time (business_discovery is point-in-time); "best time to post" is folklore not data.

## Survivors (→ transcendence rows)
C1 compare (9), C2 growth (9), C3 best-time (8), C4 top-posts (8), C5 formats (8), C7 rivals (7), C6 hashtag-perf (6). All hand-code, all local-store leverage.

## Killed candidates
- C8 engagement-rate report → absorbed into compare (ER is the ranked metric).
- C9 comment sentiment → LLM dependency, no mechanical version.
- C10 AI caption/hashtag recs → LLM dependency + write-adjacent, out of scope.
- C11 posting-cadence consistency → folded into best-time as a posting-gap column.
- C12 demographics-compare across brands → fails weekly-use bar (lifetime demographics move slowly); single-account demographics already absorbed.
- C13 story completion-rate → verifiability/scope fail (24h window rarely captured in snapshots).
- C14 saves/shares leaderboard → sibling of top-posts (becomes a `--metric` flag).
- C15 portfolio weekly digest → scope creep / composable from compare+growth+top-posts via pipe.
- C16 stale-account/sync-health → plumbing, belongs in doctor/sync not the transcendence set.
