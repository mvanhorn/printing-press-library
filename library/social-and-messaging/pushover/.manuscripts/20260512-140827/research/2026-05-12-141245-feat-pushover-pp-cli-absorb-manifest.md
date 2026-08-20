# Pushover Absorb Manifest

## Existing Features To Absorb

| Source | Feature | Shipping Requirement |
|---|---|---|
| Pushover official docs | Full message send parameter surface | `messages send` must expose token/user/message/title/device/html/monospace/priority/sound/timestamp/ttl/url/url-title/attachment-base64/attachment-type/encrypted/retry/expire/callback/tags. |
| Pushover official docs | Receipt poll and cancellation | `receipts get`, `receipts cancel`, and `receipts cancel-by-tag` must work. |
| Pushover official docs | Quota endpoint | `apps limits` must expose limit/remaining/reset. |
| Pushover official docs | User/group/device validation | `users validate` must support app token, user key, and optional device. |
| Pushover official docs | Groups API | create/list/get/add/remove/disable/enable/rename group commands must exist. |
| Pushover official docs | Glances API | `glances update` must expose title/text/subtext/count/percent. |
| Pushover official docs | Subscription migration | `subscriptions migrate` must exist and be labeled as a migration action. |
| Pushover official docs | Teams API | team info/add/remove must exist and use `--team-token`, not `--app-token`. |
| Pushover official docs | Licensing API | license credits/assign must exist and warn that assignment is irreversible. |
| Pushover official docs | Open Client API | user login, device registration, message download, and delete-through commands must exist. |
| `python-pushover` | Config/profile-driven sends | CLI must support config/env defaults rather than requiring credentials on every invocation. |
| `po-notify` | Stdin send and priority aliases | Provide a send wrapper that can read message body from stdin and accept human priority names. |
| MCP Pushover servers | Agent tool for send notifications | MCP server must expose the send wrapper and receipt/quota tools with clear destructive hints. |
| Lifecoach scripts | Local env compatibility | Support `PUSHOVER_APP_TOKEN`/`PUSHOVER_USER_KEY`; tolerate legacy `PUSHOVER_TOKEN`/`PUSHOVER_USER`. |

## Transcendence Features

| # | Feature | Command | Score | How It Works | Evidence |
|---|---|---|---:|---|---|
| 1 | Agent send wrapper | `notify` | 9/10 | Calls `POST /1/messages.json`, reads message from arg or stdin, fills credentials from env/config, supports named priorities, and validates emergency retry/expire rules before sending. | `po-notify` has stdin and priority aliases; Lifecoach scripts use env sends; official docs require retry/expire for emergency priority. |
| 2 | Emergency lifecycle | `emergency watch` | 9/10 | Sends priority-2 notifications through `messages send`, stores the receipt, polls `GET /1/receipts/{receipt}.json` no faster than every 5s, and can cancel by receipt or tag. | Official receipt docs define polling, cancellation, and cancel-by-tag; competitors advertise emergency support but usually not full lifecycle UX. |
| 3 | Quota cockpit | `quota` | 8/10 | Calls `GET /1/apps/limits.json`, renders limit/remaining/reset with reset time, and can be used before fanout or test sends. | Official docs expose quota headers and a dedicated limits endpoint; operations users need to avoid 429s. |
| 4 | Local notification ledger | `history` | 8/10 | Records successful CLI sends and receipt polls in local SQLite with request id, receipt, priority, tags, timestamp, and redacted target fingerprint; supports search/export without storing secrets. | Pushover messages disappear from servers/devices; local audit is the only way to answer what this CLI sent. |
| 5 | Inbox sync/search | `inbox sync` | 7/10 | Uses Open Client `GET /1/messages.json` to download messages into local SQLite before optional delete-through; `search` can query retained local messages. | Official Open Client docs require clients to download then delete messages; local storage makes the destructive server delete safe and auditable. |

## Buildability Proof

- `notify` uses the official messages endpoint and no external service.
- `emergency watch` uses only messages plus receipt endpoints and respects the documented 5-second receipt polling limit.
- `quota` uses the official app limits endpoint.
- `history` records CLI-owned send and receipt outputs, not synthetic Pushover state.
- `inbox sync` uses official Open Client download/delete endpoints and stores downloaded messages before deletion.

## Explicit Deferrals

- End-to-end encryption implementation is not required for v1; the raw `messages send` endpoint exposes `encrypted=1` and encrypted fields can be supplied by callers.
- Multipart binary attachment upload can be deferred if the generator only supports `attachment_base64`; base64 attachment fields must ship.
- WebSocket real-time Open Client listening can be deferred; the HTTP sync path must ship.

