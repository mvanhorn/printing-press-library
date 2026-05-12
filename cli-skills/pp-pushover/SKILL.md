---
name: pp-pushover
description: "Pushover from the terminal: send alerts, watch emergency receipts, inspect quota, and keep a redacted local... Trigger phrases: `send me a pushover`, `notify me on my phone`, `check pushover quota`, `watch pushover emergency receipt`, `validate pushover user key`, `use pushover-pp-cli`."
author: "Todd Dailey"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pushover-pp-cli
---

# Pushover — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `pushover-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install pushover --cli-only
   ```
2. Verify: `pushover-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI covers the full official API instead of stopping at send-message. It adds agent-safe env defaults, receipt lifecycle tools, quota preflight, and local history so alerts are auditable.

## When to Use This CLI

Use pushover-pp-cli for agent or ops workflows that need mobile push notifications, emergency receipt tracking, quota-aware sends, group delivery management, or Open Client message retrieval.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`notify`** — Send a notification from an argument or stdin with env/config defaults, named priorities, and emergency validation.

  _Lets agents send test or operational notifications without leaking token values or remembering the raw endpoint shape._

  ```bash
  pushover-pp-cli notify "Printing Press test" --priority low --json
  ```

### Receipt operations
- **`emergency watch`** — Send or monitor an emergency notification receipt, polling acknowledgement status and supporting cancellation.

  _Turns emergency notifications from fire-and-forget sends into an auditable on-call workflow._

  ```bash
  pushover-pp-cli emergency watch --receipt <receipt> --poll-interval 30s --cancel-on-timeout --json
  ```

### Operational safety
- **`quota`** — Show monthly send limit, remaining sends, and reset time from the app limits endpoint.

  _Agents can check send budget before fanout or repeated test sends._

  ```bash
  pushover-pp-cli quota --json
  ```

### Local state that compounds
- **`history`** — Search and export a local redacted ledger of notifications sent by this CLI and receipt outcomes.

  _Shows what this CLI sent, when, with which priority, without storing raw user keys or tokens._

  ```bash
  pushover-pp-cli history list --since 24h --json
  ```
- **`inbox sync`** — Download Open Client messages into local SQLite before optional server-side delete-through.

  _Makes Pushover usable as both send channel and retrievable inbox for agents that own an Open Client device._

  ```bash
  pushover-pp-cli inbox sync --device-id <device-id> --json
  ```

## Command Reference

**notify** — Send an agent-safe notification from an argument, `--message`, or stdin

- `pushover-pp-cli notify` — Send with env defaults, named priorities, emergency validation, and redacted local history

**emergency** — Watch emergency-priority receipts

- `pushover-pp-cli emergency watch` — Poll an emergency receipt at a safe cadence until terminal state

**quota** — Check application send quota

- `pushover-pp-cli quota` — Show monthly limit, remaining sends, and reset time

**history** — Inspect the local redacted notification ledger

- `pushover-pp-cli history` — List locally recorded notification sends
- `pushover-pp-cli history show` — Show one local history row

**inbox** — Sync Open Client messages locally

- `pushover-pp-cli inbox sync` — Store downloaded Open Client messages in local SQLite
- `pushover-pp-cli inbox list` — List locally synced Open Client messages

**apps** — Inspect application-level message quotas

- `pushover-pp-cli apps` — Check monthly message limit, remaining sends, and reset time

**devices** — Manage Open Client devices

- `pushover-pp-cli devices create` — Register a new Open Client desktop device
- `pushover-pp-cli devices delete-through` — Delete downloaded Open Client messages up to a highest message id

**glances** — Update Pushover Glances widgets

- `pushover-pp-cli glances` — Update Glances widget fields

**groups** — Create, inspect, and manage Pushover delivery groups

- `pushover-pp-cli groups add-user` — Add a user to a delivery group
- `pushover-pp-cli groups create` — Create a delivery group
- `pushover-pp-cli groups disable-user` — Temporarily disable a group user
- `pushover-pp-cli groups enable-user` — Re-enable a disabled group user
- `pushover-pp-cli groups get` — Get a delivery group's name and users
- `pushover-pp-cli groups list` — List delivery groups owned by the account
- `pushover-pp-cli groups remove-user` — Remove a user from a delivery group
- `pushover-pp-cli groups rename` — Rename a delivery group

**licenses** — Assign and inspect Pushover license credits

- `pushover-pp-cli licenses assign` — Assign a pre-paid license credit to a user or email
- `pushover-pp-cli licenses credits` — Check remaining license credits

**messages** — Send application notifications and download Open Client messages

- `pushover-pp-cli messages download` — Download pending messages for an Open Client device
- `pushover-pp-cli messages send` — Send a Pushover notification

**receipts** — Inspect and cancel emergency-priority message receipts

- `pushover-pp-cli receipts cancel` — Cancel retries for one emergency-priority receipt
- `pushover-pp-cli receipts cancel-by-tag` — Cancel active emergency-priority retries matching a tag
- `pushover-pp-cli receipts get` — Get emergency-priority receipt status

**sounds** — Discover notification sounds

- `pushover-pp-cli sounds` — List built-in and account custom notification sounds

**subscriptions** — Manage Pushover subscription migrations

- `pushover-pp-cli subscriptions` — Migrate an existing user key into a subscription user key

**teams** — Manage Pushover for Teams membership

- `pushover-pp-cli teams add-user` — Add a user to a team
- `pushover-pp-cli teams get` — Show team information and users
- `pushover-pp-cli teams remove-user` — Remove a user from a team

**users** — Validate users and log in for Open Client sessions

- `pushover-pp-cli users login` — Log in a user for Open Client and return a user key plus session secret
- `pushover-pp-cli users validate` — Validate a user or group key and optional device


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pushover-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Send a low-priority test

```bash
pushover-pp-cli notify "Printing Press test" --priority low --json
```

Uses env/config credentials and records a local ledger row.

### Check quota before a batch

```bash
pushover-pp-cli quota --json --select remaining,reset
```

Reads the dedicated app limits endpoint.

### Validate a destination

```bash
pushover-pp-cli users validate --user-key "$PUSHOVER_USER_KEY" --json
```

Confirms the key is valid before relying on it in scripts.

### Watch an emergency receipt

```bash
pushover-pp-cli emergency watch --receipt <receipt> --json
```

Polls the official receipt endpoint at a safe cadence.

## Auth Setup

Set `PUSHOVER_APP_TOKEN` and `PUSHOVER_USER_KEY` for send, quota, users, groups, sounds, receipts, glances, subscriptions, and license workflows. Legacy `PUSHOVER_TOKEN` and `PUSHOVER_USER` are accepted as local compatibility aliases. Open Client workflows use `PUSHOVER_CLIENT_SECRET` and `PUSHOVER_DEVICE_ID` when their flags are omitted.

Run `pushover-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  pushover-pp-cli groups list --app-token your-token-here --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
pushover-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pushover-pp-cli feedback --stdin < notes.txt
pushover-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.pushover-pp-cli/feedback.jsonl`. They are never POSTed unless `PUSHOVER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PUSHOVER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
pushover-pp-cli profile save briefing --json
pushover-pp-cli --profile briefing groups list --app-token your-token-here
pushover-pp-cli profile list --json
pushover-pp-cli profile show briefing
pushover-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `pushover-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add pushover-pp-mcp -- pushover-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pushover-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   pushover-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `pushover-pp-cli <command> --help`.
