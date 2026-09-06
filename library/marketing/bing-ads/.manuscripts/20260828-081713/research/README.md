# Research notes -- bing-ads engine

## Why this print exists

This print brings Bing Ads / Microsoft Advertising to parity with this
library's existing `google-ads` engine. No Bing/Microsoft Advertising engine
existed anywhere in this catalog prior to this PR.

## Spec sourcing

Microsoft does not publish a downloadable OpenAPI/Swagger file for the
Microsoft Advertising REST API. `spec.json` in this print was mechanically
reconstructed from Microsoft's own generated client,
[`BingAds/BingAds-Python-SDK`](https://github.com/BingAds/BingAds-Python-SDK)'s
`openapi_client/` package -- that package is openapi-generator *output*
(Pydantic models + a thin API-method wrapper per operation) with no source
spec file shipped alongside it, but every operation's HTTP method and
resource path is a literal in the generated `..._serialize()` helpers, and
every request/response shape is a literal Pydantic model. Both were parsed
mechanically (not by hand) to reconstruct a real OpenAPI 3.0 document:

- **Operation inventory**: parsed every `<verb>_serialize()` method across
  all 6 generated service API files (`campaign_management_service_api.py`,
  `customer_management_service_api.py`, `reporting_service_api.py`,
  `ad_insight_service_api.py`, `customer_billing_service_api.py`,
  `bulk_service_api.py`) for its `method=`/`resource_path=` literals, paired
  with the public method's request-type annotation and
  `_response_types_map`. Recovered 287 operations total.
- **Schema reconstruction**: recursively walked the SDK's Pydantic model
  files (`openapi_client/models/**/*.py`), mapping each `Field(..., alias=)`
  declaration to an OpenAPI property (respecting `Optional`/`List`/`Dict`
  wrapping, enum classes, and single-parent inheritance flattening). 976
  schemas resolved with only 3 unresolved edge-case type annotations.

## Auth model

Confirmed directly from the SDK's own `openapi_client/configuration.py`
`auth_settings()` method, not inferred: 3 independent header credentials --
`Authorization` (Entra/Azure AD OAuth bearer), `DeveloperToken` (Microsoft
Advertising developer token), `CustomerAccountId` (acting account id).
`CustomerId`, despite appearing in each operation's declared
`_auth_settings` list, is NOT wired as a header anywhere in
`configuration.py` -- it's a request-body field on the operations that need
it (confirmed by inspecting `GetAccountsInfoRequest`'s own field list).

## Known gap

~70 of the reconstructed schemas are Microsoft discriminated-union base
types (`Criterion`, `AdExtension`, `BiddingScheme`, `Recommendation`, etc.)
that the SDK expresses via a runtime `type` discriminator and a large
`__init__`-time class-name-to-type mapping, not via distinct request/response
fields. `spec.json` does not yet model these as `oneOf` unions -- they come
through as generic JSON-object properties. Concrete subtypes (e.g.
`CampaignPerformanceReportRequest`) are fully modeled where a specific
operation references them directly.
