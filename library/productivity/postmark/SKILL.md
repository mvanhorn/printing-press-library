---
name: pp-postmark
description: "One account token for every Postmark server, and a local archive that outlasts Postmark's 45-day retention. Trigger phrases: `did my email get delivered`, `check postmark bounces`, `why didn't they get the reset email`, `push postmark templates`, `postmark sending stats this week`, `set up postmark for a new project`, `use postmark`, `run postmark`."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - postmark-pp-cli
    install:
      - kind: go
        bins: [postmark-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/postmark/cmd/postmark-pp-cli
---

# Postmark — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `postmark-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install postmark --cli-only
   ```
2. Verify: `postmark-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/postmark/cmd/postmark-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Postmark gives every server its own token and keeps message history for 45 days by default. This CLI takes one account token and works across all your servers. Pick a server by name with `--server`, sync messages and bounces into a local SQLite archive that keeps what Postmark expires, and ask questions that span the account, like which server stopped sending or why one customer never got a password reset. It covers all 87 documented API operations, including message streams, webhooks, suppressions, and bulk email, which Postmark's official specs leave out.

Sending is safe to hand to an agent. Send commands print a preview until you add `--send`, `--sandbox` validates a message with Postmark's test token without delivering it, and `email send-once` won't send the same message twice when a script retries.

## When to Use This CLI

Use this CLI to operate Postmark from a terminal or an agent: check whether a specific recipient got an email, triage bounces and suppressions, move templates between servers and gate them in CI, send transactional mail safely, and watch sending health across every server in one account. The local archive answers questions about messages older than Postmark's retention window.

## Anti-triggers

Do not use this CLI for:
- Do not use it for Postmark billing, team members, or account settings that only exist in the web dashboard.
- Do not use it for DMARC digest reports; those come from Postmark's separate DMARC API with its own token.
- Do not use it to receive inbound email in real time; configure an inbound webhook instead.
- Do not use it for newsletter list management; Postmark sends broadcasts but does not manage subscriber lists.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Account-wide oversight
- **`pulse`** — Compares each server, stream, or tag with its own recent weeks and flags any that stopped sending, dropped off, or spiked in bounces or spam, across every server on the account token.

  _A mailer that breaks quietly stops sending instead of throwing errors, so run this weekly to catch the drop._

  ```bash
  postmark-pp-cli pulse --window 7d --baseline 28d --agent
  ```
- **`recipient-domains`** — Groups sends, bounces by type, complaints, suppressions, and opens by the recipient's mailbox domain across every synced server, so one provider's problem stands out. Run sync first.

  _Start here when several missing-email reports come from the same provider or company._

  ```bash
  postmark-pp-cli recipient-domains --window 7d --min-sent 20 --agent
  ```
- **`servers bootstrap`** — Creates a new project's Postmark server with its message streams, webhooks, sending-domain DNS records, and templates copied from another server. Prints the plan until you add --apply.

  _Hand this to an agent setting up email for a new project. Reruns never create a duplicate server._

  ```bash
  postmark-pp-cli servers bootstrap "Lumen" --broadcast-stream --agent
  ```

### Safe sending
- **`email send-once`** — Sends an email at most once per key within a window (15 minutes by default), so an agent or script that retries after an error cannot deliver a second copy. Previews until you add --send.

  _Built for one-time codes, receipts, and follow-ups that an agent might retry._

  ```bash
  postmark-pp-cli email send-once --key otp-4821 --from "$SENDER" --to "$RECIPIENT" --subject "Your code" --text "Code: 4821" --agent
  ```

### Templates as code
- **`templates check`** — Validates every template and layout on a server or in a pulled folder, and exits non-zero on render errors, missing test-model keys, or broken layout links.

  _Run it in CI before pushing template changes to a server._

  ```bash
  postmark-pp-cli templates check --agent
  ```

## Command Reference

**bounces** — Manage bounces

- `postmark-pp-cli bounces activate` — Activate a bounce
- `postmark-pp-cli bounces delivery-stats` — Get delivery stats
- `postmark-pp-cli bounces dump` — Get bounce dump
- `postmark-pp-cli bounces get` — Get a single bounce
- `postmark-pp-cli bounces list` — Get bounces

**clicks** — Manage clicks

- `postmark-pp-cli clicks get` — Retrieve Message Clicks
- `postmark-pp-cli clicks list` — Clicks for a all messages

**data_removals** — Manage data removals

- `postmark-pp-cli data-removals create` — Request irreversible removal of a recipient's data (enabled on request by Postmark)
- `postmark-pp-cli data-removals get` — Check a data removal request

**domains** — Manage domains

- `postmark-pp-cli domains create` — Create a Domain
- `postmark-pp-cli domains delete` — Delete a Domain
- `postmark-pp-cli domains get` — Get a Domain
- `postmark-pp-cli domains list` — List Domains
- `postmark-pp-cli domains rotate-dkim` — Creates a new DKIM key to replace your current key.
- `postmark-pp-cli domains update` — Update a Domain
- `postmark-pp-cli domains verify-dkim` — Request DNS Verification for DKIM
- `postmark-pp-cli domains verify-return-path` — Request DNS Verification for Return-Path
- `postmark-pp-cli domains verify-spf` — Request DNS Verification for SPF (deprecated by Postmark)

**email** — Manage email

- `postmark-pp-cli email bulk-list` — List bulk requests on this server, newest first
- `postmark-pp-cli email bulk-status` — Get the status of one bulk request
- `postmark-pp-cli email send` — Send a single email
- `postmark-pp-cli email send-batch` — Send a batch of emails
- `postmark-pp-cli email send-batch-with-templates` — Send a batch of email using templates.
- `postmark-pp-cli email send-bulk` — Send one message to many recipients (bulk). Requires account approval.
- `postmark-pp-cli email send-with-template` — Send an email using a Template

**inbound** — Manage inbound

- `postmark-pp-cli inbound bypass` — Bypass rules for a blocked inbound message
- `postmark-pp-cli inbound get` — Inbound message details
- `postmark-pp-cli inbound list` — Inbound message search
- `postmark-pp-cli inbound retry` — Retry a failed inbound message for processing

**inbound_rules** — Manage inbound rules

- `postmark-pp-cli inbound-rules create` — Create an inbound rule trigger
- `postmark-pp-cli inbound-rules delete` — Delete a single trigger
- `postmark-pp-cli inbound-rules list` — List inbound rule triggers

**messages** — Manage messages

- `postmark-pp-cli messages dump` — Outbound message dump
- `postmark-pp-cli messages get` — Outbound message details
- `postmark-pp-cli messages list` — Outbound message search

**opens** — Manage opens

- `postmark-pp-cli opens get` — Retrieve Message Opens
- `postmark-pp-cli opens list` — Opens for all messages

**senders** — Manage senders

- `postmark-pp-cli senders create` — Create a Sender Signature
- `postmark-pp-cli senders delete` — Delete a Sender Signature
- `postmark-pp-cli senders get` — Get a Sender Signature
- `postmark-pp-cli senders list` — List Sender Signatures
- `postmark-pp-cli senders request-new-dkim` — Requests a new DKIM key to be created.
- `postmark-pp-cli senders resend-confirmation` — Resend Signature Confirmation Email
- `postmark-pp-cli senders update` — Update a Sender Signature
- `postmark-pp-cli senders verify-spf` — Request DNS Verification for SPF (deprecated by Postmark)

**server** — Manage server

- `postmark-pp-cli server get` — Get Server Configuration
- `postmark-pp-cli server update` — Edit Server Configuration

**servers** — Manage servers

- `postmark-pp-cli servers create` — Create a Server
- `postmark-pp-cli servers delete` — Delete a Server
- `postmark-pp-cli servers get` — Get a Server
- `postmark-pp-cli servers list` — List servers
- `postmark-pp-cli servers update` — Edit a Server

**stats** — Manage stats

- `postmark-pp-cli stats bounces` — Get bounce counts
- `postmark-pp-cli stats click-browsers` — Get browser usage by family
- `postmark-pp-cli stats click-locations` — Get clicks by body location
- `postmark-pp-cli stats click-platforms` — Get browser plaform usage
- `postmark-pp-cli stats clicks` — Get click counts
- `postmark-pp-cli stats open-clients` — Get email client usage
- `postmark-pp-cli stats open-platforms` — Get email platform usage
- `postmark-pp-cli stats open-read-times` — Open counts by read time (seconds spent reading)
- `postmark-pp-cli stats opens` — Get email open counts
- `postmark-pp-cli stats overview` — Get outbound overview
- `postmark-pp-cli stats sends` — Get sent counts
- `postmark-pp-cli stats spam` — Get spam complaints
- `postmark-pp-cli stats tracked` — Get tracked email counts

**streams** — Manage streams

- `postmark-pp-cli streams archive` — Archive a message stream (purged 45 days later unless unarchived)
- `postmark-pp-cli streams create` — Create a message stream
- `postmark-pp-cli streams get` — Get one message stream
- `postmark-pp-cli streams list` — List message streams on this server
- `postmark-pp-cli streams unarchive` — Restore an archived message stream before its purge date
- `postmark-pp-cli streams update` — Edit a message stream

**suppressions** — Manage suppressions

- `postmark-pp-cli suppressions create` — Suppress up to 50 addresses on a message stream
- `postmark-pp-cli suppressions delete` — Remove up to 50 suppressions from a message stream (SpamComplaint suppressions cannot be removed)
- `postmark-pp-cli suppressions list` — List suppressed addresses on a message stream

**templates** — Manage templates

- `postmark-pp-cli templates create` — Create a Template
- `postmark-pp-cli templates delete` — Delete a Template
- `postmark-pp-cli templates get` — Get a Template
- `postmark-pp-cli templates list` — Get the Templates associated with this Server
- `postmark-pp-cli templates push-between-servers` — Push templates from one server to another
- `postmark-pp-cli templates update` — Update a Template
- `postmark-pp-cli templates validate` — Test Template Content

**webhooks** — Manage webhooks

- `postmark-pp-cli webhooks create` — Create a webhook
- `postmark-pp-cli webhooks delete` — Delete a webhook
- `postmark-pp-cli webhooks get` — Get one webhook
- `postmark-pp-cli webhooks list` — List webhooks on this server
- `postmark-pp-cli webhooks statistics` — Delivery statistics for one webhook over the last 24 hours
- `postmark-pp-cli webhooks update` — Edit a webhook (omitted triggers stay unchanged)
- `postmark-pp-cli webhooks verify` — Send a test request for each enabled trigger and record verified or unverified


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
postmark-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Hand-written Extensions

These commands are hand-written on top of the generated endpoint commands. They follow the same output flags (`--json`, `--agent`, `--select`, `--csv`) and the global `--server <name>` and `--sandbox` flags. Server tokens are masked in every response.

- `postmark-pp-cli diagnose [email]`: one recipient's recent messages, bounces, and suppressions with a verdict (delivered, sent, bounced, suppressed, spam_complaint, queued, not_found) and the next command to run. `--all-servers` checks every server.
- `postmark-pp-cli overview`: every server in the account with this window's sends, bounces, spam complaints, and stream count.
- `postmark-pp-cli servers tokens [name]`: print the full API token for one server or all servers; the only command that reveals tokens, never cached or stored, and not exposed over MCP.
- `postmark-pp-cli servers use [name]`: save the default server and optional default sender and stream for send commands.
- `postmark-pp-cli templates pull <dir>`: write every template and layout to disk in the official postmark-cli folder layout, with no template cap.
- `postmark-pp-cli templates push <dir>`: diff a template folder against the server; prints the plan unless `--yes` is given, and `--prune --yes` removes templates missing locally only after every local template was pushed.
- `postmark-pp-cli templates render [alias]`: render one template (remote or from a pulled folder) with its layout and test model.
- `postmark-pp-cli suppressions check <email>`: whether an address is suppressed on each message stream, with the command that removes it (SpamComplaint suppressions cannot be removed).
- `postmark-pp-cli bounces reactivate`: plan reactivation of inactive, reactivatable bounces filtered by type, domain, email, window, and `--stream` (default outbound); `--yes` reactivates them.
- `postmark-pp-cli bounces resend-blocked`: plan resending messages that hard-bounced, to the bounced recipient only, on `--stream` (default outbound); `--send` reactivates and resends.
- `postmark-pp-cli webhooks health`: 24-hour delivery statistics and verification status for each webhook, flagging failures.
- `postmark-pp-cli domains health`: DKIM, Return-Path, and sender confirmation status with the fix command for each failure.
- `postmark-pp-cli streams health`: bounce and spam rates per message stream against the 10% and 0.1% thresholds.
- `postmark-pp-cli service-status`: Postmark's public status page, current state and open notices.

The generated `email send`, `send-with-template`, `send-batch`, `send-batch-with-templates`, and `send-bulk` commands print the request they would make unless `--send` is given. `--sandbox` validates a send with Postmark's POSTMARK_API_TEST token and never delivers.

For the hand-written commands, `--dry-run` only confirms the command resolves. To see what one would do, run it without `--send`, `--apply`, or `--yes`: `email send-once`, `bounces resend-blocked`, `servers bootstrap`, `templates push`, and `bounces reactivate` print their plan and change nothing until given that flag.

`sync --full` never prunes here, so messages Postmark has expired and rows synced from other servers stay in the local archive. `--no-prune=false` is refused; to start a fresh archive, sync into a new file with `--db <path>`.

Syncing a different server into the same database resets the sync checkpoints and reads that server from the beginning, without pruning. Syncs into one database take turns.

If `email send-once` cannot tell whether Postmark accepted a message (a timeout or a 5xx), it keeps the key reserved for the window and later runs report `delivery_unknown` instead of sending again.

Over MCP, `postmark_execute` returns a preview instead of calling the API for every DELETE and for data removals, stream archives, suppression deletes, and template pushes between servers, unless the call includes `confirm: true`.

## Recipes

### Monday health pass

```bash
postmark-pp-cli pulse --window 7d --baseline 28d --agent
```

Compares each server's last week against its prior four and lists anything that stopped, dropped, or spiked.

### Why didn't they get the reset email

```bash
postmark-pp-cli diagnose jane@example.com --server "Staging" --agent
```

Checks outbound messages, bounces, and suppressions for one address and returns a verdict with the fix command.

### Narrow a big message search

```bash
postmark-pp-cli messages list --recipient jane@example.com --count 50 --offset 0 --agent --select MessageID,Subject,Status,ReceivedAt
```

Keeps only the fields an agent needs from a large outbound search response.

### One-time code without double sends

```bash
postmark-pp-cli email send-once --key otp-4821 --server "Production" --from app@example.com --to jane@example.com --subject "Your code" --text "Code: 4821"
```

Prints the payload and dedupe decision; add --send to deliver, and a retry with the same key returns the original MessageID.

### Gate template changes in CI

```bash
postmark-pp-cli templates check --dir ./templates --server "Production"
```

Validates every template and layout in a pulled folder and exits non-zero on any failure.

### Spin up email for a new project

```bash
postmark-pp-cli servers bootstrap "Lumen" --domain lumen.example.com --broadcast-stream --agent
```

Prints the server, stream, and domain plan with the Return-Path DNS record; the DKIM record appears after --apply creates the domain, and a rerun never duplicates the server.

## Auth Setup

Postmark has two kinds of token. A server token (`X-Postmark-Server-Token`) covers one server: sending, messages, bounces, templates, stats, streams, suppressions, and webhooks. The account token (`X-Postmark-Account-Token`) covers servers, domains, sender signatures, template pushes between servers, and data removals. The CLI sends only the header each endpoint accepts.

The simplest setup is the account token alone. Set `POSTMARK_ACCOUNT_TOKEN`, choose a server with `--server <name>` or `POSTMARK_SERVER`, and the CLI looks up that server's token through the account API. `servers use <name>` saves a default server.

If you only use one server, export `POSTMARK_SERVER_TOKEN` instead, or store it with `auth set-token`, which reads the token from stdin. `--server` and `POSTMARK_SERVER` take precedence over `POSTMARK_SERVER_TOKEN`, which takes precedence over the saved default.

`--sandbox` swaps in Postmark's `POSTMARK_API_TEST` token, so sends are validated but never delivered.

Run `postmark-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  postmark-pp-cli bounces list --count 100 --offset 0 --agent --select BouncedAt,CanActivate,Content
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

- Use `--home <dir>` for one invocation, or set `POSTMARK_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `POSTMARK_CONFIG_DIR`, `POSTMARK_DATA_DIR`, `POSTMARK_STATE_DIR`, `POSTMARK_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `POSTMARK_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml` and `data.db` (the local archive and send-once ledger). `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `postmark-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "postmark": {
        "command": "postmark-pp-mcp",
        "env": {
          "POSTMARK_HOME": "/srv/postmark"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `POSTMARK_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `POSTMARK_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
postmark-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

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
      "next_action": ["<trial command>", "postmark-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `postmark-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `postmark-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `postmark-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
postmark-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
postmark-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
postmark-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
postmark-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
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

`postmark-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `POSTMARK_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
postmark-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
postmark-pp-cli feedback --stdin < notes.txt
postmark-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `POSTMARK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `POSTMARK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
postmark-pp-cli profile save briefing --json
postmark-pp-cli --profile briefing bounces list --count 100 --offset 0
postmark-pp-cli profile list --json
postmark-pp-cli profile show briefing
postmark-pp-cli profile delete briefing --yes
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
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `postmark-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/postmark/cmd/postmark-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add postmark-pp-mcp -- postmark-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which postmark-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   postmark-pp-cli diagnose jane@example.com --agent
   ```
4. If ambiguous, drill into subcommand help, for example `postmark-pp-cli templates --help`.
