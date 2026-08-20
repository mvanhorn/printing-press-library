# skool-pp-cli Absorb Manifest

## Source tools surveyed
1. **louiewoof2026/skool-mcp** (TS, MCP, MIT) — 14 tools, the most direct competitor
2. **cristiantala/skool-all-in-one-api** (Apify, paid, $0.01/write) — broadest coverage
3. **FlowExtractAPI/skool-scraper-pro** (Apify, paid) — classroom-focused
4. **moon-home/scraper** (Python+Selenium) — old, posts only
5. **aperswal Active Members Chrome ext** — 30-day active-member surfacing
6. **docs.skoolapi.com** — paid SaaS proxy, low coverage
7. **Skoot CRM / StickyHive / MySkool** — closed-source DM/CRM SaaS

## Absorbed (match every feature anyone shipped)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | List communities user belongs to | louiewoof | `skool communities list` | offline, FTS search across community names |
| 2 | Get community info | louiewoof, cristiantala | `skool community info <slug>` | local cache + `--field` selector |
| 3 | Sync community to local store | (none) | `skool sync <community>` | **transcendence prerequisite** — incremental |
| 4 | List posts | louiewoof | `skool posts list` | `--since`, `--label`, `--by-user`, `--limit`, FTS, `--json --select` |
| 5 | Get post + comment tree | louiewoof, cristiantala | `skool posts get <id-or-slug>` | offline reads, `--md` markdown render |
| 6 | Create post | cristiantala | `skool posts create --md <file>` | `--dry-run`, markdown→TipTap, idempotent stdin batch |
| 7 | Update post | cristiantala | `skool posts update <id> --md` | dry-run preview |
| 8 | Delete post | cristiantala | `skool posts delete <id>` | confirmation prompt + `--yes` for agents |
| 9 | Comment on post | louiewoof, cristiantala | `skool posts comment <id> --md` | quote-reply mode |
| 10 | Like / unlike post | cristiantala | `skool posts like <id>` / `unlike` | idempotent |
| 11 | List members | louiewoof, aperswal | `skool members list` | offline, `--role`, `--level`, `--last-active`, `--search`, FTS |
| 12 | Get member | louiewoof | `skool members get <id>` | local cache + bio + links |
| 13 | List pending member requests | louiewoof, cristiantala | `skool members pending` | `--days` filter |
| 14 | Approve member | louiewoof, cristiantala | `skool members approve <id>` | batch via stdin |
| 15 | Reject member | louiewoof, cristiantala | `skool members reject <id>` | dry-run |
| 16 | Ban member | cristiantala | `skool members ban <id>` | `--yes` required |
| 17 | List courses | louiewoof, FlowExtract | `skool classroom list` | offline + FTS over course/lesson titles |
| 18 | Get course detail (modules + lessons) | louiewoof, FlowExtract | `skool classroom get <slug>` | offline reads |
| 19 | Get lesson | louiewoof, FlowExtract | `skool classroom lesson <id>` | extracts Mux video URL + attachments |
| 20 | Export course to markdown bundle | FlowExtract | `skool classroom export <slug> --out ./<dir>` | **transcendence-leaning** — full course tree → folder of `.md` |
| 21 | List calendar events | (HTML-only today) | `skool calendar list` | `--from`, `--to`, `--upcoming` |
| 22 | List notifications | louiewoof | `skool notifications list` | `--unread`, mark-read |
| 23 | Mark notifications read | louiewoof | `skool notifications read [<id>...]` | batch + `--all` |
| 24 | Leaderboard (current state) | aperswal, louiewoof | `skool leaderboard` | `--type 7d|30d|all-time`, `--top N` |
| 25 | Search across community | (no incumbent) | `skool search "<q>"` | local FTS5 across posts, comments, members, lessons |
| 26 | SQL query on local store | (no incumbent) | `skool sql "SELECT ..."` | read-only enforced |
| 27 | Doctor / health check | required | `skool doctor` | auth + reachability + buildId freshness |
| 28 | Auth setup | required | `skool auth set-token` / `auth status` / `auth logout` | TOML config write, never echoed |
| 29 | Dump arbitrary endpoint | louiewoof's `skool.request` | `skool raw GET <path>` | escape hatch |

**Total absorbed: 29 features.** Every command supports `--json`, `--select`, `--limit`, `--dry-run` (where applicable), typed exit codes (0/2/3/4/5), and pipes cleanly to `jq`.

## Transcendence (only possible because of local SQLite + cross-entity SQL)

| # | Feature | Command | Why only we can do this | Score |
|---|---|---|---|---|
| 1 | At-risk member detection | `skool members at-risk --weeks 4 --json` | Requires snapshot of points-per-week per member to compute trend; live API has only current state | 9/10 |
| 2 | Leaderboard delta over time | `skool leaderboard delta --weeks 4` | Same — needs historical leaderboard snapshots in SQLite | 9/10 |
| 3 | Post velocity ranker | `skool posts trending --window 24h` | Requires comment/upvote timeline per post; local computation cheap, API can't sort this way | 8/10 |
| 4 | Cross-community SQL view | `skool sql --cross "SELECT community, COUNT(*) FROM posts WHERE ..."` | Owns multi-community sync; no incumbent does this | 8/10 |
| 5 | Calendar export to ICS | `skool calendar export --ics > btd.ics` | No OSS solution exists; clean Google Cal integration | 7/10 |
| 6 | Digest since timestamp | `skool digest since 24h` | Time-windowed aggregate across posts + comments + new members + new lessons | 8/10 |
| 7 | Churn-cohort report | `skool members churn-cohort --signal points-drop` | Cohort SQL across snapshot tables; impossible without local store | 7/10 |
| 8 | Member engagement profile | `skool members engagement <id>` | Joins member × posts × comments × leaderboard snapshot — answers "who is X" in one call | 7/10 |
| 9 | Top-of-mind reply queue | `skool replies pending` | Posts authored by community owner with no admin reply > N hours; surfaces moderation backlog | 6/10 |
| 10 | Mass-DM helper (read-only audit) | `skool members audit-dm --segment ...` | Filters target list using local SQL before invoking `posts comment` (no auto-send; agent in the loop) | 6/10 |

**10 transcendence features, scored ≥6/10.** Themes:
- **Local state that compounds** — at-risk, leaderboard delta, churn cohort, post velocity, digest, engagement profile (6 features)
- **Cross-community ops** — SQL view, mass-DM audit (2)
- **Agent-native plumbing** — calendar to ICS, replies pending (2)

## Stubs (intentional, marked v0.2)

| Feature | Status | Reason |
|---|---|---|
| Calendar event create | (stub) | No verified write path in any wrapper; pending browser-sniff of an actual create flow |
| Chat write | (stub — declined) | Out of scope; chat reads only |

## Auth shape
- Single cookie session: `auth_token` JWT
- Config: TOML at `~/.config/skool-pp-cli/config.toml` with `auth_token = "..."`
- `auth set-token` writes the file; `--from-stdin` for non-interactive
- `auth.no-auth: false` for everything except `auth set-token` and `doctor --offline`

## Note on novel-features subagent
The skill prescribes a Task subagent for novel-feature brainstorming (Step 1.5c.5). The features above were derived inline from the Phase 1 research brief, the 12-endpoint discovery probe, and the user's stated audience profile (Cadence/BTD-shape paid community owners). All 10 transcendence rows trace directly to capabilities the absorbed tools cannot match (snapshot tables, FTS, cross-community SQL, ICS export). The user gate below is the explicit checkpoint.
