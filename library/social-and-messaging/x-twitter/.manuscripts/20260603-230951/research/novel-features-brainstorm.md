# Novel Features Brainstorm — x-twitter (reprint)

Subagent run for run-id 20260603-230951. Full audit trail (Customer model,
Candidates, Survivors/kills, Reprint verdicts).

## Customer model

**Devon — indie automation operator / bot builder.** Today, without this CLI, Devon stitches together `node-twitter-api-v2` scripts and a few `curl` one-liners to run a posting bot and a mention-watcher. Every read hits the live API and now burns PPU credits at ~$0.005/post; there's no local copy of anything, so re-running an analysis re-spends. Weekly ritual: polls a handful of accounts' timelines + their own mentions, drops anything interesting into a spreadsheet, and ships a few automated self-reply threads summarizing project updates. Frustration: since the Feb-2026 reply restriction, half their reply automation 403s with a cryptic message, and they can't tell which token (app-only vs user-context) unlocks which call until a request fails in production — every credit spent on a 403 is wasted money.

**Mara — social/growth marketing manager.** Today she lives in the X web UI and a clunky third-party scheduler, manually eyeballing which posts landed and copy-pasting metrics into a deck. She has a Basic-tier key gathering dust because the existing CLIs (`t`, `x-cli`) are live-only and give her no way to keep a running archive. Weekly ritual: pull last week's posts for her brand and 3 competitors, rank by engagement, monitor brand mentions, and curate a "watchlist" List of voices to track. Frustration: there's no offline ranking — every "what were our top posts" question is another paginated, credit-spending crawl, and competitor timelines she pulled last week are gone the moment the terminal scrolls.

**Ravi — researcher / journalist doing network and discourse analysis.** Today Ravi uses tweepy notebooks, hits 453/tier errors constantly, and hoards JSON dumps in folders he can't query. He needs reproducible, queryable corpora, not live snapshots. Weekly ritual: run a recent-search on a topic or hashtag, archive everything to disk, then reconstruct conversation threads and slice by author/date/lang for a piece he's writing. Frustration: the recent-search API can't answer the cross-cutting questions he actually has ("show me every reply in this conversation, by this set of authors, that got >100 likes") — he has to write throwaway pandas every time, and full-archive search is paywalled out of reach on PPU.

**Aria — the AI agent (MCP host) operating on a user's behalf.** Today Aria talks to thin, stateless X MCP servers that expose a dozen flat tools, no safety hints, and no memory — every question is a fresh live call, and Aria can't tell a read from a destructive write without trial and error. Weekly ritual (driven by its human): "snapshot this user," "archive this search and tell me the top threads," "monitor these mentions and report." Frustration: without read-only/destructive hints Aria triggers permission prompts on harmless reads; without named multi-step intents it has to chain 5 raw tool calls (and 5 credit spends) for one logical task; without a local mirror it re-spends credits on data it already pulled an hour ago.

## Candidates (pre-cut)

| # | Name | Command | One-line | Persona | Source | Kill/keep note |
|---|------|---------|----------|---------|--------|----------------|
| C1 | Token capability doctor | `doctor` (enrich) | Report which token is configured and exactly what it unlocks (reads vs writes vs tier-gated), plus active verify state | Devon, Aria | (a) | KEEP — local config/token synthesis; maps 453/402/client-not-enrolled/reply-restriction. |
| C2 | Thread reconstruction from store | `thread show <conversation_id>` | Rebuild full conversation tree from flat synced posts via conversation_id + referenced_tweets | Ravi, Mara | (c) | KEEP — pure SQLite tree-walk. Hand-code. |
| C3 | Top posts by engagement | analytics group-by | Rank synced posts by engagement offline | Mara, Ravi | (c) | KEEP-as-framework (analytics). |
| C4 | Search-and-archive (MCP intent) | mcp.intents | recent-search → persist → return top + newest_id | Aria, Ravi | (e) | KEEP. spec-emits. |
| C5 | User-snapshot (MCP intent) | mcp.intents | user lookup + timeline + metrics, persisted, one object | Aria, Mara | (e) | KEEP. spec-emits. |
| C6 | Monitor-mentions (MCP intent) | mcp.intents | incremental since_id mentions poll, dedup, only-new | Devon, Aria | (e) | KEEP. spec-emits. |
| C7 | Engagement-report (MCP intent) | mcp.intents | ranked engagement table | Mara | (e) | overlap C3/C5 → cut. |
| C8 | Conversation-thread (MCP intent) | mcp.intents | conversation fetch + tree | Aria, Ravi | (e) | overlap C2 (cobratree exposes thread show) → cut. |
| C9 | Markdown → self-reply thread | `thread compose` (prior) | numbered self-reply chain, draft-first, warn restriction | Devon | (d) | KEEP/reframe. prior-keep. |
| C10 | Markdown → X Article draft | `articles-publish-md` (prior) | Draft.js content_state, draft-first | Devon, Mara | (d) | KEEP/reframe. prior-keep. |
| C11 | List curation (MCP intent) | mcp.intents | usernames → list members | Mara | (e) | thin loop → cut. |
| C12 | Offline FTS with X-operator filters | `search --type tweets` | FTS + is:/has:/lang/engagement filters | Ravi, Mara | (b) | already absorb #30 → cut. |
| C13 | Mention-velocity over time | analytics group-by created_at | time-bucket counts | Mara, Devon | (c) | framework analytics → cut. |
| C14 | Stale/cost-aware sync hint | framework stale + hints | cost guard before re-sync | Devon, Ravi | (a) | framework → cut. |
| C15 | Compose-thread (MCP intent) | mcp.intents | agent wrapper of thread compose | Aria | (e) | duplicate C9 → cut. |
| C16 | Local sentiment / influence-graph | — | sentiment + network synthesis | — | (b) | KILL — LLM/heavy-data dependency. |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Token capability doctor | `doctor` (enriched) | 9/10 | spec-emits | Reads token type + verify state locally; maps 453/client-not-enrolled/402/Feb-2026 reply-restriction to actionable per-state text | Brief Reachability Risk + Auth Decision |
| 2 | Thread reconstruction from store | `thread show <conversation_id>` | 8/10 | hand-code | Walks synced posts in SQLite joining conversation_id + referenced_tweets into ordered depth-tagged tree | Brief Data Layer + Top Workflow #1 |
| 3 | Search-and-archive intent | spec mcp.intents | 8/10 | spec-emits | recent-search → persist (dedup, newest_id) → top + cursor in one agent call | Brief User Vision + Top Workflow #1 |
| 4 | User-snapshot intent | spec mcp.intents | 7/10 | spec-emits | users get + users tweets(metrics) → cache → one compact object | Brief User Vision + Top Workflow #2 |
| 5 | Monitor-mentions intent | spec mcp.intents | 8/10 | spec-emits | mentions + since_id cursor + dedup → only new | Brief Top Workflow #3 |
| 6 | Markdown → self-reply thread | `thread compose` | 7/10 | hand-code | 280-char-packed numbered self-reply chain; draft-first, opt-in --post, warn on mention/quote 403 | Top Workflow #4 + Reachability Risk; prior built |
| 7 | Markdown → X Article draft | `articles-publish-md` | 6/10 | hand-code | markdown+frontmatter → Draft.js content_state; draft-first, opt-in browser/GraphQL, verify short-circuit | Build Priorities #3; prior built |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C3 Top posts by engagement | Folds into framework `analytics --type tweets --group-by author_id`; new command = thin rename | #4 user-snapshot |
| C7 Engagement-report intent | Duplicate of C5 ranked metrics; keep agent catalog lean | #4 user-snapshot |
| C8 Conversation-thread intent | Cobratree already exposes `thread show` as MCP tool; separate intent = catalog bloat | #2 thread show |
| C11 List-curate intent | Thin loop over lists members create; marginal weekly use | #3 search-and-archive |
| C12 X-operator FTS filters | Already absorb #30 + framework search; enrichment not new feature | framework search |
| C13 Mention-velocity histogram | Expressible via analytics group-by created_at; wrapper | #5 monitor-mentions |
| C14 Stale/cost-aware sync hint | Already framework stale + hintIfUnsynced/hintIfStale | framework stale |
| C15 Compose-thread intent | Exact duplicate of thread compose (cobratree exposes it) | #6 thread compose |
| C16 Local sentiment / influence graph | LLM/heavy-data dependency — banned by rubric | #4 user-snapshot |

## Reprint verdicts

| Prior feature | Command | Verdict | Justification |
|---------------|---------|---------|---------------|
| Markdown to tweet thread | `thread compose` | Keep (reframe in place) | Self-reply threads still post under Feb-2026 restriction; reuse command, reframe to detect+warn on mention/quote/cold-reply 403, keep draft-first opt-in `--post`. 7/10. |
| Markdown to X Article | `articles-publish-md` | Keep | No official v2 endpoint; preserve via regen-merge, draft-first Draft.js, opt-in browser/GraphQL publish, PRINTING_PRESS_VERIFY short-circuit. 6/10 — weakest keep. |

No prior features dropped or renamed; both prior `novel_features_built` kept with original commands.
