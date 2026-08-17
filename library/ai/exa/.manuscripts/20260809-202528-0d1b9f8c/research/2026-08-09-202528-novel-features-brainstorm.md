# Novel Features Brainstorm (subagent output, 2026-08-09)

## Customer model
1. Mira — AI app developer building a research agent on Exa. Re-runs query variants, wants to replay past queries, wants per-request cost visibility.
2. Dev — lead-gen / G2M researcher. Watches a fixed set of competitors weekly; webset entity data never meets search results.
3. Sam — content analyst / journalist. Reviews monitor runs weekly, diffs by hand, needs citation trail archived and searchable.

## Candidates (pre-cut)
C1 spend report KEEP | C2 monitor run diff KEEP | C3 entity report KEEP | C4 webset new KEEP
C5 replay query KILL (wrapper of /search) | C6 research brief KILL (app-shaped orchestration) | C7 page change diff KILL (occasional use) | C8 topic digest KILL (overlaps analytics) | C9 citation leaderboard KILL (speculative) | C10 smoke check KILL (wrapper, no local power)

## Survivors and kills
### Survivors
| # | Feature | Command | Score | Buildability | Long Description |
|---|---------|---------|-------|--------------|------------------|
| 1 | Spend report | exa-pp-cli spend | 8/10 | hand-code | Use this command to understand cumulative spend across every Exa call. Do NOT use it for counts or groupings of synced records; use 'analytics' instead. |
| 2 | Monitor run diff | exa-pp-cli monitor diff <monitor-id> | 8/10 | hand-code | Use this command to see what changed between two runs of a scheduled monitor. Do NOT use it for new items in a live webset; use 'webset new'. Do NOT use it for a named entity's timeline; use 'entity report'. |
| 3 | Entity report | exa-pp-cli entity report "Name" | 8/10 | hand-code | Use this command for a first-seen / last-seen / mention-count timeline of a named company or person across synced webset items and search results. Do NOT use it to compare two scheduled monitor runs; use 'monitor diff'. Do NOT use it for new items in a live webset; use 'webset new'. Do NOT use it to run a fresh search; use 'search'. |
| 4 | Webset new | exa-pp-cli webset new <webset-id> | 7/10 | hand-code | Use this command for what is new inside one live webset since you last looked. Do NOT use it to compare scheduled monitor runs; use 'monitor diff'. Do NOT use it for a named entity's timeline; use 'entity report'. |

### Killed candidates
| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| Topic digest | overlaps analytics, drifts to summarization | analytics |
| Citation leaderboard | speculative pain | analytics |
| Smoke check | wrapper of 3 endpoints, no local power | search |
| Replay query | thin wrapper of /search | search |
| Research brief | app-shaped orchestration >200 LoC | search/contents/answer |
| Page change diff | occasional use, DIY equivalent | contents |
