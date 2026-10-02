# Yamato CLI brief

## API identity and source priority
Yamato Transport first-party public service pages and Japanese date calculator. No official consumer OpenAPI. Plain GET HTML and read-only URL-encoded POST calculator work without credentials; not shipment creation. Primary URL https://www.kuronekoyamato.co.jp/ytc/en/send/services/takkyubin/.

## User research and workflows
Japan travelers moving hotels need an insured, correctly sized bag to arrive before check-in, without confusing next-day TA-Q-BIN with limited-area same-day delivery. Travelers departing airports must distinguish boarding day from shipping deadline and actual counter hours. Round-trip users need both-leg prices, return window and acceptance evidence. The expensive mistakes are late bag dropoff, wrong terminal, assuming any hotel accepts luggage, treating close-of-business as same-day dispatch cutoff, and underpricing a heavy suitcase.
1. Classify dimensions and weight using the greater category; reject >200cm/>30kg or >170cm longest side (100cm for upright-only).
2. Fetch current source postal-code routes, cash/cashless tariffs and estimated arrival or source cutoff dates.
3. Select an airport/terminal using current upstream IDs, source boarding deadlines and roundtrip tariff.
4. Inspect airport counter send/pickup/floor evidence, current map links, and fingerprint-guarded image-only hours.
5. Review distinct same-day service areas, cutoffs and example fees with explicit acceptance unknowns.

## Table stakes and ecosystem
Official UI is the competitor and ground truth. Search found B2 Cloud commercial waybill/printing tools, yamato-printer-mcp-server and third-party private tracking wrappers; those features are out of the explicitly requested read-only consumer travel scope. No suitable public traveler rate/date SDK or competing CLI was found. No community code contributed runtime features; no SDK reachability claims are relied on. Registry has no Yamato entry. No auth or paid key.

## Reachability evidence
200 public primary HTML (59,669 bytes); TakkyubinSmp source POST returns Tokyo 1000005 → Kyoto 6008216 ship 2026-10-02, estimated 2026-10-03, size160 cash3160 Exact current capture is source proof and implementation must read it rather than this prose. Browser verified source UI and direct HTTP replay matched. Native iab unavailable; isolated Chrome native Browser Use succeeded for airport 006 cutoff October6 for boarding October8, one-way size160 cash3680/cashless3674. Earlier isolated Playwright flow also verified postal route before user steering. Native screenshot confirmed Narita2 send 06:30–22:30 image-only hours.

## Product thesis and data layer
Name yamato-pp-cli: source-grounded luggage planning with product distinctions and honest unknowns. Stateless fresh read-through quote and current directory requests. A tiny versioned rule catalogue is useful for parcel sizing; live rates are never stored as current offline quotes. Finite image/PDF-derived schedules are fingerprint checked against live bytes before use, otherwise suppressed as unknown. No account data, private tracking, booking, payment, label or shipment mutation.

## Constraints
Estimated delivery is not guaranteed. Local counter shipping cutoff varies; calendar source includes dates, not all exceptional closures or disruption guarantees. Air-restricted contents can change transport and delay delivery; islands/holidays/weather caveats retained. Hotel acceptance and contents/packaging remain unknown until sender verifies. Discounts listed conditionally, never automatically applied. TA-Q-BIN max weight vs size basis preserved, airport660 included, roundtrip200 source discount included; Kansai return fee caveat preserved.

## Build priorities
quote, parcel, airports, counters, same-day, products. Roundtrip lodging uses LeisureTakkyubinSmp; airport roundtrip uses table2 from KuukouTakkyubinSmp. Only selected Narita/Haneda same-day schedule rows get structured cutoff records; other routes remain source handoff through full directory. Native-browser screenshot/manual semantic transcription is evidence, not an OCR runtime.
