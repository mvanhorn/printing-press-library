# Odds-for-team query family

The canonical repeat-query shape on prediction-goat: "odds $TEAM wins $EVENT" where $TEAM is a country, team, or candidate inside a multi-outcome event (World Cup, NBA Finals, US Election, etc.). The Polymarket `/public-search` and Kalshi `topic` paths both rank by FTS5 + light volume signal and routinely **bury or miss** the right market for this shape. The reliable answer is the event-walk recipe.

## Why `topic` is the wrong starting point here

`topic kanye --agent` historically returned the Polymarket series shell with no prices and AND-joined multi-token queries ("kanye west" required both tokens to match). The 2026-05-22 dogfood plan landed `--with-prices` + OR-mode tokenization + `polymarket siblings`, but the **canonical answer for odds-for-team is still the event walk**, not topic — because topic caps at `--limit 100` and the force-include pass only catches whole-word title hits. A team like "Ghana" can rank below 20 high-volume sibling markets and get dropped from the truncated result set.

```
# Don't:
prediction-goat-pp-cli topic 'odds USA wins world cup' --agent
# Returns the World Cup series row but rarely the USA-specific market.
```

## The reliable path: polymarket siblings

```
prediction-goat-pp-cli polymarket siblings will-ghana-win-the-2026-fifa-world-cup --agent
```

`polymarket siblings` walks gamma's `/markets?slug=` -> `events[0].slug` -> `/events?slug=` and returns every sibling market under the parent event. The parent event aggregates **every team's market in one response**. Pick any one team slug you know exists (Ghana, USA, France, Brazil — pick whichever you saw in the input or last conversation) as the seed; the response carries the full set.

Response shape (truncated):

```json
{
  "event": {
    "slug": "2026-fifa-world-cup-winner-595",
    "title": "2026 FIFA World Cup Winner",
    "endDate": "2026-07-20T00:00:00Z",
    "marketCount": 48
  },
  "markets": [
    {"slug": "will-france-win-...", "question": "Will France win...", "yesProbability": 0.179, "yesPercent": 17.9, "volumeNum": 42000000, "endDate": "2026-07-20T00:00:00Z"},
    {"slug": "will-usa-win-...",    "question": "Will USA win...",    "yesProbability": 0.012, "yesPercent": 1.2,  "volumeNum": 34200000, "endDate": "..."}
  ]
}
```

Filter the `markets` array on the question text for `$TEAM` (case-insensitive whole-word match).

## When the seed slug is stale

Polymarket reuses event slugs across cycles but markets close after resolution. If `polymarket siblings <seed>` returns `404 market_not_found`, try another team slug from the same event family. Slug patterns are stable: `will-<country>-win-the-<year>-fifa-world-cup` for World Cup, `will-the-<team>-win-the-<year>-nba-championship` for NBA Finals, `will-<candidate>-win-the-<year>-<office>-election` for elections. If three guesses fail, fall back to topic with expand on:

```
prediction-goat-pp-cli topic '<event name>' --expand --with-prices --agent
```

`--expand` (default true) live-fetches sibling markets for any Polymarket event hit with `marketCount > 2`, so it backfills the same set indirectly through the topic ranker.

## Cross-venue: Kalshi side

Kalshi mirrors most World Cup / election outcomes under series tickers like `KXMENWORLDCUP-26-<COUNTRY_ABBREV>` (KXMENWORLDCUP-26-USA, KXMENWORLDCUP-26-ARG). The series ticker family is **stable across cycles** — KXMENWORLDCUP is the men's World Cup series; year suffix is the event ticker.

The cross-venue check via `compare 'world cup'` is unreliable because Jaccard pairing on title tokens collapses when both sides have multi-outcome shells. Use this instead:

```
prediction-goat-pp-cli kalshi events get KXMENWORLDCUP-26 --with-markets --agent
```

Returns every child market with `ticker`, `yes_sub_title` (the team name, e.g. "USA"), `yes_ask_dollars`, `no_ask_dollars`, `volume_24h_fp`. Filter the `markets[]` array on `yes_sub_title == $TEAM`. This is the cleanest cross-venue path the dogfood-gaps plan landed (Finding 12, U8 helper).

## Untraded-flag check (Kalshi side only)

When you read a Kalshi sibling market on a long-tail team (Finland, Cambodia, etc.), check `untraded: true` before quoting an implied probability. Untraded Kalshi markets carry the platform's default 17c ask with zero volume — the percent column will show `untraded` in text mode and JSON output sets `untraded: true` + omits `yesPercent`. Polymarket doesn't have this failure mode (`volumeNum: 0` is rare on listed siblings).

## Apples-to-apples between venues

Both venues' JSON output now carries `yesPercent` (rounded 0-100, for display) alongside `yesProbability` (0-1, for math). For human-readable answers always pick `yesPercent`. mispriced pairs additionally carry `deltaPercent` so the divergence is in percentage points.

## When to amend this playbook

If a recall hit on this family ever surfaces with a stale Ghana seed slug after a World Cup cycle has fully resolved, `playbook amend` the seed slug to a known-good live one for the next cycle. Same for NBA Finals after a championship resolves and a new series_ticker comes online.
