# Twilio CLI Absorb Manifest

> Build floor: every absorbed row is a feature we MUST ship to match the existing
> ecosystem. Transcendence rows are 12 novel features the brainstorm survived.
> Total feature count = ~29 absorbed surfaces + 12 transcendence = 41.

## Absorbed (match or beat everything that exists)

Sources surveyed: twilio-cli (twilio/twilio-cli, 189★, Node oclif), twilio-labs/mcp (104★, auto-generated MCP tools from OpenAPI), steampipe-plugin-twilio (4★, SQL via FDW), twilio-python (2,057★), twilio-node (1,535★), twilio-go (370★).

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Send SMS / MMS / WhatsApp | twilio-cli `api:core:messages:create`, twilio-labs/mcp | `messages create` (typed Cobra) | --json/--csv/--select/--dry-run, batch via --stdin, idempotency on MessageSid |
| 2 | List messages with filters (DateSent>=, From, To, Status) | twilio-cli, mcp | `messages list` | Page-token paginate, --json/--csv, FTS over local store via `search` |
| 3 | Get / update / delete message | twilio-cli, mcp | `messages get/update/delete` | --dry-run, exit-code 4 on 404 |
| 4 | List/fetch/delete message media | twilio-cli, mcp | `messages media list/get/delete` | --json typed, media URL preserved |
| 5 | Place outbound call | twilio-cli, mcp | `calls create` | --dry-run, --json |
| 6 | List/get/update/delete calls | twilio-cli, mcp | `calls list/get/update/delete` | Page-token paginate, FTS over From/To |
| 7 | List/get/delete recordings | twilio-cli, mcp | `recordings list/get/delete` | Pair with Calls via local store |
| 8 | Conferences: list/get/update + participants | twilio-cli, mcp | `conferences list/get/update`, `conferences participants list/get/update/delete` | --json/--dry-run |
| 9 | Search available phone numbers (Local/Mobile/TollFree, area code, locality, capabilities) | twilio-cli, mcp | `available-phone-numbers list` | Capability filters, --csv export |
| 10 | Buy / list / configure / release IncomingPhoneNumbers | twilio-cli, mcp | `incoming-phone-numbers create/list/update/delete` | --dry-run, idempotency, FTS over FriendlyName |
| 11 | Outgoing CallerIds (validate, list, delete) | twilio-cli, mcp | `outgoing-caller-ids …` | --json |
| 12 | Subaccount management (create, list, suspend, close) | twilio-cli, mcp | `accounts create/list/update` | --json/--dry-run |
| 13 | API Keys + SigningKeys CRUD | twilio-cli, mcp | `keys list/get/create/delete`, `signing-keys …` | --dry-run, no secret echo on subsequent reads |
| 14 | Usage records (per category × period: today, thisMonth, lastMonth, daily, monthly, yearly, allTime) | twilio-cli, mcp | `usage records list` | --json/--csv, period flag |
| 15 | Usage triggers (spend alerts) | twilio-cli, mcp | `usage triggers list/create/update/delete` | --dry-run |
| 16 | Applications (TwiML app handles) | twilio-cli, mcp | `applications list/get/create/update/delete` | --dry-run |
| 17 | Queues + Members | twilio-cli, mcp | `queues list/get/create/update/delete`, `queues members list/get/update` | --json |
| 18 | Sip Domains, ACLs, Credentials | twilio-cli, mcp | `sip domains …`, `sip ip-access-control-lists …`, `sip credential-lists …` | Full CRUD |
| 19 | Addresses + DependentPhoneNumbers | twilio-cli, mcp | `addresses list/get/create/update/delete`, `addresses dependent-phone-numbers list` | --dry-run |
| 20 | ShortCodes (list, update) | twilio-cli, mcp | `short-codes list/get/update` | --json |
| 21 | ConnectApps + AuthorizedConnectApps | twilio-cli, mcp | `connect-apps …`, `authorized-connect-apps …` | --json |
| 22 | Account balance | twilio-cli, mcp | `balance get` | --json |
| 23 | Token (NTS for WebRTC) | twilio-cli, mcp | `tokens create` | --json |
| 24 | Notifications (deprecated but in spec) | twilio-cli, mcp | `notifications list/get/delete` | --json |
| 25 | Transcriptions (deprecated) | twilio-cli, mcp | `transcriptions list/get/delete` | --json |
| 26 | Validation requests (caller-id verification) | twilio-cli, mcp | `validation-requests create` | --dry-run |
| 27 | SQL access to resource tables | steampipe-plugin-twilio | `sql "<query>"` over local SQLite store | One static binary, no Postgres FDW required |
| 28 | Full-text search across resources | (none) | `search "<term>"` over Messages.Body, Calls/Messages From & To, IncomingPhoneNumbers.FriendlyName | Offline-first; no equivalent in any Twilio tool |
| 29 | Doctor: auth-mode detect, scoped-key warn, parent/sub mismatch detect | (none) | `doctor` | Catches the most common Twilio auth misconfig classes |

## Transcendence (only possible with our approach)

12 novel features survived the customer-modeled brainstorm + adversarial cut. Customer model + killed candidates archived in `2026-05-09-120941-novel-features-brainstorm.md`.

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---|---|---|---|
| 1 | Failed-message breakdown | `delivery-failures --since 7d` | 9/10 | Local SQLite groupby on synced Messages by ErrorCode + To country prefix, summing Price. The API has no aggregation by ErrorCode×country; twilio-cli has no aggregation; Steampipe needs a Postgres FDW. |
| 2 | Subaccount spend matrix | `subaccount-spend --period last-month --csv` | 10/10 | Walks synced Subaccounts, calls `usage/records/{period}` per subaccount via `// pp:client-call`, pivots categories to columns. The cross-subaccount aggregation does not exist in any Twilio API call. |
| 3 | Call timeline stitch | `call-trace <Sid>` | 9/10 | Reads Calls + Recordings + Transcriptions + Conferences/Participants from local store keyed by CallSid (or live API when not synced). Three round trips collapsed to one command. |
| 4 | TCPA opt-out reconciliation | `opt-out-violations` | 9/10 | Local query joining inbound Messages with Body matching `^(STOP\|UNSUBSCRIBE\|END\|QUIT)$`i to subsequent outbound Messages to the same `From`/`To` pair. Twilio has no opt-out resource. |
| 5 | Message status funnel | `message-status-funnel --since 24h` | 8/10 | Local groupby over Status enum (queued→sent→delivered/failed/undelivered/received) + median (DateUpdated - DateCreated). The Twilio Console graphs this; no CLI/MCP exposes the numbers. |
| 6 | Call disposition cross-tab | `call-disposition --since 24h` | 7/10 | Local cross-tab on synced Calls Status × AnsweredBy (human/machine_start/fax) with $ cost per bucket. Voicemail-detection rates are key Voice metrics; no CLI exposes them. |
| 7 | Idle number reclamation | `idle-numbers --since 30d` | 9/10 | Three-way LEFT JOIN: IncomingPhoneNumbers ⟕ Messages on From=PhoneNumber ⟕ Calls on From=PhoneNumber, filter MAX(activity) < cutoff, show $ wasted/month. Real $1/number/month savings. |
| 8 | Webhook orphan audit | `webhook-audit [--probe]` | 7/10 | Local groupby over IncomingPhoneNumbers VoiceUrl/SmsUrl flagging unique-use URLs; opt-in `--probe` HEAD-requests each unique URL. No Twilio tool detects orphan webhooks. |
| 9 | Number conversation feed | `conversation <number>` | 8/10 | UNION over synced Messages and Calls where From=<num> OR To=<num>, sorted by timestamp, formatted as in/out arrow + body/duration timeline. Console has it; no CLI does. |
| 10 | Error code explainer | `error-code-explain --since 7d` | 7/10 | Local groupby on synced Messages+Calls by ErrorCode joined to curated `// pp:novel-static-reference` table of top-50 Twilio error codes (cause + fix). Removes the constant Google-the-error-code round trip. |
| 11 | Sync watermark inspector | `sync-status` | 6/10 | Local-only read of last DateUpdated/StartTime per resource and row counts. Without freshness UX every analytic command is suspect. |
| 12 | Live failure tail | `tail-messages --status failed --follow` | 7/10 | Polling loop with `DateUpdated>=last_seen` every N seconds; short-circuits when `cliutil.IsVerifyEnv()`. twilio-cli has no `--follow`. |

## Stub list

None. Every feature above is shippable with the v2010 spec, the local SQLite store, and either real client calls or local joins. No external paid services, no headless browser, no LLM dependency. The only feature with a non-trivial dependency is `webhook-audit --probe`, which makes outbound HEAD requests to user-configured webhook URLs — the default behavior is local-only.
