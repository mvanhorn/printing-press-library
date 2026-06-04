# DriveThruRPG CLI Research Brief

Date: 2026-05-09
Candidate CLI: `drivethrurpg-pp-cli`
Printing Press binary: `4.2.0`

## Status

No existing printed CLI was found for DriveThruRPG:

- `printing-press lock status --cli drivethrurpg-pp-cli --json` reported no held lock and `has_cli: false`.
- `printing-press catalog search drivethrurpg --json` returned an empty list.

The API is usable enough for a strong first CLI, but the generated spec should be curated rather than copied from `crowd-sniff`. The crawler found only a noisy subset, while live probes and public Hydra metadata exposed many more resources.

## Confirmed Source Surfaces

- Official Library App help page documents Application Keys and the `My Library Access` toggle.
- Official RSS help page links the new-product feed at `https://www.drivethrurpg.com/rss.php`.
- API root responds at `https://api.drivethrurpg.com/`.
- Main API base is `https://api.drivethrurpg.com/api/vBeta/`.
- API docs are exposed as JSON-LD/Hydra at `https://api.drivethrurpg.com/documentation?_format=jsonld`.
- Existing community client: `drpg`, a Python CLI/client for downloading and updating purchases.

## Confirmed Public Endpoints

These were probed without authentication:

- `GET /api/vBeta/products?pageSize=1`
- `GET /api/vBeta/products/{id}`
- `GET /api/vBeta/products?keyword=traveller&pageSize=3`
- `GET /api/vBeta/products?publisherId=26694&pageSize=3`
- `GET /api/vBeta/search_ahead?keyword=traveller&pageSize=5`
- `GET /api/vBeta/publishers/{id}`
- `GET /api/vBeta/categories?pageSize=1`
- `GET /api/vBeta/categories/{id}`
- `GET /api/vBeta/filters?pageSize=1`
- `GET /api/vBeta/filters/{id}`
- `GET /api/vBeta/reviews?pageSize=2`
- `GET /api/vBeta/reviews?productId=1&pageSize=2`
- `GET /api/vBeta/special_offers?pageSize=1`
- `GET https://www.drivethrurpg.com/rss.php`

Observed response shapes are JSON:API-like for catalog resources, with `links`, `meta`, `data`, and sometimes `included`. Product detail is rich and includes price fields, publisher, rating, content flags, files, previews, page count, categories, filters, and HTML descriptions.

## Confirmed Authenticated Library Flow

The community `drpg` client and live endpoint probes indicate this flow:

1. User creates/copies an Application Key from DriveThruRPG account settings.
2. CLI posts to `POST /api/vBeta/auth_key?applicationKey=<key>`.
3. Response includes `token`, `refreshToken`, and `refreshTokenTTL`.
4. Subsequent requests use the token in the `Authorization` header.
5. Library list uses `GET /api/vBeta/order_products` with params:
   - `getChecksum=1`
   - `getFilters=0`
   - `page=<n>`
   - `pageSize=<n>`
   - `library=1`
   - `archived=0`
6. Download URL prep uses:
   - `GET /api/vBeta/order_products/{orderProductId}/prepare?siteId=10&index=<fileIndex>&getChecksums=0`
   - Poll `GET /api/vBeta/order_products/{orderProductId}/check?...` while status begins with `Preparing`.

The official app help page notes that keys need `My Library Access` enabled, so the CLI should surface that clearly in auth errors.

## Brittle Or Unknown Areas

- `GET /api/vBeta` returns no useful route, despite appearing in crawler output.
- `GET /api/vBeta/products/{id}/search` returned a server error in probing.
- Product filtering by filter id is not yet understood. Several plausible parameter names were ignored, and `filters=10117` produced a server error.
- `GET /api/vBeta/daily_deals` returned access denied anonymously.
- `GET /api/vBeta/wishlists` returned an anonymous-user cache error.
- `GET /api/vBeta/publishers?pageSize=1` returned unauthorized, while `GET /api/vBeta/publishers/{id}` worked.
- Website pages and legacy hosts can trigger Cloudflare. Prefer the API host and the official RSS feed.
- Hydra docs list many write operations and account resources. A first CLI should avoid mutating remote state.

## Recommended First CLI

Start with read-only public catalog commands and make authenticated library/download commands opt-in.

Public commands:

- `search <keyword>`: product search via `/products?keyword=...`
- `suggest <keyword>`: search-ahead via `/search_ahead`
- `product <id>`: product detail, with readable and JSON output
- `reviews <product-id>`: product reviews via `/reviews?productId=...`
- `publisher <id>`: publisher detail
- `categories` and `category <id>`
- `filters` and `filter <id>`
- `sales`: active special offers
- `new`: RSS feed parser for newest products

Authenticated commands:

- `auth token`: validate an Application Key without printing secrets
- `library`: list owned products
- `download <order-product-id> [file-index]`: prepare/poll/download
- `sync`: mirror purchased files with dry-run support
- `updates`: show owned products with newer `fileLastModified`

Agent-friendly options:

- `--json`
- `--limit`
- `--page`
- `--raw`
- `--compact`
- `--dry-run`
- `--download-dir`

## Next Printing Press Step

Create a small OpenAPI seed by hand from confirmed endpoints, then generate `drivethrurpg-pp-cli` from that seed. Do not depend on the crowd-sniff YAML as the only spec source; it found only two code-search endpoints and missed the useful authenticated library/download behavior.

