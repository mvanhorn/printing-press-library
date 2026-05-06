# flightgoat v2 Rebuild Proof

## What changed

Original build led with FlightAware (spec-driven), demoted Google Flights and Kayak.
User corrected: Google Flights > Kayak > FlightAware, and wanted free by default.

## v2 architecture

- **Google Flights**: native Go via `github.com/krisukox/google-flights-api`. No Python, no subprocess, no API key. Falls back to `fli` Python CLI only if Google's abuse-detection rejects the native path.
- **Kayak**: custom parser at `internal/kayak/kayak.go`. Fetches `www.kayak.com/direct/<airport>` with a browser User-Agent and extracts the embedded `"routes":[...]` server-rendered JSON blob. No browser automation, no scraping in the pejorative sense (Kayak serves this data intentionally in the initial HTML).
- **FlightAware**: still present as the tracking layer, now explicitly optional.

## v2 primary commands (registered first, free)

- `flightgoat flights <origin> <dest> <date>` — Google Flights search (native-go)
- `flightgoat dates <origin> <dest>` — cheapest dates (still via fli subprocess, krisukox has no calendar)
- `flightgoat longhaul <airport> --min-hours N` — Kayak-backed nonstop discovery (headline feature)
- `flightgoat explore <airport>` — Kayak nonstop matrix

## Research receipts

- Vetted `punitarani/fli` as the Python reference implementation
- Read `ayushsaraswat.com/writing/reverse-engineering-google-flights/` for the protobuf approach
- Discovered `krisukox/google-flights-api` (Go, MIT, 67 stars, Jan 2025 active) as a pure-Go alternative
- Confirmed no public Kayak API exists
- Investigated `kayak.com/direct/SEA` HTML and found the embedded `"routes":[...]` blob server-side
- Extracted 127 destinations + durations for SEA in one grep of the HTML

## Verified output (v2)

```
$ flightgoat longhaul SEA --min-hours 8
20 nonstop destinations from SEA with flights >= 8.0h (source: kayak-direct)
CODE  CITY                         COUNTRY  DURATION  DISTANCE  FLIGHTS  AIRLINES
SIN   Singapore, Singapore         SG       16h50m    8059 mi   2        SQ
DXB   Dubai, United Arab Emirates  AE       15h55m    7406 mi   1        EK
DOH   Doha, Qatar                  QA       15h25m    7393 mi   2        QR
...
```

```
$ flightgoat flights SEA LHR 2026-06-15 --stops non_stop
5 flights found for SEA -> LHR on 2026-06-15 (source: native-go)
PRICE  DURATION  STOPS  AIRLINES  DEPART            ARRIVE
$826   9h50m     0      VS        2026-06-15 18:20  2026-06-16 12:10
$826   9h20m     0      AS        2026-06-15 21:45  2026-06-16 15:05
...
```

## Retro filed

GH issue: mvanhorn/cli-printing-press#168 — combo CLI priority gap.
