# X (Twitter) CLI Absorb Manifest

Run: 20260603-230951 · API: x-twitter · Spec: official X v2 OpenAPI (v2.165, 139 paths / 163 ops)

## Scope summary

- **Absorbed:** 36 features across the X ecosystem — MCP servers (mcp-twitter-server 53 tools, x-autonomous-mcp ~30, twitter-mcp, Dishant27), CLIs (sferik/x-cli 5.6k★ — the bar; `t`; twurl), SDKs (twitter-api-v2, tweepy). The vast majority map directly to generated endpoint commands from the 163-operation official spec; the framework adds offline store + FTS + SQL + agent output that **no** competing tool has.
- **Transcendence (novel on top):** 7 survivors (4 spec-emits, 3 hand-code) — see table below.
- **vs the best competitor (sferik/x-cli):** match its full command tree + streaming + retry/backoff, and beat it with offline SQLite mirror, FTS5/SQL, `--json/--select` agent output, a 163-operation MCP surface (Cloudflare pattern: stdio+http transport, code orchestration, hidden endpoint tools, named intents), and honest tier/reachability diagnostics.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Post a tweet | twitter-mcp / x-cli | (generated endpoint) tweets create | --dry-run, --json, typed exit codes |
| 2 | Delete a tweet | mcp-twitter-server | (generated endpoint) tweets delete | destructiveHint, --dry-run |
| 3 | Get tweet by id | mcp-twitter-server | (generated endpoint) tweets get | --select projection, store cache |
| 4 | Search recent tweets | all MCP / x-cli | (generated endpoint) tweets search-recent | persist to store, offline re-query |
| 5 | Full-archive search | tweepy / x-cli (Pro+) | (generated endpoint) tweets search-all | tier-gated, store-backed |
| 6 | Get user by id/username | all | (generated endpoint) users get / users by-username | --select, store cache |
| 7 | User timeline | all | (generated endpoint) users tweets | public_metrics, store-backed |
| 8 | User mentions | x-autonomous-mcp | (generated endpoint) users mentions | since_id incremental |
| 9 | Home timeline | x-cli | (generated endpoint) users timelines reverse-chronological | user-context |
| 10 | Follow / unfollow | most MCP | (generated endpoint) users following create/delete | --dry-run |
| 11 | Followers / following | most MCP | (generated endpoint) users followers / following | store-backed, paginated |
| 12 | Like / unlike | most MCP | (generated endpoint) users likes create/delete | --dry-run |
| 13 | Liked tweets | mcp-twitter-server | (generated endpoint) users liked-tweets | store-backed |
| 14 | Retweet / unretweet | most MCP | (generated endpoint) users retweets create/delete | --dry-run |
| 15 | Bookmark / unbookmark | x-autonomous-mcp | (generated endpoint) users bookmarks create/delete | user-context |
| 16 | Get bookmarks | — | (generated endpoint) users bookmarks | store-backed |
| 17 | Create list | Dishant27/twitter-mcp | (generated endpoint) lists create | --dry-run |
| 18 | Add/remove list member | mcp-twitter-server | (generated endpoint) lists members create/delete | --dry-run |
| 19 | List members / user lists | mcp-twitter-server | (generated endpoint) lists members / users owned-lists | store-backed |
| 20 | List timeline | x-cli | (generated endpoint) lists tweets | store-backed |
| 21 | Spaces lookup / search | — | (generated endpoint) spaces get / search | snapshot ephemeral state |
| 22 | DM read / send | — | (generated endpoint) dm-conversations / dm-events | user-context |
| 23 | Media upload | x-autonomous-mcp | (generated endpoint) media upload | chunked |
| 24 | Trends | XActions | (generated endpoint) trends | store-backed |
| 25 | Community Notes | — | (generated endpoint) notes search | read-only |
| 26 | Communities | — | (generated endpoint) communities | read-only |
| 27 | Usage metering | — | (generated endpoint) usage tweets | cost awareness |
| 28 | Compliance jobs | — | (generated endpoint) compliance jobs | batch |
| 29 | Filtered / sampled streaming | x-cli / tweepy (Pro+) | (generated endpoint) tweets search-stream / sample-stream | tier-gated |
| 30 | Offline full-text search | NONE | (behavior in x-twitter-pp-cli search) | FTS5, regex, --type filter |
| 31 | SQL over local data | NONE | (behavior in x-twitter-pp-cli sql) | composable, SELECT-only |
| 32 | Sync to local store | NONE | (behavior in x-twitter-pp-cli sync) | since_id cursor, dedup |
| 33 | CSV / JSON / --select output | t (CSV only) | (behavior in any command --csv/--json/--select) | agent-native |
| 34 | Retry / backoff on 429 | x-cli / twitter-api-v2 | (behavior in x-twitter-pp-cli — client) | cliutil.AdaptiveLimiter + RateLimitError |
| 35 | Markdown to tweet thread | (prior novel) | x-twitter-pp-cli thread compose | atom-aware splitter, 280-char packing |
| 36 | Markdown to X Article | (prior novel) | x-twitter-pp-cli articles-publish-md | Draft.js content_state, draft-first |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Score | Why Only We Can Do This |
|---|---------|---------|--------------|-------|-------------------------|
| 1 | Token capability doctor | doctor (enriched) | spec-emits | 9/10 | Local synthesis of token type + verify state + X error palette (453 / client-not-enrolled / 402 / Feb-2026 reply-restriction) into actionable diagnosis — no live call |
| 2 | Thread reconstruction from store | thread show <conversation_id> | hand-code | 8/10 | Offline SQLite tree-walk joining conversation_id + referenced_tweets; no single API call returns an ordered conversation tree |
| 3 | Search-and-archive intent | (MCP intent) search-and-archive | spec-emits | 8/10 | Multi-step orchestration: recent-search → persist (dedup, newest_id cursor) → top + cursor in one token-efficient agent call (PPU cost feature) |
| 4 | User-snapshot intent | (MCP intent) user-snapshot | spec-emits | 7/10 | Joins users get + users tweets(public_metrics) into one cached object, halving agent round-trips/credits |
| 5 | Monitor-mentions intent | (MCP intent) monitor-mentions | spec-emits | 8/10 | Incremental since_id store cursor + dedup → agent-shaped "only new" mentions |
| 6 | Markdown to self-reply thread | thread compose | hand-code | 7/10 | X content pattern: 280-char-packed self-reply chain, draft-first + opt-in --post, detects/warns Feb-2026 mention/quote 403 |
| 7 | Markdown to X Article draft | articles-publish-md | hand-code | 6/10 | No official v2 endpoint exists; bespoke Draft.js content_state via browser/GraphQL, draft-first + verify short-circuit |

**Hand-code commitment:** 3 features require hand-written Go after generate (`thread show`, `thread compose`, `articles-publish-md`; each ~50-150 LoC plus root.go wiring). `thread compose` and `articles-publish-md` are preserved prior novels carried through regen-merge; `thread show` is net-new this reprint.
**Spec-emits:** 4 (`doctor` enrichment + 3 MCP intents shipped via the spec's `mcp.intents` block).

**Stubs:** none.

## Reprint reconciliation (prior novels)
- `thread compose` → **KEEP (reframe in place):** self-reply threads survive the Feb-2026 restriction; reframe to detect+warn on mention/quote/cold-reply 403.
- `articles-publish-md` → **KEEP:** no official API; preserve via regen-merge, draft-first, opt-in publish, verify short-circuit. Weakest survivor (6/10).
- No prior features dropped or renamed.
