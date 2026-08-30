# Flighty Airports Browser-Sniff Discovery Report

## 1. User Goal Flow
- **Goal:** "Check the live status of any airport — delays, weather, arrivals/departures, performance — from the terminal, without opening the website."
- Steps completed:
  1. Probed `https://flighty.com/airports` — `standard_http` (200 via stdlib + Surf), no WAF/clearance.
  2. Studied homepage (meltdown map) in Chrome DevTools MCP — airport grid of 156 airports across 8 regions, per-airport status + delay stats.
  3. Captured raw SSR HTML for homepage, TV mode, DEN airport page, JFK airport page, DEN arrivals board, DEN departures board.
  4. Extracted Next.js RSC flight payloads (`self.__next_f.push`) from each page — the complete embedded data contract.
  5. Opened the search dialog (⌘K) — client-side filter over embedded catalog, no separate network call.
  6. Verified no XHR/API calls in browser console — the site is pure SSR.
- Steps skipped: none (read-only site; no auth required).
- Secondary flows: TV mode page (same regions payload), arrivals/departures boards.

## 2. Pages & Interactions
| URL | Purpose | Data captured |
|---|---|---|
| `https://flighty.com/airports` | Meltdown map homepage | 156 airports, 8 regions, per-airport `status` + `arrival`/`departure` delay stats + `warnings` + `cumulativeDelay` |
| `https://flighty.com/airports/tv` | Disrupted airports dashboard | Same regions payload (all-airport catalog) |
| `https://flighty.com/airports/denver-intl-den` | Airport detail page | Full airport object: `iata`, `name`, `city`, `country`, `timezone`, `airportWeather`, `airportId`, `slug`, `shareLink`; `today.departurePerformance` + `arrivalPerformance` with `onTime`/`delayed`/`canceled`/`diverted`, `disruptedRoutes[]`, `disruptedAirlines[]`, `mostDisruptedRoutesAmount` |
| `https://flighty.com/airports/john-f-kennedy-jfk` | Airport detail page | Confirms identical structure with arrival + departure performance |
| `https://flighty.com/airports/denver-intl-den/arrivals` | Arrivals board | `initialFlights[]`: `id`, `city`, `status[]`, `originalTime`, `newTime`, `secondaryCorner` (belt), `airline`, `flightNumber`, `departure`/`arrival` (iata, terminal, gate, belt, flag) |
| `https://flighty.com/airports/denver-intl-den/departures` | Departures board | Same `initialFlights[]` shape with departure-side fields |

Interaction: clicked search button / ⌘K → dialog opens; Escape closes. No network calls fired.

## 3. Browser-Sniff Configuration
- Backend: **chrome-devtools MCP** (Chrome extension, user's real Chrome) + raw HTTP `fetch-docs`/`curl` captures for SSR HTML.
- `probe-reachability` result: **`standard_http`** (confidence 0.95) — runtime needs no Surf, no clearance cookie. Printed CLI will use plain HTTP.
- Proxy pattern: **not detected** (no proxy envelope; site is SSR HTML).
- Pacing: N/A (no API calls; 6 raw page fetches).

## 4. Endpoints Discovered
| Method | Path | Status | Content-Type | Auth |
|---|---|---|---|---|
| GET | `/airports` | 200 | text/html (SSR + RSC) | public |
| GET | `/airports/tv` | 200 | text/html (SSR + RSC) | public |
| GET | `/airports/{slug}-{iata}` | 200 | text/html (SSR + RSC) | public |
| GET | `/airports/{slug}-{iata}/arrivals` | 200 | text/html (SSR + RSC) | public |
| GET | `/airports/{slug}-{iata}/departures` | 200 | text/html (SSR + RSC) | public |
| GET | `/airports/{slug}-{iata}/tv` | 200 | text/html (SSR + RSC) | public |
| GET | `/airports/image/*` | 200 | images | public |
| GET | `/airports/svg/*` | 200 | svg | public |

No XHR/fetch API endpoints. All data is embedded in the Next.js RSC flight payload (`self.__next_f.push([1,"..."])`) inside the SSR HTML.

## 5. Traffic Analysis
- **Protocol:** `ssr_embedded_data` (Next.js App Router RSC flight chunks). Confidence: high.
- **Auth signals:** none — fully public, no cookies, no tokens, no headers required.
- **Parameter-name evidence:** URL path slugs are the only params: `{slug}-{iata}` (e.g. `denver-intl-den`). Region filter + search are client-side.
- **Protection signals:** none (Vercel served, no challenge).
- **Generation hints:** `requires_js_rendering` NOT required — data is in initial HTML; a plain HTML parser can extract everything. `response_format: html` with embedded-JSON extraction is the right shape.
- **Candidate commands:** `airports list`, `airports show <iata>`, `airports boards <iata> --arrivals/--departures`, `airports top --region`, `airports status <iata>`, `airports search`.

## 6. Coverage Analysis
- Resource types exercised: airport catalog (homepage/tv), airport detail (DEN, JFK), flight boards (DEN arrivals/departures).
- Likely missed: per-airport `/tv` (identical board payload — low risk), region-specific filtering (client-side over same catalog).
- The 156-airport catalog is the same across homepage + TV mode; airport detail pages add weather + performance; boards add flights. **Complete coverage.**

## 7. Response Samples
- **Homepage catalog entry:**
  ```json
  {"id":"0f6792c2-...","slug":"manchester-man","name":"Manchester","iata":"MAN",
   "location":{"latitude":53.36,"longitude":-2.27},"city":"Manchester",
   "status":"MAJOR_ISSUES",
   "arrival":{"current":{"delay":"1h 38m","onTimeValue":0.12,"onTimePercentage":"12%","canceledPercentage":"0%"},
              "today":{"onTimePercentage":"39%","onTimeValue":0.3869,"canceledPercentage":"3%"}},
   "departure":{"current":{"delay":"1h 10m","onTimeValue":0.2381,"onTimePercentage":"24%","canceledPercentage":"0%"},
                "today":{"onTimePercentage":"37%","onTimeValue":0.3669,"canceledPercentage":"2%"}},
   "warnings":[],"cumulativeDelay":4508}
  ```
- **Airport detail weather:**
  ```json
  {"iata":"DEN","name":"Denver Intl.","city":"Denver","country":"United States","timezone":"America/Denver",
   "airportWeather":{"temperature":22.2,"conditionTitle":"Few Clouds","conditionIcon":"few-clouds-day.svg",
    "ceiling":null,"ceilingBucket":4,"windSpeed":5,"windSpeedBucket":0.333,"windGustSpeed":5,"windGustBucket":0.333,
    "windDirectionValue":220,"visibility":10,"visibilityBucket":4,"flightRules":"VFR",
    "flightRulesTitle":"Visual Flight Rules","summaryBody":"","warnings":[],
    "metarLastUpdated":"2026-08-27T14:53:00Z","rawMetar":"METAR KDEN 271453Z 22005KT 10SM FEW080 FEW120 FEW220 22/12 A3026 RMK AO2 SLP178 T0222-122 51007"}}
  ```
- **Airport detail performance:**
  ```json
  {"today":{"departurePerformance":{"numOperations":1057,
    "onTime":{"absolute":911,"percentage":"86%"},"delayed":{"absolute":136,"percentage":"13%"},
    "canceled":{"absolute":10,"percentage":"1%"},"diverted":{"absolute":0,"percentage":"0%"},
    "disruptedRoutes":[{"airport":{"id":"...","iata":"LGA","city":"New York","name":"LaGuardia"},
      "delayedPercentage":"100%","canceledPercentage":"0%","divertedPercentage":"0%","delayed":8,"canceled":0,"diverted":0,"total":8}],
    "disruptedAirlines":[{"airline":{"id":"...","iata":"UA","name":"United"},"delayedPercentage":"94%","canceledPercentage":"6%","divertedPercentage":"0%","delayed":91,"canceled":6,"diverted":0,"total":97}]}}}
  ```
- **Flight board entry:**
  ```json
  {"id":"ea368a0b-...","city":"Riverton",
   "status":[{"type":"icon","icon":"BULLET","style":"RED"},{"type":"text","text":"4h 1m Late","style":"RED"}],
   "originalTime":{"text":"06:36","style":"GRAY_STRIKETHROUGH"},"newTime":{"text":"10:37","style":"RED"},
   "secondaryCorner":"Belt 18","airline":{"id":"...","iata":"UA","name":"United"},
   "flightNumber":"5072",
   "departure":{"iata":"RIW","terminal":"","gate":"","flag":"/airports/svg/flag/US.svg"},
   "arrival":{"iata":"DEN","terminal":"Main","gate":"B11","belt":"18","flag":"/airports/svg/flag/US.svg"}}
  ```

## 8. Rate Limiting Events
- None observed. 6 page fetches, all 200, no 429s.

## 9. Authentication Context
- No authenticated session used. Site is fully public; no login wall for any captured surface.

## 10. Bundle Extraction
- Not run (not needed): browser-sniff discovered the complete surface (catalog + detail + boards). The Next.js chunks are build-hashed; the data contract lives in SSR HTML, not JS bundles.
