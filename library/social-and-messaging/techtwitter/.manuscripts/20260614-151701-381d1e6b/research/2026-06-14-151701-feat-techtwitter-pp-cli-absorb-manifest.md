# Tech Twitter CLI — Absorb Manifest

No CLI/MCP/skill exists for Tech Twitter itself. Features are absorbed from adjacent
categories: X/Twitter CLIs (`sferik/x-cli`, `public-clis/twitter-cli`,
`hay/twitter-cli`, `Infatoshi/x-cli`), HN readers, RSS/feed-reader CLIs, and
agent-evidence/MCP tools. Every absorbed feature is matched **and beaten** with
offline FTS, `--json`/`--select`/`--csv`, typed exit codes, browser-UA transport,
and a compounding local SQLite store.

**Scope note:** Products are excluded at the user's request — no `products`
resource/command and no products in the local store/sync. The live `agent context`
/ `agent send` endpoints still accept all 6 kinds (including `launches`) as a
server-side passthrough; only the offline composition (`digest`, `evidence`) drops
the product source.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search tweets by text | sferik/x-cli `search`, hay/twitter-cli | `(generated endpoint) tweets search` | curated/deduped/quality-scored corpus, not the firehose; `--json/--select` |
| 2 | Offline full-text search | feed-reader local cache | `(behavior in techtwitter-pp-cli search)` framework FTS over synced store | works offline, regex, SQL-composable |
| 3 | Latest item / feed head | HN front-page, RSS latest | `(generated endpoint) tweets latest` | single latest tweet + `nextTweetId` chain |
| 4 | Trending feed | HN trending | `(generated endpoint) tweets trending` | ranked curated stream, cacheable |
| 5 | User/author timeline | x-cli user timeline | `(generated endpoint) tweets author` | curated author stream by handle |
| 6 | Topic/tag feed | tag feeds | `(generated endpoint) tweets topic` | topic-slug-scoped curated tweets |
| 7 | Monthly archive | archive browsers | `(generated endpoint) tweets monthly` | historical month pull |
| 8 | Day stream | archive browsers | `(generated endpoint) command tweets-by-date` | tweets for a given date |
| 9 | Single item get | x-cli `show`, HN item get | `(generated endpoint) tweets get` | single tweet by UUID + neighbors |
| 10 | Long-form articles | RSS reader articles | `(generated endpoint) articles list` | long-form rows w/ tweet provenance |
| 12 | Author/user lookup | x-cli user lookup | `(generated endpoint) profiles search` | engagement-ranked author search |
| 13 | Newsletter/feed archive | RSS reader | `(generated endpoint) newsletters list` | in-app "Threads" newsletters |
| 14 | Comment-heavy / debate view | HN "most discussed" | `(generated endpoint) command hot-takes` | high-reply tweets w/ `replyRatio` |
| 15 | Top contributor of day | (none — novel-ish) | `(generated endpoint) command main-character` | day's most-engaged author |
| 16 | Trend/keyword dashboard | trends dashboards | `(generated endpoint) command heatmap` | topic momentum: keyword/count/engagement |
| 17 | Corpus/status stats | CLI `status` cmds | `(generated endpoint) command stats` | index health: tweets/topics/profiles |
| 18 | Cited evidence bundle | agent-evidence/MCP tools | `(generated endpoint) agent context` | 6 kinds, per-row provenance + citations |
| 19 | A2A JSON-RPC evidence | A2A agents | `(generated endpoint) agent send` (POST /api/a2a) | stateless `SendMessage` evidence |
| 20 | Open in browser | HN/x-cli `open` | `(behavior in techtwitter-pp-cli open)` | open canonical X/article/product URL |
| 21 | Offline cache / mirror | feed readers | `(behavior in techtwitter-pp-cli sync)` framework sync | full local SQLite mirror, all entities |
| 22 | Structured/pipeable output | all modern CLIs | `(behavior in techtwitter-pp-cli search)` framework `--json/--select/--csv/--compact` | universal agent-native output |
| 23 | Health/doctor check | modern CLIs | `(behavior in techtwitter-pp-cli doctor)` framework `doctor` | reachability + cache report |
| 24 | MCP server exposure | MCP tools | `(behavior in techtwitter-pp-cli mcp)` Cobra-tree MCP mirror | every read command as an MCP tool |

## Transcendence (only possible with our compounding local store)

Customer model → 3 personas: (1) the **AI agent** needing cited, offline-fast evidence;
(2) the **tech-Twitter watcher/curator** (often the site owner) tracking narratives and
"what did I miss"; (3) the **researcher/writer** mining curated discourse. Candidates were
generated against "what can they NOT answer with one stateless API call," then cut
adversarially. The live API only ever returns a *current snapshot*; persisting snapshots
and a sync cursor unlocks the time dimension nothing upstream can serve.

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | What changed since last sync | `since` | hand-code | Requires a persisted local sync cursor; the API has no "delta since timestamp T" call | Use this for newly-curated/newly-hot tweets since your last `sync`. Do NOT use it for all-time trending; use the generated `tweets trending` for that. |
| 2 | Topic momentum over time | `momentum` | hand-code | Requires historical heatmap snapshots stored across syncs; the live heatmap is a single snapshot | Use this to see which topics are rising, falling, or newly appearing across your stored snapshots. For a single current snapshot use `command heatmap`. |
| 3 | Emerging-narrative detector | `narrative` | hand-code | Requires diffing keyword sets across stored snapshots to find newly-emerged/accelerating keywords, grounded in stored tweets | Use this to surface emerging keywords vs prior snapshots with supporting evidence. The live `agent context --kind narrative-alert` is a snapshot; this tracks emergence over time. |
| 4 | Offline composed digest | `digest` | hand-code | Requires cross-entity local composition (top tweets + recent articles + top authors) for a window, offline | Use this to assemble a read-list/digest from the local store for a window. Agent-formatted with canonical-URL citations; no network needed after sync. |
| 5 | Offline cited evidence bundle | `evidence` | hand-code | Mirrors the agent-context kinds but assembles from local SQLite with citations, zero network | Use this when an agent needs cited evidence offline. Mirrors `agent context` kinds (what-changed/arguments/read-list/narrative-alert) from the local store; `launches` stays live-only since products aren't stored. |
| 6 | Author engagement leaderboard | `author-rank` | hand-code | Requires accumulated per-author engagement across stored syncs plus each author's best stored tweet | Use this for a local leaderboard of top authors by stored engagement over a window. For a one-shot day winner use `command main-character`. |
| 7 | Time travel by date | `time-travel` | hand-code | Branded port of the homepage Time Travel panel: curated tweets for a chosen date, served live or fully offline from the local store with a panel-style render | Use this to see the curated tweets for a specific date (`YYYY-MM-DD`, `today`, `yesterday`, `latest`), optionally filtered by `--topic`. Pulls live from `command tweets-by-date` or offline from the local store. |

Minimum 5 transcendence features satisfied (7). All hand-code, all powered by the
compounding local store + repeated syncs.
