# Vendored FedEx OpenAPI documents

These files are the OpenAPI 3.0 documents the FedEx Developer Portal loads into its documentation pages. They were downloaded on 2026-10-27. `provenance.json` records the catalog page, the exact JSON URL, the `info.version` inside the file, and the `Last-Modified` header.

| File | API | Source page |
| --- | --- | --- |
| `authorization.openapi.json` | API Authorization (`POST /oauth/token`) | https://developer.fedex.com/api/en-us/catalog/authorization/docs.html |
| `ship.openapi.json` | Ship (`/ship/v1/shipments`, cancel, validate) | https://developer.fedex.com/api/en-us/catalog/ship/docs.html |
| `pickup.openapi.json` | Pickup Request | https://developer.fedex.com/api/en-us/catalog/pickup/docs.html |
| `rate.openapi.json` | Rates and Transit Times | https://developer.fedex.com/api/en-us/catalog/rate/docs.html |
| `address.openapi.json` | Address Validation | https://developer.fedex.com/api/en-us/catalog/address-validation/docs.html |

FedEx publishes each operation body as a `oneOf` of a structural `Full_Schema_*` plus many example-only schemas that declare no properties. Those empty schemas match every JSON value, so a validator rejects the `oneOf` itself. The contract tests validate requests against the structural schema named in that `oneOf`, and they read example payloads from the example-only schemas.

`requestedShipment.totalWeight` is marked required on the Ship structural schema, but the field description limits it to international shipments and the published domestic and express examples omit it. The tests allow that one schema error and no others.

Error responses are the `application/json` examples on each non-2xx response. Almost all of them use `errors[].code`. The shipment-validation 404 example is the exception: `{"error":"NOT.FOUND.ERROR","error_detail":"..."}`. The client surfaces that code and still redacts the message. Shipment validation's `pickupType` enum also contains the typo `CONTACT_FEDEX TO_SCHEDULE`; the CLI keeps the underscore form used by the Ship schema and does not accept the typo.

Prism is not a useful mock for these files: the server URL is `/`, and the example-only `oneOf` branches make the published operation schema unusable as a request validator. The offline stand-in is `go test` in this package, which serves the spec's own examples from `httptest`. No sandbox credentials are required.
