# HotelTonight Novel-Features Brainstorm (audit trail)

> Full output of the Phase 1.5c.5 novel-features subagent (customer model, candidates, adversarial cut). Source of truth for the transcendence rows in the absorb manifest.

## Customer model

**Dana — "I'm here now, where's cheap tonight" road-tripper.** Decides where to sleep ~4pm daily; app shows only deals at her exact pin with no price memory. Frustration: can never tell if a deal is actually cheap or just cheap-looking, because the app erases yesterday's prices.

**Marcus — deal-hunter who wants alerts, not refreshing.** Flexible dates, picky price, wants "ping me when downtown dips below $150." App has no watchlist/alert; he reopens it constantly and misses drops (including the Daily Drop) while away. Wants a cron job, not a habit.

**Priya — trip-planner optimizing date and place.** "Tonight, tomorrow, or Saturday? Which of four neighborhoods?" Runs the same search five times changing date/tab, holds five result sets in her head. No single comparison view.

**Sam — agent/automation builder.** No public API/SDK/MCP exists for HotelTonight; "have an agent watch a city for price drops" is currently unbuildable. Wants an offline, SQL-queryable, MCP-exposed time-series of deals.

## Survivors (transcendence table)

| # | Feature | Command | Score | Buildability | How It Works | Persona |
|---|---------|---------|-------|--------------|--------------|---------|
| 1 | Price-drop watch | `watch --lat --lng --when tonight --below 150` | 9/10 | hand-code | Snapshots `/v6/inventory` into `price_snapshots`, diffs against prior observations for same (geo, check-in), flags rooms now below threshold or dropped vs last seen | Marcus |
| 2 | Price history | `history <hotel> --days 30` | 8/10 | hand-code | Reads `price_snapshots` time-series for one hotel from SQLite (no live call); renders price/%off over the window | Dana, Marcus |
| 3 | Is-this-a-deal verdict | `verdict <hotel>` | 8/10 | hand-code | Computes the hotel's own observed low/median/high from stored snapshots, classifies current quote into cheap/typical/expensive — mechanical percentiles, no LLM | Dana |
| 4 | Neighborhood compare | `compare-neighborhoods --metro <id> --when tonight` | 7/10 | hand-code | Groups live inventory by `hotel.neighborhood`, ranks groups by median price / best %off — a local group-by the flat feed doesn't return | Priya |
| 5 | Date-shift scan | `datescan --lat --lng` | 7/10 | hand-code | Fans out real inventory calls for tonight/tomorrow/weekend over same geo, assembles ranked side-by-side comparison | Priya |
| 6 | Daily Drop tracker | `daily-drop --metro <id> [--history]` | 7/10 | hand-code | Filters live inventory on `deal_type=daily_drop` and persists it; `--history` reads the longitudinal record | Marcus |

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Deal-type filter/breakdown (`deals --type`) | Just a filter flag on base search; no standalone leverage | Daily Drop tracker |
| Best-%off leaderboard (`top-discounts`) | Subset of base search + sort flag + metro scope; redundant once watch + daily-drop exist | Price-drop watch |
| "Should I wait?" forecast (`forecast`) | Predictive modeling on a noisy ephemeral feed is unverifiable speculation, no dogfood ground truth | Is-this-a-deal verdict |
| Vibe / sentiment label (`vibe`) | LLM-dependent summarization; mechanical version is just printing the raw field | (print `why_we_like_it` in base search) |
| Map/route export (`nearby-route`) | Needs external routing service not in spec; scope creep into a routing app | Date-shift scan |
| Sold-out / scarcity alert (`scarcity`) | Lowest weekly use, weakest signal, needs dense polling the geo-gated feed can't supply; duplicates watch's diff engine | Price-drop watch |

## Base / parity commands (ship but not novel)
- `search` (inventory by lat/lng+dates), `markets list/get/nearby` — spec-emitted endpoint mirrors.
- `sync`, `search`(FTS), `sql` — framework commands populating/querying the local store.
