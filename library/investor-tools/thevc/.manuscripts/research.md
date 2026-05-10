# THE VC API Research

## Discovery
theVC.kr (더브이씨) is Korea's leading startup investment database. The site is a Nuxt 3 SSR application. All data is served via public REST APIs — no authentication required.

## Endpoints Discovered

1. **Rankings** — `GET /api/interaction/hits/organizations/rankings/{ALL|STARTUP}` — ranked by weekly view count
2. **Latest Registered** — `GET /api/information/organizations/latest-registered`
3. **Profiles** — `GET /api/information/organizations/profiles/{slug}` — full company data (funding, employees, products, investors, patents)
4. **News Items** — `GET /api/information/organizations/profiles/{slug}/news-article/items`
5. **News Stats** — `GET /api/information/organizations/profiles/{slug}/news-article/stats`

## Data Quality
- Profiles contain 60+ fields per company
- 100+ companies in rankings endpoint
- Weekly view tracking data available
- Some fields gated behind plan requirements (totalFundingAmount, avgAmount)
- Company types: 스타트업 (Startup), 중소기업 (SME), 금융회사 (Financial), 사모투자회사 (PE), 액셀러레이터 (Accelerator)

## Why This Matters
Korean startup data is notoriously hard to access programmatically. Naver, Kakao, and government databases have significant API barriers. theVC.kr's open API fills this gap for the English-speaking investment community.
