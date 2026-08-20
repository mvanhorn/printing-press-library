# Gumroad Printing Press Research Brief

## Sources
- `https://gumroad.com/api`
- `https://github.com/antiwork/gumroad`
- `app/javascript/pages/Public/Api.tsx`
- `app/javascript/components/ApiDocumentation/Endpoints/*`
- `config/routes.rb`

## Endpoint Scope
The generated Gumroad surface covers the documented JSON API resources: products, file upload helpers, covers, variant categories, variants, offer codes, custom fields, user, resource subscriptions, sales, subscribers, licenses, payouts, tax forms, and earnings.

The raw tax-form PDF download endpoint is intentionally excluded from the MCP surface because the generated Printing Press HTTP client is JSON-oriented and MCP tools should return structured JSON rather than a binary PDF stream.

## Authentication
Gumroad uses OAuth access tokens through Doorkeeper. The generated CLI/MCP reads `GUMROAD_ACCESS_TOKEN` and sends it as a bearer token. `/licenses/verify` is documented as callable by buyers without an access token, but authenticated sellers can still use the same client configuration.

## Value-Add Features
- Local seller snapshot sync through `sync`.
- Cross-resource full-text search through `search`.
- Local grouping/count analytics through `analytics`.
- Polling-based NDJSON event tail through `tail`.
