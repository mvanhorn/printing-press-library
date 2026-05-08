---
name: pp-multimail
description: "Every MultiMail feature from the terminal, plus inbox health, stale-thread detection, and offline search no other MultiMail tool has. Trigger phrases: `check my multimail inbox`, `send email via multimail`, `multimail health check`, `how many emails pending oversight`, `multimail trust status`, `use multimail`, `run multimail`."
author: "H179922"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - multimail-pp-cli
    install:
      - kind: go
        bins: [multimail-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/multimail/cmd/multimail-pp-cli
---

# MultiMail — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `multimail-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install multimail --cli-only
   ```
2. Verify: `multimail-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/multimail/cmd/multimail-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

A Go CLI that matches all 47 MCP server tools as shell commands, adds a local SQLite cache with FTS5 search, and introduces compound commands that surface compliance insights the API alone cannot answer. Agent-native by default: auto-JSON when piped, --compact for token-efficient output, typed exit codes for scripting.

## When to Use This CLI

Use the MultiMail CLI when you need email operations from a shell environment — CI/CD pipelines, Codex tasks, agentic shells, or automation scripts. The CLI excels at batch operations (sync + search + health check in one pipeline), offline analysis (stale threads, inbox health after a single sync), and compliance monitoring (oversight dashboard, trust ladder status). Prefer the MCP server when operating inside a chat-based agent that supports MCP natively.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Compliance observatory

- **`health`** — Single-number composite score combining unread ratio, response time, bounce rate, and quota headroom — instantly tells you if an inbox needs attention.

  _When managing multiple agent mailboxes, reach for this to triage which inbox needs attention first without checking each one individually._

  ```bash
  multimail-pp-cli health --mailbox primary --json
  ```
- **`stale`** — Find conversation threads that have gone unanswered past a configurable threshold — surface the emails you're dropping the ball on.

  _Before composing new emails, check stale threads first — replying to an existing conversation is almost always higher-value than starting a new one._

  ```bash
  multimail-pp-cli stale --days 3 --json
  ```
- **`oversight summary`** — See pending approval count, average decision time, approval rate, and most-gated senders across all mailboxes — the operator's command center.

  _When the operator hasn't checked in, run this to know if the oversight queue is backing up and which senders are triggering the most gates._

  ```bash
  multimail-pp-cli oversight summary --json
  ```
- **`trust status`** — Current trust ladder position per mailbox, what's needed for the next level, and progression history — the agent's autonomy roadmap.

  _Before requesting an oversight upgrade, check your current position and what the operator needs to see before granting more autonomy._

  ```bash
  multimail-pp-cli trust status --json
  ```

### Local state that compounds

- **`quota forecast`** — Predict when your email quota will be exhausted based on rolling send rate — days remaining with confidence interval.

  _Before scheduling a large email batch, check quota forecast to know if you'll hit limits and need to suggest a plan upgrade._

  ```bash
  multimail-pp-cli quota forecast --json
  ```
- **`stats`** — Send/receive volume, top correspondents, peak hours, and delivery rate over a configurable period — understand email patterns at a glance.

  _When planning outreach campaigns or reviewing agent communication patterns, stats gives you the baseline numbers to work from._

  ```bash
  multimail-pp-cli stats --period 30d --json
  ```

### Agent-native plumbing

- **`search`** — Full-text search across cached emails — works without network after sync, searches subjects, bodies, senders, and recipients.

  _When searching for a specific email or topic, use search instead of paginating through inbox results — especially useful in CI/CD where network calls add latency._

  ```bash
  multimail-pp-cli search 'invoice overdue' --json --compact
  ```
- **`sync`** — Cursor-tracked incremental sync of all entities to local SQLite — enables every compound command and offline access.

  _Run sync before any local query to ensure fresh data. After the first full sync, incrementals are fast and cheap._

  ```bash
  multimail-pp-cli sync --full
  ```

## Command Reference

**account** — Manage account

- `multimail-pp-cli account create` — Create a new account
- `multimail-pp-cli account create-challenge` — Request a verification challenge for account creation
- `multimail-pp-cli account create-resendconfirmation` — Resend the account activation email
- `multimail-pp-cli account delete` — Permanently delete account and all associated data
- `multimail-pp-cli account list` — Get current account info and usage
- `multimail-pp-cli account update` — Update account settings

**admin** — Create and email a new API key to the account owner

- `multimail-pp-cli admin` — Admin-only. Creates a new API key. Required: reason, tenant_id.

**api-keys** — Manage api keys

- `multimail-pp-cli api-keys create` — Create a new API key. Requires admin scope.
- `multimail-pp-cli api-keys delete` — Delete an API key (requires admin scope, two-step approval)
- `multimail-pp-cli api-keys list` — Requires admin scope. Returns key prefix, scopes, and metadata.
- `multimail-pp-cli api-keys update` — Update API key name or scopes

**audit-log** — Manage audit log

- `multimail-pp-cli audit-log` — Returns audit log entries with cursor pagination. Requires admin scope.

**billing** — Manage billing

- `multimail-pp-cli billing create` — Cancel subscription (retains access until end of billing period)
- `multimail-pp-cli billing create-checkout` — Create a checkout session for plan upgrade
- `multimail-pp-cli billing create-cryptocheckout` — Create a crypto payment checkout
- `multimail-pp-cli billing create-portal` — Open the billing management portal
- `multimail-pp-cli billing create-pricingcheckout` — Start the signup checkout flow
- `multimail-pp-cli billing list` — Retrieve your API key after checkout

**confirm** — Manage confirm

- `multimail-pp-cli confirm create` — Activate account with confirmation code

**contacts** — Manage contacts

- `multimail-pp-cli contacts create` — Add a contact to the address book. Requires send scope.
- `multimail-pp-cli contacts delete` — Requires admin scope.
- `multimail-pp-cli contacts list` — Search address book by name or email. Omit query to list all. Requires read scope.

**domains** — Manage domains

- `multimail-pp-cli domains create` — Add a custom domain (Pro/Scale only)
- `multimail-pp-cli domains delete` — Delete a custom domain
- `multimail-pp-cli domains get` — Get custom domain detail
- `multimail-pp-cli domains list` — Requires admin scope.

**emails** — List spam and quarantined emails across all mailboxes

- `multimail-pp-cli emails` — Requires read scope. List emails with optional status filter.

**funnel** — Track funnel analytics events

- `multimail-pp-cli funnel` — Record a funnel analytics event.

**mailboxes** — Manage mailboxes

- `multimail-pp-cli mailboxes create` — Create a new mailbox. Requires admin scope.
- `multimail-pp-cli mailboxes delete` — Requires admin scope.
- `multimail-pp-cli mailboxes list` — Requires read scope.
- `multimail-pp-cli mailboxes update` — Update mailbox settings. Requires admin scope.

**multimail-export** — Manage multimail export

- `multimail-pp-cli multimail-export` — Requires admin scope. Rate limited to 1 request per hour.

**multimail-health** — Check API health status

- `multimail-pp-cli multimail-health` — Health check. No auth required.

**operator** — Manage operator

- `multimail-pp-cli operator create` — End operator session. Requires admin scope.
- `multimail-pp-cli operator create-startsession` — Start operator session. Sends a verification code. Requires admin scope.
- `multimail-pp-cli operator create-verifysession` — Verify operator session with one-time code. Requires admin scope.
- `multimail-pp-cli operator list` — Check operator session status. Requires admin scope.

**oversight** — Manage oversight

- `multimail-pp-cli oversight create` — Requires oversight scope. Approved outbound emails are sent immediately.
- `multimail-pp-cli oversight list` — List emails pending oversight approval

**slug-check** — Check if an account name is available

- `multimail-pp-cli slug-check <slug>` — Check if an account name is available. Returns suggestions if taken or reserved. No auth required.

**support** — Submit a support request

- `multimail-pp-cli support` — Send a support message. Requires a verification challenge.

**suppression** — Manage suppression

- `multimail-pp-cli suppression delete` — Allows future emails to be sent to this address again. Requires admin scope.
- `multimail-pp-cli suppression list` — Returns addresses suppressed due to bounces, spam complaints, or manual unsubscribes. Requires admin scope.

**unsubscribe** — Manage unsubscribe

- `multimail-pp-cli unsubscribe create` — Process unsubscribe request
- `multimail-pp-cli unsubscribe get` — Process unsubscribe (CAN-SPAM)

**usage** — Manage usage

- `multimail-pp-cli usage` — Requires read scope. Returns usage counts for the current billing period.

**webhook-deliveries** — Manage webhook deliveries

- `multimail-pp-cli webhook-deliveries` — Returns recent webhook delivery attempts. Requires admin scope.

**webhooks** — Manage webhooks

- `multimail-pp-cli webhooks create` — Subscribe to email events. Requires admin scope.
- `multimail-pp-cli webhooks delete` — Delete a webhook subscription
- `multimail-pp-cli webhooks get` — Get webhook details. Requires admin scope.
- `multimail-pp-cli webhooks list` — List webhook subscriptions. Requires admin scope.

**well-known** — Manage well known

- `multimail-pp-cli well-known get` — Look up sender identity by hash
- `multimail-pp-cli well-known list` — Get the public signing key


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
multimail-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning inbox triage

```bash
multimail-pp-cli sync && multimail-pp-cli health && multimail-pp-cli stale --days 2
```

Sync fresh data, check overall health, then surface any threads needing a response.

### Agent-friendly inbox scan

```bash
multimail-pp-cli inbox --limit 20 --json --compact --select id,subject,from,date
```

Fetch recent emails with only the fields an agent needs, keeping context window small.

### Oversight queue check

```bash
multimail-pp-cli oversight summary --json --select pending_count,avg_decision_time,approval_rate
```

Quick operator dashboard with just the key metrics.

### Send from pipeline

```bash
echo '# Monthly Report\n\nAttached.' | multimail-pp-cli send --to user@example.com --subject 'Monthly Report' --stdin
```

Pipe markdown content directly to send, useful in CI/CD.

### Quota monitoring

```bash
multimail-pp-cli quota forecast --json | jq '.days_remaining'
```

Extract days until quota exhaustion for alerting pipelines.

## Auth Setup
Set your API key via environment variable:

```bash
export MULTIMAIL_API_KEY="<your-key>"
```

Or persist it in `~/.config/multimail-pp-cli/config.toml`.

Run `multimail-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  multimail-pp-cli account list --agent --select id,name,status
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
multimail-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
multimail-pp-cli feedback --stdin < notes.txt
multimail-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.multimail-pp-cli/feedback.jsonl`. They are never POSTed unless `MULTIMAIL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MULTIMAIL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration.

```
multimail-pp-cli profile save briefing --json
multimail-pp-cli --profile briefing account list
multimail-pp-cli profile list --json
multimail-pp-cli profile show briefing
multimail-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `multimail-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/multimail/cmd/multimail-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add multimail-pp-mcp -- multimail-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which multimail-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   multimail-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `multimail-pp-cli <command> --help`.
