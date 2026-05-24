---
name: pp-multimail
description: "Every MultiMail feature, plus cross-mailbox search, oversight analytics, and trust ladder tracking no other tool has. Trigger phrases: `check my multimail inbox`, `send email via multimail`, `search emails across mailboxes`, `check oversight approval queue`, `add recipient to allowlist`, `register agent with multimail`, `check trust ladder status`."
author: "H179922"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - multimail-pp-cli
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

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Full CLI access to MultiMail's agent email platform. Manage mailboxes, send and read emails, configure oversight modes, and manage sending allowlists. Local SQLite cache with FTS5 search enables cross-mailbox queries, oversight velocity tracking, and trust progression analysis that the API alone cannot provide.

## When to Use This CLI

Use this CLI when you need shell-level access to MultiMail — in CI/CD pipelines, Docker containers, SSH sessions, or agentic shells where MCP is unavailable. The local SQLite cache and FTS5 search make it the right choice for cross-mailbox queries, oversight analytics, and trust progression tracking that the per-mailbox API cannot provide.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`search`** — Full-text search across all synced mailboxes at once — find any email regardless of which mailbox received it.

  _When an agent needs to find a specific email and doesn't know which mailbox has it, one search beats iterating._

  ```bash
  multimail-pp-cli search 'invoice Q2' --json --select subject,from,mailbox
  ```
- **`mailboxes allowlist coverage`** — See what percentage of recent recipients are covered by allowlist patterns vs gated.

  _Before adding allowlist entries, an agent should know which recipients are already covered and which cause the most gating friction._

  ```bash
  multimail-pp-cli mailboxes allowlist coverage --mailbox primary --days 30 --json
  ```
- **`inbox health`** — Per-mailbox health snapshot: unread count, oldest unread age, reply rate, and thread depth.

  _An agent monitoring its own inbox health can detect when it's falling behind on replies before the operator notices._

  ```bash
  multimail-pp-cli inbox health --json
  ```
- **`mailboxes threads stale`** — List conversation threads with no reply in N days — surfaces dropped conversations.

  _A dropped conversation thread is a customer-facing failure; this is the agent's early warning system._

  ```bash
  multimail-pp-cli mailboxes threads stale --days 3 --json
  ```

### Agent-native plumbing
- **`oversight velocity`** — See approval/rejection rates and median decision latency per mailbox across your entire fleet.

  _When an agent's sends are stuck in approval queues, this pinpoints which mailbox's operator is the bottleneck._

  ```bash
  multimail-pp-cli oversight velocity --json --days 7
  ```
- **`trust status`** — Fleet-wide view of each mailbox's oversight mode, time-at-level, and upgrade history.

  _Before requesting a trust upgrade, an agent should know which mailboxes are ready and which have been at their current level longest._

  ```bash
  multimail-pp-cli trust status --json
  ```

## Command Reference

**account** — Manage account

- `multimail-pp-cli account create` — Requires a solved proof-of-work challenge. Creates a pending signup and sends a confirmation email. Response is...
- `multimail-pp-cli account create-challenge` — Returns an ALTCHA challenge. Solve it and include the solution as pow_solution in POST /v1/account. Challenge...
- `multimail-pp-cli account create-resendconfirmation` — Public endpoint (no auth required). Resends the activation email with a new code for unconfirmed accounts. Rate...
- `multimail-pp-cli account delete` — Hard-deletes all tenant data (mailboxes, emails, API keys, usage, audit log). Frees the slug for re-registration....
- `multimail-pp-cli account list` — Get current tenant info and usage
- `multimail-pp-cli account update` — Update tenant settings

**admin** — Manage admin

- `multimail-pp-cli admin` — Admin-only. Creates a new API key and emails it to the tenant's oversight email. Used when welcome email failed or...

**agent** — Manage agent

- `multimail-pp-cli agent create` — Initiates agent registration using verified_email identity assertion. Sends a 6-digit OTP to the provided email and...
- `multimail-pp-cli agent create-auth` — Completes the auth.md registration by validating the claim_token and OTP. On success, atomically creates the tenant...
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

- `multimail-pp-cli auth-md` — Returns a markdown document describing MultiMail's agent registration flow, trust ladder, and scope model. Used by...

**billing** — Manage billing

- `multimail-pp-cli billing create` — Requires admin scope. Sets cancel_at_period_end on the Stripe subscription so the tenant retains access until the...
- `multimail-pp-cli billing create-checkout` — Create a Stripe checkout session for plan upgrade
- `multimail-pp-cli billing create-coinbasewebhook` — Coinbase Commerce webhook handler (public, signature-verified)
- `multimail-pp-cli billing create-cryptocheckout` — Create a Coinbase Commerce checkout (crypto payment)
- `multimail-pp-cli billing create-portal` — Requires admin scope. Returns a URL to the Stripe-hosted billing portal for self-service invoice, payment method,...
- `multimail-pp-cli billing create-pricingcheckout` — Creates an inactive tenant, provisions a default mailbox, and returns a Stripe checkout URL. After payment, call GET...
- `multimail-pp-cli billing create-stripewebhook` — Stripe webhook handler (public, signature-verified)
- `multimail-pp-cli billing list` — Public endpoint. Returns the API key stored during pricing-checkout, then deletes it. Key expires after 1 hour if...

**confirm** — Manage confirm

- `multimail-pp-cli confirm create` — JSON response includes: status, name, oversight_mode, api_key, mailbox_id, mailbox_address, oversight_email,...
- `multimail-pp-cli confirm get` — Redirect to frontend confirmation page with code prefilled
- `multimail-pp-cli confirm list` — Redirect to frontend confirmation page at multimail.dev/confirm

**contacts** — Manage contacts

- `multimail-pp-cli contacts create` — Add a contact to the address book. Requires send scope.
- `multimail-pp-cli contacts delete` — Requires admin scope.
- `multimail-pp-cli contacts list` — Search address book by name or email. Omit query to list all. Requires read scope.

**domains** — Manage domains

- `multimail-pp-cli domains create` — Add a custom domain (Pro/Scale only)
- `multimail-pp-cli domains delete` — Delete a custom domain
- `multimail-pp-cli domains get` — Get custom domain detail
- `multimail-pp-cli domains list` — Requires admin scope.

**emails** — Manage emails

- `multimail-pp-cli emails` — Requires read scope. Without a status filter, returns spam_flagged and spam_quarantined emails across all tenant...

**funnel** — Manage funnel

- `multimail-pp-cli funnel` — Pricing page beacon hit via navigator.sendBeacon to track open/submit/error events on the signup modal....

**mailboxes** — Manage mailboxes

- `multimail-pp-cli mailboxes create` — Requires admin scope. Address can be a local part (appended to tenant subdomain) or full address on a verified...
- `multimail-pp-cli mailboxes delete` — Requires admin scope.
- `multimail-pp-cli mailboxes list` — Requires read scope.
- `multimail-pp-cli mailboxes update` — Requires admin scope. Oversight mode can only be downgraded here; upgrades require the upgrade flow.

**multimail-export** — Manage multimail export

- `multimail-pp-cli multimail-export` — Requires admin scope. Rate limited to 1 request per hour.

**multimail-health** — Manage multimail health

- `multimail-pp-cli multimail-health` — Verifies D1 and R2 connectivity. No auth required.

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
- `multimail-pp-cli webhooks create-postmarkinbound` — Receives inbound emails from Postmark. Authenticated via HTTP Basic Auth with the Postmark webhook secret. Not a...
- `multimail-pp-cli webhooks delete` — Delete a webhook subscription
- `multimail-pp-cli webhooks get` — Includes signing secret. Requires admin scope.
- `multimail-pp-cli webhooks list` — Requires admin scope. Signing secrets are not included in the list.

**well-known** — Manage well known

- `multimail-pp-cli well-known get` — Rate-limited to 10 lookups per IP per hour.
- `multimail-pp-cli well-known list` — Returns the ECDSA P-256 public key used to sign X-MultiMail-Identity headers.
- `multimail-pp-cli well-known list-wellknown` — Returns OAuth authorization server metadata with an agent_auth extension block describing the auth.md agent...
- `multimail-pp-cli well-known list-wellknown-2` — Returns metadata about MultiMail as an OAuth-protected resource, including supported scopes and authorization...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
multimail-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning inbox triage

```bash
multimail-pp-cli inbox health --json --select mailbox,unread,oldest_unread_age
```

Check inbox health across all mailboxes to prioritize which needs attention first.

### Find a conversation across mailboxes

```bash
multimail-pp-cli search 'quarterly report' --json --select subject,from,mailbox,date
```

Full-text search all synced mailboxes — no need to know which mailbox has the email.

### Check oversight bottlenecks

```bash
multimail-pp-cli oversight velocity --days 7 --json --select mailbox,approval_rate,median_latency
```

See which mailboxes have slow approval rates so you can address operator bottlenecks.

### Allowlist gap analysis

```bash
multimail-pp-cli mailboxes allowlist coverage --mailbox primary --days 30 --json
```

See what percentage of recent sends were covered by allowlist patterns vs gated.

### Detect dropped conversations

```bash
multimail-pp-cli mailboxes threads stale --days 5 --json --select subject,last_activity,mailbox
```

Find threads with no reply in 5 days — each is a potential customer-facing failure.

## Auth Setup

Run `multimail-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
multimail-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `MULTIMAIL_BEARER_AUTH` as an environment variable.

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

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add multimail-pp-mcp -- multimail-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which multimail-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   multimail-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `multimail-pp-cli <command> --help`.
