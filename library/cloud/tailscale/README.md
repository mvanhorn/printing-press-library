# Tailscale CLI

**Tailscale admin from the terminal, with safe route and policy edits an agent can plan before it writes.**

Nearly every Tailscale admin API endpoint (tailnet deletion is deliberately left out), plus commands that make the two dangerous whole-object writes safe: routes approve keeps the routes a device already has, and policy add-entry keeps comments, backs up, validates, diffs, and writes with an ETag check. Those safe-write commands (routes approve and unapprove, shares revoke, policy add-entry and restore) write only with --yes and are plan-only as MCP tools; in --agent mode every other mutation is a dry run unless --yes. devices expiry, routes overview, and shares audit answer fleet questions the admin console shows one machine at a time, and access check and devices inspect give an agent one target's full picture before it acts.

Learn more at [Tailscale](https://tailscale.com).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `tailscale-pp-cli` binary and the `pp-tailscale` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install tailscale
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install tailscale --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install tailscale --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install tailscale --agent claude-code
npx -y @mvanhorn/printing-press-library install tailscale --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/tailscale/cmd/tailscale-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tailscale-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine tailscale-pp-cli`. On Unix, mark it executable: `chmod +x tailscale-pp-cli`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install tailscale --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tailscale --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tailscale --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install tailscale --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tailscale-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TAILSCALE_API_KEY` when Claude Desktop prompts you. Leave the tailnet as `-` unless you manage a different tailnet. The bundle has no OAuth fields; use an API key there.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/tailscale/cmd/tailscale-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "tailscale": {
      "command": "tailscale-pp-mcp",
      "env": {
        "TAILSCALE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Create an API access token on the admin console Keys page (https://login.tailscale.com/admin/settings/keys) and export TAILSCALE_API_KEY, or set TAILSCALE_OAUTH_CLIENT_ID and TAILSCALE_OAUTH_CLIENT_SECRET to use a scoped OAuth client; the CLI exchanges the client for a short-lived token. The tailnet defaults to '-', meaning the token's own tailnet. Set TAILSCALE_TAILNET only to target a different one.

## Quick Start

```bash
# Check config, auth, and API reachability (read-only)
tailscale-pp-cli doctor

# See which device keys expire in the next two weeks
tailscale-pp-cli devices expiry --within 14

# Find routes and exit nodes waiting for approval
tailscale-pp-cli routes overview --pending

# Plan an exit-node approval that keeps the device's other routes
tailscale-pp-cli routes approve home-mac --exit-node --dry-run

# List share invites anyone could still redeem
tailscale-pp-cli shares audit --pending

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Safe writes to whole-object endpoints
- **`routes approve`** — Approve a subnet route or exit node on one device without dropping the routes it already has.

  _Reach for this instead of the raw routes endpoint whenever an agent is asked to enable an exit node or subnet route._

  ```bash
  tailscale-pp-cli routes approve self --all-advertised
  ```
- **`routes unapprove`** — Remove approval for one route or the exit-node pair while leaving every other approved route in place.

  _Use it to turn off an exit node without breaking the subnet routes on the same machine._

  ```bash
  tailscale-pp-cli routes unapprove self --exit-node
  ```
- **`policy add-entry`** — Append one nodeAttrs, grants, acls, or ssh entry to the policy file with comments preserved, a local backup, validation, a diff, and a concurrency check.

  _Use it whenever an agent needs to add Taildrive, Funnel, or access grants without rewriting the whole policy file._

  ```bash
  tailscale-pp-cli policy add-entry nodeAttrs --entry '{"target":["autogroup:member"],"attr":["drive:access"]}' --dry-run
  ```
- **`policy restore`** — Roll the policy file back to a local backup after validating it and showing the diff.

  _This is the undo for every policy write the CLI makes._

  ```bash
  tailscale-pp-cli policy restore --list
  ```

### Tailnet state an agent can reason about
- **`routes overview`** — See approved, pending, and stale routes and exit-node state for every device in one table.

  _Run it before approving anything to see what is actually waiting._

  ```bash
  tailscale-pp-cli routes overview --pending
  ```
- **`devices expiry`** — List every device by days until its key expires and flag the ones inside a window.

  _Use it as a weekly check or a CI gate so nobody gets logged out by surprise._

  ```bash
  tailscale-pp-cli devices expiry --within 14
  ```
- **`shares audit`** — List every machine-share invite across the tailnet with who accepted it and which invites are still redeemable.

  _Answer who has access to a shared machine in one call._

  ```bash
  tailscale-pp-cli shares audit --pending
  ```
- **`shares revoke`** — Revoke machine-share invites by ID or by device and acceptor, with a plan before any delete.

  _Use it to revoke a contractor's share invites without hunting IDs device by device, then confirm in the admin console that an accepted share is gone._

  ```bash
  tailscale-pp-cli shares revoke --device self --pending
  ```

### Agent-native networking
- **`access check`** — Show which policy rules let a user reach a device and port, against the live policy or a candidate file.

  _Check access before and after a policy change without guessing at rule evaluation._

  ```bash
  tailscale-pp-cli access check --to self:22
  ```
- **`devices inspect`** — Get one device's identity, routes, key expiry, shares, and recent changes in a single record.

  _Make this the first call in any task about a specific machine._

  ```bash
  tailscale-pp-cli devices inspect self --agent
  ```

## Recipes

### Turn a Mac into an exit node safely

```bash
tailscale-pp-cli routes approve home-mac --exit-node --dry-run
```

Prints the before and after enabled routes; rerun with --yes to apply.

### Give members Taildrive access

```bash
tailscale-pp-cli policy add-entry nodeAttrs --entry '{"target":["autogroup:member"],"attr":["drive:access"]}' --dry-run
```

Shows the diff and validation result; rerun with --yes to write with the ETag check.

### Expiry report for agents

```bash
tailscale-pp-cli devices expiry --within 30 --agent --select items.machine,items.days_left,items.flagged
```

Narrows the report to the three fields an agent needs.

### Who can reach the NAS over SMB

```bash
tailscale-pp-cli access check --to nas:445
```

Lists the policy rules that grant access to that device and port.

### Roll back the last policy change

```bash
tailscale-pp-cli policy restore latest
```

Shows the diff and validation for restoring the newest local backup; add --yes to restore it, or use --list to pick an older one.

## Usage

Run `tailscale-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml` and the `data.db` store filled by `sync` |
| `state` | Runtime state such as `teach.log` and `policy-backups/` (HuJSON snapshots taken before every policy write, filed per credential and tailnet selector; they live only on the machine that made them) |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TAILSCALE_CONFIG_DIR`, `TAILSCALE_DATA_DIR`, `TAILSCALE_STATE_DIR`, or `TAILSCALE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TAILSCALE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TAILSCALE_HOME=/srv/tailscale
tailscale-pp-cli doctor
```

Under `TAILSCALE_HOME=/srv/tailscale`, the four dirs resolve to `/srv/tailscale/config`, `/srv/tailscale/data`, `/srv/tailscale/state`, and `/srv/tailscale/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "tailscale": {
      "command": "tailscale-pp-mcp",
      "env": {
        "TAILSCALE_HOME": "/srv/tailscale"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TAILSCALE_DATA_DIR` overrides an explicit `--home` for that kind. Use `TAILSCALE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TAILSCALE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `tailscale-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

Every Tailscale admin API endpoint except tailnet deletion, grouped by resource. Writes (POST/PUT/PATCH/DELETE) run as dry runs under `--agent` unless `--yes` is passed. Add `--help` to any command for its flags.

**device**

- `tailscale-pp-cli device attributes delete <deviceId> <attributeKey>` (DELETE): Delete a posture attribute from the specified device.
- `tailscale-pp-cli device attributes list <deviceId>` (GET): Retrieve all posture attributes for the specified device.
- `tailscale-pp-cli device attributes set <deviceId> <attributeKey>` (POST): Create or update a custom posture attribute on the specified device.
- `tailscale-pp-cli device authorized set <deviceId>` (POST): This call marks a device as authorized or revokes its authorization for tailnets where device authorization is required.
- `tailscale-pp-cli device delete <deviceId>` (DELETE): Deletes the device from its tailnet.
- `tailscale-pp-cli device device-invites create <deviceId>` (POST): Create new share invites for a device.
- `tailscale-pp-cli device device-invites list <deviceId>` (GET): List all share invites for a device. OAuth Scope: `device_invites:read`.
- `tailscale-pp-cli device expire key <deviceId>` (POST): Mark a device's node key as expired.
- `tailscale-pp-cli device get <deviceId>` (GET): Retrieve the details for the specified device. OAuth Scope: `devices:core:read`.
- `tailscale-pp-cli device ip set <deviceId>` (POST): When a device is added to a tailnet, its Tailscale IPv4 address is set at random either from the CGNAT range.
- `tailscale-pp-cli device key set <deviceId>` (POST): When a device is added to a tailnet, its key expiry is set according to the tailnet's key expiry setting.
- `tailscale-pp-cli device name set <deviceId>` (POST): When a device is added to a tailnet, its Tailscale device name.
- `tailscale-pp-cli device routes list <deviceId>` (GET): Retrieve the list of subnet routes that a device is advertising, as well as those that are enabled for it.
- `tailscale-pp-cli device routes set <deviceId>` (POST): Set a device's enabled subnet routes by replacing the existing list of subnet routes with the supplied parameters.
- `tailscale-pp-cli device tags set <deviceId>` (POST): Tags let you assign an identity to a device that is separate from human users.

**device-invites**

- `tailscale-pp-cli device-invites accept` (POST): Accepts the invitation to share a device into the requesting user's tailnet.
- `tailscale-pp-cli device-invites delete <deviceInviteId>` (DELETE): Delete a specific device invite. OAuth Scope: `device_invites`.
- `tailscale-pp-cli device-invites get <deviceInviteId>` (GET): Retrieve a specific device invite. OAuth Scope: `device_invites:read`.
- `tailscale-pp-cli device-invites resend device-invite <deviceInviteId>` (POST): Resend a device invite by email.

**organizations**

- `tailscale-pp-cli organizations tailnets create <organization>` (POST): Create an API-only tailnet in the organization.
- `tailscale-pp-cli organizations tailnets list <organization>` (GET): List all tailnets in the organization, including the original tailnet and any API-only tailnets.

**posture**

- `tailscale-pp-cli posture delete <id>` (DELETE): Delete a specific posture integration. OAuth Scope: `feature_settings`.
- `tailscale-pp-cli posture get <id>` (GET): Gets the posture integration identified by `{id}`. OAuth Scope: `feature_settings:read`.
- `tailscale-pp-cli posture update <id>` (PATCH): Updates the posture integration identified by `{id}`.

**tailnet**

- `tailscale-pp-cli tailnet acl get` (GET): Retrieves the current policy file for the given tailnet.
- `tailscale-pp-cli tailnet acl preview` (POST): When given a user or IP port to match against, returns the tailnet policy rules that apply to that resource.
- `tailscale-pp-cli tailnet acl set` (POST): Sets the ACL for the given tailnet.
- `tailscale-pp-cli tailnet acl validate` (POST): This endpoint works in one of two modes, neither of which modifies your current tailnet policy file: - Run ACL tests.
- `tailscale-pp-cli tailnet aws-external-id create-or-get` (POST): Get an AWS external id to use for streaming tailnet logs to S3 using role-based authentication.
- `tailscale-pp-cli tailnet aws-external-id validate <id>` (POST): Validate that Tailscale can assume your IAM role with (and only with) this external ID. OAuth Scope: `log_streaming`.
- `tailscale-pp-cli tailnet contacts get` (GET): Retrieve the tailnet's current contacts. OAuth Scope: `account_settings:read`.
- `tailscale-pp-cli tailnet contacts resend-verification-email` (POST): Resends the verification email for this contact, if and only if verification is still pending.
- `tailscale-pp-cli tailnet contacts update` (PATCH): Update the preferences for this type of contact.
- `tailscale-pp-cli tailnet device-attributes batch-update` (PATCH): Batch updates posture attributes across devices in a tailnet.
- `tailscale-pp-cli tailnet devices list` (GET): Lists the devices in a tailnet. OAuth Scope: `devices:core:read`.
- `tailscale-pp-cli tailnet dns get-configuration` (GET): Retrieves the full DNS configuration for a tailnet, including global nameservers, split DNS routes, search paths.
- `tailscale-pp-cli tailnet dns get-preferences` (GET): Retrieves the DNS preferences that are currently set for the given tailnet.
- `tailscale-pp-cli tailnet dns get-split` (GET): Retrieves the split DNS settings, which is a map from domains to lists of nameservers.
- `tailscale-pp-cli tailnet dns list-nameservers` (GET): Lists the global DNS nameservers for a tailnet.
- `tailscale-pp-cli tailnet dns list-search-paths` (GET): Retrieves the list of search paths, also referred to as *search domains*, that is currently set for the given tailnet.
- `tailscale-pp-cli tailnet dns set-configuration` (POST): Replaces the DNS configuration for the given tailnet.
- `tailscale-pp-cli tailnet dns set-nameservers` (POST): Replaces the list of global DNS nameservers for the given tailnet with the list supplied in the request.
- `tailscale-pp-cli tailnet dns set-preferences` (POST): Set the DNS preferences for a tailnet; specifically, the MagicDNS setting.
- `tailscale-pp-cli tailnet dns set-search-paths` (POST): Replaces the list of search paths for the given tailnet.
- `tailscale-pp-cli tailnet dns set-split` (PUT): Replaces the split DNS settings for a given tailnet.
- `tailscale-pp-cli tailnet dns update-split` (PATCH): Performs partial updates of the split DNS settings for a given tailnet.
- `tailscale-pp-cli tailnet keys create` (POST): Creates a new auth key.
- `tailscale-pp-cli tailnet keys delete <keyId>` (DELETE): Deletes a specific api access token or auth key.
- `tailscale-pp-cli tailnet keys get <keyId>` (GET): Returns a JSON object with information about a specific api access token, OAuth client, federated identity, or auth key.
- `tailscale-pp-cli tailnet keys list` (GET): Returns a list of active auth keys, API access tokens and trust credentials.
- `tailscale-pp-cli tailnet keys set <keyId>` (PUT): Set the configuration for an existing OAuth client or federated identity.
- `tailscale-pp-cli tailnet logging disable-log-streaming` (DELETE): Delete the log streaming configuration for the provided log type. OAuth Scope: `log_streaming`.
- `tailscale-pp-cli tailnet logging get-log-streaming-configuration` (GET): Retrieve the log streaming configuration for the provided log type. OAuth Scope: `log_streaming:read`.
- `tailscale-pp-cli tailnet logging get-log-streaming-status` (GET): Retrieve the log streaming status for the provided log type. OAuth Scope: `log_streaming:read`.
- `tailscale-pp-cli tailnet logging list-configuration-audit-logs` (GET): List all configuration audit logs for a tailnet. OAuth Scope: `logs:configuration:read`.
- `tailscale-pp-cli tailnet logging list-network-flow-logs` (GET): List all network flow logs for a tailnet. OAuth Scope: `logs:network:read`.
- `tailscale-pp-cli tailnet logging set-log-streaming-configuration` (PUT): Set the log streaming configuration for the provided log type. OAuth Scope: `log_streaming`.
- `tailscale-pp-cli tailnet oauth-apps create` (POST): Create an OAuth app within a tailnet.
- `tailscale-pp-cli tailnet oauth-apps delete <appId>` (DELETE): Delete a specific OAuth app. OAuth Scope: `oauth_apps`.
- `tailscale-pp-cli tailnet oauth-apps get <appId>` (GET): Retrieve a specific OAuth app. OAuth Scope: `oauth_apps:read`.
- `tailscale-pp-cli tailnet oauth-apps list` (GET): List all OAuth apps for a tailnet. OAuth Scope: `oauth_apps:read`.
- `tailscale-pp-cli tailnet oauth-apps update <appId>` (PUT): Update a specific OAuth app.
- `tailscale-pp-cli tailnet posture create` (POST): Create a posture integration, returning the resulting PostureIntegration.
- `tailscale-pp-cli tailnet posture list` (GET): List all of the posture integrations for a tailnet. OAuth Scope: `feature_settings:read`.
- `tailscale-pp-cli tailnet services delete <serviceName>` (DELETE): Delete the specified Service from the tailnet. OAuth Scope: `services`.
- `tailscale-pp-cli tailnet services get <serviceName>` (GET): Retrieve the details for the specified Service. OAuth Scope: `services:read`.
- `tailscale-pp-cli tailnet services get-device-approval <serviceName> <deviceId>` (GET): Retrieve the approval status of the specified Service on a specific device. OAuth Scope: `services`.
- `tailscale-pp-cli tailnet services list` (GET): List all Services configured for the tailnet.
- `tailscale-pp-cli tailnet services list-hosts <serviceName>` (GET): List all devices that are hosting the specified Service. OAuth Scope: `services`.
- `tailscale-pp-cli tailnet services update <serviceName>` (PUT): Update or create the specified Service.
- `tailscale-pp-cli tailnet services update-device-approval <serviceName> <deviceId>` (POST): Update the approval status of the specified Service on a specific device. OAuth Scope: `services`.
- `tailscale-pp-cli tailnet settings get` (GET): Retrieve the settings for a specific tailnet.
- `tailscale-pp-cli tailnet settings update` (PATCH): Update the settings for a specific tailnet.
- `tailscale-pp-cli tailnet user-invites create` (POST): Create, and optionally email out, new user invites to join the tailnet.
- `tailscale-pp-cli tailnet user-invites list` (GET): List all open (not yet accepted) user invites to the tailnet.
- `tailscale-pp-cli tailnet users list` (GET): List all users of a tailnet. OAuth Scope: `users:read`.
- `tailscale-pp-cli tailnet webhooks create` (POST): Create a webhook within a tailnet. OAuth Scope: `webhooks`.
- `tailscale-pp-cli tailnet webhooks list` (GET): List all webhooks for a tailnet. OAuth Scope: `webhooks:read`.

**user-invites**

- `tailscale-pp-cli user-invites delete <userInviteId>` (DELETE): Deletes a specific user invite.
- `tailscale-pp-cli user-invites get <userInviteId>` (GET): Retrieve a specific user invite.
- `tailscale-pp-cli user-invites resend user-invite <userInviteId>` (POST): Resend a user invite by email.

**users**

- `tailscale-pp-cli users <userId>` (GET): Retrieve details about the specified user. OAuth Scope: `users:read`.
- `tailscale-pp-cli users approve user <userId>` (POST): Approve a pending user's access to the tailnet.
- `tailscale-pp-cli users delete user <userId>` (POST): Delete a user from their tailnet.
- `tailscale-pp-cli users restore user <userId>` (POST): Restores a suspended user's access to their tailnet.
- `tailscale-pp-cli users role set <userId>` (POST): Update the role for the specified user. OAuth Scope: `users`.
- `tailscale-pp-cli users suspend user <userId>` (POST): Suspends a user from their tailnet.

**webhooks**

- `tailscale-pp-cli webhooks delete <endpointId>` (DELETE): Delete a specific webhook. OAuth Scope: `webhooks`.
- `tailscale-pp-cli webhooks get <endpointId>` (GET): Retrieve a specific webhook. OAuth Scope: `webhooks:read`.
- `tailscale-pp-cli webhooks rotate webhook-secret <endpointId>` (POST): Rotate and generate a new secret for a specific webhook.
- `tailscale-pp-cli webhooks test webhook <endpointId>` (POST): Test a specific webhook by sending out a test event to the endpoint URL.
- `tailscale-pp-cli webhooks update <endpointId>` (PATCH): Update a specific webhook. OAuth Scope: `webhooks`.

### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`tailscale-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`tailscale-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`tailscale-pp-cli learnings list`** - Inspect taught rows
- **`tailscale-pp-cli learnings forget <query>`** - Undo a teach
- **`tailscale-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`tailscale-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`tailscale-pp-cli teach-pattern`** - Install a query/resource template up front
- **`tailscale-pp-cli teach-lookup`** - Add an entity mapping (e.g. a device or resource alias) for pattern substitution

Pass `--no-learn` or set `TAILSCALE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store is kept per credential: each API key or OAuth client gets its own database file under the data directory, so two tailnets never share synced data, search results, or learnings.

The local store's schema version stamp is one-way: once this version of `tailscale-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tailscale-pp-cli device get n1234567890CNTRL

# JSON for scripting and agents
tailscale-pp-cli device get n1234567890CNTRL --json
# Filter to specific fields
tailscale-pp-cli device get n1234567890CNTRL --json --select addresses,advertisedRoutes,authorized

# Dry run: show the request without sending (the safe-write commands still read live data and print a plan)
tailscale-pp-cli device get n1234567890CNTRL --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tailscale-pp-cli device get n1234567890CNTRL --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending; routes approve/unapprove, shares revoke, and policy add-entry/restore read live state and print a plan instead, and only write with `--yes`
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear. Under `--agent`, mutating commands run as dry runs unless `--yes` is passed, the five safe-write commands are plan-only as MCP tools, and the MCP `tailscale_execute` tool returns the planned request for any write endpoint unless it is called with `confirm: true`
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline search** - `search` reads only the local SQLite store filled by `sync` (Tailscale has no search endpoint); run `sync` first
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `1` general error (also `devices expiry --fail-on-flagged` when something is inside the window), `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `TAILSCALE_TAILNET` resolves `{tailnet}`

Base URL: `https://api.tailscale.com/api/v2`

## Health Check

```bash
tailscale-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `tailscale-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/tailscale-pp-cli/config.toml`; `--home`, `TAILSCALE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TAILSCALE_API_KEY` | per_call | Unless OAuth is set | API access token (tskey-api-...) from the admin console Keys page. |
| `TAILSCALE_OAUTH_CLIENT_ID` | auth_flow_input | With the secret, instead of an API key | OAuth client ID; exchanged in memory for a short-lived token on each run. |
| `TAILSCALE_OAUTH_CLIENT_SECRET` | auth_flow_input | With the client ID | OAuth client secret (tskey-client-...). Never written to disk by this CLI. |
| `TAILSCALE_TAILNET` | endpoint | No (default `-`) | Tailnet ID; `-` means the credential's own tailnet. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `tailscale-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `tailscale-pp-cli doctor` to check credentials
- Check which credential is in use without printing it: `tailscale-pp-cli auth status`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- List what exists first, e.g. `tailscale-pp-cli tailnet devices list` or `tailscale-pp-cli tailnet users list`

### API-specific
- **HTTP 403 on routes approve or policy add-entry** — The token or OAuth client lacks write scope; use an admin token or add devices:routes / policy_file scopes to the OAuth client
- **policy write fails with 412 precondition failed** — Someone changed the policy since it was read; rerun tailscale-pp-cli policy add-entry to diff against the new version
- **routes approve refuses a CIDR as not advertised** — Advertise it on the device first with tailscale set --advertise-routes, then rerun
- **devices inspect self cannot find this machine** — Install the tailscale command or pass the device hostname or nodeId instead of self
- **doctor says auth is not configured but commands work** — Expected with only TAILSCALE_OAUTH_CLIENT_ID/SECRET set; the token is minted per command. Run tailscale-pp-cli routes overview to confirm
- **sync prints sync_warning or sync_error for acl or user-invites** — Non-critical: the policy file is a single document and user-invites returns null when empty; sync still exits 0 and stores devices and users

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**tscli**](https://github.com/jaxxstorm/tscli) — Go (82 stars)
- [**tailscale-mcp (YawLabs)**](https://github.com/YawLabs/tailscale-mcp) — TypeScript (30 stars)
- [**tailscale-skill**](https://github.com/tailscale/tailscale-skill) — Markdown (30 stars)
- [**scurgery**](https://github.com/nopoz/scurgery) — Go
- [**tailscale-superpowers**](https://github.com/cathrynlavery/tailscale-superpowers) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
