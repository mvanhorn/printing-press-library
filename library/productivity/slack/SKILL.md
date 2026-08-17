---
name: pp-slack
description: "Every Slack read your token can make, mirrored to local SQLite so your history outlives Slack's own retention wall. Trigger phrases: `search old slack messages`, `what did I miss in slack`, `which slack channels are dying`, `find unanswered slack threads`, `who is this slack user id`, `use slack-pp-cli`, `run slack`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - slack-pp-cli
    install:
      - kind: go
        bins: [slack-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/slack/cmd/slack-pp-cli
---

# Slack — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `slack-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install slack --cli-only
   ```
2. Verify: `slack-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/slack/cmd/slack-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Slack hides messages past the free-plan retention window and gates export behind admin. This CLI syncs conversations, users, files, and reactions into a local SQLite database with full-text search, so `archive recall` finds decisions Slack itself will no longer serve you. On top of the mirror it computes things no endpoint returns: `catchup` for what is still waiting on you, `threads stale` for unanswered threads, and `health` for which channels are dying.

## When to Use This CLI

Reach for this CLI when the question is about Slack content rather than Slack app development, and when the answer benefits from history, aggregation, or repeated querying. It is the right tool for finding old decisions, summarizing what you missed, auditing which channels are alive, and resolving opaque Slack IDs into people. It is also the right tool when you hold only a bot token, because the local mirror answers search-shaped questions that Slack's own search endpoint refuses to serve bot tokens.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to build, scaffold, or deploy a Slack app; that is what the official slackapi/slack-cli does.
- Do not use this CLI for Enterprise Grid administration; admin.* methods are deliberately excluded and would fail with a normal workspace token.
- Do not use this CLI as a real-time bot or event listener; it is a request-response tool with no Socket Mode or Events API runtime.
- Do not use this CLI to read a workspace you have not synced; every archive-backed command reports an unsynced mirror rather than silently returning empty.
- Do not use this CLI for compliance or legal export; Slack's Discovery API and admin export exist for that and carry guarantees this tool does not.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local archive that outlives Slack
- **`archive recall`** — Find messages in your local archive, including ones Slack has already hidden behind the 90-day retention wall.

  _Reach for this instead of search when the answer may be older than 90 days, or when you only hold a bot token, since Slack's search endpoint rejects bot tokens outright._

  ```bash
  slack-pp-cli archive recall "deploy" --agent --limit 20
  ```
- **`archive coverage`** — Show what date range your local mirror actually holds per channel, and where the gaps are.

  _Check this before trusting an empty recall result, so you can tell a genuine absence from an unsynced range._

  ```bash
  slack-pp-cli archive coverage --json
  ```

### Obligation over volume
- **`catchup`** — See what happened while you were away: new volume per channel, messages that mention you, and threads still waiting on your reply.

  _Use this as the single call that replaces listing channels, pulling history per channel, and resolving mentions by hand._

  ```bash
  slack-pp-cli catchup --since 24h --agent
  ```
- **`threads stale`** — List threads across the archive where the last reply is not yours and nobody has answered since.

  _Pick this when the question is what is unanswered, rather than what is recent._

  ```bash
  slack-pp-cli threads stale --days 3 --json
  ```

### Workspace stewardship
- **`health`** — Compare your channels by messages per day, distinct posters, median first-reply latency, and days idle.

  _Use this for archive-candidate decisions when you are not a workspace admin and have no export or admin console._

  ```bash
  slack-pp-cli health --dying --json
  ```
- **`users activity`** — Profile where one person posts, which threads they carry, and when they were last seen.

  _Reach for this for handoff and standup context about a named teammate, not about yourself._

  ```bash
  slack-pp-cli users activity chris --days 30 --json
  ```

### Identity resolution
- **`users whois`** — Turn an opaque Slack ID, handle, or email into one card with shared channels, timezone, DND state, and last-seen.

  _Use this the moment a payload hands you a bare U-prefixed or C-prefixed ID, instead of calling users info per identifier._

  ```bash
  slack-pp-cli users whois U04AB9XYZ --agent
  ```

## Command Reference

**api-test** — Manage api test

- `slack-pp-cli api-test` — Checks API calling code.

**auth_api** — Test and manage authentication

- `slack-pp-cli auth-api revoke` — Revoke the current authentication token
- `slack-pp-cli auth-api test` — Test the authentication token and get identity info

**bots** — Get information about bot users

- `slack-pp-cli bots` — Get information about a bot user

**canvases** — Create, edit, share, and delete Slack canvases

- `slack-pp-cli canvases create` — Create a canvas, standalone or bound to a channel
- `slack-pp-cli canvases read` — Read a canvas's content
- `slack-pp-cli canvases edit` — Apply a change to an existing canvas
- `slack-pp-cli canvases delete` — Delete a canvas
- `slack-pp-cli canvases access-set` — Grant users or channels access to a canvas
- `slack-pp-cli canvases sections` — Look up section IDs in a canvas, for targeted edits

Needs `canvases:write` + `canvases:read`, and `files:read` for `read`. `read` returns
**HTML, not Markdown** (Slack has no get-canvas-content endpoint, so it downloads the
backing file) — so create/read is not a lossless round trip. To target an edit, read
the canvas and take the section id from the HTML; `sections` returns ids without the
content they cover.

**chat-delete-scheduled-message** — Manage chat delete scheduled message

- `slack-pp-cli chat-delete-scheduled-message` — Deletes a pending scheduled message from the queue.

**chat-me-message** — Manage chat me message

- `slack-pp-cli chat-me-message` — Share a me message into a channel.

**chat-post-ephemeral** — Manage chat post ephemeral

- `slack-pp-cli chat-post-ephemeral` — Sends an ephemeral message to a user in a channel.

**chat-unfurl** — Manage chat unfurl

- `slack-pp-cli chat-unfurl` — Provide custom unfurl behavior for user-posted URLs

**conversations** — Read channel history, list channels, manage channel membership

- `slack-pp-cli conversations archive` — Archive a channel
- `slack-pp-cli conversations create` — Create a new channel
- `slack-pp-cli conversations get` — Get information about a channel
- `slack-pp-cli conversations history` — Fetch message history for a channel
- `slack-pp-cli conversations invite` — Invite users to a channel
- `slack-pp-cli conversations list` — List all channels in the workspace
- `slack-pp-cli conversations mark` — Mark a channel as read up to a specific message
- `slack-pp-cli conversations members` — List members of a channel
- `slack-pp-cli conversations replies` — Fetch replies in a thread
- `slack-pp-cli conversations set-purpose` — Set the purpose for a channel
- `slack-pp-cli conversations set-topic` — Set the topic for a channel
- `slack-pp-cli conversations unarchive` — Unarchive a channel

**conversations-close** — Manage conversations close

- `slack-pp-cli conversations-close` — Closes a direct message or multi-person direct message.

**conversations-join** — Manage conversations join

- `slack-pp-cli conversations-join` — Joins an existing conversation.

**conversations-kick** — Manage conversations kick

- `slack-pp-cli conversations-kick` — Removes a user from a conversation.

**conversations-leave** — Manage conversations leave

- `slack-pp-cli conversations-leave` — Leaves a conversation.

**conversations-open** — Manage conversations open

- `slack-pp-cli conversations-open` — Opens or resumes a direct message or multi-person direct message.

**conversations-rename** — Manage conversations rename

- `slack-pp-cli conversations-rename` — Renames a conversation.

**dnd** — Manage Do Not Disturb settings

- `slack-pp-cli dnd end-dnd` — End the current Do Not Disturb session immediately
- `slack-pp-cli dnd end-snooze` — End the current Do Not Disturb session
- `slack-pp-cli dnd get` — Get DND status for the authenticated user
- `slack-pp-cli dnd set-snooze` — Turn on Do Not Disturb for a specified number of minutes
- `slack-pp-cli dnd team-info` — Get DND status for multiple users

**emoji** — List custom emoji in the workspace

- `slack-pp-cli emoji` — List all custom emoji for the workspace

**files** — Upload, list, and manage files

- `slack-pp-cli files delete` — Delete a file
- `slack-pp-cli files get` — Get information about a file
- `slack-pp-cli files list` — List files in the workspace
- `slack-pp-cli files upload` — Upload a file to Slack. `--file <path>`, or
  `--content` with `--filename` for inline text; `--channel` shares it, `--thread-ts`
  puts it in a thread. Runs the external upload flow (the old `files.upload` is
  retired). `--channels` takes one channel only.

**files-comments-delete** — Manage files comments delete

- `slack-pp-cli files-comments-delete` — Deletes an existing comment on a file.

**files-remote-add** — Manage files remote add

- `slack-pp-cli files-remote-add` — Adds a file from a remote service

**files-remote-info** — Manage files remote info

- `slack-pp-cli files-remote-info` — Retrieve information about a remote file added to Slack

**files-remote-list** — Manage files remote list

- `slack-pp-cli files-remote-list` — Retrieve information about a remote file added to Slack

**files-remote-remove** — Manage files remote remove

- `slack-pp-cli files-remote-remove` — Remove a remote file.

**files-remote-share** — Manage files remote share

- `slack-pp-cli files-remote-share` — Share a remote file into a channel.

**files-remote-update** — Manage files remote update

- `slack-pp-cli files-remote-update` — Updates an existing remote file.

**files-revoke-public-url** — Manage files revoke public url

- `slack-pp-cli files-revoke-public-url` — Revokes public/external sharing access for a file

**files-shared-public-url** — Manage files shared public url

- `slack-pp-cli files-shared-public-url` — Enables a file for public/external sharing.

**messages** — Send, read, update, and delete messages in channels and DMs

- `slack-pp-cli messages delete-message` — Delete a message
- `slack-pp-cli messages get-permalink` — Get a permalink URL for a message
- `slack-pp-cli messages list-scheduled` — List scheduled messages
- `slack-pp-cli messages post-message` — Send a message to a channel, DM, or thread
- `slack-pp-cli messages schedule-message` — Schedule a message for later delivery
- `slack-pp-cli messages update-message` — Update an existing message

**pins** — Pin and unpin messages in channels

- `slack-pp-cli pins add` — Pin a message to a channel
- `slack-pp-cli pins list` — List pinned items in a channel
- `slack-pp-cli pins remove` — Unpin a message from a channel

**reactions** — Add and remove emoji reactions on messages

- `slack-pp-cli reactions add` — Add an emoji reaction to a message
- `slack-pp-cli reactions get` — Get reactions for a message
- `slack-pp-cli reactions list` — List reactions made by the authenticated user
- `slack-pp-cli reactions remove` — Remove an emoji reaction from a message

**reminders** — Create and manage personal reminders

- `slack-pp-cli reminders add` — Create a new reminder
- `slack-pp-cli reminders complete` — Mark a reminder as complete
- `slack-pp-cli reminders delete` — Delete a reminder
- `slack-pp-cli reminders get` — Get info about a reminder
- `slack-pp-cli reminders list` — List all reminders for the authenticated user

**search_api** — Search messages and files across the workspace

- `slack-pp-cli search-api` — Search for messages matching a query

**stars** — Star and unstar messages and files

- `slack-pp-cli stars add` — Star a message, file, or channel
- `slack-pp-cli stars list` — List starred items
- `slack-pp-cli stars remove` — Remove a star from an item

**team** — Get workspace information

- `slack-pp-cli team access-logs` — Get workspace access logs (requires admin)
- `slack-pp-cli team billable-info` — Get billable information for workspace users
- `slack-pp-cli team get` — Get information about the workspace

**team-integration-logs** — Manage team integration logs

- `slack-pp-cli team-integration-logs` — Gets the integration logs for the current team.

**team-profile-get** — Manage team profile get

- `slack-pp-cli team-profile-get` — Retrieve a team's profile.

**usergroups** — Manage workspace user groups

- `slack-pp-cli usergroups create` — Create a new user group
- `slack-pp-cli usergroups list` — List all user groups in the workspace
- `slack-pp-cli usergroups update` — Update an existing user group
- `slack-pp-cli usergroups users-list` — List users in a user group
- `slack-pp-cli usergroups users-update` — Update the members of a user group

**usergroups-disable** — Manage usergroups disable

- `slack-pp-cli usergroups-disable` — Disable an existing User Group

**usergroups-enable** — Manage usergroups enable

- `slack-pp-cli usergroups-enable` — Enable a User Group

**users** — List and look up workspace users

- `slack-pp-cli users get` — Get information about a user
- `slack-pp-cli users get-presence` — Get a user's online presence status
- `slack-pp-cli users list` — List all users in the workspace
- `slack-pp-cli users lookup-by-email` — Find a user by their email address
- `slack-pp-cli users profile-get` — Get a user's profile information
- `slack-pp-cli users profile-set` — Set the user's profile fields
- `slack-pp-cli users set-presence` — Set the user's presence status

**users-conversations** — Manage users conversations

- `slack-pp-cli users-conversations` — List conversations the calling user may access.

**users-delete-photo** — Manage users delete photo

- `slack-pp-cli users-delete-photo` — Delete the user profile photo

**users-identity** — Manage users identity

- `slack-pp-cli users-identity` — Get a user's identity.

**users-set-active** — Manage users set active

- `slack-pp-cli users-set-active` — Marked a user as active. Deprecated and non-functional.

**users-set-photo** — Manage users set photo

- `slack-pp-cli users-set-photo` — Set the user profile photo


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
slack-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find a decision Slack has forgotten

```bash
slack-pp-cli archive recall "deploy" --agent --select results.channel,results.user,results.ts,results.text
```

Searches the local mirror and narrows the payload to just the fields worth reading, so an agent does not burn context on full message objects.

### Morning obligation check

```bash
slack-pp-cli catchup --since 24h --json
```

Returns new volume per channel, messages mentioning you, and threads whose last reply is not yours.

### Find archive candidates without admin access

```bash
slack-pp-cli health --dying --json
```

Ranks your channels by idle days and posting volume so you can propose archives with evidence.

### Resolve an opaque ID from a payload

```bash
slack-pp-cli users whois U04AB9XYZ --agent
```

Turns a bare Slack user ID into shared channels, timezone, DND state, and last-seen in one call.

### Confirm the mirror before trusting an empty result

```bash
slack-pp-cli archive coverage --json
```

Shows the mirrored timestamp range per channel so an empty recall can be read as a sync gap rather than an absence.

## Auth Setup

Authentication is a bearer token, but Slack splits capability across two token types and no scope grant crosses that line. A bot token (`xoxb-`, set as `SLACK_BOT_TOKEN`) reaches conversations, users, files, reactions, usergroups, emoji, team, dnd, and bots. A user token (`xoxp-`, set as `SLACK_USER_TOKEN`) is required for the `search`, `stars`, and `reminders` families, which return `not_allowed_token_type` for bot tokens no matter which scopes you add. Run `doctor` to see which token types are configured and therefore which command families are reachable. Note also that Slack moved `conversations.history` and `conversations.replies` to a far stricter rate limit for apps redistributed outside the Marketplace; internal apps you create and install in your own workspace keep the original limits.

Run `slack-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  slack-pp-cli bots --bot example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `SLACK_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SLACK_CONFIG_DIR`, `SLACK_DATA_DIR`, `SLACK_STATE_DIR`, `SLACK_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SLACK_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `slack-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "slack": {
        "command": "slack-pp-mcp",
        "env": {
          "SLACK_HOME": "/srv/slack"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SLACK_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SLACK_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
slack-pp-cli recall "<user's question>" --agent
```

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "slack-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `slack-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `slack-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `slack-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
slack-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
slack-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
slack-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
slack-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`slack-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `SLACK_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
slack-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
slack-pp-cli feedback --stdin < notes.txt
slack-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SLACK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SLACK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
slack-pp-cli profile save briefing --json
slack-pp-cli --profile briefing bots --bot example-value
slack-pp-cli profile list --json
slack-pp-cli profile show briefing
slack-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `slack-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/slack/cmd/slack-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add slack-pp-mcp -- slack-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which slack-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   slack-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `slack-pp-cli <command> --help`.
