---
name: pp-roam
description: "Every Roam HQ surface — chat, transcripts, On-Air events, SCIM, webhooks — in one local-first CLI with offline... Trigger phrases: `search roam transcripts`, `post a message to roam`, `what did we decide in roam`, `roam attendance report`, `use roam-pp-cli`, `run roam-pp-cli`."
author: "Greg Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - roam-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/roam/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Roam — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `roam-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install roam --cli-only
   ```
2. Verify: `roam-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/roam/cmd/roam-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Roam HQ ships a remote MCP but no CLI. roam-pp-cli unifies all five Roam HQ APIs (HQ, On-Air, Chat, SCIM, Webhooks) into a single binary with a local SQLite cache and FTS5 search across messages and transcripts. Cron-friendly chat relay, decision extraction, attendance drift, and SCIM roster diff are built-in.

## When to Use This CLI

Reach for roam-pp-cli when you need to script Roam HQ from a shell or agent: tail webhook deliveries, post deploy notifications, search transcripts across many meetings at once, reconcile SCIM membership against an HRIS roster, or extract decisions from last week's recordings. The remote MCP at api.ro.am/mcp is great for ad-hoc agent chat; this CLI is the choice when you want pipelines, cron, or local-first search.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`grep`** — Search across every chat message and meeting transcript at once with --since, --from-user, --in-meeting, --in-group filters.

  _Reach for this when an agent needs to recall what was said across many meetings without re-paging the rate-limited transcript API._

  ```bash
  roam-pp-cli grep "pricing" --since 14d --in-group eng --json --select transcript_id,line
  ```
- **`decisions`** — Surface decision-shaped lines ("we decided", "action item", "agreed", "let's go with") from synced transcripts.

  _Use when an agent owes the team a Monday recap; deterministic and citation-bearing._

  ```bash
  roam-pp-cli decisions --since 7d --in-group product --agent
  ```
- **`onair-attendance-drift`** — Compare invited guests vs actual attendance for an On-Air event; print invited-no-show and walk-in sets.

  _Use when an agent is asked who didn't show or who attended without an invite._

  ```bash
  roam-pp-cli onair-attendance-drift --event evt_123 --json
  ```
- **`webhook-tail`** — Tail recent webhook deliveries from the local subscription registry, --since filtered.

  _Use when debugging a webhook integration without standing up a listener._

  ```bash
  roam-pp-cli webhook-tail --since 1h --json
  ```
- **`mention-inbox`** — Local FTS over messages.text for @user tokens with --since filter; tail format.

  _Use when an agent must surface unread mentions across all groups without round-tripping the API per group._

  ```bash
  roam-pp-cli mention-inbox --user @me --since 7d --agent
  ```

### Mutation safety
- **`onair-reaper`** — Find recurring On-Air events with zero attendance over N days. --apply cancels them via the absorbed cancel endpoint.

  _Use when cleaning up dead recurring events; safe by default._

  ```bash
  roam-pp-cli onair-reaper --stale-days 60 --dry-run
  ```
- **`scim-diff`** — Diff a CSV/JSON HRIS roster against /Users SCIM list; print add/update/remove sets. --apply runs SCIM CRUD.

  _Use when an agent needs to reconcile an external roster with Roam membership without clicking through admin UI._

  ```bash
  roam-pp-cli scim-diff --roster hris.csv --apply --dry-run
  ```

### Agent-native plumbing
- **`transcript-fanout`** — Run a single question against every transcript in a date range; one row per transcript with answer + citation.

  _Use when an agent must scan many meetings for a single question without re-prompting each one._

  ```bash
  roam-pp-cli transcript-fanout --question "did anyone mention Q3 hiring?" --since 30d --agent
  ```
- **`relay`** — Pipe arbitrary stdin lines into a Roam group via /chat.post with deterministic idempotency keys and 429 backoff.

  _Use when an agent needs to forward a stream (CI, alerts, logs) into Roam without writing a custom webhook._

  ```bash
  tail -F deploys.log | roam-pp-cli relay --to eng-deploys --idempotent-key-prefix deploys
  ```

### Reachability mitigation
- **`doctor token`** — Probe one representative GET per spec family (HQ, On-Air, Chat, SCIM, Webhooks) and print which families this key can reach.

  _Use when an agent needs to know which Roam commands will work with the credential it has before attempting them._

  ```bash
  roam-pp-cli doctor token
  ```

## Command Reference

**addr-info** — Manage addr info

- `roam-pp-cli addr-info` — Get information about a chat address, which is the name for any entity that may participate in a chat, such as a...

**app-uninstall** — Manage app uninstall

- `roam-pp-cli app-uninstall` — Revoke an access token and uninstall your app. On successful response, your access token will no longer be...

**chat-delete** — Manage chat delete

- `roam-pp-cli chat-delete` — Delete a previously posted bot message. The bot must own the message being deleted (matched by address ID). Personal...

**chat-history** — Manage chat history

- `roam-pp-cli chat-history` — List messages in a chat, filtered by date range (after/before). The ordering of results depends on the filter...

**chat-list** — Manage chat list

- `roam-pp-cli chat-list` — List all accessible chats, which consist of all DMs, MultiDMs, and Channels that your bot has been added to, in...

**chat-post** — Manage chat post

- `roam-pp-cli chat-post` — Post a message to a chat. Messages can be plain markdown text, rich [Block Kit](/docs/guides/block-kit) layouts, or...

**chat-send-message** — Manage chat send message

- `roam-pp-cli chat-send-message` — Sends the given message to the specified recipients. At this time, we only support sending to a single group...

**chat-typing** — Manage chat typing

- `roam-pp-cli chat-typing` — Notify other chat participants that you are working on a response. If they have the chat open, they will see '(Bot...

**chat-update** — Manage chat update

- `roam-pp-cli chat-update` — Edit a previously posted bot message. The updated message can contain plain markdown text or rich [Block...

**group-add** — Manage group add

- `roam-pp-cli group-add` — Add one or more group members and/or admins. Apps may add members to a group if one of the following conditions is...

**group-archive** — Manage group archive

- `roam-pp-cli group-archive` — Archive a group by ID. **Access:** Organization only. **Required scope:** `group:write` --- **OpenAPI Spec:**...

**group-create** — Manage group create

- `roam-pp-cli group-create` — Create a group address that can be used for chat. Groups which specify an admin will operate in an 'Admin only'...

**group-members** — Manage group members

- `roam-pp-cli group-members` — List members in a group. Apps may list members if one of the following conditions is true: 1. It is a public group...

**group-remove** — Manage group remove

- `roam-pp-cli group-remove` — Remove one or more group members. Apps may remove members from a group if one of the following conditions is true:...

**group-rename** — Manage group rename

- `roam-pp-cli group-rename` — Rename a group by ID. Apps may only rename groups for which they are an admin. **Access:** Organization only....

**groups** — Manage groups

- `roam-pp-cli groups 02-create` — Creates a new group in your Roam organization. **Required fields:** `displayName` (max 64 characters). **Optional:**...
- `roam-pp-cli groups 02-delete` — Archives a group in Roam. This is a **soft delete** — the group data is retained but becomes inactive....
- `roam-pp-cli groups 02-get` — Retrieves a single group by its Roam Address ID. The response includes the group's `displayName` and `members` array...
- `roam-pp-cli groups 02-list` — Returns a paginated list of groups in your Roam organization. **Pagination:** Use `startIndex` (1-based) and `count`...
- `roam-pp-cli groups 02-patch` — Partially updates a group. Use this to add or remove members without replacing the entire group. **Supported...
- `roam-pp-cli groups 02-replace` — Fully replaces a group's attributes. The entire `members` list is replaced with the provided values. **Required...

**groups-list** — Manage groups list

- `roam-pp-cli groups-list` — Lists all public, non-archived groups in your home Roam. **Access:** Organization only. **Required scope:**...

**item-upload** — Manage item upload

- `roam-pp-cli item-upload` — Upload a file so that it can be sent as a chat message attachment. The returned object contains an item ID which can...

**lobby-booking-list** — Manage lobby booking list

- `roam-pp-cli lobby-booking-list` — Lists bookings for a specific lobby configuration, filtered by date range (after/before). The ordering of results...

**lobby-list** — Manage lobby list

- `roam-pp-cli lobby-list` — Lists active lobbies in your account. A lobby URL has the form `ro.am/{handle}` or `ro.am/{handle}/{slug}`. - The...

**magicast-info** — Manage magicast info

- `roam-pp-cli magicast-info` — Retrieve a magicast by ID. **Access:** Organization and Personal. In Personal mode, only magicasts owned by the...

**magicast-list** — Manage magicast list

- `roam-pp-cli magicast-list` — Lists all magicasts in your Roam, sorted in reverse chronological order. **Access:** Organization and Personal. In...

**meeting-list** — Manage meeting list

- `roam-pp-cli meeting-list` — Lists all meetings in your home Roam, filtered by date range (after/before). The ordering of results depends on the...

**meetinglink-create** — Manage meetinglink create

- `roam-pp-cli meetinglink-create` — Create a meeting link. **Access:** Organization and Personal. In Organization mode, specify the host by email. In...

**meetinglink-info** — Manage meetinglink info

- `roam-pp-cli meetinglink-info` — Get a meeting link. **Access:** Organization only. **Required scope:** `meetinglink:read` --- **OpenAPI Spec:**...

**meetinglink-update** — Manage meetinglink update

- `roam-pp-cli meetinglink-update` — Update a meeting link. **Access:** Organization only. **Required scope:** `meetinglink:write` --- **OpenAPI Spec:**...

**messageevent-export** — Manage messageevent export

- `roam-pp-cli messageevent-export` — Obtain a daily message event export containing DMs and group chats within your account. For customers with archival...

**onair-attendance-list** — Manage onair attendance list

- `roam-pp-cli onair-attendance-list` — Returns the attendance report for an On-Air event, combining RSVP data with join/duration information. Guests with...

**onair-event-cancel** — Manage onair event cancel

- `roam-pp-cli onair-event-cancel` — Cancels an On-Air event. This action cannot be undone. **Access:** Organization and Personal. **Required scope:**...

**onair-event-create** — Manage onair event create

- `roam-pp-cli onair-event-create` — Creates a new On-Air event. The calendar host must be a member of the organization. When using a personal access...

**onair-event-info** — Manage onair event info

- `roam-pp-cli onair-event-info` — Returns details for a single On-Air event. **Access:** Organization and Personal. **Required scope:** `onair:read`...

**onair-event-list** — Manage onair event list

- `roam-pp-cli onair-event-list` — Returns a paginated list of On-Air events for the organization. Results are sorted by start time in descending order...

**onair-event-update** — Manage onair event update

- `roam-pp-cli onair-event-update` — Updates an existing On-Air event. Only the fields provided in the request body are updated; omitted fields remain...

**onair-guest-add** — Manage onair guest add

- `roam-pp-cli onair-guest-add` — Adds one or more guests to an On-Air event. **Access:** Organization and Personal. **Required scope:** `onair:write`...

**onair-guest-info** — Manage onair guest info

- `roam-pp-cli onair-guest-info` — Returns details for a single On-Air event guest. **Access:** Organization and Personal. **Required scope:**...

**onair-guest-list** — Manage onair guest list

- `roam-pp-cli onair-guest-list` — Returns a paginated list of guests for an On-Air event. Optionally filter by RSVP status. Results are sorted by...

**onair-guest-remove** — Manage onair guest remove

- `roam-pp-cli onair-guest-remove` — Removes a guest from an On-Air event. **Access:** Organization and Personal. **Required scope:** `onair:write` ---...

**onair-guest-update** — Manage onair guest update

- `roam-pp-cli onair-guest-update` — Updates the RSVP status of a guest. **Access:** Organization and Personal. **Required scope:** `onair:write` ---...

**reaction-add** — Manage reaction add

- `roam-pp-cli reaction-add` — Add a reaction to a message in a chat. **Access:** Organization only. **Required scope:** `chat:send_message` or...

**recording-list** — Manage recording list

- `roam-pp-cli recording-list` — Lists all recordings in your home Roam, filtered by date range (after/before). The ordering of results depends on...

**resource-types** — Manage resource types

- `roam-pp-cli resource-types` — Returns the list of resource types supported by Roam: `User` and `Group`. **No authentication required** for this...

**schemas** — Manage schemas

- `roam-pp-cli schemas 03-metadata-get` — Returns the definition of a specific SCIM schema by its URN identifier. **No authentication required** for this...
- `roam-pp-cli schemas 03-metadata-list` — Returns all SCIM schemas supported by Roam, including the core User and Group schemas plus Roam's custom role...

**service-provider-config** — Manage service provider config

- `roam-pp-cli service-provider-config` — Returns Roam's SCIM capabilities and supported features. Use this endpoint to discover which SCIM operations are...

**test** — Manage test

- `roam-pp-cli test` — Test endpoint

**token-info** — Manage token info

- `roam-pp-cli token-info` — Get information about the access token, such as the Chat Address. **Access:** Organization and Personal. **No...

**transcript-info** — Manage transcript info

- `roam-pp-cli transcript-info` — Retrieve a transcript by ID from your home Roam. Works for both completed and live (ongoing) meetings. For live...

**transcript-list** — Manage transcript list

- `roam-pp-cli transcript-list` — Lists all transcripts in your home Roam, filtered by date range (after/before). This endpoint returns transcript...

**transcript-prompt** — Manage transcript prompt

- `roam-pp-cli transcript-prompt` — Ask a question about a specific meeting transcript and receive an AI-generated answer based on its content. Use...

**user-info** — Manage user info

- `roam-pp-cli user-info` — Get detailed information about a single user by ID. **Access:** Organization only. **Required scope:** `user:read`...

**user-list** — Manage user list

- `roam-pp-cli user-list` — List all users in the account. Users are returned in the order they were added to the account. **Access:**...

**user-lookup** — Manage user lookup

- `roam-pp-cli user-lookup` — Look up users in the account by email. **Access:** Organization only. **Required scopes:** `user:read` and...

**userauditlog-list** — Manage userauditlog list

- `roam-pp-cli userauditlog-list` — Get a list of user audit log entries for the account. **Access:** Organization only. **Required scope:**...

**users** — Manage users

- `roam-pp-cli users 01-create` — Provisions a new user in your Roam organization. **Required fields:** `userName`, `name.givenName`,...
- `roam-pp-cli users 01-delete` — Archives a user in Roam. This is a **soft delete** — the user's data is retained but they lose access....
- `roam-pp-cli users 01-get` — Retrieves a single user by their Roam Person ID. The response includes all user attributes, the `active` status, and...
- `roam-pp-cli users 01-list` — Returns a paginated list of users in your Roam organization. **Filtering:** Supports SCIM filter expressions, e.g.,...
- `roam-pp-cli users 01-patch` — Partially updates a user. Roam supports SCIM PATCH for Users with **limited semantics**. **Supported operations:** -...
- `roam-pp-cli users 01-replace` — Fully replaces a user's attributes. You must provide all required fields in the request body. **Roam-specific...

**webhook-subscribe** — Manage webhook subscribe

- `roam-pp-cli webhook-subscribe` — Create or update a webhook subscription for a given event. If a subscription already exists for the same event and...

**webhook-unsubscribe** — Manage webhook unsubscribe

- `roam-pp-cli webhook-unsubscribe` — Remove a webhook subscription by ID. **Access:** Organization only. **Required scope:** `webhook:write` ---...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
roam-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily standup digest

```bash
roam-pp-cli decisions --since 1d --in-group standup --agent --select transcript_id,line,speaker
```

Pull yesterday's decisions from the standup group, formatted for agent consumption with citation.

### Cross-meeting question scan

```bash
roam-pp-cli transcript-fanout --question "did anyone mention Q3 hiring?" --since 30d --agent
```

Run one question against every transcript in the last 30 days.

### Reconcile HRIS roster with Roam SCIM

```bash
roam-pp-cli scim-diff --roster hris.csv --apply --dry-run
```

See add/update/remove sets before applying; drop --dry-run to execute.

### Pipe CI alerts into Roam

```bash
tail -F /var/log/ci.log | roam-pp-cli relay --to eng-alerts --idempotent-key-prefix ci
```

Stream CI log lines into a Roam group with deterministic dedupe.

### Narrow a deeply-nested transcript response

```bash
roam-pp-cli transcript-info --id <id> --json --select segments.speaker,segments.text
```

Use --select with dotted paths to keep agent context small on long transcripts.

## Auth Setup

Bearer auth via ROAM_API_KEY. Roam supports two key tiers: full-access organization keys and personal access tokens. Some HQ v1 endpoints require full-access; Chat/Transcript v0 endpoints work with PATs. Run `roam-pp-cli doctor token` to see which families your key can reach.

Run `roam-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  roam-pp-cli addr-info --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
roam-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
roam-pp-cli feedback --stdin < notes.txt
roam-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.roam-pp-cli/feedback.jsonl`. They are never POSTed unless `ROAM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ROAM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
roam-pp-cli profile save briefing --json
roam-pp-cli --profile briefing addr-info
roam-pp-cli profile list --json
roam-pp-cli profile show briefing
roam-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `roam-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add roam-pp-mcp -- roam-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which roam-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   roam-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `roam-pp-cli <command> --help`.
