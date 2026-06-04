# GoDaddy Documentation And Community Sweep

Date: 2026-05-26

## What Was Checked

- Official developer portal: `https://developer.godaddy.com/doc`
- Official endpoint doc: `https://developer.godaddy.com/doc/endpoint/domains`
- Official Swagger JSON set: `https://developer.godaddy.com/swagger/swagger_{api}.json`
- Official reseller docs: `https://reseller.godaddy.com/docs`
- Official Help API-access article: `https://www.godaddy.com/help/how-do-i-access-domain-related-apis-42424`
- GitHub repository search for GoDaddy API/DNS/domain clients.
- GitHub code search for `api.godaddy.com/v1/domains`.
- npm search for GoDaddy API packages.
- Logged-in Chrome CDP account capture on the dedicated four-tab browser session.

## Findings

The public developer portal still exposes 12 Swagger groups: `abuse`, `aftermarket`, `agreements`, `ans`, `auctions`, `certificates`, `countries`, `domains`, `orders`, `parking`, `shoppers`, and `subscriptions`. Rechecking those specs on 2026-05-26 returned 111 paths and 138 operations, matching the bundled route map.

The Domains endpoint page remains the richest official public source. It explicitly points to `/swagger/swagger_domains.json`, documents OTE and production base URLs, and covers domain list/detail, purchase/register, DNS records, forwarding, privacy, renewal, transfer, notifications, actions, and API usage routes.

The reseller documentation adds an important adjacent official surface: `Domains API`, `Shoppers API`, `Certificates API`, `Countries API`, and `Websites API`. `Websites API` is not present in the public developer Swagger set, so it is now tracked as a follow-up source instead of being silently omitted.

The GoDaddy Help article confirms the product-facing domain API families: Aftermarket, Auctions, Domains, Parking, and Valuation. It also documents current plan/usage-limit gates, including access based on active domains, 50+ active domains for domain availability checks, or monthly spend. That matters for live-verification errors because a 403 can mean account eligibility, not bad auth.

The GitHub and npm ecosystem remains heavily DNS-centered. Public code and packages mostly wrap `/v1/domains`, `/v1/domains/{domain}/records`, ACME DNS-01 flows, DDNS updaters, or Terraform-style DNS management. Recent package hits include `@godaddy/cli`, `@pipedream/godaddy`, `@itentialopensource/adapter-godaddy`, `@framers/agentos-ext-domain-godaddy`, `godaddy-api`, `godaddy`, and `godaddy-dns`. None of those appears to aggregate the full official registrar/cert/order/subscription surface plus account-browser routes.

The logged-in browser capture adds 61 sanitized account-portal routes across product, billing/renewals, subscription, domain portfolio, domain-control, cart, notification, and internal domain APIs. That evidence is saved in `api-map/browser-cdp-routes-2026-05-26.json`.

## Reproducibility

```bash
gh search repos godaddy --limit 25 --json fullName,description,stargazersCount,updatedAt,url,language
gh search code 'api.godaddy.com/v1/domains' --limit 30 --json repository,path,url
npm search godaddy --json
python3 - <<'PY'
import json, urllib.request
apis=['abuse','aftermarket','agreements','ans','auctions','certificates','countries','domains','orders','parking','shoppers','subscriptions']
for api in apis:
  url=f'https://developer.godaddy.com/swagger/swagger_{api}.json'
  spec=json.load(urllib.request.urlopen(url, timeout=20))
  paths=spec.get('paths') or {}
  ops=sum(1 for p in paths.values() for m in p if m.lower() in {'get','post','put','patch','delete','head','options'})
  print(api, len(paths), ops)
PY
```

## Map Updates

- Added `api-map/documentation-sources-2026-05-26.json`.
- Added `api-map/browser-cdp-routes-2026-05-26.json`.
- Added CLI commands:
  - `godaddy-cli api-map browser-routes --summary --json`
  - `godaddy-cli api-map documentation-sources --json`

## Remaining Gaps

- Reseller `Websites API` needs a structured extraction pass.
- Valuation API is documented by the Help article but lives outside the developer Swagger set; it needs separate source and auth/access mapping.
- Account UI internals are route-mapped but not response-modeled; keep browser captures sanitized and add fixtures only from deliberate redacted live reads.
