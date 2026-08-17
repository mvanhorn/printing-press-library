# TicketData Novel-Features Brainstorm (subagent audit trail)

## Customer model

### Maya - the fan waiting for a dip
Today: opens the TicketData event page every day or two, eyeballs get-in price + one-line forecast, flips to StubHub. Can't answer "is $186 actually low, or just today's number?"
Weekly ritual: check the price on the 1-2 shows she'll attend, decide buy-now vs wait.
Frustration: single forecast number + chart give no historical context (low, percentile, cheapest weekday).

### Devon - the reseller watching floors
Today: watches a dozen+ events, clicks each event page each morning, hand-updates a spreadsheet, hunting for floors that dropped hard (buy) or spiked (list).
Weekly ritual: daily sweep of tracked events sorted by biggest overnight move.
Frustration: no board, no "what changed since yesterday" — N tabs + manual diffing.

### Carlos - the season-long sports price-watcher
Today: follows his team's home slate, checks a few games' pages; comparing "which home game is cheapest" or "which zone is the best deal" means opening every page/zone chart by hand.
Weekly ritual: weekly scan of upcoming home games to pick game + section.
Frustration: no cross-game comparison, no per-zone deal ranking vs history.

### Priya - the sports-business journalist
Today: writing about ticket-price trends; needs raw time series, historical lows, volatility, multi-event comparison. Site gives a chart image, not the 791 points.
Weekly ritual: 1-2 data-backed claims/week ("floor fell X% two weeks after on-sale across five shows").
Frustration: data trapped behind per-event charts; can't get raw series or compare across events.

## Survivors (transcendence, >=5/10)

| # | Feature | Command | Score | Buildability | Persona | Long Description |
|---|---------|---------|-------|--------------|---------|-----------------|
| A | Watchlist management | `watch add/list/rm` | 8/10 | hand-code | all | none |
| B | Watchlist price board | `board` | 8/10 | hand-code | Devon, Maya | Use `board` for a current snapshot of the whole watchlist. For what CHANGED since your last sync or for price-target alerts use `drift`; for one event's historical distribution use `stats`. |
| C | Drift + target alerts | `drift` | 9/10 | hand-code | Devon, Maya | Use `drift` for what moved since the last sync and for price-target alerts. For a full current snapshot use `board`; for one event's history use `stats`. |
| D | History stats + best-time-to-buy | `stats <id>` | 9/10 | hand-code | Maya, Priya | Use `stats` for one event's price distribution and best day to buy. To compare multiple events use `compare`; for the whole-watchlist snapshot use `board`. |
| E | Cross-event comparison | `compare <ids...>` / `--performer` | 8/10 | hand-code | Carlos, Priya, Devon | Use `compare` to rank multiple watched events or one performer's events head-to-head. For a single event's own history use `stats`; for the full watchlist snapshot use `board`. |
| F | Best-zone / section opportunity | `zones <id>` | 7/10 | hand-code | Carlos, Devon | Use `zones` to rank an event's zones by price and by opportunity vs their history. For the plain section name catalog use `events sections`. |
| G | Offline multi-result search | `search "<q>" --type <events\|performers\|venues>` | 9/10 | spec-emits | all | Use `search` to browse multiple offline matches. For the single canonical resolve of a name to its stats use `performers search` / `venues search`. |

Hand-code count: A, B, C, D, E, F (6). spec-emits: G (1).

## Killed candidates
| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| `performer floor` | same as ranking a performer's watched events by price | `compare --performer` |
| `venue prices` | same as ranking watched events at a venue | `compare`/`board` |
| `movers` | N-day-move ranking is `board` sorted by change + `drift` | `board` |
| `forecast score` | verifiability failure, needs long accumulation, can't dogfood | `stats` |
| `alert --target` | duplicate of drift's alerting | `drift` |
| `export --csv` | covered by global --csv/--json/--select | `stats`/`events history` |
| `spark` | cosmetic; numeric distribution carries the decision | `stats` |
