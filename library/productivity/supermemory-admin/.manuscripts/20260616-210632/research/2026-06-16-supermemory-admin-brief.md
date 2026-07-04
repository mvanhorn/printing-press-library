# Supermemory Admin Research Brief

## Source Surface

- Official OpenAPI: `https://api.supermemory.ai/v3/openapi`
- Base URL: `https://api.supermemory.ai`
- Auth: bearer token, exposed as `SUPERMEMORY_ADMIN_TOKEN`
- Optional project scoping: `x-sm-project`, exposed as `SUPERMEMORY_ADMIN_PROJECT`

## Scope

This print is intentionally an operator/admin CLI for Supermemory. It covers documents, memories, recall search, profiles, conversations, container tags, connections, and settings from the official API without trying to replace the hosted MCP server.

## Generator Notes

The official OpenAPI resource names include shapes that collide with Printing Press internals:

- `/v4/profile` would generate a reserved `profile` command.
- `/v4/search` would generate a reserved `search` command.
- `/v3/connections/{connectionId}/resources` would generate a typed `resources` table that collides with the local generic `resources` table.

The archived spec uses `x-pp-resource` aliases:

- `profiles`
- `supermemory_recall`
- `connection_resources`

## Built Capabilities

- Project-scoped memory/admin operations via `SUPERMEMORY_ADMIN_PROJECT`.
- Agent-readable recall search through `supermemory-recall post-v4-search`.
- Local SQLite sync/search for compatible resources.

## Verification

- `go test ./...`
- `cli-printing-press dogfood --dir . --spec spec.json --json`
- `cli-printing-press verify-skill --dir . --json`
- `cli-printing-press publish validate --dir . --json`
