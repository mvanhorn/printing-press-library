# Focused Asoview absorb manifest
Approved by explicit user authorization. Sole builder overrides novel-feature worker requirement. No competitor runtime integration; provider website features define scope. No stubs.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Regional/category/party/date discovery | Public search HTML and browser query state | asoview-pp-cli discover | Compact cards, bounded pages, local keyword filter with honest scope |
| 2 | Product detail and option bands | ASOVIEW_DATASOURCE | asoview-pp-cli product | Exact Japanese terms, advertised price basis, age labels, validity vs slots |
| 3 | Dated calendars and slots | Public GET stock surfaces | asoview-pp-cli availability | Date prices and status; never conflates unknown/sold out; no holds |
| 4 | Dated fee bands | Public category-sales-situations/basicfees | asoview-pp-cli options | Exact units/IDs and age bands; optional selected slot; no party quote guess |
| 5 | Canonical booking handoff | Canonical product path | asoview-pp-cli handoff | URL-only human handoff with no booking action |
| 6 | Region/category reference | First-party /location/ and /leisure/ HTML | asoview-pp-cli inventory | Bundled first-party inventory; explicit refresh only |

## Transcendence
| # | Feature | Command | Buildability | Why useful |
|---|---------|---------|--------------|------------|
| 1 | Side-by-side terms | compare | hand-code | Read a bounded shortlist in one compact schema |
| 2 | Honest keyword relevance | discover | hand-code | Exact local substring filtering, scan coverage and continuation |
| 3 | Date/quantity-aware availability | availability | hand-code | Preserves request/full/closed/unknown; unit and max/min limits |
| 4 | Lazy dated options | options | hand-code | Fee bands fetched only on request, preserves conflicting source age claims |
| 5 | Explicit taxonomy refresh | inventory | hand-code | Avoid surprising full inventory refresh calls |

## Acceptance
Every shipping command has meaningful live assertions, deterministic tests for parsing/state/limits, typed failures, JSON, field projection, freshness and nulls. HTTP GET only, first-party www.asoview.com allowlist, redirect guard, response/body/request limits, no paid account or browser runtime. Native search dates/party narrow candidates; checkout-specific total remains null. General tickets have no inventory confirmation unless stock public; unavailable fields explicitly null/unknown. Options limit 100, slots limit 100, comparison <=5, pages <=5, limit default10 <=50; timeout <=60 seconds per operation, retries <=1, concurrency sequential. Bounded disk cache 32MiB/128 files with short stock TTL and explicit refresh.
