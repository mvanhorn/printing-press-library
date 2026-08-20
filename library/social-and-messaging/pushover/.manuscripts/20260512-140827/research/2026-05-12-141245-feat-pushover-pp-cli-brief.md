# Pushover CLI Research Brief

## Target

Build `pushover-pp-cli` for the official Pushover API surface at `https://pushover.net/api`.

Source type: official documentation, converted into an internal YAML spec because Pushover does not publish an OpenAPI document.

Primary source: Pushover official API docs.

Spec artifact: `/Users/todd_1/printing-press/.runstate/cli-printing-press-ecceef83/runs/20260512-140827/pushover.internal.yaml`

## Product Thesis

Pushover is not just a notification endpoint. It is a small operational paging system: every send, receipt, quota header, group membership, and Open Client message is a signal about whether an alert was deliverable, acknowledged, rate-limited, or silently routed somewhere surprising.

The CLI should make that operational state visible from the terminal, not only wrap `POST /messages`.

## Official Surface

The internal spec covers 26 endpoints across 11 resources:

- `messages`: send application notifications, download Open Client messages.
- `apps`: inspect monthly message limits.
- `sounds`: list built-in and custom sounds.
- `users`: validate user/group/device keys, Open Client login.
- `devices`: register Open Client devices, delete downloaded messages through a highest id.
- `receipts`: inspect and cancel emergency receipts, including cancellation by tag.
- `groups`: create/list/get/rename groups and add/remove/disable/enable users.
- `glances`: update watch/widget glance fields.
- `subscriptions`: migrate user keys into subscription user keys.
- `teams`: inspect team info and add/remove team users.
- `licenses`: assign license credits and check remaining credits.

Credential model:

- `PUSHOVER_APP_TOKEN` and `PUSHOVER_USER_KEY` are the primary local test credentials.
- Legacy compatibility names in the Lifecoach repo are `PUSHOVER_TOKEN` and `PUSHOVER_USER`.
- Team/Open Client/license flows use distinct credentials or dangerous mutations and must be gated.

## Top Workflows

1. Send a test or production notification with agent-safe defaults, stdin support, and clear priority semantics.
2. Send an emergency notification, capture its receipt, watch acknowledgement status, and cancel retries on demand.
3. Check quota before high-volume sends and surface reset time in human and JSON formats.
4. Validate user/group/device keys before persisting them in scripts or group membership.
5. Manage delivery groups without dropping users silently.
6. Update Glances fields for low-priority status widgets.
7. Download and locally retain Open Client messages before deleting them from Pushover servers.

## Competitor/Existing Tool Findings

- `python-pushover` provides Python bindings plus a small CLI, config profiles, and basic sends, but it appears old and focused on message creation.
- `po-notify` provides a send-only Node CLI with stdin, named priorities, and emergency retry/expire flags.
- `mcp-pushover` style servers expose one MCP send-message tool with common message parameters and retry behavior.
- `freeformz/pushover-mcp` advertises a Go MCP server for notifications and emergency-priority handling.
- Lifecoach has local send-only scripts/MCP servers using Pushover credentials, but no full CLI, no receipts/quota/group surface, and no generated skill/MCP pair.

Crowd-sniff result: `printing-press crowd-sniff` found only 2 noisy community endpoints unrelated to the official Pushover surface, so it did not change the spec.

## Gaps To Beat

- Most existing tools stop at send-message.
- Emergency receipt lifecycle is usually not first-class.
- Quota headers and `/apps/limits` are not surfaced as an operational budget command.
- Credential names vary across tools; this CLI should support clear env defaults and legacy Lifecoach names without leaking values.
- Open Client receive/delete is a separate workflow and should be represented honestly.

## Data Worth Persisting

High-gravity local tables:

- Sent messages: timestamp, title, message hash/preview, priority, target fingerprint, request id, receipt, tags, status.
- Emergency receipts: receipt id, tags, acknowledged/expired/cancelled status, last poll time.
- Open Client inbox: downloaded messages retained locally before Pushover deletion.

Do not persist raw user keys, app tokens, client secrets, passwords, or full message bodies unless the user explicitly requests it; default to previews and fingerprints.

## Auth And Safety

Live dogfood may send low-priority test notifications to Todd using credentials from `/Users/todd_1/repo/claude/lifecoach`. Do not print or archive credential values.

Default live tests:

- `messages send` with `priority=-1` or `sound=none`.
- `users validate` against Todd's user key.
- `sounds list`.
- `apps limits`.

Dangerous or costly paths:

- Emergency priority sends require explicit scope and immediate cancellation/watch behavior.
- Team add/remove, license assignment, subscription migration, group mutations, and Open Client login/device registration require explicit user approval and disposable fixtures.

## Ship Criteria

- Full endpoint wrapper for the 26-endpoint spec.
- Agent-native send wrapper with env defaults and stdin.
- Emergency receipt lifecycle command.
- Quota/status command.
- Local sent/receipt ledger with search/history.
- Full shipcheck and live dogfood with redacted proofs.

