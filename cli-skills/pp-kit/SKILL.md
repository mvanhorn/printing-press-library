---
name: pp-kit
description: "Printing Press CLI for Kit."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - kit-pp-cli
    install:
      - kind: go
        bins: [kit-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/kit/cmd/kit-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/marketing/kit/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Kit — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `kit-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install kit --cli-only
   ```
2. Verify: `kit-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/kit/cmd/kit-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Agentic Kit Workflows

Prefer these read-only workflows before raw endpoint mirrors when the user asks for planning, diagnosis, audit, support, or account context:

- `kit-pp-cli workflow creator-snapshot --agent` — one operating snapshot with account, growth, audience, content, webhook, and broadcast stats.
- `kit-pp-cli workflow audience-health --agent` — subscriber status counts, growth stats, and largest tags by subscriber count.
- `kit-pp-cli workflow content-inventory --agent` — sequences, sequence emails, snippets, forms, email templates, and recent broadcast stats for content planning.
- `kit-pp-cli workflow subscriber-lookup --email <email> --agent` — one subscriber profile with custom fields, tags, attribution, and email stats.

Use raw endpoint commands when the user needs a specific CRUD operation, exact endpoint parity, pagination beyond a workflow limit, or a write operation. These workflows call real Kit v4 endpoints; they are not cached summaries or mock payloads.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Creator operations
- **`workflow creator-snapshot`** — One-call read-only operating snapshot for Kit account, growth, audience, content, webhooks, and broadcast stats.

  _Use this first when an agent needs current creator-account context without manually fanning out across endpoint mirrors._

  ```bash
  kit-pp-cli workflow creator-snapshot --agent
  ```

### Audience intelligence
- **`workflow audience-health`** — Read-only subscriber status counts, recent growth stats, and largest tags by subscriber count.

  _Use this before list cleaning, segmentation, or campaign planning to avoid multiple fragile subscriber and tag calls._

  ```bash
  kit-pp-cli workflow audience-health --agent
  ```
- **`workflow subscriber-lookup`** — Read-only subscriber dossier by email or id with profile, custom fields, tags, attribution, and email stats.

  _Use this for support, segmentation checks, and personalization debugging before raw subscriber endpoint calls._

### Content planning
- **`workflow content-inventory`** — Read-only inventory of sequences, sequence emails, snippets, forms, templates, and recent broadcast stats.

  _Use this for content audits and planning instead of separately listing each Kit content surface._

  ```bash
  kit-pp-cli workflow content-inventory --agent
  ```

### Trends and ranking
- **`growth-trends`** — Correlates account growth stats with broadcast performance over a date range. Optionally caches results via the typed `UpsertBroadcastsStats` write path.
- **`tag-performance`** — Ranks tags by subscriber count with share-of-total percentages; optional `--subscriber-query` uses `SearchSubscribers` to narrow to a segment.

### MCP intent tools

Four first-class MCP intent tools expose the workflow commands with typed input schemas and read-only annotations. Use these via the MCP server (`kit-pp-mcp`) when an agent host wants typed parameters and explicit safety hints:

- `intent_workflow_creator_snapshot`
- `intent_workflow_audience_health`
- `intent_workflow_content_inventory`
- `intent_workflow_subscriber_lookup`

The MCP server supports both stdio and streamable HTTP transports (`--transport http --addr :7777` or `PP_MCP_TRANSPORT=http`).

## Command Reference

**workflow** — Kit-specific compound workflows for agents

- `kit-pp-cli workflow creator-snapshot` — Summarize account, audience, content, and broadcast health
- `kit-pp-cli workflow audience-health` — Summarize subscriber status, recent growth, and largest tags
- `kit-pp-cli workflow content-inventory` — Inventory sequences, emails, snippets, forms, templates, and broadcasts
- `kit-pp-cli workflow subscriber-lookup` — Fetch one subscriber with tags, custom fields, attribution, and email stats

**account** — Manage account

- `kit-pp-cli account list` — Get current account
- `kit-pp-cli account list-colors` — List colors
- `kit-pp-cli account list-creatorprofile` — Get Creator Profile
- `kit-pp-cli account list-emailstats` — Get email stats
- `kit-pp-cli account list-growthstats` — Get growth stats for a specific time period. Defaults to last 90 days.<br/><br/>NOTE: We return your stats in your...
- `kit-pp-cli account update` — Update colors

**broadcasts** — Manage broadcasts

- `kit-pp-cli broadcasts create` — Draft or schedule to send a broadcast to all or a subset of your subscribers.<br/><br/>To save a draft, set...
- `kit-pp-cli broadcasts delete` — Delete a broadcast
- `kit-pp-cli broadcasts get` — Get a broadcast
- `kit-pp-cli broadcasts list` — List broadcasts
- `kit-pp-cli broadcasts list-stats` — Get stats for a list of broadcasts
- `kit-pp-cli broadcasts update` — Update an existing broadcast. Continue to draft or schedule to send a broadcast to all or a subset of your...

**bulk** — Manage bulk

- `kit-pp-cli bulk create` — See '[Bulk & async processing](#bulk-amp-async-processing)' for more information.
- `kit-pp-cli bulk create-customfields` — Bulk update subscriber custom field values
- `kit-pp-cli bulk create-forms` — Adding subscribers to double opt-in forms will trigger sending an Incentive Email. Subscribers already added to the...
- `kit-pp-cli bulk create-subscribers` — See '[Bulk & async processing](#bulk-amp-async-processing)' for more information.
- `kit-pp-cli bulk create-tags` — See '[Bulk & async processing](#bulk-amp-async-processing)' for more information.
- `kit-pp-cli bulk create-tags-2` — The subscribers being tagged must already exist. Subscribers can be created in bulk using the '[Bulk create...
- `kit-pp-cli bulk delete` — See '[Bulk & async processing](#bulk-amp-async-processing)' for more information.

**custom-fields** — Manage custom fields

- `kit-pp-cli custom-fields create` — Create a custom field for your account. The label field must be unique to your account. Whitespace will be removed...
- `kit-pp-cli custom-fields delete` — This will remove all data in this field from your subscribers.
- `kit-pp-cli custom-fields list` — A custom field allows you to collect subscriber information beyond the standard fields of first name and email...
- `kit-pp-cli custom-fields update` — Updates a custom field label (see [Create a custom field](/api-reference/custom-fields/create-a-custom-field) for...

**email-templates** — Manage email templates

- `kit-pp-cli email-templates` — List email templates

**forms** — Manage forms

- `kit-pp-cli forms` — List forms

**posts** — Manage posts

- `kit-pp-cli posts get` — Get a post
- `kit-pp-cli posts list` — List posts

**purchases** — Manage purchases

- `kit-pp-cli purchases create` — Create a purchase
- `kit-pp-cli purchases get` — Get a purchase
- `kit-pp-cli purchases list` — List purchases

**segments** — Manage segments

- `kit-pp-cli segments` — List segments

**sequences** — Manage sequences

- `kit-pp-cli sequences create` — Creates an empty sequence — the container that holds sequence emails. After creating the shell, use [Create a...
- `kit-pp-cli sequences delete` — Soft-deletes a sequence. The sequence is removed from active delivery immediately, with cleanup of associated state...
- `kit-pp-cli sequences get` — Fetches a single sequence by `id`. Use this when you need the current schedule, the `active` / `repeat` / `hold`...
- `kit-pp-cli sequences list` — Returns every sequence on the account. A sequence is a self-contained set of automated emails — subscribers join,...
- `kit-pp-cli sequences update` — Updates any sequence settings — `name`, `email_address`, schedule (`send_days`, `send_hour`, `time_zone`),...

**snippets** — Manage snippets

- `kit-pp-cli snippets create` — Snippets are reusable pieces of email content you can drop into a broadcast or sequence email using Liquid: `{{...
- `kit-pp-cli snippets get` — Fetches a single snippet by `id`. Unlike [List snippets](/api-reference/snippets/list-snippets), this endpoint...
- `kit-pp-cli snippets list` — Returns every snippet on the account. Each snippet's `key` is the identifier used in Liquid — `{{ snippet.key }}`...
- `kit-pp-cli snippets update` — Rename a snippet, replace its body, or archive/restore it. Updates apply on the next send of any email that...

**subscribers** — Manage subscribers

- `kit-pp-cli subscribers create` — Behaves as an upsert. If a subscriber with the provided email address does not exist, it creates one with the...
- `kit-pp-cli subscribers create-filter` — Filter subscribers based on engagement
- `kit-pp-cli subscribers get` — Get a subscriber
- `kit-pp-cli subscribers list` — List subscribers
- `kit-pp-cli subscribers update` — If you include a custom field key that does not exist on your account, the request returns an error. Use [List...

**tags** — Manage tags

- `kit-pp-cli tags create` — Create a tag
- `kit-pp-cli tags list` — List tags
- `kit-pp-cli tags update` — Update tag name

**webhooks** — Manage webhooks

- `kit-pp-cli webhooks create` — Available event types:<br/>- `subscriber.subscriber_activate`<br/>- `subscriber.subscriber_unsubscribe`<br/>-...
- `kit-pp-cli webhooks delete` — Delete a webhook
- `kit-pp-cli webhooks list` — Webhooks are automations that will receive subscriber data when a subscriber event is triggered, such as when a...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
kit-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `kit-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export KIT_API_KEY="<your-key>"
```

Or persist it in `~/.config/kit-pp-cli/config.toml`.

Run `kit-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  kit-pp-cli account list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
kit-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
kit-pp-cli feedback --stdin < notes.txt
kit-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.kit-pp-cli/feedback.jsonl`. They are never POSTed unless `KIT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `KIT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
kit-pp-cli profile save briefing --json
kit-pp-cli --profile briefing account list
kit-pp-cli profile list --json
kit-pp-cli profile show briefing
kit-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `kit-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/kit/cmd/kit-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add kit-pp-mcp -- kit-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which kit-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Prefer the Agentic Kit Workflows above for broad account, audience, content, or subscriber-context requests.
3. Match narrower user queries to the best command from the Command Reference above.
4. Execute with the `--agent` flag:
   ```bash
   kit-pp-cli <command> [subcommand] [args] --agent
   ```
5. If ambiguous, drill into subcommand help: `kit-pp-cli <command> --help`.
