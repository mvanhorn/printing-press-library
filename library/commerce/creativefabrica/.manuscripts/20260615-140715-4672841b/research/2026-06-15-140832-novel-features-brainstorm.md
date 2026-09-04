# Creative Fabrica Novel Features Brainstorm (subagent audit trail)

## Customer model
- P1 Cricut/Silhouette crafter ("format hunter") — wants cut files in SVG/PNG/DXF/EPS; format not a first-class facet.
- P2 Print-on-demand seller ("license-safe sourcer") — must verify commercial use (hasPod) + outsideSubscription; bulk sourcing.
- P3 Deal-hunter / budget crafter — wants genuine free + real discount depth (regularPrice vs price).
- P4 Designer-watcher / competitor researcher — wants designer catalog profiled + compared + new-release detection.

## Survivors (>=5/10, all hand-code)
1. File-format filter `search --format svg,dxf,png,eps,pes` (8) — local FTS over tags/name/description; format is not an Algolia facet.
2. Real-discount ranking `deals <query>` (7) — hasPromotions + local (regularPrice-price)/regularPrice ranking.
3. Designer profile stats `designer-stats <id|name>` (8) — local aggregation: type mix, price band, free/POD counts, newest.
4. Designer comparison `designer-compare <A> <B>` (6) — SQLite join, side-by-side stats.
5. New-since-snapshot diff `new-since <query|--designer>` (7) — diff objectIDs vs prior local snapshot (anonymous, catalog-scoped).
6. Subscription-free filter `search --no-subscription` (6) — local post-filter on outsideSubscription (not a server facet).
7. Tag/facet explorer `tags <query>` (6) — local tag/category frequency rollup to refine searches.

## Killed
- Freebie expiry tracker — no expiry field in Algolia; needs auth freebie page (out of scope).
- Library/favorites diff — requires logged-in account (out of scope).
- AI tag/style summarizer — LLM dependency (reframed to mechanical `tags`).
- Price-history tracker — Algolia returns current price only; no series.
- Cross-type bundle finder — LLM + no structural font/graphic link.
- Collection export — covered by absorbed --csv/--json.
- POD bulk source — thin over absorbed --pod + --csv.
