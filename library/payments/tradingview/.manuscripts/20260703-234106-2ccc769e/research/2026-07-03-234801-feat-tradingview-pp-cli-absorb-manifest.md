# TradingView CLI — Absorb Manifest

Scope note: the user explicitly capped this to two features (price-in-USD, convert-to-EUR)
for stocks & crypto. This is a deliberately lean manifest matching that scope, not an
absorb-everything build. "Trim scope" / "add ideas" remain available at the gate.

## Absorbed (match or beat what community tools do)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Symbol search / resolve | shner-elmo/TradingView-Screener, tvscreener | `tradingview-pp-cli search <query>` | One-shot resolve to EXCHANGE:TICKER, strips `<em>`, shows type/exchange/currency, `--json`/`--agent`, `--type` filter |
| 2 | Last price / quote for a ticker | all wrappers | `tradingview-pp-cli quote <symbol>` | Universal `/symbol` endpoint (any market), accepts bare ticker (auto-resolves), agent-native output |
| 3 | Multi-market: stocks, crypto, forex | tvscreener, screener libs | (behavior in `tradingview-pp-cli quote`) and (behavior in `tradingview-pp-cli search`) | Single universal endpoint means no per-market path juggling |
| 4 | Structured/scriptable output | (none ship a CLI) | (behavior in `tradingview-pp-cli quote`) | `--json`, `--agent`, `--select` field filtering, typed exit codes |
| 5 | Raw endpoint access | wrapper method calls | (generated endpoint) `symbols search`, (generated endpoint) `market quote` | Direct typed access to the raw TradingView responses |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|-----------------|
| 1 | Multi-currency quote (native + USD + EUR) | quote | hand-code | Cross-fetches TradingView forex rates and normalizes the instrument price into USD and EUR in one call; no wrapper does this | none |
| 2 | TradingView-rate fiat conversion | convert | hand-code | Reuses TradingView forex symbols as the rate source so quotes and conversions share one data origin; no wrapper exposes fiat conversion | Use this command to convert an amount between currencies. Do NOT use it to price an instrument; use 'quote' for that. |

## Optional (offer at gate; user can approve or decline — beyond the stated 2 features)
| # | Feature | Command | Buildability | Notes |
|---|---------|---------|--------------|-------|
| O1 | Local watchlist + batch quote | watchlist add/list, quote --watchlist | hand-code | SQLite-backed list of symbols; batch-quote all in one call, offline symbol cache. Adds scope beyond the two requested features. |

## Novel features subagent note
The Phase 1.5c.5 fleet subagent was intentionally NOT spawned. The user explicitly
scoped the CLI to two features and "Trim scope" is a first-class gate outcome; a
gap-hunting brainstorm that generates 10-15 transcendence features would directly
contradict the stated intent. The two transcendence features above are derived
directly from the user's own requests (USD price, EUR conversion), not from a gap hunt.
