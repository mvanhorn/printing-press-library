# TicketData Browser-Sniff Discovery Report

**Target:** https://www.ticketdata.com/ (consumer ticket price-tracker)
**Capture method:** Claude-in-Chrome MCP (real Chrome session, passed Cloudflare)
**Data API host:** `https://data.ticketdata.com/api`
**Auth:** none required for the full public surface (price-history returns `auth_required: false`, all 791 points anonymously)
**Transport:** `standard_http` (probe-reachability: stdlib 200, surf-chrome 200, confidence 0.95). Cloudflare challenge is intermittent on browser-UA curl but Go stdlib clears it. Ship plain HTTP; keep Surf/Chrome-fingerprint as fallback.
**Frontend:** Next.js App Router on Vercel. Browse/homepage tables are RSC server-rendered (`?_rsc=`), so there is no public JSON *browse/list* endpoint — the replayable surface is the per-entity data API below.

## Site URL structure (for slug/id inputs)
- `/events/{id}` — event detail (numeric id, e.g. 22323960, 855396)
- `/performer/{slug}` — performer page (e.g. ariana-grande, world-cup-soccer)
- `/venue/{slug}` — venue page (e.g. state-farm-arena, lumen-field)
- `/directory/{category}` — category browse (nfl-football, mlb-baseball, mls) — RSC only, no JSON API

## Confirmed data API endpoints (all GET, host `data.ticketdata.com`, no auth)

### 1. `GET /api/events/{id}` — event detail
```
data.found (bool)
data.event { id, title, datetime{local,utc}, event_timezone }
data.performer { id, name, slug, performer_away }
data.venue { id, name, slug, location{city,state,country} }
data.category { id, name, type }
data.tickets { get_in_price, max_price, tenth_percentile_price,
               total_quantity, max_total_quantity, number_of_listings }
data.three_day_price_change { raw, percent, direction, insufficient_data }
data.price_trend { three_day_change_percentage, direction, insufficient_data, last_updated }
data.forecast_value, forecast_is_available, forecast_layover_text, forecast_hover_text
data.urls { vivid, stubhub, primary }        # marketplace buy links
data.metadata { stubhub_event_id, inserted_at, timestamps{...} }
```

### 2. `GET /api/events/{id}/price-history` — get-in-price time series
```
data.found, data.auth_required (=false anonymously)
data.data [ { id, get_in_price, inserted_at, timezone } ]   # 791 points for the sampled event
data.total_records, data.current_get_in_price
data.event_stats_last_updated_at (+ _timezone / _utc)
data.onsale_datetime {utc,local,timezone}, data.presale_datetime {...}
data.zones [ { zone, data[ {...} ], is_custom } ]           # per-zone series (761 pts/zone)
data.zone_management { should_show_zones, can_create_custom_zones,
                       has_existing_custom_zone, has_user_custom_zones, is_eligible_for... }
```

### 3. `GET /api/events/{id}/zones/sections` — section catalog
```
data.sections [ string ]        # ~80 section names
data.total_sections, data.event_id
```

### 4. `GET /api/performers/{slug}` — performer detail
```
data.found
data.performer { id, name, slug, event_category_type,
  stats { num_events_upcoming, average_get_in_price_upcoming_events,
          min_get_in_price_upcoming, max_get_in_price_upcoming },
  stats_card { type, official_artist_website,
               resale_links{stubhub,vivid,seatgeek,tickpick,gametime},
               social_links{setlistfm}, socials_approved },
  icons { primary{axs,paciolan,seatgeek,ticketmaster,audienceview_ovation,...}, resale, social } }
```

### 5. `GET /api/venues/{slug}` — venue detail
```
data.found
data.venue { id, name, slug, location{city,state}, stats{num_events_upcoming,...} }
```

### 6. `GET /api/performers/search?q={query}` — resolve best-match performer
```
data.found, data.performer (single object), data.message
```

### 7. `GET /api/venues/search?q={query}` — resolve best-match venue
```
data.found, data.venue (single object), data.message
```

## Endpoints that do NOT exist as JSON (RSC/server-only, 404 on data API)
- No `/events` list, `/events/popular`, `/events/trending`, `/browse`, `/homepage/events`
- No `/categories`, `/directory/{cat}` JSON
- No global `/search` autocomplete as a clean GET (returns 400; site uses per-type resolvers 6 & 7)
- `/performers/popular` returns `found:false` — it is `/performers/{slug}` matching slug "popular", not a list

## Replayability verdict: PASS
All 7 endpoints are plain HTTP GET returning normalized JSON, no auth, no cookies, no page-context execution. The printed CLI ships direct HTTP transport. No resident browser needed.
