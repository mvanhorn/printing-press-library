# Research Brief: American Express Offers CLI

## API Overview

**Target:** `https://global.americanexpress.com/offers`  
**Type:** Session-authenticated SPA (React)  
**Auth:** Cookie-based session (login at americanexpress.com)  
**Discovery method:** Browser-sniff via agent-browser  
**CLI slug:** `american-express-offers`

## User Goals

1. **List all current offers** — show every active offer on the card (merchant, discount, category, expiry, enrolled status)
2. **Show total savings** — the current amount saved/redeemed through offers

## What We Know (Pre-Sniff)

### API Surface (Inferred)

The Amex Offers SPA makes authenticated REST API calls to the backend. Based on known SPA patterns and community project analysis:

- **Base host:** likely `global.americanexpress.com` (same-host SPA) with internal API paths  
- **Auth:** Session cookies set after login. Likely includes `SID`, `custr`, and/or `amex_session` cookies.  
- **Likely endpoints:**
  - `GET /dashboard/...` or `GET /api/...` — offers list with pagination
  - Offers include: merchant name, offer type (%, $ off, points), discount value, min spend, expiry date, enroll status
  - Savings summary: total saved, YTD redeemed, number of enrolled offers

### Authentication

- User must be logged into americanexpress.com in Chrome
- Cookies carry the session (not Bearer tokens)
- `auth login --chrome` pattern: extract cookies from running Chrome session
- Cookie replay validation required before confirming auth pattern in CLI

### Known Community Projects

- `350HP/AmexOffers` — Java + Selenium: opens Chrome, prompts login, displays offers. Confirms the site requires browser authentication.
- `qibinlou/Amex-Offers-Helper` — JS console snippet: DOM-based (clicks `.offer-cta` buttons). Confirms offers are rendered in the DOM with class `offer-cta`.

No community project has published raw API endpoint URLs — browser-sniff is required.

## CLI Commands to Build

| Command | Description |
|---------|-------------|
| `offers list` | List all active offers on the card |
| `offers savings` | Show total amount saved via enrolled offers |

### Output columns for `offers list`

- Merchant name
- Discount (e.g. "10% back", "$5 off $25+")
- Category
- Expiry date
- Enrolled (yes/no)
- Amount saved (if redeemed)

### Flags

- `--enrolled` — filter to only enrolled offers
- `--category <cat>` — filter by category
- `--json` — machine-readable output

## Discovery Strategy

1. Ask user whether they're logged into amex.com in Chrome
2. Use agent-browser with authenticated session (cookie transfer or headed login)
3. Navigate to `https://global.americanexpress.com/offers`, interact with the page
4. Capture XHR/fetch requests to identify the offers API endpoints
5. Capture response shapes to inform spec generation
6. Validate cookie replay via curl before confirming auth pattern

## Phase 2 Generate Command (preliminary)

```
cli-printing-press generate \
  --spec <browser-sniff-spec> \
  --name american-express-offers \
  --output <cli-work-dir>
```

## Notes

- No public API, no official SDK for consumer card offers
- The targeted-offers and defaultoffers SDKs are for B2B partner integrations, not consumer card-linked offers
- Discovery is entirely dependent on browser-sniff
- The user has agent-browser v0.27.0 available (browser-use not installed — no Python package manager)
