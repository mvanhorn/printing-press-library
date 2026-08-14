# Sanitized iFood web API notes

These endpoints were observed in the authenticated iFood web application and
cross-checked against its public JavaScript bundles. They are private web APIs,
not a published compatibility contract, and may change without notice.

No bearer tokens, cookies, device identifiers, customer names, street
addresses, or production response payloads are stored here.

## Authentication

- Base URL: `https://www.ifood.com.br/site-api`
- Session material: bearer authorization plus dynamic browser request headers.
- CLI sources: `IFOOD_BEARER_AUTH` and the owner-only JSON file selected by
  `IFOOD_HEADERS_FILE`.
- The headers file must use mode `0600` and is never printed by the CLI.
- `session import-curl` and `session import-headers` validate Authorization,
  preview names only, and atomically install the file with mode `0600` after
  `--yes`.
- PerimeterX/anti-bot responses may return HTTP 403 even when account
  authentication is otherwise valid. The CLI reports this error and does not
  attempt to bypass a challenge.

## Saved addresses

- Method/path: `GET /v1/customers/me/addresses`
- Classification: read-only
- Observed response: JSON array
- Fields used by the CLI: `id`, `active`, `favorite`, `city`, `neighborhood`,
  `coordinates.latitude`, `coordinates.longitude`
- Privacy: the CLI intentionally omits street, number, complement, reference,
  postal code, and account identifiers from normalized output.

## Grocery catalog

- Method/path: `GET /v1/merchants/multicategory/{merchant-id}/catalog`
- Query: `latitude`, `longitude`
- Classification: read-only
- Used for: available delivery methods and catalog context
- Failure modes: expired session, dynamic-header drift, anti-bot HTTP 403,
  merchant unavailable at the selected coordinates

## Product search

- Method/path: `GET /v2/search/merchants/{merchant-id}/catalog-items`
- Query: `latitude`, `longitude`, `channel=IFOOD`, `term`, `size`, `page`,
  `item_from_merchant_ids`
- Classification: read-only
- IDs: product `id` values are accepted by cart item payloads

## Create cart

- Method/path: `POST /v1/carts` (the site can feature-flag `/v2/carts`)
- Classification: remote write
- Required CLI gates: preview by default; live request requires both
  `--execute` and `--yes`
- Sanitized request shape:

```json
{
  "items": [
    {"id": "<product-id>", "quantity": 1}
  ],
  "merchant": {"id": "<merchant-id>", "context": "DEFAULT"},
  "address": {
    "id": "<address-id>",
    "coordinates": {"latitude": 0, "longitude": 0}
  },
  "delivery": {
    "id": "<delivery-method-id>",
    "now": true,
    "mode": "<optional-mode>",
    "deliveryBy": "<optional-provider>"
  }
}
```

- Response ID: commonly `cartResponse.id`, sometimes nested under `data`
- The CLI does not create an order, start checkout, or submit payment.

## Read and add cart items

- Read: `GET /v1/carts/{cart-id}`
- Add: `POST /v1/carts/{cart-id}/items`
- The site can feature-flag v2 for both paths.
- Add payload: array of `{id, quantity, observation?, subItems?}`
- Add is a remote write and has the same `--execute --yes` gate.
