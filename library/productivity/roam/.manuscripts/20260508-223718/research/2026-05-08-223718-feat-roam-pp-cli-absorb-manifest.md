# Roam Absorb Manifest

## Absorbed (match or beat every Roam HQ surface)

The generator emits one typed Cobra command per endpoint across all 5 specs (58 endpoints total). Every absorbed command supports `--json`, `--select`, `--csv`, `--dry-run` (mutations), and typed exit codes. Examples:

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Send chat (HQ) | `/chat.sendMessage` (HQ v1) | `chat sendmessage` | --stdin batch, idempotent client-msg-id |
| 2 | Chat CRUD (Alpha) | `/chat.{post,update,delete,list,history,typing}` | `chat post/update/delete/list/history/typing` | Local FTS5 mirror via sync |
| 3 | Reactions, uploads | `/reaction.add`, `/item.upload` | `reaction add`, `item upload` | --dry-run |
| 4 | Transcript list/info/prompt | `/transcript.{list,info,prompt}` | `transcript list/get/prompt` | Local SQLite + FTS5 |
| 5 | Meetings + lobby | `/meeting.list`, `/lobby.list`, `/lobbyBooking.list` | `meeting list`, `lobby list/bookings` | Local cache |
| 6 | On-Air events | `/onair.event.{create,update,cancel,list,info}` | `onair event create/update/cancel/list/get` | --dry-run, --stdin |
| 7 | On-Air guests | `/onair.guest.{add,update,remove,list,info}` | `onair guest add/update/remove/list/get` | Stdin batch RSVP changes |
| 8 | On-Air attendance | `/onair.attendance.list` | `onair attendance list` | CSV export |
| 9 | Recordings, magicasts, groups | `/recording.list`, `/magicast.{list,info}`, `/groups.list` | mirror commands | Local store + sync |
| 10 | User lookup + audit log | `/user.{list,info,lookup}`, `/userauditlog.list` | `user list/get/lookup`, `auditlog list` | Local mirror |
| 11 | Meeting links | `/meetinglink.{create,info,update}` | `meetinglink create/get/update` | Idempotent |
| 12 | Group admin (Alpha) | `/group.{create,rename,archive,members,add,remove}` | `group create/rename/archive/members/add/remove` | --dry-run |
| 13 | App management | `/app.uninstall` | `app uninstall` | --dry-run default |
| 14 | Token info | `/token.info` | `auth status` | Tier classification |
| 15 | SCIM users | `/Users` GET/POST/PUT/PATCH/DELETE/{id} | `scim users list/create/get/update/delete/patch` | --stdin batch |
| 16 | SCIM groups | `/Groups` GET/POST/PUT/PATCH/DELETE/{id} | `scim groups list/create/get/update/delete/patch` | Idempotent |
| 17 | SCIM metadata | `/ServiceProviderConfig`, `/ResourceTypes`, `/Schemas` | `scim service-provider`, `scim resource-types`, `scim schemas` | JSON-native |
| 18 | Webhooks | `/webhook.{subscribe,unsubscribe}` | `webhook subscribe/unsubscribe` | Local subscription registry |
| 19 | Compliance export | `/messageevent.export` | `compliance export` | Background-friendly |

Every endpoint above ships from the merged 58-path OpenAPI spec.

## Transcendence (only possible with our approach — local SQLite + cross-spec join + agent-native)

| # | Feature | Command | Score | Persona | Why Only We Can Do This |
|---|---------|---------|-------|---------|-------------------------|
| 1 | Cross-resource grep | `roam grep "<query>" [--since --from-user --in-meeting --in-group]` | 9 | Priya | FTS5 join across `messages` + `transcripts` in local store; Roam's own search is meeting-scoped |
| 2 | Decision extractor | `roam decisions --since 7d [--in-group]` | 7 | Priya | Regex scan of locally-synced transcript text for decision-anchored phrases; no API call, no LLM |
| 3 | Attendance drift | `roam onair attendance drift --event <id>` | 8 | Sam | Local SQL join of `guests` and `attendance` tables; emits invited-no-show + walk-in sets |
| 4 | Stale on-air reaper | `roam onair reaper [--stale-days 60] [--apply]` | 7 | Sam | Local aggregation of attendance over time + absorbed cancel; --dry-run default |
| 5 | Transcript prompt fan-out | `roam transcript fanout --question "X" [--since 7d]` | 7 | Priya | Iterates `/transcript.prompt` over a date-range set from local store; one row per transcript with citation |
| 6 | Stdin chat relay | `roam relay --to <group> [--idempotent-key-prefix p]` | 8 | Marcus | Stream loop over `/chat.post` with deterministic client-msg-id from line hash; honors Retry-After |
| 7 | Webhook tail | `roam webhook tail [--since 1h]` | 6 | Marcus | One-shot read of local subscription registry + delivery log table |
| 8 | Token tier doctor | `roam doctor token` | 7 | Elena, Marcus | Calls /token.info + probes one GET per spec family; prints 5-row pass/fail matrix |
| 9 | SCIM roster diff | `roam scim diff --roster users.csv [--apply]` | 8 | Elena | CSV + SCIM list + set diff; --apply runs SCIM CRUD with --dry-run default |
| 10 | Mention inbox | `roam mention inbox [--user @me] [--since 7d]` | 6 | Priya, Marcus | Local FTS over `messages.text` for `@user` tokens, joined with group + sender |

All 10 score ≥6/10 and survive the 4 cut questions (weekly use, not-a-wrapper, transcendence proof, sibling kill). Killed: 8 candidates (compliance-export-resume, summarize-transcript, standup-digest, guest-bulkrsvp-csv, recording-orphans, ratelimit-budget, digest-daily, meeting-reaper). Audit trail at `2026-05-08-223718-novel-features-brainstorm.md`.
