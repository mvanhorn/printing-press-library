# Drudge Report Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Position-grouped headlines (TOP_LEFT, MAIN_HEADLINE, COLUMN1, COLUMN2) | mattrasband/drudge_parser (Python, archived 2021) | Comment-marker parser over current HTML + 3-column table position | Captures `font color=red` flag that the original misses; agent-native JSON; offline historical archive via local SQLite |
| 2 | CSV/Excel dump of current headlines | ghayward/drudge_report_headline_scraper (Python+pandas) | `--csv`, `--json` flags built into every list command | Typed exit codes, `--select` field filter, agent-native streaming, no pandas required |
| 3 | Periodic re-fetch with persistence | lukerosiak/drudge (Python+postgres, every 5 min) | `sync` writes a new snapshot row + per-story rows into local SQLite | Pure local SQLite (no postgres); stable story_ids across snapshots; FTS5 search free |
| 4 | "Types of stories" / source frequency stats | lukerosiak/drudge | `sources` aggregates outbound domains over snapshot history | Adds rising/falling delta vs prior window; `--by-slot` crosstab; SQL-queryable surface |
| 5 | Most-recent stories first | JonathanBrownCFA/scrape-it-like-you-mean-it (Node+cheerio) | `headlines --limit N` orders by composite editorial rank (slot + red + image) | No mongoose/web-app overhead; one binary; correct importance order, not just position |
| 6 | RSS-style item view with pubDate | feedpress.me/drudgereportfeed (community RSS) | RSS feed parsed alongside HTML as cross-check source | Both signals merged; canonical outbound URL extracted from `<A HREF>` (the HTML form) AND RSS guid; deduped |
| 7 | Related stories grouping | feedpress.me/drudgereportfeed CDATA | Captured under each main story when present | Returned in same `--json` payload; agents can reason about parent/child link relationships |
| 8 | Direct outbound URLs (no Drudge proxy) | drudgereport.com `<A HREF>` | Canonical outbound URL extracted directly from anchor | Already the user's ask; no extra value-add needed (this is a `must-have`) |
| 9 | Splash image extraction | drudgereport.com `<IMG>` near MAIN HEADLINE comment | `image_url` field populated for splash + column-top stories | Surfaced in `splash --json`, `headlines --json`, available for downstream image hashing |
| 10 | Real-time email alerts on new headlines | lukerosiak/drudge | Out of scope (stub) — `tail --since 1h --json` lets users pipe to their own alerter | Status: out-of-scope. The `--json` output is the alerter primitive; we don't ship SMTP / Mailgun bindings. User briefing did not ask for alerts. |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Splash now | `drudgereport splash` | 9/10 | hand-code | Parses current HTML for `<! MAIN HEADLINE >` marker, extracts title + `<A HREF>` + adjacent `<IMG>` + `<font color=red>` flag, computes splash_tenure by querying local `story` table | User vision ("more love of in the middle spot"); brief workflow #1; no peer surfaces splash slot alone with red flag; RSS confirms `(Main headline, 1st story)` |
| 2 | Breaking (red items) | `drudgereport breaking` | 8/10 | hand-code | Filters parsed stories on `is_red=true`, returns ordered by slot rank (splash > column-top > column-body) | User vision ("colors in terms of ranking importance"); mattrasband/drudge_parser does NOT capture color; brief Build Priority #1 |
| 3 | Ranked headlines | `drudgereport headlines [--limit N]` | 8/10 | hand-code | Composite editorial rank: slot weight (splash=100, red=80, col-top=60, col-body=40, top-left=30) + is_red bonus + has_image bonus | Brief Build Priority #1; user vision verbatim; no peer ranks by editorial signal |
| 4 | Promotions / demotions since last fetch | `drudgereport tail [--since DURATION]` | 9/10 | hand-code | SQL over `slot_event` rows: {appeared, promoted_to_splash, demoted_from_splash, went_red, went_black, disappeared} keyed on stable story_id | Brief workflow #2; no peer keeps snapshot history; data layer commitment |
| 5 | Splash tenure | `drudgereport tenure [--history]` | 8/10 | hand-code | Queries local store for earliest captured_at of current splash story_id; `--history` returns top-N longest-tenured splashes | Brief workflow #5; Sam's weekly chart; no peer has this concept |
| 6 | Source leaderboard with deltas | `drudgereport sources [--window 7d] [--by-slot]` | 8/10 | hand-code | Aggregates outbound_domain over window with COUNT and prior-window delta; `--by-slot` crosstabs domain × {splash, red, column} | Brief workflow #4; absorb #4 (lukerosiak baseline + new deltas/crosstab); Dana's frustration |
| 7 | On-this-date | `drudgereport on-date YYYY-MM-DD[THH:MM]` | 7/10 | hand-code | SELECT against snapshots table for nearest snapshot, reconstructs ranked headlines + splash + red items as of that moment | Greg's Sunday-argument problem; brief data layer; Drudge has no archive |
| 8 | Editorial-bent index | `drudgereport bent [--window 7d]` | 7/10 | hand-code | For each outbound_domain, COUNT(red items) / COUNT(total items) over window, ranked | Dana's Friday newsletter; joins stories.is_red × outbound_domain × snapshots |
| 9 | Story timeline | `drudgereport story <story_id>` | 6/10 | hand-code | SELECT all slot_event rows for one story_id ordered by ts, plus first/last captured_at and total tenure | Stable story_id is THE data model commitment; Sam's narrative tracking |
| 10 | Weekly digest | `drudgereport digest [--week\|--day]` | 6/10 | hand-code | Composes splash-history + tenure + sources + bent into one JSON-and-pretty-print summary | Dana's Friday newsletter; assembles primitives; no peer summarizes |

**Killed candidates (audit trail):**

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| `diff --since 12h` | Overlaps `tail` (events) and `tenure --history` (ranges) | `tenure --history` |
| `mention <term>` | Built-in `search` covers FTS5; `--slot splash` is a flag, not a new command | built-in `search` |
| `images` | Thin weekly use; `splash --json` / `headlines --json` already expose `image_url` | `splash` |
| `watch --once` | Duplicate of `tail` minus the polling implication | `tail` |
| `driving` (Drudge-driving-narrative detector) | Requires cross-source "who broke first" data we don't have; unverifiable in dogfood | `bent` |
