# Novel Features Brainstorm — Rocket League Tracker CLI

> Source: Phase 1.5 Step 1.5c.5 subagent (general-purpose).
> Customer model + candidates + adversarial cut. Survivors feed the absorb manifest's transcendence table.

## Customer model

**The grinder — "Mason, 22, Champ-1 hardstuck"**

*Today (without this CLI):* After every 2-hour competitive session, Mason alt-tabs to tracker.gg, refreshes his profile, screenshots the 2v2 MMR number, and pastes it into a Notes app where he keeps a running log. He keeps the tracker.gg tab pinned and refreshes every 10-15 minutes mid-session because the "session" widget on the site resets weirdly. He cannot answer "what was my MMR exactly seven days ago at this time?" or "how does my Tuesday-night MMR drift compare to my Saturday-morning MMR?"

*Weekly ritual:* Five evenings a week, queue 2v2, log the start MMR + end MMR + win/loss count by hand, and compare to last week's column in the Notes file.

*Frustration:* The hand-logging. tracker.gg shows him *now*, never *last Tuesday at 9pm*. If he forgets to log one night, that data point is gone forever — the website doesn't persist his history at the granularity he wants.

**The friend-group sweat — "Priya, 28, plays with a 4-stack"**

*Today (without this CLI):* Once a week she copy-pastes five tracker.gg URLs into the group chat with the message "ranking by who tilted most this week." She eyeballs each profile, jots numbers on a napkin, and announces a "leaderboard." When her friend Ben claims he's "basically GC now," she has no rolling evidence — only the snapshot in front of her.

*Weekly ritual:* Sunday-night recap post in the Discord with each friend's playlist deltas and a "most improved" call-out.

*Frustration:* Five separate browser tabs, no diff view, no way to say "who gained the most MMR in 3v3 this week" without manual subtraction. No way to catch Ben overstating his rank — she'd need a record of his MMR from last Sunday, and tracker.gg doesn't give her one she controls.

**The Discord-bot author — "Theo, 31, runs a 120-member casual server"**

*Today (without this CLI):* Maintains a Tusk-style bot that hits an unofficial scraper. It 403s every two months. He has a `register` command that maps Discord IDs to RL accounts and a `!rank` command that hits the scraper. When the scraper dies he scrambles for a replacement and his users yell at him.

*Weekly ritual:* Wake up Saturday, check the bot logs, fix whatever upstream change broke last night's scraper, restart the bot, post in the server "we're back."

*Frustration:* The brittleness. He doesn't want to maintain a scraper — he wants a stable command-line that returns JSON he can pipe into his bot, and a local DB he can query when the upstream is down. The drop-in resilience is what he'd pay for.

**The agentic player coach — "Devi, 25, uses an AI agent for self-review"**

*Today (without this CLI):* Hand-curates a CSV of her last 30 matches' MMR + playlist + W/L, pastes it into Claude with a prompt like "what playlist should I grind tonight, given my recent trends?" The agent gives generic advice because the data shape is rough. She can't ask the agent "given my last 30 days of 2v2 MMR slope and my last 30 days of 3v3 MMR slope, which playlist is converting practice into rank?"

*Weekly ritual:* Sunday-evening review session — pull recent data, paste into agent, get coaching, pick a playlist focus for the week.

*Frustration:* The agent has no clean structured stream. Every Sunday she rebuilds the CSV by hand. She wants `rl-tracker trend devi --days 30 --json` piped straight into the agent, with `--select` to keep token budgets sane.

## Candidates (pre-cut)

(See subagent transcript — 18 candidates across sources (a)/(b)/(c)/(f). Six were cut at this stage as siblings or scope-creep.)

## Survivors and kills

### Survivors (12 features, all ≥5/10)

| # | Feature | Command | Score | Persona |
|---|---------|---------|-------|---------|
| 1 | Daily snapshot delta | `peek <player>` | 9/10 | Mason |
| 2 | Time-series MMR curve | `trend <player> --playlist <k> --days N` | 9/10 | Mason, Devi |
| 3 | Session summary | `session-summary <player>` | 8/10 | Mason |
| 4 | Group ranker | `group <name> [--rank-by mmr-delta-7d \| win-delta-7d \| mvp-delta-7d]` | 8/10 | Priya |
| 5 | Save a friend group | `group save <name> <player>...` / `group list` | 6/10 | Priya, Theo |
| 6 | Agent context blob | `agent-context <player> [--days 30] [--select fields]` | 9/10 | Devi |
| 7 | Promo distance | `promo <player>` | 7/10 | Mason |
| 8 | Tournament fit | `tournament-fit <player>` | 6/10 | Mason |
| 9 | Best queue time | `population-best-time --playlist <k> [--days 7]` | 5/10 | Mason |
| 10 | MMR sparkline | `mmr-curve <player> --playlist <k>` | 6/10 | Mason |
| 11 | Liar check | `liar-check <player> --claimed-rank <tier>` | 5/10 | Priya |
| 12 | Import collector snapshot | `import-collector-snapshot <path>` | 7/10 | Devi |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|--------|-------------|--------------------------|
| `watch-mode` standalone | Scope creep; user can `loop rank --json`; reframe as `--watch` flag | `rank` (absorbed) |
| `shop-watch --notify-on` | Background process + external notification = scope creep | `shop` (absorbed) |
| `improver-of-the-week` | Sibling: same query as `group --rank-by mmr-delta-7d` | `group` (#4) |
| `reconcile <player>` | Verifiability poor; use case is CLI debugging | `peek` (#1) |
| `link --primary-id` | Already absorbed (#10 of absorb table) | absorb #10 |
| `session-window --since` | Subset of `session-summary` logic | `session-summary` (#3) |
