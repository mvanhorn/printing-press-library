# Series-summary query family

"Summarize Kalshi series $SERIES_TICKER" — give me the state of a whole series (active events, top markets by volume, current odds across the family). This shape doesn't exist on Polymarket — Polymarket uses event slugs, not series tickers — so this playbook is Kalshi-only.

## Why this isn't `series-summary get-by-id`

prediction-goat ships a `series-summary` command group that wraps Kalshi's `/series-summary/{id}` endpoint, but that endpoint returns a thin metadata payload (series title, start/end dates, status) without child events or live prices. It is **not** the answer to "summarize the series" — it's a metadata lookup. The right path is the three-call walk: series-search -> events list -> events get-with-markets.

## The canonical three-call walk

```
prediction-goat-pp-cli kalshi-series-search '<substring>' --agent
prediction-goat-pp-cli kalshi events list --series <series-ticker> --agent --limit 25
prediction-goat-pp-cli kalshi events get <event-ticker> --with-markets --agent
```

Each step has a quirk worth knowing.

### `kalshi-series-search`

Local-only substring grep over `kalshi_series` rows. Skips the upstream `/series` paginated endpoint because Kalshi's series catalog is small (~500 rows total) and fits in the local store. **Requires a prior `prediction-goat-pp-cli sync`** — on a fresh install this returns `count:0` even for KXMENWORLDCUP. The 2026-05-22 dogfood plan added this as Finding 14's fix; before that landed, agents hit `topic` for series discovery and burned 3-5 calls grepping FTS results.

If the user typed the full ticker (extractor classifies it under `query.tickers`), skip this step.

### `kalshi events list --series`

The `--series` flag **forces `--data-source live`** because the local store doesn't index events by `series_ticker`. There's no way to satisfy this from cache. Two flags matter:

- `--limit 25`: response gets large for top-of-the-cycle series (World Cup has ~50 events when counting the team-level breakdown). 25 keeps it ~5KB.
- `--status open`: defaults to all statuses. Pass `--status open` if the user wants live action only.

### `kalshi events get --with-markets`

The `--with-markets` flag passes `with_nested_markets=true` to the `/events/{ticker}` upstream endpoint. The response includes a `markets` array with child markets pre-populated — saves a follow-up call per child. **Critical**: the upstream endpoint omits `markets` from the response when `with_nested_markets` is absent OR false; if you forget the flag, you get just the event shell.

## Untraded-flag check (Kalshi list endpoint gotcha)

The Kalshi list endpoint (`/markets?series_ticker=...`) **omits price fields** from its payload by design — it's the performance-optimized list shape. The detail endpoint `/markets/{ticker}` includes them. prediction-goat's sync issues a detail GET for every active high-volume market (volume_fp > 1000) and overwrites the stored JSON with the priced payload. Long-tail markets stay list-shape and surface with null `yes_ask_dollars` from local cache.

When you hit a market with `yes_ask=0.17`, `no_ask=1.00`, `volume_24h=0`, `last_price=0` — that's the platform-default listing, not a real implied probability. The CLI sets `untraded: true` at render time and the text-mode YES column shows `untraded`. **Never quote 17c as the odds** for an untraded market; mark it explicitly as "no live trading" in the summary.

The formal check is: `last_price_dollars == 0 && volume_24h_fp == 0 && (yes_ask_dollars + no_ask_dollars) - 1.0 > 0.10`. Helper lives at `isUntradedKalshi` in `internal/cli/helpers.go`.

## Status field values

The events list endpoint's `status` field has these values, with these implications for the summary:

- `open` — accepting trades. Include in the summary's active count.
- `closed` — accepting trades but final-resolution-pending. Include.
- `settled` — resolution finalized, no further trading. Skip from "active" count; surface separately if the user asked for historical results.
- `unopened` — listed but not yet open. Skip from default active count; include only if the user asked about upcoming events.

The `--active-only` flag on `topic` does the equivalent filter on the kalshi_events JOIN side; events list doesn't have that flag (the upstream API doesn't expose it as a query param) so the agent must filter client-side.

## Series-ticker conventions Kalshi follows

Series tickers are 5-15 character uppercase. Common stems:

- `KXMENWORLDCUP` / `KXWOMENWORLDCUP` — FIFA World Cup
- `KXNBAWEST` / `KXNBAEAST` — NBA conference standings
- `KXNFLSB` — NFL Super Bowl
- `KXPRES<YY>` — US Presidential election (e.g., KXPRES24)
- `KXSUPERBOWL` (alternate stem for some seasons)
- `KXBTC` / `KXETH` — crypto price brackets

Event tickers append the cycle suffix: `KXMENWORLDCUP-26`, `KXPRES24-DJT`. Market tickers append the outcome: `KXMENWORLDCUP-26-USA`.

## When summary is "the series is dead"

If `events list --series <ticker>` returns count 0 OR every event is `settled`, the summary should explicitly say so — don't try to fetch markets under a fully-resolved series; the response is misleading. The format the agent should emit:

```
KXMENWORLDCUP-22 ("2022 FIFA World Cup"): settled. Winner: Argentina (KXMENWORLDCUP-22-ARG resolved YES at 100c).
```

## Token-budget tip

If the user asked for a quick summary, cap the third call:

```
prediction-goat-pp-cli kalshi events get <event-ticker> --with-markets --agent \
  --select 'meta,results.markets.ticker,results.markets.yes_sub_title,results.markets.yes_ask_dollars,results.markets.volume_24h_fp'
```

Drops every nested object field that doesn't drive the summary. Response shrinks from ~8KB to ~1KB for a 48-team event.

## When to amend this playbook

If a series ticker convention ever changes (Kalshi has historically renamed series between cycles — KXPRES24 -> KXPRES28 — and the agent's mental model has to track that), `playbook amend` the example tickers to current-cycle ones. Same if Kalshi adds a `--with-markets` equivalent to the events list endpoint that lets us collapse to two calls instead of three.
