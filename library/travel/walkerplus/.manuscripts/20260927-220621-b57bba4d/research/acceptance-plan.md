# Root acceptance plan

## Source-grounded positive cases
- Kyoto festival listing: 2026-10-03 Kyoto Nantan castle festival ar0726e612292; bread festival at Kamigamo 2026-10-10..11. Every result must have Kyoto source region/category and inclusive query overlap.
- Hokkaido fireworks: ar0101e66092 2026-04-28..10-31; Tomakomai 2026-10-10. Keep weather condition distinct from current cancellation.
- Tokyo ar0313e603640: paid weekday1800/weekend-holiday2100, children-free caveat, reservation required, ends2026-09-30; JSON-LD price None does not override source text.
- Osaka ar0727e612159: 2026-10-11, explicit indoor venue; conditional indoor-on-rain must not inherit this certainty.
- Miyagi ar0204e612432: 2026-10-10..12-27, Monday closures with next-weekday holiday exception; unsupported holiday resolution remains possible.
- Kyoto foliage: displayed mid-November/early-December despite schema Nov11..Dec10; seasonal approximation remains explicit.

## Negative and boundary assertions
Wrong-year query returns [] and coverage context rather than old-edition rollover. Wrong city/category never leaks irrelevant rows. Shared page budget bounds multi-month scans. Events fetched twice dedup by stable source ID. Closures/explicit exclusions eliminate only supported days. Conditional cancellation is not current cancellation. Unknown/mixed prices do not pass --free. Unknown/mixed/conditional indoor does not pass --indoor. Requested constraints use AND. Malformed URLs, dates, ranges, bounds and field names fail predictably. Exact dates use Asia/Tokyo calendar arithmetic across Dec31/Jan1.

## Efficiency and completion
Capture cold/warm search, shortlist and event stdout bytes, HTTP requests, wall elapsed and peak RSS. Search stays listing-only; detail fetches budgeted. Whole-command timeout, per-request timeout, response size, retries, concurrency, cache size and pagination all bounded. Build/test/vet and full shipcheck pass. Required live dogfood marker matches final source hash before promotion; receipts close through21. Root checkout copied from promoted library and builds without go.work/shared configuration changes.
