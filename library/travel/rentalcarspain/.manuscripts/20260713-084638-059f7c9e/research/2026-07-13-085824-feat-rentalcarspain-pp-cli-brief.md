# Rental Car Spain CLI Brief

## API Identity
- **Domain:** Car rental price search for Spain, focused on Málaga (AGP). Aggregator-first, with a direct-supplier cross-check.
- **Users:** A traveler who repeatedly rents in Málaga, watches DoYouSpain for the cheapest supplier, then confirms the price directly with the company — always booking full insurance (zero excess).
- **Data profile:** Rental offers (supplier, car group/class, per-day + total price, base vs full-insurance tier, deposit/excess, dates, location). Time-series price snapshots for the same search enable drift/watch.

## Sources (confirmed priority)
- **Primary — DoYouSpain** (`www.doyouspain.com`): aggregator that returns the cheapest suppliers in an area. Headline search command.
- **Direct cross-check — Delpaso** (`www.delpasocarhire.com`): Málaga-only supplier, live quote.
- **Record Go / Wiber:** covered *through* DoYouSpain (they appear as suppliers `RBC`/Wiber in results). No standalone live client — see Reachability Risk.

## Reachability Risk
- **DoYouSpain — None.** Full flow verified replayable with plain HTTP. WAF quirk: **must use a non-browser User-Agent** (`curl/…`); spoofing a Chrome UA over a non-browser TLS stack returns HTTP 406. No cookie/CSRF needed for search or autocomplete.
- **Delpaso — Low.** Verified end-to-end with curl. Needs Laravel `_token` parsed from homepage + `delpaso_session`/`XSRF-TOKEN` cookies carried per session. No login. Prices already include total coverage / no excess.
- **Record Go — High (geo-block).** `recordrentacar.com` returns Cloudflare 403 from US egress; `recordgo.com`/`.es` refuse TCP. Endpoints could not be inspected from the generation host. Likely reachable from the user's Spain machine, but unbuildable/untestable here. **Decision: aggregator-only** (user-approved).
- **Wiber — High (JS challenge).** Real domain is `wiberrentacar.com` (wiber.es is an unrelated telecom). Whole origin behind a Fastly client challenge; every path returns the same 3038-byte challenge shell. Not plain-HTTP replayable. **Decision: aggregator-only** (user-approved).

## DoYouSpain flow (verified 2026-07-13)
1. **Autocomplete:** `POST /do2/ajax/autocomplete`, body `idioma=EN&destino=<query>&origen=&experimento=[CAR][M]`. Returns HTML `<li>` fragments carrying `data-destino`, `data-pais`, `data-iata`, `data-destino-description`. Málaga Airport (AGP) = **`MAL02`**; City `MAL01`; Train `MAL03`; Port `MAL04`. Marbella `MBA01`, Fuengirola `FEN01`, Torremolinos `TRM02`, Benalmádena `BEN01`.
2. **Submit:** `POST /do/list/en` (form-urlencoded): `pais=ES`, `destino=<code>`, `destino_final=<code>`, `fechaRecogida`/`fechaDevolucion`=`dd/mm/yyyy`, `horarecogida`/`minutosrecogida`/`horadevolucion`/`minutosdevolucion`, `fechaRecogidaSelHour`/`fechaDevolucionSelHour`=`HH:MM`, `edad`=driver age (default 35), `chkOneWay=on`, `chkAge=on`, `idioma=en`. Returns ~12 KB shell containing `window.location.replace('/do/list/en?s=<uuid>&b=<uuid>')`.
3. **Results:** `GET /do/list/en?s=<uuid>&b=<uuid>` → ~2.1 MB server-rendered HTML, ~200 offers inline (no async polling). Offer rows carry `data-prv` (supplier code), `data-rent`, `data-order`; prices in CSS classes `price-day-euros`, `pr-euros`, `old-price-*`, `discount-price`. Insurance is a per-offer variant: "Full Insurance" / "Zero Excess" tier vs base rate (204× "Full Insurance", 358× "excess" on a sample page).
- **Supplier codes:** `PAS`=Delpaso, `SXT`=Sixt, `EUK`/`EU2`=Europcar, `GOB`=Goldcar, `OKR1`=OK Mobility, `NIZ1`=Niza, `RBC`=Record Go, plus Firefly/Centauro/Drivalia (to confirm by diffing offer blocks).
- **Sibling engine:** doyouitaly.com + CarJet share the identical Gesmarket backend; French/Portuguese are language paths on doyouspain.com.

## Delpaso flow (verified 2026-07-13)
- Bespoke Laravel app, single location (Málaga) so **no location or age param at search**.
- `GET /` → cookies + `_token`. `POST /offers` (form-urlencoded: `_token`, `pickup_date`/`dropoff_date`=`DD/MM/YYYY`, `pickup_time`/`dropoff_time`=`HH:MM`) → HTML with every car group + price. Coverage strings: "Total coverage", "No excess", "Obligatory insurance"; "Mandatory security deposit (950€) for Premium groups". Age collected later via `birthdate` at `/extras`.

## Data Layer
- **Primary entities:** `offer` (supplier, group/class, per_day, total, base_total, full_insurance_total, deposit, excess, currency), `search` (location code, dates, driver age, timestamp), `snapshot` (search_id → offers at time T for drift/watch), `location` (code, name, iata).
- **Sync cursor:** each search execution writes a timestamped snapshot; drift/watch diff snapshots.
- **FTS/search:** supplier + car-class local filtering; SQL over snapshots for price history.

## Table Stakes (from absorb research)
Per-day AND total price; deposit + excess amounts; full-coverage/insurance filter; fuel policy; mileage limit; supplier/depot rating; free-cancellation filter; prepay-vs-counter; driver-age sensitivity; distance from terminal. (Not all are exposed by DoYouSpain's HTML — build what the source carries: price tiers, supplier, class, deposit/excess where present.)

## Product Thesis
- **Name:** Rental Car Spain (`rentalcarspain-pp-cli`).
- **Why it should exist:** No consumer car-rental price CLI exists anywhere (verified — all "car rental CLI" repos are fleet-management toys). This one turns the user's real manual ritual — watch DoYouSpain, cross-check the supplier, always full insurance — into one command with local price history and drop alerts that AutoSlash does for the US market but nobody does for Spain. Full-insurance pricing is the default, not an afterthought.

## Build Priorities
1. **Data layer** — SQLite for offers/searches/snapshots; the foundation for drift/watch.
2. **`search`** (DoYouSpain, headline) — full-insurance default, `--supplier`/`--class`/`--max-total`/`--base` filters, `--sort cheapest`.
3. **`locations`** — autocomplete resolver (name → DoYouSpain code).
4. **`delpaso`** — live direct quote (Málaga).
5. **`compare`** — the user's ritual as one command: aggregator cheapest vs Delpaso direct.
6. **`watch` / `drift` / `dates`** — AutoSlash-style price tracking, drop alerts (typed exit codes), cheapest-date sweep.
7. **`doctor`** — reachability + WAF-UA sanity check.
