# Unipile CLI

**Every Unipile endpoint as a typed command, plus a local mirror that makes cross-provider search possible and an invitation ledger that shows how much LinkedIn headroom you have left.**

Unipile unified LinkedIn, WhatsApp, Telegram, Instagram, Messenger, X, Gmail, Outlook, IMAP, and calendars into one API, then stopped at the API. This is the missing operator layer: the full endpoint surface as shell-native commands with structured errors and cursor auto-pagination, a local SQLite mirror that answers cross-provider questions no single call can, and a budget command that counts invitations already sent against the caps LinkedIn enforces but Unipile does not.

## Install

The recommended path installs both the `unipile-pp-cli` binary and the `pp-unipile` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install unipile
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install unipile --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install unipile --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install unipile --agent claude-code
npx -y @mvanhorn/printing-press-library install unipile --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/unipile/cmd/unipile-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/unipile-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install unipile --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-unipile --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-unipile --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install unipile --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/unipile-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `UNIPILE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/unipile/cmd/unipile-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "unipile": {
      "command": "unipile-pp-mcp",
      "env": {
        "UNIPILE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Unipile authenticates with an Access Token sent as the X-API-KEY header, and every customer gets their own DSN base URL. Set UNIPILE_API_KEY to your token and UNIPILE_BASE_URL to your DSN from the Unipile dashboard. If your environment blocks custom ports, Unipile also accepts the port as a query parameter on standard 443.

## Quick Start

```bash
# confirm the DSN and token are wired before touching a provider
unipile-pp-cli doctor --dry-run

# see which provider accounts are connected and healthy
unipile-pp-cli accounts list --json

# pull chats, messages, attendees, and relations into the local mirror
unipile-pp-cli sync

# triage everything unread across every provider in one table
unipile-pp-cli inbox --limit 10

# check LinkedIn headroom before running any outreach
unipile-pp-cli budget

```

## Unique Features

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

## Usage

Run `unipile-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `UNIPILE_CONFIG_DIR`, `UNIPILE_DATA_DIR`, `UNIPILE_STATE_DIR`, or `UNIPILE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `UNIPILE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export UNIPILE_HOME=/srv/unipile
unipile-pp-cli doctor
```

Under `UNIPILE_HOME=/srv/unipile`, the four dirs resolve to `/srv/unipile/config`, `/srv/unipile/data`, `/srv/unipile/state`, and `/srv/unipile/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `UNIPILE_DATA_DIR` overrides an explicit `--home` for that kind. Use `UNIPILE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `UNIPILE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `unipile-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### accounts

Accounts  management

- **`unipile-pp-cli accounts connect`** - Link to Uniple an account of the given type and provider.
- **`unipile-pp-cli accounts delete`** - Unlink the given account to Unipile.
- **`unipile-pp-cli accounts get`** - Retrieve the details of an account.
- **`unipile-pp-cli accounts list`** - Returns a list of the accounts linked to Unipile.
- **`unipile-pp-cli accounts reconnect`** - Reconnect an account previously linked to Unipile that has been disconnected.
- **`unipile-pp-cli accounts resend-checkpoint`** - Might it be 2FA, OTP or In-app Validation, this route makes you able on certain providers to resend the notification.
- **`unipile-pp-cli accounts solve-checkpoint`** - Allows you to provide a code which will solve a checkpoint encountered during a native authentication. A checkpoint is a security step added by a provider which needs to be solved to complete the authentication. Checkpoints that require a code are 2FA (two-factor authentication) and OTP (one-time password). Depending on the way you have configured the account you are trying to authenticate, you can get your code from various ways such as a mail, a SMS or from a two-factor authentication app.
- **`unipile-pp-cli accounts update`** - Update the proxy configuration of an existing account. You can provide a custom proxy, or request a new Unipile proxy by country/IP.

### calendars

Calendars management

- **`unipile-pp-cli calendars get`** - Retrieve the details of a calendar.
- **`unipile-pp-cli calendars list`** - Returns a list of calendars.

### chat-attendees

Manage chat attendees

- **`unipile-pp-cli chat-attendees get`** - The id of the wanted attendee.
- **`unipile-pp-cli chat-attendees list`** - Returns a list of messaging attendees. Some optional parameters are available to filter the results.

### chats

Manage chats

- **`unipile-pp-cli chats delete`** - Delete a chat. Supported for WhatsApp and LinkedIn only.
- **`unipile-pp-cli chats get`** - Retrieve the details of a chat.
- **`unipile-pp-cli chats list`** - Returns a list of chats. Some optional parameters are available to filter the results.
- **`unipile-pp-cli chats start`** - Start a new conversation with one or more attendee. ⚠️ Interactive documentation does not work for Linkedin specific parameters (child parameters not correctly applied in snippet), the correct format is linkedin[inmail] = true, linkedin[api]...
- **`unipile-pp-cli chats update`** - Perform an action like changing the read status, muting the chat, retrieving a group invite link, etc.

### drafts

Manage drafts

- **`unipile-pp-cli drafts`** - ⚠️ Interactive documentation does not work on this route (child parameters not correctly applied in snippet), Create a new draft.

### emails

Emails management

- **`unipile-pp-cli emails delete`** - Delete an email by moving it to the Trash folder.
- **`unipile-pp-cli emails get`** - Retrieve the details of an email.
- **`unipile-pp-cli emails list`** - Returns a list of emails.
- **`unipile-pp-cli emails list-contacts`** - Returns a list of contacts from the email provider. Supported for Gmail (Google OAuth) and Microsoft (Outlook) only.
- **`unipile-pp-cli emails send`** - ⚠️ Interactive documentation does not work on this route (child parameters not correctly applied in snippet), please use our ready to copy past example of this page : https://developer.unipile.com/docs/send-email
- **`unipile-pp-cli emails update`** - Update an email.

### folders

Manage folders

- **`unipile-pp-cli folders get`** - Retrieve the details of a mail folder.
- **`unipile-pp-cli folders list`** - Returns a list of all email folders.

### hosted

Manage hosted

- **`unipile-pp-cli hosted`** - Create a url which redirect to Unipile's hosted authentication to connect or reconnect an account.

### linkedin

Manage linkedin

- **`unipile-pp-cli linkedin action-user`** - Add a candidate to a Recruiter pipeline, save a Sales Navigator lead, etc.
- **`unipile-pp-cli linkedin close-jobs`** - Close a job offer you have posted.
- **`unipile-pp-cli linkedin company`** - Get a company profile from its name or ID.
- **`unipile-pp-cli linkedin create-jobs`** - Create a new job offer draft.
- **`unipile-pp-cli linkedin endorse-profile`** - This route can be used to endorse a skill of a user profile.
- **`unipile-pp-cli linkedin get-applicants`** - Retrieve the details of a user that has applied to a given offer. Applies to Classic job posting only.
- **`unipile-pp-cli linkedin get-jobs`** - Retrieve a job offer.
- **`unipile-pp-cli linkedin get-projects`** - Retrieve Recruiter hiring project from ID
- **`unipile-pp-cli linkedin inmail-balance`** - Get balance for subscribed premium features.
- **`unipile-pp-cli linkedin list-applicants`** - Retrieve all the users that have applied to a given offer.
- **`unipile-pp-cli linkedin list-contracts`** - Returns a list of your LinkedIn available contracts
- **`unipile-pp-cli linkedin list-jobs`** - Retrieve the job offers you have posted on LinkedIn whether they are open, closed or still drafts.
- **`unipile-pp-cli linkedin list-projects`** - Retrieve list of LinkedIn Recruiter hiring projects.
- **`unipile-pp-cli linkedin parameters-search`** - LinkedIn doesn't accept raw text as search parameters, but IDs. This route will help you get the right IDs for your inputs. Check out our Guide with examples to master LinkedIn search : https://developer.unipile.com/docs/linkedin-search
- **`unipile-pp-cli linkedin publish-jobs`** - Publish the job posting draft you have been working on.
- **`unipile-pp-cli linkedin raw`** - This magic route is intended for advanced users who wish to use LinkedIn's features beyond our current capabilities. It enables you to create custom functionalities that are not yet supported by our platform, using connected accounts. To utilize this route, you will need to identify the specific endpoint containing the desired data using web developer tools on LinkedIn, and then copy the URL along with its parameters for implementation.
- **`unipile-pp-cli linkedin resume-applicants`** - This route can be used to download the resume of a job applicant.
- **`unipile-pp-cli linkedin search`** - Search people and companies from the Linkedin Classic as well as Sales Navigator APIs. Check out our Guide with examples to master LinkedIn search : https://developer.unipile.com/docs/linkedin-search
- **`unipile-pp-cli linkedin select-contracts`** - Select a Recruiter or Sales navigator contract to be used on your account
- **`unipile-pp-cli linkedin solve-checkpoint-jobs`** - Solve a checkpoint to verify your member privilegies.
- **`unipile-pp-cli linkedin update-jobs`** - Edit an existing job posting.

### messages

Manage messages

- **`unipile-pp-cli messages delete`** - Delete a message. Supported for WhatsApp and LinkedIn only. For LinkedIn, it is only possible within the first 60 minutes after the message is sent.
- **`unipile-pp-cli messages get`** - Retrieve the details of a message.
- **`unipile-pp-cli messages list`** - Returns a list of messages. Some optional parameters are available to filter the results.
- **`unipile-pp-cli messages update`** - Edit a message. Supported for WhatsApp and LinkedIn Classic only. WhatsApp may reject edits after an indicative 15-minute window following the original send time. LinkedIn Classic edits are only possible within the first 60 minutes after the message is sent.

### posts

Posts features

- **`unipile-pp-cli posts create`** - Publish a post.
- **`unipile-pp-cli posts get`** - Retrieve the details of a post.
- **`unipile-pp-cli posts react`** - React to either a post or a post comment.

### users

Users features

- **`unipile-pp-cli users cancel-sent`** - Cancel a pending invitation sent to someone. <br>`Instagram`: You can also use this route to unfollow users you have previously followed.
- **`unipile-pp-cli users edit-me`** - Modify informations on account owner profile. For WhatsApp accounts, only `headline` (profile display name), `summary` (profile About/status) and `picture` are available. For Instagram accounts, only `summary` (profile bio) and `picture` are available.
⚠️ Interactive documentation does not provide the expected format for nested parameters in code snippet. They should be formatted with brackets like the following examples : `location[id]=105015875`, `picture_settings[filter]=STUDIO` or `picture_settings[layout][bottomLeft][x]=1.25`.
When working with arrays, just set one field for each value with the same key, for example : `experience[skills]=development \ experience[skills]=management`.
- **`unipile-pp-cli users followers`** - Returns a list of all the followers of the current user. Ensure careful implementation of this action and consult provider limits and restrictions: https://developer.unipile.com/docs/provider-limits-and-restrictions
- **`unipile-pp-cli users following`** - Returns a list of all the followed accounts of an account. Ensure careful implementation of this action and consult provider limits and restrictions: https://developer.unipile.com/docs/provider-limits-and-restrictions
- **`unipile-pp-cli users get`** - Retrieve the profile of a user. Ensure careful implementation of this action and consult provider limits and restrictions: https://developer.unipile.com/docs/provider-limits-and-restrictions
- **`unipile-pp-cli users invite`** - Send an invitation to add someone to your contacts. Ensure careful implementation of this action and consult provider limits and restrictions: https://developer.unipile.com/docs/provider-limits-and-restrictions
- **`unipile-pp-cli users list-received`** - Returns a list of all invitations that have been received.
- **`unipile-pp-cli users list-sent`** - Returns a list of all invitations sent that are pending.
- **`unipile-pp-cli users me`** - Retrieve informations about account owner.
- **`unipile-pp-cli users relations`** - Returns a list of all the relations of an account. Ensure careful implementation of this action and consult provider limits and restrictions: https://developer.unipile.com/docs/provider-limits-and-restrictions
- **`unipile-pp-cli users respond-received`** - Accept or decline a connection invitation.

### webhooks

Webhooks management

- **`unipile-pp-cli webhooks create`** - Create a webhook.
- **`unipile-pp-cli webhooks delete`** - Delete a webhook.
- **`unipile-pp-cli webhooks list`** - Returns a list of the webhooks.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`unipile-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`unipile-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`unipile-pp-cli learnings list`** - Inspect taught rows
- **`unipile-pp-cli learnings forget <query>`** - Undo a teach
- **`unipile-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`unipile-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`unipile-pp-cli teach-pattern`** - Install a query/resource template up front
- **`unipile-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `UNIPILE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `unipile-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
unipile-pp-cli accounts list

# JSON for scripting and agents
unipile-pp-cli accounts list --json
# Filter to specific fields
unipile-pp-cli accounts list --json --select connection_params,created_at,current_signature

# Dry run — show the request without sending
unipile-pp-cli accounts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
unipile-pp-cli accounts list --agent
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

## Runtime Endpoint

This CLI targets a single Unipile tenant DSN. Set `UNIPILE_BASE_URL` to the base URL from your Unipile dashboard (including the port) to point the same binary at a different tenant. Path parameters such as `post_id` are ordinary command arguments, not environment variables.

Base URL: `https://api49.unipile.com:17995`

## Health Check

```bash
unipile-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `unipile-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/unipile-pp-cli/config.toml`; `--home`, `UNIPILE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `UNIPILE_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `unipile-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `unipile-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $UNIPILE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Every request returns 404 Cannot GET /api/v1/...** — UNIPILE_BASE_URL is pointing at the wrong DSN; copy the exact base URL from the Unipile dashboard, including the port.
- **Requests time out in a network that blocks custom ports** — Use the standard-443 form and pass the port as a query parameter, as Unipile's API Usage guide documents.
- **400 errors/invalid_parameters on a list command** — Most provider-scoped routes require an account id; pass --account with an account id or alias from 'accounts list'.
- **LinkedIn returns 422 cannot_resend_yet on an invitation** — You are at LinkedIn's invitation cap; run 'budget' to see remaining daily and weekly headroom before retrying.
- **Invitations or profile fetches start failing with 429 or 500** — Space calls out with random intervals rather than chaining them, and keep total daily volume under the caps 'budget' reports.
- **A list command returns fewer rows than expected** — Results are cursor-paginated with a 250 ceiling; pass --all to follow the cursor until it is null.
- **search, inbox, or budget return nothing right after a successful sync** — The local mirror is scoped per credential and account: set the same UNIPILE_API_KEY and UNIPILE_ACCOUNT_ID for querying as you used for sync, or pass --db explicitly.
- **You want LinkedIn's live search rather than the local mirror** — search defaults to the local mirror so it never spends LinkedIn's daily search-result budget; use 'unipile-pp-cli linkedin search' or 'search --data-source live' to opt in.
- **chats messages, chat-attendee chats, or per-user posts are missing after sync** — Those are per-parent resources and are skipped on a default sync because they cost one request per parent row. Name them explicitly, e.g. 'sync --resources chats_messages'.
- **sync reports 'using the only connected account for scope'** — No account scope was configured so the single connected account was adopted. Set UNIPILE_ACCOUNT_ID to pin it; with several accounts connected the CLI refuses and lists them instead.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**unipile-node-sdk**](https://github.com/unipile/unipile-node-sdk) — TypeScript (44 stars)
- [**mcp-unipile**](https://github.com/honeybluesky/mcp-unipile) — Python (16 stars)
- [**mcp-server-unipile**](https://github.com/Sundeepg98/mcp-server-unipile) — Python (4 stars)
- [**unipile-node**](https://github.com/unipile/unipile-node) — TypeScript (3 stars)
- [**unipile-python**](https://github.com/unipile/unipile-python) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
