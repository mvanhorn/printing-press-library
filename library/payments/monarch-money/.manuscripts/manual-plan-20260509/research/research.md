# Monarch Money CLI Research

Monarch Money does not publish an official OpenAPI specification for the personal finance surfaces covered here. This CLI is based on observed browser GraphQL operations and the public behavior of community Monarch Money clients.

## Scope

- Account balance inspection
- Household transaction tag listing
- Transaction listing with date, account, tag, and text filters
- Cashflow aggregation for a date range
- Guarded read-only GraphQL query execution for advanced workflows

## Safety Model

The CLI is read-oriented. It does not expose first-class transaction edits, rule edits, account refreshes, or other Monarch Money mutations. The advanced `query` command refuses GraphQL files containing `mutation`.

## Authentication

The CLI supports either a saved Monarch session token or `MONARCH_TOKEN`. Login accepts `MONARCH_EMAIL`, `MONARCH_PASSWORD`, and optional MFA via `--mfa`, then stores only the resulting session token locally.
