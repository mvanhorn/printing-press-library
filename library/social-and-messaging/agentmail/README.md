# AgentMail CLI

**AgentMail operations with local memory, safe sends, and fleet-wide insight.**

The CLI covers AgentMail's inbox, message, thread, draft, webhook, domain, list, metric, key, pod, and organization surfaces. It adds local triage queues, pre-send risk checks, conversation rollups, schedule audits, delivery reconciliation, and fleet health so agents can reason across time and resources instead of replaying isolated API calls.

## Install

The recommended path installs both the `agentmail-pp-cli` binary and the `pp-agentmail` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install agentmail
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install agentmail --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install agentmail --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install agentmail --agent claude-code
npx -y @mvanhorn/printing-press-library install agentmail --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/agentmail/cmd/agentmail-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/agentmail-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install agentmail --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-agentmail --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-agentmail --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install agentmail --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/agentmail-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `AGENTMAIL_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/agentmail/cmd/agentmail-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "agentmail": {
      "command": "agentmail-pp-mcp",
      "env": {
        "AGENTMAIL_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set AGENTMAIL_API_KEY to a bearer token from AgentMail. Configured credentials are never printed; newly created API-key and signup secrets are returned only by the upstream create response and are not persisted in the local mirror. Use drafts, --dry-run, and Idempotency-Key for controlled writes; verify the human OTP during first-time agent signup.

## Quick Start

```bash
# Check CLI wiring and auth configuration without contacting the API.
agentmail-pp-cli doctor --dry-run

# List the agent's inboxes before choosing a sending or receiving scope.
agentmail-pp-cli inboxes list --limit 10 --json

# Find relevant inbound mail with API-ranked full-text search.
agentmail-pp-cli inboxes messages search inb_demo --query "invoice overdue" --json

# Preview a human-reviewed draft mutation without creating it.
agentmail-pp-cli inboxes drafts create inb_demo --to review@example.com --subject "[REVIEW] Agent response" --text "Please review before sending." --dry-run

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local operational memory
- **`triage queue`** — Rank unresolved inbound conversations across inboxes with age, direction, labels, and pending drafts.

  _Choose this when an agent needs an actionable unresolved-mail queue instead of raw paginated messages._

  ```bash
  agentmail-pp-cli triage queue --db /tmp/agentmail.db --since 7d --json --agent
  ```
- **`thread rollup`** — Render compact conversation handoff context with participants, counts, latest direction, age, labels, and extracted reply content.

  _Choose this when an agent or human needs conversation context without repeated thread and message fetches._

  ```bash
  agentmail-pp-cli thread rollup thread_demo --db /tmp/agentmail.db --json --agent --select thread_id,latest_direction,message_count,pending_draft
  ```

### Safe automation
- **`send check`** — Review a draft for deterministic recipient, attachment, schedule, duplicate, and idempotency risks before sending.

  _Choose this before releasing a draft when a safe, auditable send decision matters more than simply calling send._

  ```bash
  agentmail-pp-cli send check draft_demo --db /tmp/agentmail.db --json --agent
  ```
- **`schedule audit`** — Find scheduled drafts that are overdue, orphaned, duplicated, or missing review state.

  _Choose this before a scheduled send window when stale or duplicate drafts need deterministic review._

  ```bash
  agentmail-pp-cli schedule audit --db /tmp/agentmail.db --due-within 24h --json --agent
  ```
- **`delivery reconcile`** — Reconcile outbound messages with status, thread placement, timestamps, and later inbound activity.

  _Choose this after a send batch when an agent must identify stale, failed, or unthreaded outcomes._

  ```bash
  agentmail-pp-cli delivery reconcile --db /tmp/agentmail.db --since 7d --json --agent
  ```

### Fleet operations
- **`fleet health`** — Report inbox, domain, webhook, list, metrics, API-key, pod, and organization readiness findings.

  _Choose this for a preflight fleet review before agents depend on multiple inboxes or tenants._

  ```bash
  agentmail-pp-cli fleet health --db /tmp/agentmail.db --json --agent
  ```

## Recipes

### Find overdue inbound work

```bash
agentmail-pp-cli triage queue --db /tmp/agentmail.db --since 7d --json --agent
```

Produce an action-ranked queue from synchronized inbox, thread, message, label, and draft state.

### Narrow a large search result

```bash
agentmail-pp-cli inboxes messages search inb_demo --query "invoice overdue" --agent --select messages.message_id,messages.subject,messages.from
```

Keep only high-value fields when a relevance-ranked message response is large.

### Review a draft before sending

```bash
agentmail-pp-cli send check draft_demo --db /tmp/agentmail.db --json --agent
```

Expose deterministic recipient, schedule, duplicate, and idempotency risks before an irreversible send.

### Audit scheduled sends

```bash
agentmail-pp-cli schedule audit --db /tmp/agentmail.db --due-within 24h --json
```

Find overdue, orphaned, duplicated, or unreviewed scheduled drafts.

### Reconcile recent delivery

```bash
agentmail-pp-cli delivery reconcile --db /tmp/agentmail.db --since 7d --json --agent
```

Correlate outbound outcomes with later inbound activity and thread placement.

## Usage

Run `agentmail-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `AGENTMAIL_CONFIG_DIR`, `AGENTMAIL_DATA_DIR`, `AGENTMAIL_STATE_DIR`, or `AGENTMAIL_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `AGENTMAIL_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export AGENTMAIL_HOME=/srv/agentmail
agentmail-pp-cli doctor
```

Under `AGENTMAIL_HOME=/srv/agentmail`, the four dirs resolve to `/srv/agentmail/config`, `/srv/agentmail/data`, `/srv/agentmail/state`, and `/srv/agentmail/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "agentmail": {
      "command": "agentmail-pp-mcp",
      "env": {
        "AGENTMAIL_HOME": "/srv/agentmail"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `AGENTMAIL_DATA_DIR` overrides an explicit `--home` for that kind. Use `AGENTMAIL_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `AGENTMAIL_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `agentmail-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### agent

Manage agent

- **`agentmail-pp-cli agent sign-up`** - Create a new agent organization with an inbox and API key. This endpoint is for signing up for the first time. If you've already signed up, you're all set — just use your existing API key.

A 6-digit OTP is sent to the human's email for verification.

This endpoint is idempotent. Calling it again with the same `human_email` will rotate the API key and resend the OTP if expired.

The returned API key has limited permissions until the organization is verified via the verify endpoint.

**CLI:**
```bash
agentmail agent sign-up --human-email user@example.com --username my-agent
```
- **`agentmail-pp-cli agent verify`** - Verify an agent organization using the 6-digit OTP sent to the human's email during sign-up.

On success, the organization is upgraded from `agent_unverified` to `agent_verified`, the send allowlist is removed, and free plan entitlements are applied.

The OTP expires after 24 hours and allows a maximum of 10 attempts. If you run into any difficulties receiving the OTP code, you can also create an account on [console.agentmail.to](https://console.agentmail.to) using the human email address you provided to verify your account.

**CLI:**
```bash
agentmail agent verify --otp-code 123456
```

### api-keys

Manage api keys

- **`agentmail-pp-cli api-keys create`** - **CLI:**
```bash
agentmail api-keys create --name "My Key"
```
- **`agentmail-pp-cli api-keys create-public-key`** - Register a public P-256 JWK using an existing AgentMail bearer API key
with `api_key_create`. Re-registering the same JWK creates a new
credential ID; it does not replace or recover an earlier credential.
The private key must never be sent to AgentMail.
- **`agentmail-pp-cli api-keys delete`** - **CLI:**
```bash
agentmail api-keys delete --api-key-id <api_key_id>
```
- **`agentmail-pp-cli api-keys list`** - **CLI:**
```bash
agentmail api-keys list
```
- **`agentmail-pp-cli api-keys list-public-keys`** - List only public-key credentials visible to the bearer caller's scope.
Bearer credentials are never returned, even though both credential types
share storage and pagination indexes. Requires `api_key_read`.
- **`agentmail-pp-cli api-keys revoke-all-agent-id-sign-in-keys`** - Invalidate every current public-key credential in the caller's
organization by advancing its AgentID key generation. The caller must be
organization-scoped and either have `api_key_delete` or, for a verified
self-serve agent organization, use an unrestricted unmanaged bearer
credential. No request body is accepted.

`Idempotency-Key` is required and must be a UUID. Reusing the same UUID
returns the original permanent receipt without advancing the generation
again. A new UUID performs a new generation advance.
- **`agentmail-pp-cli api-keys revoke-public-key`** - Permanently revoke one public-key credential. This hard-deletes the
credential; repeating the request returns not found. Requires
`api_key_delete`.
- **`agentmail-pp-cli api-keys update-public-key-name`** - Rename the credential. All security-relevant fields are immutable.
Requires `api_key_update`.

### domains

Manage domains

- **`agentmail-pp-cli domains create`** - **CLI:**
```bash
agentmail domains create --domain example.com
```
- **`agentmail-pp-cli domains delete`** - **CLI:**
```bash
agentmail domains delete --domain-id <domain_id>
```
- **`agentmail-pp-cli domains get`** - **CLI:**
```bash
agentmail domains get --domain-id <domain_id>
```
- **`agentmail-pp-cli domains list`** - **CLI:**
```bash
agentmail domains list
```
- **`agentmail-pp-cli domains update`** - **CLI:**
```bash
agentmail domains update --domain-id <domain_id>
```

### drafts

Manage drafts

- **`agentmail-pp-cli drafts get`** - **CLI:**
```bash
agentmail drafts get --draft-id <draft_id>
```
- **`agentmail-pp-cli drafts list`** - **CLI:**
```bash
agentmail drafts list
```

### inboxes

Manage inboxes

- **`agentmail-pp-cli inboxes create`** - **CLI:**
```bash
agentmail inboxes create --display-name "My Agent" --username myagent --domain agentmail.to
```
- **`agentmail-pp-cli inboxes delete`** - **CLI:**
```bash
agentmail inboxes delete --inbox-id <inbox_id>
```
- **`agentmail-pp-cli inboxes get`** - **CLI:**
```bash
agentmail inboxes get --inbox-id <inbox_id>
```
- **`agentmail-pp-cli inboxes list`** - **CLI:**
```bash
agentmail inboxes list
```
- **`agentmail-pp-cli inboxes update`** - **CLI:**
```bash
agentmail inboxes update --inbox-id <inbox_id> --display-name "Updated Name"
```

### lists

Manage lists

- **`agentmail-pp-cli lists create`** - **CLI:**
```bash
agentmail lists create --direction <direction> --type <type> --entry user@example.com
```
- **`agentmail-pp-cli lists delete`** - **CLI:**
```bash
agentmail lists delete --direction <direction> --type <type> --entry <entry>
```
- **`agentmail-pp-cli lists get`** - **CLI:**
```bash
agentmail lists get --direction <direction> --type <type> --entry <entry>
```
- **`agentmail-pp-cli lists list`** - **CLI:**
```bash
agentmail lists list --direction <direction> --type <type>
```

### metrics

Manage metrics

- **`agentmail-pp-cli metrics query-events`** - Counts of email events (sent, delivered, bounced, etc.) over time for
the organization. Defaults to the last 24 hours; `start` must be within
the last 90 days, and a future `end` is clamped to now. Omit `period`
for individual event counts, or set it to sum counts into buckets of
that many seconds.

**CLI:**
```bash
agentmail-pp-cli metrics query-events
```
- **`agentmail-pp-cli metrics query-usage`** - Cumulative usage series for the organization. Each point is the running
total of the usage type at that timestamp, not the change within the
bucket. Defaults to the last 24 hours; `start` must be within the last
90 days, and a future `end` is clamped to now. The range divided by
`period` must not exceed 1000 buckets.

### organizations

Manage organizations

- **`agentmail-pp-cli organizations`** - Returns the organization for the authenticated API key (usage limits, counts, and billing metadata).

**CLI:**
```bash
agentmail organizations get
```

### pods

Manage pods

- **`agentmail-pp-cli pods create`** - **CLI:**
```bash
agentmail pods create --client-id my-pod
```
- **`agentmail-pp-cli pods delete`** - **CLI:**
```bash
agentmail pods delete --pod-id <pod_id>
```
- **`agentmail-pp-cli pods get`** - **CLI:**
```bash
agentmail pods get --pod-id <pod_id>
```
- **`agentmail-pp-cli pods list`** - **CLI:**
```bash
agentmail pods list
```

### reference-auth

Manage reference auth

- **`agentmail-pp-cli reference-auth`** - Returns the identity and scope of the authenticated credential. Useful when a client holds a pod-scoped or inbox-scoped API key and needs to discover the parent organization, pod, or inbox without prior knowledge.

**CLI:**
```bash
agentmail-pp-cli reference-auth
```

### threads

Manage threads

- **`agentmail-pp-cli threads delete`** - Permanently deletes a thread and all of its messages.

**CLI:**
```bash
agentmail threads delete --thread-id <thread_id>
```
- **`agentmail-pp-cli threads get`** - **CLI:**
```bash
agentmail threads get --thread-id <thread_id>
```
- **`agentmail-pp-cli threads list`** - Lists threads, most recent first. Pass `senders`, `recipients`, or
`subject` to filter by substring. Filtered requests are served by
search, which caps `limit` at 100. For relevance-ranked full-text
search across senders, recipients, subject, and message body, use
`Search Threads`.

**CLI:**
```bash
agentmail threads list
```
- **`agentmail-pp-cli threads search`** - Full-text search across threads in the organization, ranked by
relevance. The query is matched against senders, recipients, and
subject (substring) and the message body (tokenized full text). Spam,
trash, blocked, and unauthenticated threads are always excluded.
`limit` cannot exceed 100.
- **`agentmail-pp-cli threads update`** - Updates thread labels. Cannot add or remove system labels (sent, received, bounced, etc.). Rejects requests with a `422` for threads with 100 or more messages.

### webhooks

Manage webhooks

- **`agentmail-pp-cli webhooks create`** - **CLI:**
```bash
agentmail webhooks create --url https://example.com/webhook --event-type message.received
```
- **`agentmail-pp-cli webhooks delete`** - **CLI:**
```bash
agentmail webhooks delete --webhook-id <webhook_id>
```
- **`agentmail-pp-cli webhooks get`** - **CLI:**
```bash
agentmail webhooks get --webhook-id <webhook_id>
```
- **`agentmail-pp-cli webhooks list`** - **CLI:**
```bash
agentmail webhooks list
```
- **`agentmail-pp-cli webhooks update`** - Update inbox or pod subscriptions, or replace the webhook's `event_types` in full when you pass a
non-empty `event_types` array (see request field docs). Inbox and pod changes use add/remove lists.

**CLI:**
```bash
agentmail webhooks update --webhook-id <webhook_id> --add-inbox-id <inbox_id>
```


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`agentmail-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`agentmail-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`agentmail-pp-cli learnings list`** - Inspect taught rows
- **`agentmail-pp-cli learnings forget <query>`** - Undo a teach
- **`agentmail-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`agentmail-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`agentmail-pp-cli teach-pattern`** - Install a query/resource template up front
- **`agentmail-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `AGENTMAIL_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `agentmail-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
agentmail-pp-cli api-keys list

# JSON for scripting and agents
agentmail-pp-cli api-keys list --json
# Filter to specific fields by name
agentmail-pp-cli api-keys list --json --select <field>[,<field>...]

# Dry run — show the request without sending
agentmail-pp-cli api-keys list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
agentmail-pp-cli api-keys list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
agentmail-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `agentmail-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/agentmail-pp-cli/config.toml`; `--home`, `AGENTMAIL_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `AGENTMAIL_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `agentmail-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `agentmail-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $AGENTMAIL_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 401 or missing credential** — Export AGENTMAIL_API_KEY and rerun agentmail-pp-cli doctor --json.
- **HTTP 429 Too Many Requests** — Honor Retry-After, reduce --limit or --max-pages, and retry with bounded backoff.
- **Duplicate inbox or message risk** — Use --client-id for creates and a stable --idempotency-key for sends, replies, and forwards.
- **Local triage returns no rows** — Run agentmail-pp-cli sync --resources inboxes,messages,threads,drafts --db /tmp/agentmail.db before raising the local scan window.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**AgentMail CLI**](https://github.com/agentmail-to/agentmail-cli) — Go
- [**AgentMail MCP**](https://github.com/agentmail-to/agentmail-mcp) — TypeScript
- [**AgentMail Python**](https://github.com/agentmail-to/agentmail-python) — Python
- [**AgentMail Node**](https://github.com/agentmail-to/agentmail-node) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
