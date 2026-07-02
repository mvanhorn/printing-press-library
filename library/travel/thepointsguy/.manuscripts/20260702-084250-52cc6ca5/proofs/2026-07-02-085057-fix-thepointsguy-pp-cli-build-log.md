Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

# The Points Guy CLI — Build Log

## Engine (internal/tpg)
- Algolia search (runtime credential discovery from site bundle), content + suggestions indexes.
- Monthly valuations parser (HTML tables → program/type/cpp, 34 programs verified).
- Credit-card detail extractor from __NEXT_DATA__ PdpPage (fees, APRs, welcome bonus, rewards, rating).
- Card sitemap slugs (178), category-page card slugs, article sitemaps by category.
- RSS feed parser (ISO-8601 dates), page metadata/article-body extractor.
- Adaptive rate limiting + typed RateLimitError. Offline package tests (parseValuationsHTML fixture, etc.).

## Priority 0/1 (absorbed, all built)
- search (Algolia + offline FTS fallback), suggest, latest, since (window), read, browse,
  cards (get/list/best/compare + raw), valuations, glossary, sync, doctor.

## Priority 2 (transcendence, all built)
- worth, redeem-check, portfolio (points math over valuations)
- cards compare, valuations drift, since (local state)

## Notes
- No auth, no writes, standard_http runtime. Respectful rate limiting on sync.
- valuations drift accumulates monthly snapshots in SQLite; first run shows one month.
- search credentials discovered at runtime (no key value in source).
