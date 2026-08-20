# Squarespace Internal Account/Domains API (live CDP capture)

Captured 2026-05-28 via Chrome DevTools Protocol from an authenticated `account.squarespace.com`
session. This is a **separate surface from the documented Commerce API** (Orders/Products/Inventory/
Transactions/Contacts) that the published `squarespace-pp-cli` wraps. The published CLI covers the
official Commerce API 47/47; this doc maps the *internal, undocumented* account + domains-management
API that the Squarespace dashboard itself calls — to make the venue map thorough.

IDs masked: `:oid` = 24-hex object id, `:uuid`, `:id` = numeric, `:domain` = a managed domain name.
Query strings stripped. Auth = the logged-in `account.squarespace.com` session cookie (browser session;
not the Commerce API bearer). Endpoints are read (GET) unless noted; writes (add DNS record, edit email
forwarding) fire on dashboard interaction and are not all exercised here.

## Host: account.squarespace.com

### Account / profile
| Method | Path | Purpose |
|---|---|---|
| GET | `/api/account/1/profile` | account profile |
| GET | `/api/account/1/user/domains` | domains owned by the user |
| GET | `/api/account/1/user/is-suspicious` | account risk flag |
| GET | `/api/1/location/ips/mine` | caller IP/geo |

### Domains management (the rich surface)
| Method | Path | Purpose |
|---|---|---|
| GET | `/api/account/1/domain-summaries` | all domains summary |
| GET | `/api/account/1/domains/byName/:domain` | resolve a domain to its record |
| GET | `/api/account/1/domains/:domain/category` | domain category |
| GET | `/api/account/1/domains/:oid/custom-record-set` | **DNS records** for a domain |
| GET | `/api/account/1/domains/:oid/email-forwarding` | email-forwarding rules |
| GET | `/api/account/1/domains/:oid/email-forwarding/has-conflicting-mx-records` | MX-conflict check |
| GET | `/api/account/1/domains/:oid/presets` | DNS presets |
| GET | `/api/account/1/domains/:oid/user-permissions` | per-domain permissions |

### Websites
| Method | Path | Purpose |
|---|---|---|
| GET | `/api/account/1/website-summaries` | all websites summary |
| GET | `/api/account/1/website-summaries/:oid` | one website summary |
| GET | `/api/account/1/websites/:oid/website-domains` | domains attached to a website |
| GET | `/api/account/1/clone-websites/all-jobs/status` | website-clone job status |
| GET | `/api/account/1/manifests/business-merchandising` | merchandising manifest |

### Telemetry (omit from any CLI)
- `POST /api/v1/events`, `POST /api/v1/clanker/events` — analytics beacons.

## CLI implications
- The published `squarespace-pp-cli` is the official **Commerce** API; this internal account/domains API
  is a DIFFERENT surface (registrar-style domain + DNS + email-forwarding management). It would suit a
  `squarespace domains`/`dns`/`email-forwarding` command group if extending the CLI to the account API,
  or a separate internal-API CLI. Auth differs (account session cookie vs Commerce bearer).
- Highest-value undocumented endpoints: `custom-record-set` (read DNS), `email-forwarding`, `domain-summaries`,
  `website-summaries` — none exposed by the official Commerce API.

## Reproduction
CDP: `session.use(<account.squarespace.com tab>)`, `Network.enable`, subscribe `Network.requestWillBeSent`,
reload a `/domains/managed/<domain>/...` page. Full captured list: `/tmp/sq-internal-endpoints.txt`.
