# readwise novel-features brainstorm (audit trail)

Subagent run 2026-11-18, Phase 1.5 Step 1.5c.5. Prior research: `none` (no `research.json` in the library or the manuscripts). The 10 prior novel features reached the subagent through the brief's `## User Vision` section, which is source (e). There is no separate `## Reprint verdicts` section because the prior path was `none`.

## Customer model

1. **The highlight hoarder who researches before they read.**
   - Owns thousands of highlights.
   - Before starting an essay, talk or decision, asks "what have I already read about this?"
   - The corpus is split across highlights, books and Reader documents.
   - No offline cross-corpus query exists, and the list APIs allow only 20 requests a minute.
2. **The PKM exporter.**
   - Pipes Readwise into Obsidian, Notion or Logseq, and reviews the week's highlights every Sunday.
   - Has no diffable local copy, and deletions vanish silently.
   - The same source exists as a Reader document and as a classic book (`readwise-cli#33`).
3. **The agent and automation builder.**
   - Runs Claude Code, Codex or cron jobs that need deterministic, bounded JSON and safe writes.
   - Fan-out wrappers hit the 20/min wall (`readwise-mcp-enhanced#3`).
   - Has no stateful "new since last run" for a saved query.
4. **The inbox triager.**
   - Works in a fixed weekly reading slot.
   - Guesses at reading time and item age.
   - The official triage happens only inside an LLM conversation (readwise-skills `triage`).
   - Cannot bulk-archive abandoned items safely.

## Candidates (pre-cut)

| # | Candidate | Command | Persona | Source | Inline verdict |
|---|---|---|---|---|---|
| 1 | Topic dossier | `ask "<topic>"` | 1 | a,e,c | Keep (mechanical, no LLM) |
| 2 | Time-budget backlog | `backlog --budget 30m` | 4 | a,e,c | Keep |
| 3 | Next-up ranker | `read-next` | 4,1 | e,c | Keep (mechanical scoring with tunable weights) |
| 4 | Duplicate finder | `dedupe` | 2,1 | a,e,c | Keep |
| 5 | Cross-source digest | `digest --since 7d` | 2 | a,e,c | Keep |
| 6 | Stale and orphan triage | `triage stale` | 4 | a,e,c | Keep |
| 7 | Saved-query watch | `watch add/run` | 3 | a,e | Keep (marginal) |
| 8 | Serendipity connections | `connections` | 1 | e,c | Dropped to Pass 3: mechanical version is only a tag/author join; "non-obvious" needs semantic judgment; low verifiability |
| 9 | Signed outbound webhook | `webhook` | 3 | e | Dropped to Pass 3: scope creep and external service (delivery to a user URL) |
| 10 | Highlight resurfacing | `resurface` | 1 | b | Dropped to Pass 3: duplicates daily review (row 29) and quiz recipe (row 58) |
| 11 | Tag hygiene audit | `tags audit` | 2 | c | Kept to Pass 3 (unique three-table join) |
| 12 | Changes since last sync | `changes --since 7d` | 2 | c,b | Dropped to Pass 3: overlaps `mirror` (row 61) |
| 13 | Reading stats | `stats` | 1 | b | Dropped to Pass 3: overlaps `analytics --group-by` |
| 14 | Safe save with preflight | `save-if-new` | 3 | a | Dropped to Pass 3: overlaps `documents save --idempotent` (row 8) and `dedupe` |
| 15 | Author affinity report | `authors` | 1 | c | Dropped to Pass 3: becomes a `read-next` scoring input |
| 16 | Untagged highlights | `untagged` | 2 | a | Dropped to Pass 3: wrapper over `highlights list --data-source local` |

## Survivors

The survivor table, with scores, how each works, evidence and Long Descriptions, is copied verbatim into `2026-11-18-122527-feat-readwise-pp-cli-absorb-manifest.md` under `### Transcendence`.

Scores:
- 9/10: `ask`, `backlog`
- 8/10: `dedupe`, `digest`, `triage stale`
- 7/10: `read-next`
- 5/10: `watch`

All seven are hand-code.

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| `connections` (serendipity) | The mechanical version is only a shared tag/author join; "non-obvious" needs semantic judgment (LLM dependency); output can't be verified; monthly use at best | `ask` |
| `webhook` (signed outbound delivery) | Delivery infrastructure to a user URL (external service, SSRF surface); piping `watch run --json` to `curl` covers it; fails the weekly-use bar | `watch` |
| `resurface` | Duplicates the daily review and quiz recipe; no pain evidence | `digest` |
| `tags audit` | Monthly clean-up, not a weekly ritual (soft kill) | `dedupe` |
| `changes --since` | `mirror` already carries a deletion-aware incremental manifest | `digest` |
| `stats` | Covered by `analytics --type highlights --group-by <field>` | `digest` |
| `save-if-new` | `documents save --idempotent` plus `dedupe` cover it | `dedupe` |
| `authors` | A scoring input, not a command; folded into `read-next` | `read-next` |
| `untagged` | A filter on an existing local list | `triage stale` |
