# Event-markets query family

"Give me every market under $EVENT" — World Cup, NBA Draft, NBA Finals, US Election, Eurovision, anything with a parent event aggregating multi-outcome sibling markets. Distinct from `odds_for_team` because the agent wants the full set, not a single team's probability.

## The right flag combo: `--expand --with-prices --limit 100`

The 2026-05-22 dogfood plan moved `topic`'s default behavior to OR-mode tokens + vol-weighted re-rank + `--limit` raised to 100. Three flags matter for this family:

- **`--expand`** (default `true`): when a Polymarket market hit's local data shows `events[0].slug` pointing to an event with `marketCount > 2`, the live API path follows up to N=2 such events per topic call, merging every sibling market into the result. This is the **whole point** for event-markets queries.
- **`--with-prices`** (default `false`): for Kalshi series shells, resolves to the highest-volume active market under that series and inlines `yesProbability` + `yesPercent` + `endDate`. Without this flag, Kalshi series rows show null prices.
- **`--limit 100`**: World Cup has 48 sibling markets; without raising `--limit` the cap truncates legitimate outcomes. Set `truncated: true` is visible in the JSON envelope when the cap actually bit.

```
prediction-goat-pp-cli topic 'world cup' --expand --with-prices --limit 100 --agent
```

## Topic envelope shape (after dogfood-gaps fixes)

The agent should expect this structure under the standard provenance envelope:

```json
{
  "meta": {"source": "live", "synced_at": "..."},
  "results": {
    "topic": "world cup",
    "count": 47,
    "truncated": false,
    "hits": [
      {
        "source": "polymarket",
        "kind": "market",
        "slug": "will-france-win-the-2026-fifa-world-cup",
        "title": "Will France win the 2026 FIFA World Cup?",
        "yesProbability": 0.179,
        "yesPercent": 17.9,
        "volume24h": 1200000,
        "endDate": "2026-07-20T00:00:00Z",
        "expandedFrom": "event:2026-fifa-world-cup-winner-595"
      },
      {
        "source": "kalshi",
        "kind": "market",
        "ticker": "KXMENWORLDCUP-26-USA",
        "title": "Will USA win the World Cup?",
        "yesProbability": 0.013,
        "yesPercent": 1.3,
        "volume24h": 412000,
        "untraded": false
      }
    ]
  }
}
```

Polymarket sibling markets carry `expandedFrom` so the agent can tell they came from the live event walk rather than the local FTS index. The `untraded` field appears on Kalshi market hits only.

## When `--expand` is the wrong primary path

`--expand` only triggers on Polymarket events with `marketCount > 2` AND when at least one sibling already matched FTS. If the FTS index has zero matches (e.g., a small overseas election the local Kalshi sync hasn't indexed yet), `--expand` has nothing to expand from. In that case, seed the walk explicitly:

```
prediction-goat-pp-cli polymarket siblings <any-known-sibling-slug-in-the-event> --agent
```

If you don't know a sibling slug, guess the Polymarket convention: `will-<outcome>-win-the-<year>-<event>`. Three guesses usually find a live slug; if all three 404, fall back to `topic '$EVENT' --polymarket --agent` (`--polymarket` skips Kalshi for narrower output).

## Kalshi-only events

Some event families exist on Kalshi but not Polymarket (US-only elections, esports, niche sports). For those, the same topic call works but `--expand` is a no-op (Kalshi side has no event-walk equivalent). Use:

```
prediction-goat-pp-cli kalshi events get <event-ticker> --with-markets --agent
```

When you know the event ticker. Or to discover the ticker first:

```
prediction-goat-pp-cli kalshi-series-search '<substring>' --agent
prediction-goat-pp-cli kalshi events list --series <series-ticker> --agent
prediction-goat-pp-cli kalshi events get <event-ticker> --with-markets --agent
```

That's the canonical three-call walk on Kalshi: series-search -> events list -> events get-with-markets.

## Closed-market filtering

`polymarket siblings` defaults to filtering out `closed=true` markets. Pass `--include-closed` if you specifically want resolved outcomes (post-event analysis, "who won the 2024 World Cup according to Polymarket?"). topic `--active-only` (default true) does the equivalent on the Kalshi side: suppresses series with no open events. Flip with `--active-only=false` for historical queries.

## Apples-to-apples render rule

Polymarket and Kalshi both ship `yesPercent` (rounded 0-100, for display) alongside `yesProbability` (0-1, for math) in JSON output. Always surface `yesPercent` to the user. Kalshi markets without real trading carry `untraded: true` and the percent column shows `untraded` — don't quote those as live odds.

## Token-budget tip

A World Cup event-walk pulls ~48 sibling markets * ~20 fields per row ~ 10-12KB. Use `--select` to drop fields the user doesn't need:

```
prediction-goat-pp-cli topic 'world cup' --expand --with-prices --limit 100 --agent \
  --select 'results.hits.title,results.hits.yesPercent,results.hits.volume24h,results.hits.endDate'
```

Cuts the response by ~80% with no information loss for the typical "what are the odds" answer.
