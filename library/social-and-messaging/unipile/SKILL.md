---
name: pp-unipile
description: "Every Unipile endpoint as a typed command, plus a local mirror that makes cross-provider search possible and an invitation ledger that shows how much LinkedIn headroom you have left. Trigger phrases: `check my unified inbox`, `who accepted my LinkedIn invitations`, `how many invitations can I still send today`, `find conversations nobody replied to`, `search my messages across providers`, `use unipile`, `run unipile`."
author: "fuushyn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - unipile-pp-cli
    install:
      - kind: go
        bins: [unipile-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/unipile/cmd/unipile-pp-cli
---

# Unipile — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `unipile-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install unipile --cli-only
   ```
2. Verify: `unipile-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/unipile/cmd/unipile-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Unipile unified LinkedIn, WhatsApp, Telegram, Instagram, Messenger, X, Gmail, Outlook, IMAP, and calendars into one API, then stopped at the API. This is the missing operator layer: the full endpoint surface as shell-native commands with structured errors and cursor auto-pagination, a local SQLite mirror that answers cross-provider questions no single call can, and a budget command that counts invitations already sent against the caps LinkedIn enforces but Unipile does not.

## When to Use This CLI

Reach for this CLI when a task touches messages, email, calendars, or LinkedIn activity across more than one provider, or when the answer requires history rather than a single live call. It is the right tool for triaging a unified inbox, building follow-up lists from prior conversations, measuring outreach conversion, and staying under LinkedIn's rate caps. It is also the fastest path to any single Unipile endpoint from a shell or a script.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for Unipile API v2; this targets v1 and v2 tenants use a different DSN and endpoint surface.
- Do not use this CLI to connect a brand new provider account interactively; the hosted auth wizard in the Unipile dashboard is the supported path for first-time OAuth and QR flows.
- Do not use this CLI as a webhook receiver; it can register webhooks but your own server must accept the callbacks.
- Do not use this CLI to bulk-send invitations beyond what 'budget' reports as remaining; exceeding LinkedIn's caps risks restricting the underlying account.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Account safety
- **`budget`** — See how many LinkedIn invitations you have left today and this week, counted from your synced invitation history.

  _Check this before any bulk invitation run so you do not get the underlying LinkedIn account restricted._

  ```bash
  unipile-pp-cli budget --agent
  ```

### Local state that compounds
- **`search`** — Full-text search every message, email, attendee, and relation across all connected providers at once, offline.

  _Use this instead of N per-provider list calls when you need to find where something was said._

  ```bash
  unipile-pp-cli search "pricing" --agent --limit 20
  ```
- **`digest`** — What changed across every connected provider since the last sync.

  _Run this after sync to get a single catch-up summary instead of polling nine surfaces._

  ```bash
  unipile-pp-cli digest --agent
  ```

### Cross-provider views
- **`contact`** — Everything the local mirror knows about one person: connection state, invitation history, and every conversation with per-direction message counts.

  _Reach for this before writing to someone, so the message is informed by every prior touch across every provider._

  ```bash
  unipile-pp-cli contact "Lakshya" --agent
  ```
- **`inbox`** — One table of everything unread across LinkedIn, WhatsApp, Telegram, Instagram, Messenger, and email.

  _This is the daily triage view; use it to decide what to answer before opening any provider UI._

  ```bash
  unipile-pp-cli inbox --agent --limit 25
  ```
- **`thread`** — Read one conversation end to end with attendee IDs resolved to real names.

  _Use this when you need the full context of a conversation in one readable payload._

  ```bash
  unipile-pp-cli thread --chat example-chat-id --agent
  ```

### Outreach loop
- **`silent`** — Find conversations where you sent the last message and got no reply for N days.

  _Use this to build a follow-up list without re-reading every thread._

  ```bash
  unipile-pp-cli silent --days 7 --agent
  ```
- **`accepted`** — New LinkedIn connections since your last sync that you have not messaged yet.

  _This is the highest-conversion follow-up queue in outreach; use it right after a sync._

  ```bash
  unipile-pp-cli accepted --since 7d --agent
  ```
- **`funnel`** — Sent, accepted, and replied counts with conversion rates over a time window.

  _Use this to judge whether outreach copy is working before scaling send volume._

  ```bash
  unipile-pp-cli funnel --weeks 4 --agent
  ```
- **`engagement`** — Who reacted to or commented on your posts, flagged by whether they are already a connection.

  _Use this to turn warm post engagement into a targeted invitation list._

  ```bash
  unipile-pp-cli engagement --agent --limit 20
  ```

## Command Reference

**accounts** — Accounts  management

- `unipile-pp-cli accounts connect` — Link to Uniple an account of the given type and provider.
- `unipile-pp-cli accounts delete` — Unlink the given account to Unipile.
- `unipile-pp-cli accounts get` — Retrieve the details of an account.
- `unipile-pp-cli accounts list` — Returns a list of the accounts linked to Unipile.
- `unipile-pp-cli accounts reconnect` — Reconnect an account previously linked to Unipile that has been disconnected.
- `unipile-pp-cli accounts resend-checkpoint` — Might it be 2FA, OTP or In-app Validation, this route makes you able on certain providers to resend the notification.
- `unipile-pp-cli accounts solve-checkpoint` — Allows you to provide a code which will solve a checkpoint encountered during a native authentication.
- `unipile-pp-cli accounts update` — Update the proxy configuration of an existing account.

**calendars** — Calendars management

- `unipile-pp-cli calendars get` — Retrieve the details of a calendar.
- `unipile-pp-cli calendars list` — Returns a list of calendars.

**chat-attendees** — Manage chat attendees

- `unipile-pp-cli chat-attendees get` — The id of the wanted attendee.
- `unipile-pp-cli chat-attendees list` — Returns a list of messaging attendees. Some optional parameters are available to filter the results.

**chats** — Manage chats

- `unipile-pp-cli chats delete` — Delete a chat. Supported for WhatsApp and LinkedIn only.
- `unipile-pp-cli chats get` — Retrieve the details of a chat.
- `unipile-pp-cli chats list` — Returns a list of chats. Some optional parameters are available to filter the results.
- `unipile-pp-cli chats start` — Start a new conversation with one or more attendee.
- `unipile-pp-cli chats update` — Perform an action like changing the read status, muting the chat, retrieving a group invite link, etc.

**drafts** — Manage drafts

- `unipile-pp-cli drafts` — ⚠️ Interactive documentation does not work on this route (child parameters not correctly applied in snippet)

**emails** — Emails management

- `unipile-pp-cli emails delete` — Delete an email by moving it to the Trash folder.
- `unipile-pp-cli emails get` — Retrieve the details of an email.
- `unipile-pp-cli emails list` — Returns a list of emails.
- `unipile-pp-cli emails list-contacts` — Returns a list of contacts from the email provider. Supported for Gmail (Google OAuth) and Microsoft (Outlook) only.
- `unipile-pp-cli emails send` — ⚠️ Interactive documentation does not work on this route (child parameters not correctly applied in snippet)
- `unipile-pp-cli emails update` — Update an email.

**folders** — Manage folders

- `unipile-pp-cli folders get` — Retrieve the details of a mail folder.
- `unipile-pp-cli folders list` — Returns a list of all email folders.

**hosted** — Manage hosted

- `unipile-pp-cli hosted` — Create a url which redirect to Unipile's hosted authentication to connect or reconnect an account.

**linkedin** — Manage linkedin

- `unipile-pp-cli linkedin action-user` — Add a candidate to a Recruiter pipeline, save a Sales Navigator lead, etc.
- `unipile-pp-cli linkedin close-jobs` — Close a job offer you have posted.
- `unipile-pp-cli linkedin company` — Get a company profile from its name or ID.
- `unipile-pp-cli linkedin create-jobs` — Create a new job offer draft.
- `unipile-pp-cli linkedin endorse-profile` — This route can be used to endorse a skill of a user profile.
- `unipile-pp-cli linkedin get-applicants` — Retrieve the details of a user that has applied to a given offer. Applies to Classic job posting only.
- `unipile-pp-cli linkedin get-jobs` — Retrieve a job offer.
- `unipile-pp-cli linkedin get-projects` — Retrieve Recruiter hiring project from ID
- `unipile-pp-cli linkedin inmail-balance` — Get balance for subscribed premium features.
- `unipile-pp-cli linkedin list-applicants` — Retrieve all the users that have applied to a given offer.
- `unipile-pp-cli linkedin list-contracts` — Returns a list of your LinkedIn available contracts
- `unipile-pp-cli linkedin list-jobs` — Retrieve the job offers you have posted on LinkedIn whether they are open, closed or still drafts.
- `unipile-pp-cli linkedin list-projects` — Retrieve list of LinkedIn Recruiter hiring projects.
- `unipile-pp-cli linkedin parameters-search` — LinkedIn doesn't accept raw text as search parameters, but IDs.
- `unipile-pp-cli linkedin publish-jobs` — Publish the job posting draft you have been working on.
- `unipile-pp-cli linkedin raw` — This magic route is intended for advanced users who wish to use LinkedIn's features beyond our current capabilities.
- `unipile-pp-cli linkedin resume-applicants` — This route can be used to download the resume of a job applicant.
- `unipile-pp-cli linkedin search` — Search people and companies from the Linkedin Classic as well as Sales Navigator APIs.
- `unipile-pp-cli linkedin select-contracts` — Select a Recruiter or Sales navigator contract to be used on your account
- `unipile-pp-cli linkedin solve-checkpoint-jobs` — Solve a checkpoint to verify your member privilegies.
- `unipile-pp-cli linkedin update-jobs` — Edit an existing job posting.

**messages** — Manage messages

- `unipile-pp-cli messages delete` — Delete a message. Supported for WhatsApp and LinkedIn only.
- `unipile-pp-cli messages get` — Retrieve the details of a message.
- `unipile-pp-cli messages list` — Returns a list of messages. Some optional parameters are available to filter the results.
- `unipile-pp-cli messages update` — Edit a message. Supported for WhatsApp and LinkedIn Classic only.

**posts** — Posts features

- `unipile-pp-cli posts create` — Publish a post.
- `unipile-pp-cli posts get` — Retrieve the details of a post.
- `unipile-pp-cli posts react` — React to either a post or a post comment.

**users** — Users features

- `unipile-pp-cli users cancel-sent` — Cancel a pending invitation sent to someone.
- `unipile-pp-cli users edit-me` — Modify informations on account owner profile.
- `unipile-pp-cli users followers` — Returns a list of all the followers of the current user.
- `unipile-pp-cli users following` — Returns a list of all the followed accounts of an account.
- `unipile-pp-cli users get` — Retrieve the profile of a user.
- `unipile-pp-cli users invite` — Send an invitation to add someone to your contacts.
- `unipile-pp-cli users list-received` — Returns a list of all invitations that have been received.
- `unipile-pp-cli users list-sent` — Returns a list of all invitations sent that are pending.
- `unipile-pp-cli users me` — Retrieve informations about account owner.
- `unipile-pp-cli users relations` — Returns a list of all the relations of an account.
- `unipile-pp-cli users respond-received` — Accept or decline a connection invitation.

**webhooks** — Webhooks management

- `unipile-pp-cli webhooks create` — Create a webhook.
- `unipile-pp-cli webhooks delete` — Delete a webhook.
- `unipile-pp-cli webhooks list` — Returns a list of the webhooks.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
unipile-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Daily triage

```bash
unipile-pp-cli inbox --agent --select items.provider,items.name,items.last_message,items.timestamp
```

Returns only the four fields needed to decide what to answer, instead of the full chat payload for every unread conversation.

### Follow-up queue

```bash
unipile-pp-cli silent --days 5 --agent
```

Lists conversations where you spoke last and nobody replied, which is the list worth acting on today.

### Safe outreach check

```bash
unipile-pp-cli budget --agent
```

Reports remaining daily and weekly LinkedIn invitation and profile-view headroom before a send run starts.

### Warm connection targets

```bash
unipile-pp-cli engagement --agent --limit 20
```

Shows who engaged with your posts and is not yet a connection, which converts far better than cold invitations.

### Find where something was said

```bash
unipile-pp-cli search "contract" --agent --limit 15
```

Searches messages and emails across every provider from the local mirror, with no API call and no per-provider loop.

## Auth Setup

Unipile authenticates with an Access Token sent as the X-API-KEY header, and every customer gets their own DSN base URL. Set UNIPILE_API_KEY to your token and UNIPILE_BASE_URL to your DSN from the Unipile dashboard. If your environment blocks custom ports, Unipile also accepts the port as a query parameter on standard 443.

Run `unipile-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  unipile-pp-cli accounts list --agent --select connection_params,created_at,current_signature
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

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

- Use `--home <dir>` for one invocation, or set `UNIPILE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `UNIPILE_CONFIG_DIR`, `UNIPILE_DATA_DIR`, `UNIPILE_STATE_DIR`, `UNIPILE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `UNIPILE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `unipile-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "unipile": {
        "command": "unipile-pp-mcp",
        "env": {
          "UNIPILE_HOME": "/srv/unipile"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `UNIPILE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `UNIPILE_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
unipile-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "unipile-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `unipile-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `unipile-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `unipile-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
unipile-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
unipile-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
unipile-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
unipile-pp-cli playbook amend \
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

`unipile-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `UNIPILE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
unipile-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
unipile-pp-cli feedback --stdin < notes.txt
unipile-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `UNIPILE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `UNIPILE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
unipile-pp-cli profile save briefing --json
unipile-pp-cli --profile briefing accounts list
unipile-pp-cli profile list --json
unipile-pp-cli profile show briefing
unipile-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `unipile-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/unipile/cmd/unipile-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add unipile-pp-mcp -- unipile-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which unipile-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   unipile-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `unipile-pp-cli <command> --help`.
