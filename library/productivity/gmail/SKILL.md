---
name: pp-gmail
description: "Every Gmail API surface plus the things Gmail itself will not give you: scheduled sends, sender analytics, follow-up tracking, and an offline full-text mailbox. Trigger phrases: `check my email`, `triage my inbox`, `schedule an email for later`, `who emails me the most`, `clean up my gmail storage`, `use gmail`, `run gmail-pp-cli`."
author: "Rahul Bansal"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - gmail-pp-cli
    install:
      - kind: go
        bins: [gmail-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/gmail/cmd/gmail-pp-cli
---

# Gmail — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `gmail-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install gmail --cli-only
   ```
2. Verify: `gmail-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/gmail/cmd/gmail-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

gmail-pp-cli mirrors your mailbox into local SQLite with gmail-pp-cli pull (incremental history sync), so search, sender leaderboards, storage analytics, and follow-up tracking run instantly and cost zero quota. It also fills the API's biggest gap with a local scheduled-send queue, and ships agent-native output on every command.

## When to Use This CLI

Reach for gmail-pp-cli when an agent task involves reading, searching, sending, or cleaning up Gmail: triaging unread mail, replying with correct threading, scheduling a send for later, harvesting attachments, or answering mailbox-wide questions like who emails me most or what is eating my storage. The local SQLite mirror (populated by `gmail-pp-cli pull`) makes it the right choice for any question spanning more than a page of messages, where raw API calls would burn quota and context.

## Do Not Use This CLI For

- **Other mail providers.** Outlook, Microsoft 365, Yahoo, Fastmail, or any generic IMAP/SMTP mailbox. This CLI speaks only the Gmail API.
- **Other Google products.** Calendar, Contacts, Drive, Sheets, Chat. Nothing here reaches beyond Gmail.
- **Workspace admin tasks.** Creating or suspending users, org-wide policies, domain settings, or acting on behalf of another user. That is GAM/Workspace Admin territory; this CLI operates on the single authenticated mailbox.
- **Writing copy that is never sent.** If the user just wants a draft written for them to review elsewhere, compose it in the conversation rather than calling `send` or `drafts create`.

## Prerequisites for local-store commands

`senders`, `storage`, `followups`, `unsub`, and `search` read the local mirror, not the live API. Run `gmail-pp-cli pull --since 30d` first, or they will correctly report an empty result. The framework's `sync` command is a no-op for Gmail: `messages.list` returns bare ids, so `pull` is the command that hydrates and stores full messages.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### What the API itself lacks
- **`schedule send`** — Write an email now and have it sent at any future time, even though the Gmail API itself has no schedule-send. Delivery requires `schedule run` from cron/launchd (or `schedule run --watch`).

  _Reach for this whenever a user asks to send an email later, at a specific time, or in another timezone; no other Gmail API tool can do it. Tell the user to wire `gmail-pp-cli schedule run` into cron, or the queue will sit undelivered._

  ```bash
  gmail-pp-cli schedule send --to prospect@example.com --subject "Following up" --body-file pitch.txt --at "2026-08-04T09:00:00"
  ```
- **`filters diff`** — Keep Gmail filters in a version-controlled YAML file; diff against live settings and apply the reconciliation plan.

  _Use when the user manages filters across machines or wants filter changes reviewed before they touch the account._

  ```bash
  gmail-pp-cli filters diff --file filters.yaml
  ```

### Local mailbox intelligence
- **`senders`** — See who actually fills your mailbox: per-sender message count, unread count, total size, and first/last seen.

  _Use this before any cleanup or unsubscribe pass to rank which senders matter; the live API cannot aggregate by sender at all. Requires a populated local mirror: run `gmail-pp-cli pull` first._

  ```bash
  gmail-pp-cli senders --limit 20 --agent
  ```
- **`storage`** — Rank senders and years by mailbox bytes consumed, with the exact cleanup command printed next to each group.

  _Use when the user is near their storage quota or asks what is taking up space in Gmail. Requires a populated local mirror: run `gmail-pp-cli pull` first._

  ```bash
  gmail-pp-cli storage --group-by sender --agent
  ```
- **`followups`** — List threads where the last word was theirs (you owe a reply) or yours (they never replied), aged past N days.

  _Use for outreach chasing (who never replied) and inbox accountability (what do I owe); no Gmail search operator can express either. Requires a populated local mirror: run `gmail-pp-cli pull` first._

  ```bash
  gmail-pp-cli followups --direction out --days 3 --agent
  ```
- **`unsub`** — Rank mailing lists by volume and unread ratio, and emit each list's unsubscribe target ready to act on.

  _Use when the user wants to cut inbox noise; the unread ratio identifies lists they never actually read. Requires a populated local mirror: run `gmail-pp-cli pull` first._

  ```bash
  gmail-pp-cli unsub --min-count 10 --agent
  ```

## Command Reference

**drafts** — Manage drafts

- `gmail-pp-cli drafts create` — Creates a new draft with the `DRAFT` label.
- `gmail-pp-cli drafts delete` — Immediately and permanently deletes the specified draft. Does not simply trash it.
- `gmail-pp-cli drafts get` — Gets the specified draft.
- `gmail-pp-cli drafts list` — Lists the drafts in the user's mailbox.
- `gmail-pp-cli drafts send` — Sends the specified, existing draft to the recipients in the `To`, `Cc`, and `Bcc` headers.
- `gmail-pp-cli drafts update` — Replaces a draft's content.

**history** — Manage history

- `gmail-pp-cli history <userId>` — Lists the history of all changes to the given mailbox.

**labels** — Manage labels

- `gmail-pp-cli labels create` — Creates a new label.
- `gmail-pp-cli labels delete` — Immediately and permanently deletes the specified label and removes it from any messages and threads that it is applied
- `gmail-pp-cli labels get` — Gets the specified label.
- `gmail-pp-cli labels list` — Lists all labels in the user's mailbox.
- `gmail-pp-cli labels patch` — Patch the specified label.
- `gmail-pp-cli labels update` — Updates the specified label.

**messages** — Manage messages

- `gmail-pp-cli messages batch-delete` — Deletes many messages by message ID.
- `gmail-pp-cli messages batch-modify` — Modifies the labels on the specified messages.
- `gmail-pp-cli messages delete` — Immediately and permanently deletes the specified message. This operation cannot be undone. Prefer `messages.
- `gmail-pp-cli messages get` — Gets the specified message.
- `gmail-pp-cli messages modify` — Modifies the labels on the specified message.
- `gmail-pp-cli messages trash` — Moves the specified message to the trash.
- `gmail-pp-cli messages untrash` — Removes the specified message from the trash.
- `gmail-pp-cli messages attachments get` — Gets the specified message attachment.
- `gmail-pp-cli messages import` — Imports a message into only this user's mailbox
- `gmail-pp-cli messages insert` — Directly inserts a message into only this user's mailbox similar to `IMAP APPEND`
- `gmail-pp-cli messages list` — Lists the messages in the user's mailbox.
- `gmail-pp-cli messages send` — Sends the specified message to the recipients in the `To`, `Cc`, and `Bcc` headers.

**settings** — Manage settings

- `gmail-pp-cli settings create` — Adds a delegate with its verification status set directly to `accepted`, without sending any verification email.
- `gmail-pp-cli settings create-gmail` — Creates a filter. Note: you can only create a maximum of 1,000 filters.
- `gmail-pp-cli settings create-gmail-2` — Creates a forwarding address.
- `gmail-pp-cli settings create-gmail-3` — Creates a custom 'from' send-as alias.
- `gmail-pp-cli settings create-gmail-4` — Creates and configures a client-side encryption identity that's authorized to send mail from the user account.
- `gmail-pp-cli settings create-gmail-5` — Creates and uploads a client-side encryption S/MIME public key certificate chain and private key metadata for the
- `gmail-pp-cli settings delete` — Removes the specified delegate (which can be of any verification status)
- `gmail-pp-cli settings delete-gmail` — Immediately and permanently deletes the specified filter.
- `gmail-pp-cli settings delete-gmail-2` — Deletes the specified forwarding address and revokes any verification that may have been required.
- `gmail-pp-cli settings delete-gmail-3` — Deletes the specified send-as alias. Revokes any verification that may have been required for using it.
- `gmail-pp-cli settings delete-gmail-4` — Deletes a client-side encryption identity.
- `gmail-pp-cli settings delete-gmail-5` — Deletes the specified S/MIME config for the specified send-as alias.
- `gmail-pp-cli settings disable` — Turns off a client-side encryption key pair.
- `gmail-pp-cli settings enable` — Turns on a client-side encryption key pair that was turned off.
- `gmail-pp-cli settings get` — Gets the specified delegate.
- `gmail-pp-cli settings get-auto-forwarding` — Gets the auto-forwarding setting for the specified account.
- `gmail-pp-cli settings get-gmail` — Gets a filter.
- `gmail-pp-cli settings get-gmail-2` — Gets the specified forwarding address.
- `gmail-pp-cli settings get-gmail-3` — Gets the specified send-as alias.
- `gmail-pp-cli settings get-gmail-4` — Retrieves a client-side encryption identity configuration.
- `gmail-pp-cli settings get-gmail-5` — Retrieves an existing client-side encryption key pair.
- `gmail-pp-cli settings get-gmail-6` — Gets the specified S/MIME config for the specified send-as alias.
- `gmail-pp-cli settings get-imap` — Gets IMAP settings.
- `gmail-pp-cli settings get-language` — Gets language settings.
- `gmail-pp-cli settings get-pop` — Gets POP settings.
- `gmail-pp-cli settings get-vacation` — Gets vacation responder settings.
- `gmail-pp-cli settings insert` — Insert (upload) the given S/MIME config for the specified send-as alias.
- `gmail-pp-cli settings list` — Lists the delegates for the specified account.
- `gmail-pp-cli settings list-gmail` — Lists the message filters of a Gmail user.
- `gmail-pp-cli settings list-gmail-2` — Lists the forwarding addresses for the specified account.
- `gmail-pp-cli settings list-gmail-3` — Lists the send-as aliases for the specified account.
- `gmail-pp-cli settings list-gmail-4` — Lists the client-side encrypted identities for an authenticated user.
- `gmail-pp-cli settings list-gmail-5` — Lists client-side encryption key pairs for an authenticated user.
- `gmail-pp-cli settings list-gmail-6` — Lists S/MIME configs for the specified send-as alias.
- `gmail-pp-cli settings obliterate` — Deletes a client-side encryption key pair permanently and immediately.
- `gmail-pp-cli settings patch` — Patch the specified send-as alias.
- `gmail-pp-cli settings patch-gmail` — Associates a different key pair with an existing client-side encryption identity.
- `gmail-pp-cli settings set-default` — Sets the default S/MIME config for the specified send-as alias.
- `gmail-pp-cli settings update` — Updates a send-as alias. If a signature is provided, Gmail will sanitize the HTML before saving it with the alias.
- `gmail-pp-cli settings update-auto-forwarding` — Updates the auto-forwarding setting for the specified account.
- `gmail-pp-cli settings update-imap` — Updates IMAP settings.
- `gmail-pp-cli settings update-language` — Updates language settings.
- `gmail-pp-cli settings update-pop` — Updates POP settings.
- `gmail-pp-cli settings update-vacation` — Updates vacation responder settings.
- `gmail-pp-cli settings verify` — Sends a verification email to the specified send-as alias address. The verification status must be `pending`.

**stop** — Manage stop

- `gmail-pp-cli stop <userId>` — Stop receiving push notifications for the given user mailbox.

**threads** — Manage threads

- `gmail-pp-cli threads delete` — Immediately and permanently deletes the specified thread. Any messages that belong to the thread are also deleted.
- `gmail-pp-cli threads get` — Gets the specified thread.
- `gmail-pp-cli threads modify` — Modifies the labels applied to the thread.
- `gmail-pp-cli threads trash` — Moves the specified thread to the trash.
- `gmail-pp-cli threads untrash` — Removes the specified thread from the trash.
- `gmail-pp-cli threads list` — Lists the threads in the user's mailbox.

**users_profile** — Manage users profile

- `gmail-pp-cli users-profile <userId>` — Gets the current user's Gmail profile.

**watch** — Manage watch

- `gmail-pp-cli watch <userId>` — Set up or update a push notification watch on the given user mailbox.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
gmail-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Morning triage

```bash
gmail-pp-cli triage --agent
```

Unread mail grouped by sender and category in compact agent-shaped JSON, ready for a summarize-and-archive loop.

### Narrow a live list for an agent

```bash
gmail-pp-cli messages list me --q "has:attachment newer_than:7d" --agent --select messages.id,messages.threadId
```

Typed endpoint call with dotted-path field selection so the agent gets ids without the verbose envelope.

### Schedule tomorrow's follow-up

```bash
gmail-pp-cli schedule send --to alex@example.com --subject "Re: proposal" --body-file followup.txt --in 18h
```

Queues the send locally; a cron-invoked schedule run fires it at the due time, exactly once.

### Chase silent prospects

```bash
gmail-pp-cli followups --direction out --days 3 --agent
```

Sent threads where nobody replied in 3 days, from the local store at zero quota cost.

### Cut inbox noise

```bash
gmail-pp-cli unsub --min-count 10 --agent
```

Mailing lists ranked by volume and never-read ratio with their unsubscribe targets.

## Auth Setup

Gmail uses OAuth2. Run `gmail-pp-cli auth login` once: it opens a browser consent flow using OAuth client credentials supplied via --client-id/--client-secret, the GMAIL_CLIENT_ID / GMAIL_CLIENT_SECRET environment variables, or an interactive prompt. Tokens are stored locally and refresh automatically. For headless use, set GMAIL_ACCESS_TOKEN to a pre-minted token.

Run `gmail-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  gmail-pp-cli drafts list me --agent --select drafts.id,drafts.message.threadId
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — search and analytics commands read the local SQLite store populated by `gmail-pp-cli pull`
- **Non-interactive** — never prompts, every input is a flag
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

- Use `--home <dir>` for one invocation, or set `GMAIL_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `GMAIL_CONFIG_DIR`, `GMAIL_DATA_DIR`, `GMAIL_STATE_DIR`, `GMAIL_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `GMAIL_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `gmail-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "gmail": {
        "command": "gmail-pp-mcp",
        "env": {
          "GMAIL_HOME": "/srv/gmail"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `GMAIL_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `GMAIL_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
gmail-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "gmail-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `gmail-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `gmail-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `gmail-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
gmail-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
gmail-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
gmail-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
gmail-pp-cli playbook amend \
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

`gmail-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `GMAIL_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
gmail-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
gmail-pp-cli feedback --stdin < notes.txt
gmail-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `GMAIL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GMAIL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
gmail-pp-cli profile save briefing --json
gmail-pp-cli --profile briefing drafts list me
gmail-pp-cli profile list --json
gmail-pp-cli profile show briefing
gmail-pp-cli profile delete briefing --yes
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
| 6 | Partial failure (downgrade to a warning with `--allow-partial-failure`) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `gmail-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/gmail/cmd/gmail-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add gmail-pp-mcp -- gmail-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which gmail-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   gmail-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `gmail-pp-cli <command> --help`.
