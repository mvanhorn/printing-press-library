# Cricket Data (CricAPI) CLI — Absorb Manifest

## Source Tools Surveyed

| Tool | Type | Stars | Last Touched | Commands |
|---|---|---|---|---|
| cbirajdar/cricket-cli | Python CLI | 19 | 2017 (stale) | scores, rankings, standings |
| Nshul/cricket-cli | Node CLI | 6 | ~2018 | calendar, configure |
| mikeesto/cricket-cli | Node CLI | 0 | 2014 | live score watch |
| cricketlive npm | JS library | — | 2024 | live match details |
| cricapi (npm) | JS wrapper | — | 2018 | wraps GET endpoints |

No cricket MCP server exists on GitHub. We are the first.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | List countries | `cricapi` npm `countries()` | `countries` cmd | `--json`, paginated, cached in SQLite |
| 2 | Live + upcoming matches | cbirajdar `scores`, mikeesto live | `live` cmd (currentMatches) | `--json`, agent-mode, watch loop, `--compact` |
| 3 | All matches with search | `cricapi` npm `matches()` | `matches` cmd | `--search`, paginated, store-backed |
| 4 | Match info by id | `cricapi` npm `match_info()` | `match info <id>` | typed exit codes, `--json` |
| 5 | Match scorecard | (none in CLIs; only API) | `match scorecard <id>` | structured per-innings, `--csv` |
| 6 | Match squads | (none in CLIs) | `match squad <id>` | per-team table |
| 7 | Match fantasy points | (fantasy lib only) | `match points <id>` | first CLI with this |
| 8 | Quick live-score snapshot | mikeesto `app.js` | `score` (cricScore) | tighter than `live`, single call |
| 9 | Series list | `cricapi` npm `series()` | `series list` | store-backed |
| 10 | Series details | `cricapi` npm `series_info()` | `series info <id>` | matches + squads + points combined |
| 11 | Player search | `cricapi` npm `players()` | `players search <q>` | FTS-backed when synced |
| 12 | Player info | `cricapi` npm `players_info()` | `player info <id>` | full career stats display |
| 13 | API key configuration | Nshul `configure` | `auth set-token` | first-class auth subcommands, doctor check |
| 14 | Match calendar / upcoming | Nshul `calendar` | `today`, `tomorrow`, `week` | richer time windows |
| 15 | Doctor / health check | (none) | `doctor` cmd | auth + reachability + rate limit |
| 16 | JSON output for piping | (none) | `--json` on every cmd | agent-native |
| 17 | Field filtering | (none) | `--select team,name,status` | reduce context burn |
| 18 | CSV output | (none) | `--csv` on list cmds | spreadsheet workflows |
| 19 | Dry-run | (none) | `--dry-run` shows request | safe inspection |

## Transcendence (only possible with our approach)

Status legend: shipping-scope unless marked `(stub)`.

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---|---|---|---|
| 1 | Team-aware fixture filter | `team <name> next` | Resolve team-name → filter currentMatches/matches in one shot. Solves the natural-language "when does Pakistan play next" problem that no existing tool answers. | 9 |
| 2 | Local SQLite store | `sync`, `sql <query>` | Persist matches/series/players/countries; query offline with no API hits. None of the 4 competitor CLIs do this. | 8 |
| 3 | FTS5 offline search | `search <query>` | Search match names, players, series across the local store without API calls — preserves the 100/day budget. | 8 |
| 4 | First cricket MCP server | (built-in via Cobra-tree mirror) | No cricket MCP exists. Every Cobra command becomes an agent tool. Direct cricket access from Claude Desktop, Cursor. | 9 |
| 5 | Format-split player stats | `player splits <id>` | Combine player_info career arrays + matches → Test/ODI/T20 splits in one view. CricAPI exposes the data; no CLI exposes the split. | 7 |
| 6 | Series timeline view | `series timeline <id>` | Combine series_info + match_info per match into a chronological tournament board. Single command replaces 10+ API calls. | 7 |
| 7 | Fantasy aggregator | `match fantasy <id>` | Merge match_points + match_squad + match_scorecard into one fantasy-friendly view. None of the existing tools combine these. | 7 |
| 8 | Watch loop (poll cricScore) | `watch <match-id>` | Background poll of cricScore with `--alert-on wickets,fifty,hundred,boundary-of-N`. None of the competitors have alerts. | 6 |
| 9 | "What did I miss" digest | `recap --since 2h` | Time-windowed digest of finished + status-changed matches from the store. Requires a local change history. | 6 |
| 10 | Series watchlist | `watchlist add <series>`, `watchlist refresh` | User pins series; one command refreshes all of them. Local state none of the competitors have. | 6 |
| 11 | Format breakdown of fixtures | `today --format t20i,odi` | Filter today's matches by format using local + API data. CricAPI's matches include matchType; we expose it as a filter. | 5 |
| 12 | Honest free-tier rate guard | doctor + `--budget` flag | Track daily request count in SQLite; warn before exceeding 100/day. Surfaces the constraint the user signed up under. | 6 |

All transcendence features above are shipping scope. The MCP surface (#4) is emitted automatically by the generator from the Cobra command tree; no separate implementation.

## User Vision (from briefing context)

The user originally wanted: "When does Pakistan play next?" as the natural-language query that beats everything else. Feature #1 (team-aware fixture filter) directly answers that. They also already have an ESPN-side wrapper for live scores via series IDs — this CLI complements it on the analytical/team-search axis, then surpasses it via the MCP and offline store.

## Source Priority

Single-source. No combo.

## Anything you should worry about before approving

- **Free tier rate limit.** 100 req/day is plenty for personal use but quickly exhausted by an over-eager sync. The store + `--budget` guard mitigate this.
- **API key required.** User has not yet signed up at cricketdata.org. Live smoke testing (Phase 5) will skip in this run; sign-up + live verify can happen post-generation in a follow-up.
- **No formal OpenAPI spec.** We'll write an internal YAML spec from the 12-endpoint catalog above. This is the same path ESPN's CLI took.
- **Series/match IDs change every season.** Same bit-rot problem the bash wrapper hit, but here it doesn't matter — IDs are fetched dynamically via `currentMatches`/`series`, not hardcoded.
- **Fantasy endpoints (`match_points`, etc.) may need higher tier on some accounts.** Free tier in research seems to include them; the CLI will report cleanly if a 401/403 indicates tier gating.

## Phase 1.5 Gate — what gets generated next if approved

19 absorbed + 12 transcendence = **31 features**. Of those, ~12 map directly to API endpoints (generator emits them); ~7 are wrappers/views built on top; ~12 are hand-built novel commands.

Output: a `cricapi-pp-cli` Go binary, an MCP server binary, a SKILL.md for agent discovery, and library-shape directory ready for PR to `mvanhorn/printing-press-library` under `media-and-entertainment/`.
