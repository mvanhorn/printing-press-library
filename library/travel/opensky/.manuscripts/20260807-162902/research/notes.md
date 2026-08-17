# opensky-pp-cli research notes (run 20260807-162902)

## API

OpenSky Network public API, https://opensky-network.org/api. A worldwide
volunteer network of ADS-B receivers; no registration or auth for the v1
endpoints used here. Anonymous access is rate-limited (roughly a handful of
requests per minute on time-window endpoints; states/all is more forgiving).

## Endpoints evaluated

- `GET /states/all` — live state vectors (icao24, callsign, origin country,
  position, baro/geo altitude, velocity, track, vertical rate, on-ground) for
  a bounding box (`lamin/lomin/lamax/lomax`) or a single transponder
  (`icao24`). Verified live: JFK metro box returned ~40 aircraft including
  UAL2679, JBU508, UAL3024, CXK1061 with positions and velocities. KEPT.
- `GET /flights/departure?airport=<ICAO>&begin&end` — flights that departed an
  airport in a window. Verified live: KJFK last hour returned 23-26 flights
  (ASA41, AAL493, ...). KEPT.
- `GET /flights/aircraft?icao24=<hex>&begin&end` — full trajectory for one
  aircraft. Verified live: aa07f9 (UAL3024, EWR departure) returned the full
  flight record. KEPT.
- `GET /flights/arrival?airport=<ICAO>&begin&end` — flights arriving at an
  airport. DROPPED: answered HTTP 404 with an empty array for every airport
  and window tested (KJFK, EGLL, KLAX; 1h and 2h windows, recent and older).
  The upstream endpoint is intermittently empty for extended periods; shipping
  a command that 404s on every live call would fail the verified-end-to-end
  bar. Note: the 404-with-empty-array shape (not a JSON error) is OpenSky's
  documented no-data response, which is why the CLI now normalizes it.
- `GET /routes?origin&destination` — known routes between two airports.
  DROPPED: answered HTTP 400 for every code format tried (ICAO uppercase,
  ICAO lowercase, IATA) via raw curl; the endpoint is currently broken
  upstream. Not worth shipping a command that cannot be verified.

## Key behaviors baked into the spec + patches

1. Anonymous time-window cap: a 2-hour window on /flights/* 404s; a 1-hour
   window works. The CLI defaults begin/end to now-1h..now (patch
   001-opensky-time-window-defaults) so bare invocations work.
2. 404-with-empty-array = empty result (patch 002-opensky-flights-404-empty),
   needed because the API uses it for both "no data" and (apparently) the
   broken arrival endpoint.
3. Windows test isolation (patch 003-opensky-windows-test-isolation): the
   generated test helpers only isolated HOME, which is a no-op on Windows
   (os.UserHomeDir reads USERPROFILE), so the suite leaked into the real user
   dirs. Same fix as the greenhouse/lever prints; five helpers updated.

## Rate limits

Anonymous access is fine for interactive use. Heavy scanning should add an
OpenSky account and the v2 auth header; out of scope here (no-auth bar).

## Regeneration

```bash
cli-printing-press generate --spec opensky-spec.yaml --name opensky \
  --category travel --output ./library/opensky-pp-cli
# then apply .printing-press-patches/001-003 per their instructions
# and restore printer/novel_features/search_terms in .printing-press.json
```
