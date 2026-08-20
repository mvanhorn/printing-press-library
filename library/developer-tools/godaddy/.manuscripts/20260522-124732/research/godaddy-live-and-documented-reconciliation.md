# GoDaddy API Map — Live Capture + Documented-API Reconciliation

Captured 2026-05-28 via CDP from an authenticated `account.godaddy.com` session, plus reconciliation
against GoDaddy's 12 published developer APIs.

## Two surfaces

GoDaddy exposes **two distinct surfaces**:

1. **Public developer API** (`api.godaddy.com`, sandbox `api.ote-godaddy.com`) — the 12 documented APIs.
   This is what `godaddy-pp-cli` wraps. Authoritative source = GoDaddy's published OpenAPI specs.
2. **Internal account-UI API** (`account.godaddy.com` etc.) — what the GoDaddy dashboard calls. Captured
   live below as the "manual map" / cherries. Not part of the public CLI surface but useful supplement.

## Documented-API coverage reconciliation (the CLI vs the 12)

| Documented API | CLI command present? |
|----------------|----------------------|
| Abuse | YES (`abuse`) |
| Aftermarket | YES (`aftermarket`) |
| Agreements | YES (`agreements` — `GET /v1/agreements`) |
| ANS | YES — the `agents` resource IS the ANS API (operationIds `searchANSName`/`resolveANSName`/`registerAgent`; 16 endpoints). NOT "name servers". |
| Auctions | YES (`auctions-aftermarket` bid POST + `customers auctions get-listings` read) |
| Certificates | YES (`certificates`) |
| Countries | YES (`countries`) |
| Domains | YES (`domains`, incl. DNS records) |
| Orders | YES (`orders`) |
| Parking | YES (`parking`) |
| Shoppers | YES (`shoppers`) |
| Subscriptions | YES (`subscriptions`) |

CLI also ships `agents` (=ANS), `analytics`, `customers`, `profile`, `promoted`, `search` resources.
**Coverage: 12 of 12 documented APIs — 138/138 documented endpoints** (verified by exact set-comparison of
every spec endpoint at `developer.godaddy.com/swagger/swagger_<api>.json` against the CLI's `pp:path`
annotations). No code gaps. (My earlier draft mis-mapped ANS as "name servers" — corrected.)

## Live internal account-UI endpoints (CDP capture, IDs masked, OPTIONS removed)

```
GET	account.godaddy.com/akam/13/661464f4
GET	account.godaddy.com/assets/d2c2091bc5a5229fda867f821dfa5296ee6fdd636fa
GET	account.godaddy.com/customertypeapi/v1/get-by-shopper-id/:id
POST	account.godaddy.com/dccfabric/v2/customers/:uuid/domains/get	[BODY]
POST	account.godaddy.com/dccfabric/v2/customers/:uuid/domains/getActionEligibility	[BODY]
GET	account.godaddy.com/gateway/v2/customers/:uuid/productFamilies
GET	account.godaddy.com/gateway/v2/customers/:uuid/subscriptions
GET	account.godaddy.com/gateway/v2/customers/:uuid/subscriptions-shim/entitlements
GET	account.godaddy.com/LR4QWH/t/w/MvYm11Sj9g/0kaz/HEdZDi0VAQM/aRBgT/SouL0dY
GET	account.godaddy.com/LR4QWH/t/w/MvYm11Sj9g/0kaz/HEdZDi0VAQM/HlN4W/ktsdkAq
GET	account.godaddy.com/LR4QWH/t/w/MvYm11Sj9g/0kazNrOwt7/Gyt2Di0VAQM/LytZZ/idtJBF2
POST	account.godaddy.com/LR4QWH/t/w/MvYm11Sj9g/0kazNrOwt7/Gyt2Di0VAQM/LytZZ/idtJBF2	[BODY]
POST	account.godaddy.com/LR4QWH/t/w/MvYm11Sj9g/u7azNrLpt78D2S5S/UmhvDi0VAQM/VzVOP/jILUEQB	[BODY]
GET	account.godaddy.com/LR4QWH/t/w/MvYm11Sj9g/u7azNrLpt78D2S5S/UmhvDi0VAQM/VzVOP/jILUEQB
GET	account.godaddy.com/payapi/v1/paymentprofiles
GET	account.godaddy.com/platapi/v1.1/shoppers/:id
GET	account.godaddy.com/platapi/v1/domains
GET	account.godaddy.com/platapi/v1/subscriptions
GET	account.godaddy.com/platapi/v1/subscriptions/productGroups
POST	account.godaddy.com/productgraphapi/v1/gql/customer	[BODY]
GET	account.godaddy.com/products
GET	account.godaddy.com/products/api/customer-segment/segment/get
GET	account.godaddy.com/products/commapi/v1/commerce/subscriptions
GET	account.godaddy.com/products/commercembe/core/v1/stores/stores-summary
GET	account.godaddy.com/products/proxy/hosting/cpanel/accounts
GET	account.godaddy.com/websitesapi/v2/composite/website-list
GET	airo-builder.godaddy.com/api/subscriptions
GET	cart.godaddy.com/checkoutapi/v1/carticon/experiment
POST	cart.godaddy.com/checkoutapi/v1/carticon/launch	[BODY]
POST	cart.godaddy.com/checkoutapi/v1/checkouts/CartIcon/assets
GET	gui.godaddy.com/pcjson/applicationheader
POST	notifications-api.godaddy.com/v1/assets/px-assets-notifications
POST	notifications-api.godaddy.com/v1/launch	[BODY]
POST	pg.api.godaddy.com/v1/gql/customer	[BODY]
GET	www.godaddy.com/domainfind/v1/merchrecommend/domain
```

These are `account.godaddy.com` / `cart.godaddy.com` / `notifications-api.godaddy.com` internal endpoints
(product list, billing, notifications, cart). Supplementary to the public-API CLI surface.

## Reproduction
CDP: `session.use(<account target>)`, `Network.enable`, subscribe `requestWillBeSent`, reload
`account.godaddy.com/products`. Public-API specs: developer.godaddy.com/doc (per-API OpenAPI).
