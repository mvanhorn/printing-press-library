# TicketData CLI Build Log

Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

## Built (Priority 0/1 auto-generated)
- events get / history / sections, performers get / search, venues get / search (7 spec endpoints)
- data layer: generic resources store + FTS; custom td_* tables for watchlist/snapshots/price series

## Built (Priority 2 transcendence, hand-code via Codex)
- watch (add/list/rm) — local watchlist (watchlist-driven store; no list endpoint upstream)
- sync — re-fetch watched events, append snapshots + full price series + per-zone series
- board — whole-watchlist table: get-in, N-day change, forecast dir, history percentile
- drift — snapshot diff + --threshold + --target price alerts
- stats <id> — low/high/median/percentile/volatility/best-weekday from local series
- compare <ids...>/--performer — rank events by get-in + %change
- zones <id> — rank zones by price + pct-above-own-low
- search "<q>" --type — offline multi-result FTS

## Fixes applied during build
- numeric event/performer/venue ids parsed as json.Number (were string) — watch add failed otherwise
- zone points use field `zone_get_in_price` (not `get_in_price`) — zones showed 0 prices otherwise

## Deferred / notes
- No list/browse endpoint upstream, so store is watchlist-driven (sync re-fetches watched ids).
- Transport: browser-chrome (Surf) — Cloudflare intermittently 403s non-browser clients.
