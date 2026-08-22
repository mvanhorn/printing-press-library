# Cosmos crowd-sniff report

## Scope

The approved crowd-sniff pass searched npm package metadata and authenticated GitHub code search for Cosmos API implementations. The base service under investigation was `https://api.cosmos.so`, authenticated with a bearer token.

## Automated crowd-sniff result

The npm scanner's only machine-selected match was `@azure/cosmos`, which exposes Azure Cosmos DB's unrelated `/_cosmos/remote-renderer-url` path. That result is a name collision, not evidence about cosmos.so, so it was rejected and was not merged into the generated specification.

The resulting `cosmos-crowd-spec.yaml` is retained only as negative-evidence provenance. It contributed zero endpoints to the CLI.

## Relevant community implementations reviewed manually

- `jpoindexter/cosmos-mcp` — GraphQL authentication, discovery, profile, collection creation, URL saving, collection connection, and similar-element workflows.
- `rclaycock/cosmos-scraper-mk-3` — public collection pagination and media export patterns.
- `rawpage/suggaplay` — collection download, format detection, manifests, and resume concepts.
- `rslosh/promptbox` — collection image enumeration and download patterns.
- `likeahuman-ai/roxit-masterclass` — Cosmos search skill usage and compact output patterns.

## Merge decision

No crowd-discovered spec was merged. The sanitized browser capture remained the sole generated API contract because it contained 37 directly observed Cosmos GraphQL operations. Community code was used only to cross-check the mutation documents that the intentionally read-only browser session did not exercise: login, refresh, collection creation, element creation, and element-to-collection connection edits.

## Security and privacy

No community artifact supplied credentials. The generated CLI uses `COSMOS_TOKEN` for bearer authentication and keeps access and refresh tokens in its private credential store. The crowd-sniff report contains no account identifiers or secrets.
