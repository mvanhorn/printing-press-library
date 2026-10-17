# Novel-features brainstorm audit trail (subagent output, condensed)

## Customer model
- Aarav Menon — momentum swing trader; reruns `close > sma50 and sma50 > sma200 and rs >= 70` weekly, can't script/diff the client-side expression language.
- Priya Nair — DIY long-term investor, up to 20 watchlists; no cross-list view, news is per symbol, 5y adjusted history only in chart page.
- Meera Iyer — breadth/sector-rotation tracker; hand-copies %>50/200DMA, A/D, sector strength into a spreadsheet each Friday.
- Karthik Rao — builds a cron/agent morning market note; needs deterministic --json/--select output, offline cache, "what changed" primitive.

## Survivors (all hand-code)
screen 10 | quote 10 | movers 8 | sectors 8 | changes 8 | brief 8 | breadth 7 | history 7

## Killed
signals (reimplements market overview) · rotation (unverifiable methodology) · compare (= quote A B) · peers (= screen --sector --sort) · headlines (thin over news by-symbol/brief) · watchlist pnl (no cost basis in site) · corr (per-symbol chart fetch infra, monthly use) · gapfill (needs 200-symbol history, unverifiable)
