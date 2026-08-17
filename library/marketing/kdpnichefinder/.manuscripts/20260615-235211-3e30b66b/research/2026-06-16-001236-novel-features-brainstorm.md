# KDP Niche Finder — Novel Features Brainstorm (audit trail)

## Customer model
- **Mara** (low-content book maker): Sunday niche hunt across buckets; frustration = web shows one bucket at a time, no cross-bucket "highest revenue under $9" query, no CSV.
- **Devin** (full-text author validating a niche): inspects competitor field per niche; frustration = no reverse-ASIN/competitor view, no publisher-concentration (whale vs fragmented).
- **Priya** (portfolio publisher): weekly re-check of saved shortlist; frustration = API is point-in-time only, no history/trend, timing is guesswork.

## Survivors (>=5/10) → transcendence rows
1. rank (8) hand-code — opportunity rank across ALL buckets, composite revenue/sales/price, `--select value` mode
2. drift (8) hand-code — revenue rising/fading vs earlier local snapshot (snapshots auto-captured on sync)
3. dupes (7) hand-code — same ASIN appearing in 2+ buckets
4. saturation (7) hand-code — publisher revenue-concentration per bucket (whale vs fragmented)
5. competitors (6) hand-code — reverse-ASIN: same-publisher/price-band books around a focus book
6. export (6) spec-emits — KDP CSV (title/ASIN/price/sales/revenue) via framework --csv
7. keywords (6) hand-code — title-token frequency for KDP backend keyword fields

## Killed
snapshot(→fold into sync), search(thin vs absorbed ?search), value(→rank --select value), asins(→export/competitors), folders rollup(thin), stats(descriptive-only), whatsnew(speculative; buckets stable).
