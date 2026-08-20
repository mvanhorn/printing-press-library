# TicketData CLI Live Smoke (Phase 5 Full Dogfood)

- Level: Full Dogfood. Matrix: 101 tests. Passed: 101. Failed: 0. Status: PASS. No API key (public API).
- Verified live against data.ticketdata.com with real events (Ariana Grande 22323960, World Cup 855396, Megan Moroney 65100270).
- Behavioral checks that produced correct real output:
  - events get -> get_in_price 347, forecast, 3-day change, marketplace urls
  - events history -> 791-point series stored locally
  - watch add/list/rm, sync -> watchlist + snapshots + price series + per-zone series populated
  - board -> ranked watchlist with history percentile (Ariana Grande at 1st percentile of its own history)
  - stats 22323960 -> low 333 / high 843 / median 608 / current 347 / percentile 1.0 / volatility 115 / best weekday Monday
  - drift -> snapshot diff with delta/percent/direction + --target alerts
  - compare -> events ranked by get-in price
  - zones -> Floor zone current 1101 / low 1005 / high 2621 / 9.55% above own low
  - search "ariana" -> multi-result FTS over local store
- Gate: PASS.
