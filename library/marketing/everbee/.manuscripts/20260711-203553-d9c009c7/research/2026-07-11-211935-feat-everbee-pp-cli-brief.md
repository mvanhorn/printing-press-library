# EverBee CLI Brief (v4.28 reprint, research redone 2026-07-11)

## API Identity
- Domain: Etsy seller / print-on-demand market research (EverBee Research product line; the separate EverBee Store commerce platform is out of scope).
- Users: Etsy sellers, POD creators, niche researchers, and AI agents running batch product/keyword/shop research.
- Data profile: keyword metrics (`keyword`, `vol`, `new_volume`, `competition`, `score`), product/listing metrics (`listing_type`, `price`, `est_mo_revenue`, `est_mo_sales`, `est_total_sales`, `review_count`, `listing_age_in_months`, `growth_rate`, `cached_tags_used`, `main_category`, `digital_listing_count`), shop metrics (`revenue`, `revenue_30_days`, `listing_active_count`, `review_*`, `sale_per_listing`). Verified from the live API and the synced local mirror on 2026-07-11.
- Upstream: private JSON API at `https://api.everbee.com`, session token auth (`x-access-token`) minted by Google SSO login; refresh-token flow proven working (hand-authored in the prior CLI).

## Reachability Risk
- Low. Live probe on 2026-07-11: raw GET `/users/show` with a stale access token → 401; the same read through the CLI's refresh flow → 200 with live data. No Cloudflare/bot-detection evidence anywhere (fresh web search found zero 403/blocking complaints). The practical failure mode is token/session expiry, mitigated by the existing refresh + `auth capture` flows.
- Probe-safe endpoint used: GET `/users/show` (spec `auth.verify_path`).
- Plan gating: free "Hobby" plan allows only 10 keyword searches/month; Growth ($29.99/mo) is unlimited. Seeded keyword search commands must surface plan-cap errors honestly, never as empty evidence.

## Top Workflows
1. Seeded niche research: give a seed ("dad shirt"), get relevant keyword evidence + matching product evidence + a defensible opportunity verdict (issue #1492's core ask).
2. Product opportunity search: ranked product analytics by revenue/sales/momentum, with product-type constraints (physical vs digital vs apparel, SVG/PNG exclusion).
3. Keyword opportunity research: volume, competition, score, clusters, with confidence tied to evidence coverage.
4. Shop competitor analysis: competitor sampling — result count, median price, review/sales density, listing age.
5. Snapshot + drift research: persist runs locally, diff niches over time, batch sub-niche discovery (`--parent dad --product t-shirt`).

## Table Stakes
- EverBee UI parity: product analytics table (filters, sort, pagination, time ranges), keyword suggestion table, shop analyzer table, folders. All five captured endpoints replayable.
- Competitors: eRank (keyword explorer, new "Engagement Score", low-competition keyword workflow, AI listing generator), Alura (real-time listing SEO scoring, AI review analysis), EtsyHunt (product DB filters), Sale Samurai (seed-keyword tag generator), Marmalead (keyword clustering/Storm).
- Fresh scan (2026-07-11): NO competitor ships evidence/confidence-annotated results, seeded sub-niche discovery with provenance, or competitor sampling — the #1492 evidence-aware feature set is genuinely novel across the whole Etsy-tools landscape.

## Ecosystem
- Zero third-party EverBee tools: no MCP server, CLI, npm/PyPI wrapper, or GitHub client for the research API (confirmed 2026-07-11). This CLI is the only programmatic surface.
- First-party: EverBee Developer Portal (dev.everbee.io) + Store API (OAuth, Postman docs) exist but cover ONLY the EverBee Store commerce platform — not the research data. Note in README; do not conflate.

## Data Layer
- Primary entities: keyword_research, product_analytics, shops, folders (synced resources); research_runs/snapshots (hand-authored `internal/research` store) for drift baselines and evidence provenance.
- Sync cursor: page-based; snapshots keyed by query + timestamp.
- FTS/search: keyword text, product titles, shop names, tags.
- Derived: opportunity score, evidence coverage, saturation, price bands, trend deltas, sub-niche batch scores.

## Spec Gaps (browser-sniff enrichment candidates)
The prior sniff captured only 5 default-feed GETs. UI features implying uncaptured endpoints:
- Seeded keyword search (typed seed in Keyword Research Suite — canonical workflow; F7 needs this endpoint or a `keyword=` param variant of the existing one)
- Tag Analyzer / listing-detail tags (F13, listing audit depth)
- Hot Filters: "New & Hot", "Rapid Growth", "Trending" (temporal filters, F12/trend)
- Bestseller badge filter, country filter (product analytics params)
Without a fresh capture, F7/F9/F10 are still buildable as local evidence-filtering over the default feeds, but seeded server-side search is strictly better.

## User Vision (issue #1492 — carried into novel-features brainstorm)
The prior CLI mistook transport success for trustworthy evidence: unrelated products scored as niche matches, confidence 1.0 with zero keyword evidence, no demand-vs-competition metrics. The reprint must make research evidence-aware:
- Tier 1 (validated on feat/everbee-issue-1492, port + verify): token-aware semantic relevance (F1), confidence tied to evidence coverage (F2), local-data reads after sync via isList=true (F3), output-contract fixes (F4), `which` typed exit 2 + {"matches":[]} (F5), explicit no-evidence results with resolved identity (F6).
- Tier 2: seeded keyword research --query/--seed (F7); demand/competition/saturation/trend/listing-count/price-band/evidence-count/opportunity-score fields (F8); product-type constraints physical/digital/apparel/SVG-PNG-exclusion (F9); batch sub-niche discovery --parent/--product (F10); shop-handle + listing-ID resolution (F11).
- Tier 3: competitor sampling (F12); tag/title consensus + seasonal-vs-evergreen (F13); normalized batch scoring + drift baselines (F14); raw safe-GET discovery (F15); machine-readable capability test separating transport success from semantic validity (F16).
- Acceptance anchors: ≥80% of returned product evidence contains the niche concept and product type; confidence ≤0.5 when keyword evidence is zero, 0 when nothing relevant remains; "low competition" requires documented thresholds; every ranked sub-niche carries metric provenance (source, fetched time, query scope, evidence count, fallback reason); never fabricate metrics.
- Cache policy: live-data-first, 15m default --max-age for compound research (validated); bulk-sync stale_after separately longer.

## Product Thesis
- Name: EverBee Research CLI (everbee-pp-cli)
- Why it should exist: the only programmatic surface for EverBee data, and the only Etsy research tool anywhere whose scores are evidence-audited — every verdict carries its evidence count, provenance, and an honest confidence, so agents can trust a "low competition" claim instead of inheriting a UI table's optimism.

## Build Priorities
1. Regenerate under v4.28 with enriched spec (live-first cache, MCP intents for compound research, learn seeds).
2. Port the validated `internal/research` engine + insights commands from feat/everbee-issue-1492 (carries F1–F6), re-verify the three known fixes against fresh generated code.
3. Build Tier 2 (F7–F11) — seeded research, evidence-field surface, product-type constraints, batch sub-niche discovery, identity resolution.
4. Build Tier 3 (F12–F16) — competitor sampling, consensus/seasonality, batch scoring + drift, raw discovery, capability self-test.
5. Full shipcheck + live dogfood against the #1492 reproduction commands ("dad shirt" scenario is the acceptance benchmark).

## Sources
- Live API + local mirror field inventory (2026-07-11); doctor + live read via feat-branch binary.
- Issue #1492 (mvanhorn/printing-press-library) full text.
- EverBee help: articles 2, 4, 10, 40, 142; everbee.io product/pricing pages; dev.everbee.io; github.com/everbee-io.
- Competitor reviews: help.erank.com, merchtitans.com, listing-forge.com, craftybase.com, growingyourcraft.com.
