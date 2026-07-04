---
name: pp-multimail
description: "Every MultiMail feature plus oversight velocity, trust ladder status, and cross-mailbox search no other tool has. Trigger phrases: `check my multimail inbox`, `approve pending emails`, `multimail trust status`, `search multimail for`, `sync multimail mailboxes`, `use multimail`, `run multimail`."
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
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/multimail/cmd/multimail-pp-cli
---

# MultiMail — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `multimail-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install multimail --cli-only
   ```
2. Verify: `multimail-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/multimail/cmd/multimail-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use multimail-pp-cli when you need to manage an agent email fleet — process oversight queues, monitor trust ladder progression, search across mailboxes, and track inbox health. It turns MultiMail's graduated autonomy model into a daily operator workflow.

## Anti-triggers

Do not use this CLI for:
- Sending bulk marketing email — MultiMail is for agent-to-human communication, not mass email
- Managing non-MultiMail email accounts (Gmail, Outlook) — use better-email-mcp or native MCP tools
- Real-time email streaming — use MultiMail webhooks directly for sub-second event delivery
- Account billing changes — use the MultiMail dashboard for payment and plan management

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`fleet health`** — Single-command account-wide health snapshot: mailbox count, oversight queue depth, webhook delivery rate, domain verification status, usage vs plan limits.

  _When an agent needs to know if its MultiMail integration is healthy before sending, one command replaces checking 5 separate endpoints._

  ```bash
  multimail-pp-cli fleet health --json --agent --select mailboxes.total,oversight.pending_count,webhooks.success_rate
  ```
- **`oversight bulk-decide`** — Approve or reject all pending emails matching a filter (by mailbox, sender, or age) in one command.

  _When an agent's test queue has 20 pending emails, one command clears them instead of 20 individual approve calls._

  ```bash
  multimail-pp-cli oversight bulk-decide --approve --mailbox support-agent --json
  ```
- **`oversight velocity`** — See approval/rejection rates and median decision latency per mailbox across your entire fleet.

  _When an agent's sends are stuck in approval queues, this pinpoints which mailbox's operator is the bottleneck._

  ```bash
  multimail-pp-cli oversight velocity --json --days 7
  ```
- **`trust status`** — Fleet-wide view of each mailbox's oversight mode, time-at-level, and upgrade eligibility.

  _Before requesting a trust upgrade, an agent should know which mailboxes are ready and which have been at their current level longest._

  ```bash
  multimail-pp-cli trust status --json --agent --select name,oversight_mode,time_at_level
  ```
- **`trust timeline`** — Per-mailbox chronological history of every oversight mode change with timestamps and who triggered it.

  _When evaluating whether to request a trust upgrade, an agent needs to show its history of responsible operation at each level._

  ```bash
  multimail-pp-cli trust timeline --mailbox support-agent --json
  ```
- **`emails send-and-wait`** — Send an email and block until a reply arrives or timeout — the agent test loop in one command.

  _When an agent needs to send and process the reply in one workflow step, this replaces a manual send-poll-read loop._

  ```bash
  multimail-pp-cli emails send-and-wait --mailbox test-agent --to user@example.com --subject 'Test' --body 'Hello' --timeout 5m --json
  ```

### Local state that compounds
- **`mailboxes allowlist coverage`** — See what percentage of recent recipients are covered by allowlist patterns vs gated.

  _Before adding allowlist entries, an agent should know which recipients are already covered and which cause the most gating friction._

  ```bash
  multimail-pp-cli mailboxes allowlist coverage --mailbox primary --days 30 --json --agent --select pattern,match_count,percentage
  ```
- **`inbox health`** — Per-mailbox health snapshot: unread count, oldest unread age, reply rate, and thread depth.

  _An agent monitoring its own inbox health can detect when it is falling behind on replies before the operator notices._

  ```bash
  multimail-pp-cli inbox health --json --agent --select mailbox,unread_count,oldest_unread_age,reply_rate
  ```
- **`threads stale`** — List conversation threads with no reply in N days — surfaces dropped conversations.

  _A dropped conversation thread is a customer-facing failure; this is the agent's early warning system._

  ```bash
  multimail-pp-cli threads stale --days 3 --json --agent --select thread_id,subject,last_reply_age,mailbox
  ```
- **`audit compliance`** — Cross-entity compliance report: oversight bypass count, approval/rejection counts, decision latency percentiles per mailbox.

  _When an auditor needs to verify that agents operated within their approved trust levels, this replaces manual log correlation._

  ```bash
  multimail-pp-cli audit compliance --days 30 --json
  ```
- **`webhooks health`** — Per-webhook success rate, failure count, last delivery timestamp, and consecutive failure streak.

  _When an agent's real-time event pipeline depends on webhooks, this surfaces delivery health before silent failures accumulate._

  ```bash
  multimail-pp-cli webhooks health --json --agent --select webhook_id,url,success_rate,consecutive_failures
  ```

## Command Reference

**account** — Manage account

- `multimail-pp-cli account create` — Requires a solved proof-of-work challenge. Creates a pending signup and sends a confirmation email.
- `multimail-pp-cli account create-challenge` — Returns an ALTCHA challenge. Solve it and include the solution as pow_solution in POST /v1/account.
- `multimail-pp-cli account create-resendconfirmation` — Public endpoint (no auth required). Resends the activation email with a new code for unconfirmed accounts.
- `multimail-pp-cli account delete` — Hard-deletes all tenant data (mailboxes, emails, API keys, usage, audit log). Frees the slug for re-registration.
- `multimail-pp-cli account list` — Get current tenant info and usage
- `multimail-pp-cli account update` — Update tenant settings

**admin** — Manage admin

- `multimail-pp-cli admin` — Admin-only. Creates a new API key and emails it to the tenant's oversight email.

**agent** — Manage agent

- `multimail-pp-cli agent create` — Initiates agent registration using verified_email identity assertion.
- `multimail-pp-cli agent create-auth` — Completes the auth.md registration by validating the claim_token and OTP.
- `multimail-pp-cli agent list` — Human-facing page that displays the 6-digit OTP for agent registration. Linked from the verification email.

**api-keys** — Manage api keys

- `multimail-pp-cli api-keys create` — Requires admin scope. The raw key is returned only once in the response.
- `multimail-pp-cli api-keys delete` — Requires admin scope. Returns 202 with pending_approval on first call; resend with approval_code to complete.
- `multimail-pp-cli api-keys list` — Requires admin scope. Returns key prefix, scopes, and metadata.
- `multimail-pp-cli api-keys update` — Update API key name or scopes

**approve** — Manage approve

- `multimail-pp-cli approve create` — Process approval/rejection from hosted page
- `multimail-pp-cli approve get` — Render hosted approval page for oversight decisions

**audit-log** — Manage audit log

- `multimail-pp-cli audit-log` — Returns audit log entries with cursor pagination. Requires admin scope.

**auth-md** — Manage auth md

- `multimail-pp-cli auth-md` — Returns a markdown document describing MultiMail's agent registration flow, trust ladder, and scope model.

**billing** — Manage billing

- `multimail-pp-cli billing create` — Requires admin scope.
- `multimail-pp-cli billing create-checkout` — Create a Stripe checkout session for plan upgrade
- `multimail-pp-cli billing create-coinbasewebhook` — Coinbase Commerce webhook handler (public, signature-verified)
- `multimail-pp-cli billing create-cryptocheckout` — Create a Coinbase Commerce checkout (crypto payment)
- `multimail-pp-cli billing create-portal` — Requires admin scope.
- `multimail-pp-cli billing create-pricingcheckout` — Creates an inactive tenant, provisions a default mailbox, and returns a Stripe checkout URL.
- `multimail-pp-cli billing create-stripewebhook` — Stripe webhook handler (public, signature-verified)
- `multimail-pp-cli billing list` — Public endpoint. Returns the API key stored during pricing-checkout, then deletes it.

**confirm** — Manage confirm

- `multimail-pp-cli confirm create` — JSON response includes: status, name, oversight_mode, api_key, mailbox_id, mailbox_address, oversight_email, use_case.
- `multimail-pp-cli confirm get` — Redirect to frontend confirmation page with code prefilled
- `multimail-pp-cli confirm list` — Redirect to frontend confirmation page at multimail.dev/confirm

**contacts** — Manage contacts

- `multimail-pp-cli contacts create` — Add a contact to the address book. Requires send scope.
- `multimail-pp-cli contacts delete` — Requires admin scope.
- `multimail-pp-cli contacts list` — Search address book by name or email. Omit query to list all. Requires read scope.

**data_export** — Manage data export

- `multimail-pp-cli data-export` — Requires admin scope. Rate limited to 1 request per hour.

**domains** — Manage domains

- `multimail-pp-cli domains create` — Add a custom domain (Pro/Scale only)
- `multimail-pp-cli domains delete` — Delete a custom domain
- `multimail-pp-cli domains get` — Get custom domain detail
- `multimail-pp-cli domains list` — Requires admin scope.

**emails** — Manage emails

- `multimail-pp-cli emails` — Requires read scope.

**funnel** — Manage funnel

- `multimail-pp-cli funnel` — Pricing page beacon hit via navigator.sendBeacon to track open/submit/error events on the signup modal.

**health** — Manage health

- `multimail-pp-cli health` — Verifies D1 and R2 connectivity. No auth required.

**mailboxes** — Manage mailboxes

- `multimail-pp-cli mailboxes create` — Requires admin scope.
- `multimail-pp-cli mailboxes delete` — Requires admin scope.
- `multimail-pp-cli mailboxes list` — Requires read scope.
- `multimail-pp-cli mailboxes update` — Requires admin scope. Oversight mode can only be downgraded here; upgrades require the upgrade flow.

**operator** — Manage operator

- `multimail-pp-cli operator create` — Requires admin scope. Clears the operator-session cookie.
- `multimail-pp-cli operator create-startsession` — Requires admin scope. Sends a one-time code to the oversight email and begins the operator-session OTP flow.
- `multimail-pp-cli operator create-verifysession` — Requires admin scope. Exchanges a one-time code for a short-lived HttpOnly operator-session cookie.
- `multimail-pp-cli operator list` — Requires admin scope. Reports whether the current browser has an active operator-session cookie.

**oversight** — Manage oversight

- `multimail-pp-cli oversight create` — Requires oversight scope. Approved outbound emails are sent immediately.
- `multimail-pp-cli oversight list` — List emails pending oversight approval

**slug-check** — Manage slug check

- `multimail-pp-cli slug-check <slug>` — Check if a slug is available for registration. Returns suggestions if taken or reserved. No auth required.

**support** — Manage support

- `multimail-pp-cli support` — Public endpoint. Requires a solved ALTCHA proof-of-work payload. Sends a message to support@multimail.dev.

**suppression** — Manage suppression

- `multimail-pp-cli suppression delete` — Allows future emails to be sent to this address again. Requires admin scope.
- `multimail-pp-cli suppression list` — Returns addresses suppressed due to bounces, spam complaints, or manual unsubscribes. Requires admin scope.

**unsubscribe** — Manage unsubscribe

- `multimail-pp-cli unsubscribe create` — Process unsubscribe request
- `multimail-pp-cli unsubscribe get` — Render unsubscribe page (CAN-SPAM)

**usage** — Manage usage

- `multimail-pp-cli usage` — Requires read scope. Returns usage counts for the current billing period.

**webhook-deliveries** — Manage webhook deliveries

- `multimail-pp-cli webhook-deliveries` — Returns recent webhook delivery attempts. Requires admin scope.

**webhooks** — Manage webhooks

- `multimail-pp-cli webhooks create` — Subscribe to email events. Returns the signing secret (shown only on creation). Requires admin scope.
- `multimail-pp-cli webhooks create-postmark` — Postmark bounce/complaint/delivery webhook handler
- `multimail-pp-cli webhooks create-postmarkinbound` — Receives inbound emails from Postmark. Authenticated via HTTP Basic Auth with the Postmark webhook secret.
- `multimail-pp-cli webhooks delete` — Delete a webhook subscription
- `multimail-pp-cli webhooks get` — Includes signing secret. Requires admin scope.
- `multimail-pp-cli webhooks list` — Requires admin scope. Signing secrets are not included in the list.

**well-known** — Manage well known

- `multimail-pp-cli well-known get` — Rate-limited to 10 lookups per IP per hour.
- `multimail-pp-cli well-known list` — Returns the ECDSA P-256 public key used to sign X-MultiMail-Identity headers.
- `multimail-pp-cli well-known list-wellknown` — Returns OAuth authorization server metadata with an agent_auth extension block describing the auth.
- `multimail-pp-cli well-known list-wellknown-2` — Returns metadata about MultiMail as an OAuth-protected resource, including supported scopes and authorization servers.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
multimail-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Approve all pending emails for a mailbox

```bash
multimail-pp-cli oversight list --json --select id,subject,to | multimail-pp-cli oversight bulk-decide --approve --stdin
```

Pipe pending email IDs into batch approval — useful for clearing a gated mailbox queue.

### Fleet trust ladder overview

```bash
multimail-pp-cli trust status --json --agent --select name,oversight_mode,time_at_level
```

See which mailboxes are at which trust level and how long they have been there.

### Cross-mailbox search with field selection

```bash
multimail-pp-cli search 'quarterly report' --type emails --json --agent --select subject,from,mailbox,received_at
```

Find a specific email across all synced mailboxes with narrow field output for agent context efficiency.

### Sync and check inbox health

```bash
multimail-pp-cli sync --resources mailboxes,emails --since 24h && multimail-pp-cli inbox health --json
```

Refresh local data then check unread counts, reply rates, and oldest unread age per mailbox.

### Allowlist coverage analysis

```bash
multimail-pp-cli mailboxes allowlist coverage --mailbox primary --days 30 --json --agent --select pattern,match_count,percentage
```

See what percentage of recent recipients match allowlist patterns — find which addresses cause the most gating friction.

## Auth Setup

MultiMail uses Bearer token auth with prefixed keys: mm_live_* for production, mm_test_* for staging. Get your key from the MultiMail dashboard or via the API key management endpoints.

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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
multimail-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
multimail-pp-cli feedback --stdin < notes.txt
multimail-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/multimail-pp-cli/feedback.jsonl`. They are never POSTed unless `MULTIMAIL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MULTIMAIL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/multimail/cmd/multimail-pp-mcp@latest
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
