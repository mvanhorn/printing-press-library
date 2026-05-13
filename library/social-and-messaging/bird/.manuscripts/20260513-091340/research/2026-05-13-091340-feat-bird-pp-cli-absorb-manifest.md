# Bird CLI — Absorb Manifest

Scope per user briefing: Conversations API (full surface) + SMS-relevant pieces of the Channels API. Voice and Verify are out of scope.

## Tools surveyed

- **Bird Conversations API + Channels API docs** — official docs at docs.bird.com (ground truth)
- **messagebird/openapi-specs** — official OpenAPI for SMS (legacy MessageBird REST)
- **messagebird/go-rest-api** — Go SDK; `conversation/`, `sms/` packages
- **messagebird/python-rest-api**, **php-rest-api**, **ruby-rest-api**, **csharp-rest-api**, **java-rest-api**, **dart-rest-api**, **messagebird-nodejs** — same surface in other languages
- **MessageBird Postman collection** — `messagebird-official` workspace
- **Pipedream MessageBird integrations** — workflow triggers/actions

**No CLI tool found. No MCP server found. No Claude Code plugin/skill found.** This is a clear ecosystem gap.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Create conversation | Bird Conversations API + Go SDK conversation pkg | `conversations create --participant ...` | --json/--dry-run, idempotent, agent-native |
| 2 | List conversations | Bird Conversations API | `conversations list --status active --limit N` | Local FTS over synced conversations, --json/--csv |
| 3 | Get conversation | Bird Conversations API | `conversations get <id>` | --select for narrow output, agent-native |
| 4 | Update conversation | Bird Conversations API | `conversations update <id> --status closed` | --dry-run + diff |
| 5 | Delete conversation | Bird Conversations API | `conversations delete <id>` | --confirm + --dry-run |
| 6 | Create conversation message (reply) | Bird Conversations API | `conversations messages create <conv-id> --body ...` | Stdin batch, idempotent |
| 7 | List conversation messages | Bird Conversations API | `conversations messages list <conv-id>` | --json/--select, paging |
| 8 | Get conversation message | Bird Conversations API | `conversations messages get <id>` | offline if synced |
| 9 | Update conversation message | Bird Conversations API | `conversations messages update <id>` | --dry-run |
| 10 | Delete conversation message | Bird Conversations API | `conversations messages delete <id>` | --confirm |
| 11 | Create pre-signed upload (Conversations) | Bird Conversations API | `conversations media upload <file>` | One-step file→URL upload |
| 12 | Get conversations configuration | Bird Conversations API | `channel-config get` | agent-readable JSON |
| 13 | Update conversations configuration | Bird Conversations API | `channel-config update --enabled` | --dry-run |
| 14 | Add participant to conversation | Bird Conversations API | `conversations participants add <conv-id> --type contact --identifier-key phonenumber --identifier-value +31...` | by-identifier convenience |
| 15 | List participants | Bird Conversations API | `conversations participants list <conv-id>` | --json |
| 16 | Get participant by ID | Bird Conversations API | `conversations participants get <conv-id> <participant-id>` | --json/--select |
| 17 | Get participant by identifier (key+value) | Bird Conversations API | `conversations participants find --key phonenumber --value +31...` | flag-driven lookup |
| 18 | Update participant by ID | Bird Conversations API | `conversations participants update <conv-id> <id>` | --dry-run |
| 19 | Update participant by identifier | Bird Conversations API | `conversations participants update-by-identifier --key ...` | dispatching |
| 20 | Delete participant | Bird Conversations API | `conversations participants remove <conv-id> <id>` | --confirm |
| 21 | List conversations of participant by ID | Bird Conversations API | `participants conversations <participant-id>` | --json |
| 22 | List conversations of participant by identifier | Bird Conversations API | `participants conversations-by-identifier --key phonenumber --value +31...` | --json |
| 23 | Get antispam setting | Bird Conversations API | `workspace antispam get` | --json |
| 24 | Update antispam setting | Bird Conversations API | `workspace antispam update --enabled` | --dry-run |
| 25 | Create allow/block rule | Bird Conversations API | `workspace rules create --kind block --identifier ...` | --json/--dry-run |
| 26 | Get allow/block rule | Bird Conversations API | `workspace rules get <id>` | --json |
| 27 | List allow/block rules | Bird Conversations API | `workspace rules list --kind block` | --json/--csv |
| 28 | Update allow/block rule | Bird Conversations API | `workspace rules update <id>` | --dry-run |
| 29 | Delete allow/block rule | Bird Conversations API | `workspace rules delete <id>` | --confirm |
| 30 | Add allow/block rules in bulk | Bird Conversations API | `workspace rules bulk-add --file rules.csv` | CSV input, batches transparently |
| 31 | Get allow/block bulk upload status | Bird Conversations API | `workspace rules bulk-status <upload-id>` | --json |
| 32 | List channels | Bird Channels API | `channels list --kind sms` | --json |
| 33 | Get channel | Bird Channels API | `channels get <channel-id>` | --json |
| 34 | Check messageability for a contact | Bird Channels API | `channels messageability <channel-id> <contact-id>` | --json (true/false + reason) |
| 35 | Send channel message (the SMS send) | Bird Channels API | `sms send --to +31... --body "..." [--from <originator>]` | --dry-run, --stdin batch, --idempotency-key |
| 36 | List messages by channel | Bird Channels API | `messages list --channel <channel-id>` | --json |
| 37 | List messages by workspace | Bird Channels API | `messages list-all --since 2026-05-01` | --json |
| 38 | Get message by ID | Bird Channels API | `messages get <message-id>` | --json |
| 39 | List message interactions (delivery events) | Bird Channels API | `messages interactions <message-id>` | timeline view |
| 40 | Pre-signed media upload (workspace-wide) | Bird Channels API | `media upload <file>` | one-step |
| 41 | Pre-signed media upload (channel-specific) | Bird Channels API | `channels media-upload <channel-id> <file>` | one-step |
| 42 | List/get/update compliance keyword messages (HELP/STOP/START) | Bird Channels API | `compliance keywords list/get/set` | --dry-run on update |

**Total: 42 absorbed features.** No stubs.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Persona | Why Only We Can Do This |
|---|---------|---------|-------|---------|--------------------------|
| 1 | Delivery audit | `messages audit <message-id>` | 9/10 | Priya, Sana | Two real endpoint calls (`/messages/{id}` + `/messages/{id}/interactions`) folded into one chronological timeline with terminal-state exit code |
| 2 | Failure cluster | `messages failures --since 24h [--group-by reason]` | 8/10 | Priya, Marcus | Local SQL `GROUP BY reason` over synced interactions; Bird has no aggregation endpoint |
| 3 | SMS body search | `sms search "<text>" [--from +31... \| --to +31...]` | 9/10 | all | FTS5 over `messages.body` joined to `participants.identifierValue` |
| 4 | CSV bulk dispatch | `sms send-batch --csv recipients.csv --body-template ...` | 9/10 | Marcus | One real `POST` per row + deterministic per-row idempotency key + batch persisted to local store |
| 5 | Batch reconcile | `sms reconcile <batch-id> [--retry-failed]` | 9/10 | Marcus | Reads persisted batch + calls real `/interactions` per message; folds outcomes; pairs with #4 |
| 6 | Conversation timeline | `conversations timeline <conv-id>` | 8/10 | Devon, Sana | SQL join over `conversations` + `messages` + `participants` + `interactions` — no API call returns this view |
| 7 | Cross-channel customer history | `messages from <e164>` | 8/10 | Devon, Priya | Local join: `participants(identifierValue=...)` → `conversations` → `messages` chronologically |
| 8 | Tenant readiness check | `tenant doctor [--test-contact <id>]` | 10/10 | Sana, Priya | Five real API calls (channels, channel-config, antispam, compliance keywords, messageability) sequenced into one checklist with single exit code |
| 9 | Compliance auto-block | `compliance auto-block --since 7d [--apply]` | 7/10 | Marcus, Priya | Local FTS over inbound STOP-keyword fires → CSV; `--apply` pipes to real `workspace rules bulk-add`. Print-by-default side-effect rule honored |

**Total: 9 transcendence features (all scoring ≥ 7/10). No stubs.**

## Reprint deltas vs prior CLI (12 → 9)

Three prior transcendence features were dropped after the v4.5.2 subagent re-scored them
against current personas. All drops are documented for review:

- **`sms tail`** — Drop. Single-poll value is thin once `messages failures` and `sms search`
  ship; risks scope drift toward "monitor". Failed Sibling Kill against #2 + #3.
- **`conversations stale`** — Drop. Survives kill checks but loses the second-pass cut to
  the 8 stronger survivors; the same answer is reachable via `conversations timeline` +
  a `--status active` filter on the absorbed `conversations list`.
- **`workspace rules diff`** — Drop. Thin wrapper once `compliance auto-block --apply`
  exists and `workspace rules bulk-add` is absorbed. Failed Wrapper-vs-Leverage.

## Killed candidates

See `2026-05-13-091340-novel-features-brainstorm.md` for the full audit trail.
