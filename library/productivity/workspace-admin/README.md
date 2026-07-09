# Google Workspace Admin CLI

**GAT-style Google Workspace auditing as an agent-native CLI: sync Directory, Drive, Gmail, Reports, and Alert Center into a local store, then run offline cross-API audits, offboarding, and OAuth-app risk checks that GAM and the official gws CLI cannot persist.**

A curated Google Workspace admin and audit tool built on the Admin SDK (Directory, Reports, Alert Center), Drive, and Gmail. Unlike stateless tools, it syncs Workspace metadata into a local SQLite store so audits like audit external-shares, audit app-risk, and audit user360 run offline and join data across APIs. workflow offboard executes a departing user's full lifecycle in one command, and every command is agent-native with --json, --select, --dry-run, and typed exit codes.

## Install

The recommended path installs both the `workspace-admin-pp-cli` binary and the `pp-workspace-admin` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install workspace-admin
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install workspace-admin --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install workspace-admin --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install workspace-admin --agent claude-code
npx -y @mvanhorn/printing-press-library install workspace-admin --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/workspace-admin/cmd/workspace-admin-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/workspace-admin-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install workspace-admin --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-workspace-admin --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-workspace-admin --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install workspace-admin --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/workspace-admin-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GOOGLE_WORKSPACE_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/workspace-admin/cmd/workspace-admin-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "workspace-admin": {
      "command": "workspace-admin-pp-mcp",
      "env": {
        "WORKSPACE_ADMIN_USERID": "<userId>",
        "GOOGLE_WORKSPACE_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Google Workspace admin APIs use OAuth2. This CLI accepts any Google access token as a bearer token via GOOGLE_WORKSPACE_TOKEN (for example from `gcloud auth print-access-token`). The first-class admin path is a service account with domain-wide delegation: `workspace-admin-pp-cli auth service-account --key sa.json --impersonate admin@example.com` mints and caches an access token by impersonating a super admin (required for Directory, Reports, and Alert Center). For Drive and Gmail data belonging to a specific user, impersonate that user with `--impersonate user@example.com`. Authorize the service account's scopes once in Admin Console > Security > API Controls > Domain-wide Delegation; run `doctor` to verify each scope.

## Quick Start

```bash
# Verify auth, impersonation, and per-scope access before anything else — the #1 setup pain point for Workspace tooling.
workspace-admin-pp-cli doctor --dry-run

# One-call posture for a single user once the store is synced (run sync first).
workspace-admin-pp-cli audit user360 user@example.com --agent

# The weekly external-exposure sweep over synced Drive metadata.
workspace-admin-pp-cli audit external-shares --agent

# Preview every step of a full offboard before running it for real.
workspace-admin-pp-cli workflow offboard departing@example.com --manager manager@example.com --dry-run

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Lifecycle workflows that compound
- **`workflow offboard`** — Run a departing user's entire lifecycle in one command — suspend, sign out, revoke OAuth tokens, transfer Drive ownership, set Gmail delegation, remove from groups, and move to a suspended OU.

  _Reach for this instead of running eight separate endpoint commands when a user leaves; it guarantees no step is skipped and records what completed._

  ```bash
  workspace-admin-pp-cli workflow offboard departing@example.com --manager manager@example.com --dry-run
  ```

### Cross-API audits only the local store enables
- **`audit user360`** — One per-user posture report joining the user's Drive footprint and quota, security (2SV, suspended), connected OAuth apps, devices, Gmail settings, group memberships, and recent activity.

  _Pick this to answer 'what is going on with this account' in one call instead of stitching five APIs together._

  ```bash
  workspace-admin-pp-cli audit user360 user@example.com --agent --select security,apps,devices
  ```
- **`audit external-shares`** — Find every Drive file shared 'anyone with link' or with an external domain, joined to its owner and the owner's org unit.

  _Use this for the domain-wide external Drive exposure sweep; for one user's Drive footprint use audit user360._

  ```bash
  workspace-admin-pp-cli audit external-shares --domain gmail.com --agent --select files.name,files.owner,files.permissions.emailAddress
  ```
- **`audit reconstruct`** — Merge one user's login, admin, Drive, and token activity into a single ordered timeline for post-compromise review.

  _Pick this after a suspected compromise to see everything an account did in order; for domain-wide anomalies use audit logins._

  ```bash
  workspace-admin-pp-cli audit reconstruct user@example.com --since 7d --agent
  ```
- **`audit email-exposure`** — Sweep every user's Gmail forwarding, sendAs, delegates, and filters for external forwarding, external send-as, delegates, and forward/delete rules — the standard business-email-compromise indicators.

  _Use this for the domain-wide BEC sweep of forwarding rules, delegates, and suspicious filters; for one user's full email settings use audit user360._

  ```bash
  workspace-admin-pp-cli audit email-exposure --agent --select findings.user,findings.type,findings.detail
  ```

### Security posture
- **`audit app-risk`** — Roll up every third-party OAuth app authorized in the domain by a curated scope-to-risk tier (Low/Moderate/High), flag apps with Full Drive access, and count users-per-app with the member list.

  _Reach for this during third-party app review to rank risk and find which users authorized a dangerous app._

  ```bash
  workspace-admin-pp-cli audit app-risk --min-risk high --agent
  ```
- **`audit logins`** — Surface failed-login bursts, new-country logins, and dormant accounts (no login since N days) from synced login activity joined to users.

  _Use this for the weekly login-anomaly and dormant-account review; for a single user's forensic timeline use audit reconstruct._

  ```bash
  workspace-admin-pp-cli audit logins --failures --geo --since 7d --agent
  ```

### Local state that compounds
- **`groups expand`** — Recursively flatten nested groups into their effective direct-user membership, cycle-safe.

  _Use this to answer 'who is effectively in this group' before an access decision; to list one user's group memberships use audit user360._

  ```bash
  workspace-admin-pp-cli groups expand all-staff@example.com --agent --select members.email,members.viaGroup
  ```

## Recipes

### Preview a full offboard

```bash
workspace-admin-pp-cli workflow offboard departing@example.com --manager manager@example.com --dry-run
```

Shows every step (suspend, token revoke, Drive transfer, delegation, device wipe, group removal, OU move) without changing anything; drop --dry-run to execute.

### External exposure for one domain, narrowed output

```bash
workspace-admin-pp-cli audit external-shares --domain gmail.com --agent --select files.name,files.owner,files.permissions.emailAddress
```

Uses dotted --select paths to return only the fields that matter from a deeply nested Drive permissions response, keeping agent context small.

### High-risk OAuth apps

```bash
workspace-admin-pp-cli audit app-risk --min-risk high --agent
```

Lists third-party apps tiered High risk (including Full Drive access) with their authorizing users, for third-party app review.

### Reconstruct a compromised account

```bash
workspace-admin-pp-cli audit reconstruct user@example.com --since 7d --agent
```

Produces one ordered timeline of the user's login, admin, Drive, and token activity over the last 7 days.

### Search the local audit store

```bash
workspace-admin-pp-cli search "external" --type drive_files --json
```

Full-text search over synced Drive metadata after a sync, returning structured JSON.

## Usage

Run `workspace-admin-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `WORKSPACE_ADMIN_CONFIG_DIR`, `WORKSPACE_ADMIN_DATA_DIR`, `WORKSPACE_ADMIN_STATE_DIR`, or `WORKSPACE_ADMIN_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `WORKSPACE_ADMIN_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export WORKSPACE_ADMIN_HOME=/srv/workspace-admin
workspace-admin-pp-cli doctor
```

Under `WORKSPACE_ADMIN_HOME=/srv/workspace-admin`, the four dirs resolve to `/srv/workspace-admin/config`, `/srv/workspace-admin/data`, `/srv/workspace-admin/state`, and `/srv/workspace-admin/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "workspace-admin": {
      "command": "workspace-admin-pp-mcp",
      "env": {
        "WORKSPACE_ADMIN_HOME": "/srv/workspace-admin"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `WORKSPACE_ADMIN_DATA_DIR` overrides an explicit `--home` for that kind. Use `WORKSPACE_ADMIN_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `WORKSPACE_ADMIN_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `workspace-admin-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### admin

Manage admin

- **`workspace-admin-pp-cli admin channels-stop`** - Stops watching resources through this channel.
- **`workspace-admin-pp-cli admin customer-devices-chromeos-commands-get`** - Gets command data a specific command issued to the device.
- **`workspace-admin-pp-cli admin customer-devices-chromeos-issue-command`** - Issues a command for the device to execute.
- **`workspace-admin-pp-cli admin customers-chrome-print-servers-batch-create-print-servers`** - Creates multiple print servers.
- **`workspace-admin-pp-cli admin customers-chrome-print-servers-batch-delete-print-servers`** - Deletes multiple print servers.
- **`workspace-admin-pp-cli admin customers-chrome-print-servers-create`** - Creates a print server.
- **`workspace-admin-pp-cli admin customers-chrome-print-servers-delete`** - Deletes a print server.
- **`workspace-admin-pp-cli admin customers-chrome-print-servers-get`** - Returns a print server's configuration.
- **`workspace-admin-pp-cli admin customers-chrome-print-servers-list`** - Lists print server configurations.
- **`workspace-admin-pp-cli admin customers-chrome-print-servers-patch`** - Updates a print server's configuration.
- **`workspace-admin-pp-cli admin customers-chrome-printers-batch-create-printers`** - Creates printers under given Organization Unit.
- **`workspace-admin-pp-cli admin customers-chrome-printers-batch-delete-printers`** - Deletes printers in batch.
- **`workspace-admin-pp-cli admin customers-chrome-printers-create`** - Creates a printer under given Organization Unit.
- **`workspace-admin-pp-cli admin customers-chrome-printers-list`** - List printers configs.
- **`workspace-admin-pp-cli admin customers-chrome-printers-list-printer-models`** - Lists the supported printer models.
- **`workspace-admin-pp-cli admin directory-asps-delete`** - Deletes an ASP issued by a user.
- **`workspace-admin-pp-cli admin directory-asps-get`** - Gets information about an ASP issued by a user.
- **`workspace-admin-pp-cli admin directory-asps-list`** - Lists the ASPs issued by a user.
- **`workspace-admin-pp-cli admin directory-chromeosdevices-action`** - Takes an action that affects a Chrome OS Device. This includes deprovisioning, disabling, and re-enabling devices. *Warning:* * Deprovisioning a device will stop device policy syncing and remove device-level printers. After a device is deprovisioned, it must be wiped before it can be re-enrolled. * Lost or stolen devices should use the disable action. * Re-enabling a disabled device will consume a device license. If you do not have sufficient licenses available when completing the re-enable action, you will receive an error. For more information about deprovisioning and disabling devices, visit the [help center](https://support.google.com/chrome/a/answer/3523633).
- **`workspace-admin-pp-cli admin directory-chromeosdevices-get`** - Retrieves a Chrome OS device's properties.
- **`workspace-admin-pp-cli admin directory-chromeosdevices-list`** - Retrieves a paginated list of Chrome OS devices within an account.
- **`workspace-admin-pp-cli admin directory-chromeosdevices-move-devices-to-ou`** - Moves or inserts multiple Chrome OS devices to an organizational unit. You can move up to 50 devices at once.
- **`workspace-admin-pp-cli admin directory-chromeosdevices-patch`** - Updates a device's updatable properties, such as `annotatedUser`, `annotatedLocation`, `notes`, `orgUnitPath`, or `annotatedAssetId`. This method supports [patch semantics](/admin-sdk/directory/v1/guides/performance#patch).
- **`workspace-admin-pp-cli admin directory-chromeosdevices-update`** - Updates a device's updatable properties, such as `annotatedUser`, `annotatedLocation`, `notes`, `orgUnitPath`, or `annotatedAssetId`.
- **`workspace-admin-pp-cli admin directory-customers-get`** - Retrieves a customer.
- **`workspace-admin-pp-cli admin directory-customers-patch`** - Patches a customer.
- **`workspace-admin-pp-cli admin directory-customers-update`** - Updates a customer.
- **`workspace-admin-pp-cli admin directory-domain-aliases-delete`** - Deletes a domain Alias of the customer.
- **`workspace-admin-pp-cli admin directory-domain-aliases-get`** - Retrieves a domain alias of the customer.
- **`workspace-admin-pp-cli admin directory-domain-aliases-insert`** - Inserts a domain alias of the customer.
- **`workspace-admin-pp-cli admin directory-domain-aliases-list`** - Lists the domain aliases of the customer.
- **`workspace-admin-pp-cli admin directory-domains-delete`** - Deletes a domain of the customer.
- **`workspace-admin-pp-cli admin directory-domains-get`** - Retrieves a domain of the customer.
- **`workspace-admin-pp-cli admin directory-domains-insert`** - Inserts a domain of the customer.
- **`workspace-admin-pp-cli admin directory-domains-list`** - Lists the domains of the customer.
- **`workspace-admin-pp-cli admin directory-groups-aliases-delete`** - Removes an alias.
- **`workspace-admin-pp-cli admin directory-groups-aliases-insert`** - Adds an alias for the group.
- **`workspace-admin-pp-cli admin directory-groups-aliases-list`** - Lists all aliases for a group.
- **`workspace-admin-pp-cli admin directory-groups-delete`** - Deletes a group.
- **`workspace-admin-pp-cli admin directory-groups-get`** - Retrieves a group's properties.
- **`workspace-admin-pp-cli admin directory-groups-insert`** - Creates a group.
- **`workspace-admin-pp-cli admin directory-groups-list`** - Retrieves all groups of a domain or of a user given a userKey (paginated).
- **`workspace-admin-pp-cli admin directory-groups-patch`** - Updates a group's properties. This method supports [patch semantics](/admin-sdk/directory/v1/guides/performance#patch).
- **`workspace-admin-pp-cli admin directory-groups-update`** - Updates a group's properties.
- **`workspace-admin-pp-cli admin directory-members-delete`** - Removes a member from a group.
- **`workspace-admin-pp-cli admin directory-members-get`** - Retrieves a group member's properties.
- **`workspace-admin-pp-cli admin directory-members-has-member`** - Checks whether the given user is a member of the group. Membership can be direct or nested, but if nested, the `memberKey` and `groupKey` must be entities in the same domain or an `Invalid input` error is returned. To check for nested memberships that include entities outside of the group's domain, use the [`checkTransitiveMembership()`](https://cloud.google.com/identity/docs/reference/rest/v1/groups.memberships/checkTransitiveMembership) method in the Cloud Identity Groups API.
- **`workspace-admin-pp-cli admin directory-members-insert`** - Adds a user to the specified group.
- **`workspace-admin-pp-cli admin directory-members-list`** - Retrieves a paginated list of all members in a group. This method times out after 60 minutes. For more information, see [Troubleshoot error codes](https://developers.google.com/admin-sdk/directory/v1/guides/troubleshoot-error-codes).
- **`workspace-admin-pp-cli admin directory-members-patch`** - Updates the membership properties of a user in the specified group. This method supports [patch semantics](/admin-sdk/directory/v1/guides/performance#patch).
- **`workspace-admin-pp-cli admin directory-members-update`** - Updates the membership of a user in the specified group.
- **`workspace-admin-pp-cli admin directory-mobiledevices-action`** - Takes an action that affects a mobile device. For example, remotely wiping a device.
- **`workspace-admin-pp-cli admin directory-mobiledevices-delete`** - Removes a mobile device.
- **`workspace-admin-pp-cli admin directory-mobiledevices-get`** - Retrieves a mobile device's properties.
- **`workspace-admin-pp-cli admin directory-mobiledevices-list`** - Retrieves a paginated list of all user-owned mobile devices for an account. To retrieve a list that includes company-owned devices, use the Cloud Identity [Devices API](https://cloud.google.com/identity/docs/concepts/overview-devices) instead. This method times out after 60 minutes. For more information, see [Troubleshoot error codes](https://developers.google.com/admin-sdk/directory/v1/guides/troubleshoot-error-codes).
- **`workspace-admin-pp-cli admin directory-orgunits-delete`** - Removes an organizational unit.
- **`workspace-admin-pp-cli admin directory-orgunits-get`** - Retrieves an organizational unit.
- **`workspace-admin-pp-cli admin directory-orgunits-insert`** - Adds an organizational unit.
- **`workspace-admin-pp-cli admin directory-orgunits-list`** - Retrieves a list of all organizational units for an account.
- **`workspace-admin-pp-cli admin directory-orgunits-patch`** - Updates an organizational unit. This method supports [patch semantics](/admin-sdk/directory/v1/guides/performance#patch)
- **`workspace-admin-pp-cli admin directory-orgunits-update`** - Updates an organizational unit.
- **`workspace-admin-pp-cli admin directory-privileges-list`** - Retrieves a paginated list of all privileges for a customer.
- **`workspace-admin-pp-cli admin directory-resources-buildings-delete`** - Deletes a building.
- **`workspace-admin-pp-cli admin directory-resources-buildings-get`** - Retrieves a building.
- **`workspace-admin-pp-cli admin directory-resources-buildings-insert`** - Inserts a building.
- **`workspace-admin-pp-cli admin directory-resources-buildings-list`** - Retrieves a list of buildings for an account.
- **`workspace-admin-pp-cli admin directory-resources-buildings-patch`** - Patches a building.
- **`workspace-admin-pp-cli admin directory-resources-buildings-update`** - Updates a building.
- **`workspace-admin-pp-cli admin directory-resources-calendars-delete`** - Deletes a calendar resource.
- **`workspace-admin-pp-cli admin directory-resources-calendars-get`** - Retrieves a calendar resource.
- **`workspace-admin-pp-cli admin directory-resources-calendars-insert`** - Inserts a calendar resource.
- **`workspace-admin-pp-cli admin directory-resources-calendars-list`** - Retrieves a list of calendar resources for an account.
- **`workspace-admin-pp-cli admin directory-resources-calendars-patch`** - Patches a calendar resource.
- **`workspace-admin-pp-cli admin directory-resources-calendars-update`** - Updates a calendar resource. This method supports patch semantics, meaning you only need to include the fields you wish to update. Fields that are not present in the request will be preserved.
- **`workspace-admin-pp-cli admin directory-resources-features-delete`** - Deletes a feature.
- **`workspace-admin-pp-cli admin directory-resources-features-get`** - Retrieves a feature.
- **`workspace-admin-pp-cli admin directory-resources-features-insert`** - Inserts a feature.
- **`workspace-admin-pp-cli admin directory-resources-features-list`** - Retrieves a list of features for an account.
- **`workspace-admin-pp-cli admin directory-resources-features-patch`** - Patches a feature.
- **`workspace-admin-pp-cli admin directory-resources-features-rename`** - Renames a feature.
- **`workspace-admin-pp-cli admin directory-resources-features-update`** - Updates a feature.
- **`workspace-admin-pp-cli admin directory-role-assignments-delete`** - Deletes a role assignment.
- **`workspace-admin-pp-cli admin directory-role-assignments-get`** - Retrieves a role assignment.
- **`workspace-admin-pp-cli admin directory-role-assignments-insert`** - Creates a role assignment.
- **`workspace-admin-pp-cli admin directory-role-assignments-list`** - Retrieves a paginated list of all roleAssignments.
- **`workspace-admin-pp-cli admin directory-roles-delete`** - Deletes a role.
- **`workspace-admin-pp-cli admin directory-roles-get`** - Retrieves a role.
- **`workspace-admin-pp-cli admin directory-roles-insert`** - Creates a role.
- **`workspace-admin-pp-cli admin directory-roles-list`** - Retrieves a paginated list of all the roles in a domain.
- **`workspace-admin-pp-cli admin directory-roles-patch`** - Patches a role.
- **`workspace-admin-pp-cli admin directory-roles-update`** - Updates a role.
- **`workspace-admin-pp-cli admin directory-schemas-delete`** - Deletes a schema.
- **`workspace-admin-pp-cli admin directory-schemas-get`** - Retrieves a schema.
- **`workspace-admin-pp-cli admin directory-schemas-insert`** - Creates a schema.
- **`workspace-admin-pp-cli admin directory-schemas-list`** - Retrieves all schemas for a customer.
- **`workspace-admin-pp-cli admin directory-schemas-patch`** - Patches a schema.
- **`workspace-admin-pp-cli admin directory-schemas-update`** - Updates a schema.
- **`workspace-admin-pp-cli admin directory-tokens-delete`** - Deletes all access tokens issued by a user for an application.
- **`workspace-admin-pp-cli admin directory-tokens-get`** - Gets information about an access token issued by a user.
- **`workspace-admin-pp-cli admin directory-tokens-list`** - Returns the set of tokens specified user has issued to 3rd party applications.
- **`workspace-admin-pp-cli admin directory-two-step-verification-turn-off`** - Turns off 2-Step Verification for user.
- **`workspace-admin-pp-cli admin directory-users-aliases-delete`** - Removes an alias.
- **`workspace-admin-pp-cli admin directory-users-aliases-insert`** - Adds an alias.
- **`workspace-admin-pp-cli admin directory-users-aliases-list`** - Lists all aliases for a user.
- **`workspace-admin-pp-cli admin directory-users-aliases-watch`** - Watches for changes in users list.
- **`workspace-admin-pp-cli admin directory-users-delete`** - Deletes a user.
- **`workspace-admin-pp-cli admin directory-users-get`** - Retrieves a user.
- **`workspace-admin-pp-cli admin directory-users-insert`** - Creates a user.
- **`workspace-admin-pp-cli admin directory-users-list`** - Retrieves a paginated list of either deleted users or all users in a domain.
- **`workspace-admin-pp-cli admin directory-users-make`** - Makes a user a super administrator.
- **`workspace-admin-pp-cli admin directory-users-patch`** - Updates a user using patch semantics. The update method should be used instead, because it also supports patch semantics and has better performance. If you're mapping an external identity to a Google identity, use the [`update`](https://developers.google.com/admin-sdk/directory/v1/reference/users/update) method instead of the `patch` method. This method is unable to clear fields that contain repeated objects (`addresses`, `phones`, etc). Use the update method instead.
- **`workspace-admin-pp-cli admin directory-users-photos-delete`** - Removes the user's photo.
- **`workspace-admin-pp-cli admin directory-users-photos-get`** - Retrieves the user's photo.
- **`workspace-admin-pp-cli admin directory-users-photos-patch`** - Adds a photo for the user. This method supports [patch semantics](/admin-sdk/directory/v1/guides/performance#patch).
- **`workspace-admin-pp-cli admin directory-users-photos-update`** - Adds a photo for the user.
- **`workspace-admin-pp-cli admin directory-users-sign-out`** - Signs a user out of all web and device sessions and reset their sign-in cookies. User will have to sign in by authenticating again.
- **`workspace-admin-pp-cli admin directory-users-undelete`** - Undeletes a deleted user.
- **`workspace-admin-pp-cli admin directory-users-update`** - Updates a user. This method supports patch semantics, meaning that you only need to include the fields you wish to update. Fields that are not present in the request will be preserved, and fields set to `null` will be cleared. For repeating fields that contain arrays, individual items in the array can't be patched piecemeal; they must be supplied in the request body with the desired values for all items. See the [user accounts guide](https://developers.google.com/admin-sdk/directory/v1/guides/manage-users#update_user) for more information.
- **`workspace-admin-pp-cli admin directory-users-watch`** - Watches for changes in users list.
- **`workspace-admin-pp-cli admin directory-verification-codes-generate`** - Generates new backup verification codes for the user.
- **`workspace-admin-pp-cli admin directory-verification-codes-invalidate`** - Invalidates the current backup verification codes for the user.
- **`workspace-admin-pp-cli admin directory-verification-codes-list`** - Returns the current set of valid backup verification codes for the specified user.

### admin-sdk-admin

Manage admin sdk admin

- **`workspace-admin-pp-cli admin-sdk-admin channels-stop`** - Stop watching resources through this channel.
- **`workspace-admin-pp-cli admin-sdk-admin reports-activities-list`** - Retrieves a list of activities for a specific customer's account and application such as the Admin console application or the Google Drive application. For more information, see the guides for administrator and Google Drive activity reports. For more information about the activity report's parameters, see the activity parameters reference guides.
- **`workspace-admin-pp-cli admin-sdk-admin reports-activities-watch`** - Start receiving notifications for account activities. For more information, see Receiving Push Notifications.
- **`workspace-admin-pp-cli admin-sdk-admin reports-customer-usage-reports-get`** - Retrieves a report which is a collection of properties and statistics for a specific customer's account. For more information, see the Customers Usage Report guide. For more information about the customer report's parameters, see the Customers Usage parameters reference guides.
- **`workspace-admin-pp-cli admin-sdk-admin reports-entity-usage-reports-get`** - Retrieves a report which is a collection of properties and statistics for entities used by users within the account. For more information, see the Entities Usage Report guide. For more information about the entities report's parameters, see the Entities Usage parameters reference guides.
- **`workspace-admin-pp-cli admin-sdk-admin reports-user-usage-report-get`** - Retrieves a report which is a collection of properties and statistics for a set of users with the account. For more information, see the User Usage Report guide. For more information about the user report's parameters, see the Users Usage parameters reference guides.

### alerts

Manage alerts

- **`workspace-admin-pp-cli alerts delete`** - Marks the specified alert for deletion. An alert that has been marked for deletion is removed from Alert Center after 30 days. Marking an alert for deletion has no effect on an alert which has already been marked for deletion. Attempting to mark a nonexistent alert for deletion results in a `NOT_FOUND` error.
- **`workspace-admin-pp-cli alerts get`** - Gets the specified alert. Attempting to get a nonexistent alert returns `NOT_FOUND` error.
- **`workspace-admin-pp-cli alerts list`** - Lists the alerts.
- **`workspace-admin-pp-cli alerts undelete`** - Restores, or "undeletes", an alert that was marked for deletion within the past 30 days. Attempting to undelete an alert which was marked for deletion over 30 days ago (which has been removed from the Alert Center database) or a nonexistent alert returns a `NOT_FOUND` error. Attempting to undelete an alert which has not been marked for deletion has no effect.

### alerts-batch-delete

Manage alerts batch delete

- **`workspace-admin-pp-cli alerts-batch-delete`** - Performs batch delete operation on alerts.

### alerts-batch-undelete

Manage alerts batch undelete

- **`workspace-admin-pp-cli alerts-batch-undelete`** - Performs batch undelete operation on alerts.

### changes

Manage changes

- **`workspace-admin-pp-cli changes get-start-page-token`** - Gets the starting pageToken for listing future changes.
- **`workspace-admin-pp-cli changes list`** - Lists the changes for a user or shared drive.
- **`workspace-admin-pp-cli changes watch`** - Subscribes to changes for a user. To use this method, you must include the pageToken query parameter.

### channels

Manage channels

- **`workspace-admin-pp-cli channels`** - Stop watching resources through this channel

### drafts

Manage drafts

- **`workspace-admin-pp-cli drafts create`** - Creates a new draft with the `DRAFT` label.
- **`workspace-admin-pp-cli drafts delete`** - Immediately and permanently deletes the specified draft. Does not simply trash it.
- **`workspace-admin-pp-cli drafts get`** - Gets the specified draft.
- **`workspace-admin-pp-cli drafts list`** - Lists the drafts in the user's mailbox.
- **`workspace-admin-pp-cli drafts send`** - Sends the specified, existing draft to the recipients in the `To`, `Cc`, and `Bcc` headers.
- **`workspace-admin-pp-cli drafts update`** - Replaces a draft's content.

### drive-about

Manage drive about

- **`workspace-admin-pp-cli drive-about`** - Gets information about the user, the user's Drive, and system capabilities.

### drives

Manage drives

- **`workspace-admin-pp-cli drives create`** - Creates a shared drive.
- **`workspace-admin-pp-cli drives delete`** - Permanently deletes a shared drive for which the user is an organizer. The shared drive cannot contain any untrashed items.
- **`workspace-admin-pp-cli drives get`** - Gets a shared drive's metadata by ID.
- **`workspace-admin-pp-cli drives list`** - Lists the user's shared drives.
- **`workspace-admin-pp-cli drives update`** - Updates the metadata for a shared drive.

### files

Manage files

- **`workspace-admin-pp-cli files create`** - Creates a file.
- **`workspace-admin-pp-cli files delete`** - Permanently deletes a file owned by the user without moving it to the trash. If the file belongs to a shared drive the user must be an organizer on the parent. If the target is a folder, all descendants owned by the user are also deleted.
- **`workspace-admin-pp-cli files empty-trash`** - Permanently deletes all of the user's trashed files.
- **`workspace-admin-pp-cli files generate-ids`** - Generates a set of file IDs which can be provided in create or copy requests.
- **`workspace-admin-pp-cli files get`** - Gets a file's metadata or content by ID.
- **`workspace-admin-pp-cli files list`** - Lists or searches files.
- **`workspace-admin-pp-cli files update`** - Updates a file's metadata and/or content. When calling this method, only populate fields in the request that you want to modify. When updating fields, some fields might change automatically, such as modifiedDate. This method supports patch semantics.

### gmail-settings

Manage gmail settings

- **`workspace-admin-pp-cli gmail-settings create`** - Adds a delegate with its verification status set directly to `accepted`, without sending any verification email. The delegate user must be a member of the same Google Workspace organization as the delegator user. Gmail imposes limitations on the number of delegates and delegators each user in a Google Workspace organization can have. These limits depend on your organization, but in general each user can have up to 25 delegates and up to 10 delegators. Note that a delegate user must be referred to by their primary email address, and not an email alias. Also note that when a new delegate is created, there may be up to a one minute delay before the new delegate is available for use. This method is only available to service account clients that have been delegated domain-wide authority.
- **`workspace-admin-pp-cli gmail-settings create-gmail`** - Creates a filter. Note: you can only create a maximum of 1,000 filters.
- **`workspace-admin-pp-cli gmail-settings create-gmail-2`** - Creates a forwarding address. If ownership verification is required, a message will be sent to the recipient and the resource's verification status will be set to `pending`; otherwise, the resource will be created with verification status set to `accepted`. This method is only available to service account clients that have been delegated domain-wide authority.
- **`workspace-admin-pp-cli gmail-settings create-gmail-3`** - Creates a custom "from" send-as alias. If an SMTP MSA is specified, Gmail will attempt to connect to the SMTP service to validate the configuration before creating the alias. If ownership verification is required for the alias, a message will be sent to the email address and the resource's verification status will be set to `pending`; otherwise, the resource will be created with verification status set to `accepted`. If a signature is provided, Gmail will sanitize the HTML before saving it with the alias. This method is only available to service account clients that have been delegated domain-wide authority.
- **`workspace-admin-pp-cli gmail-settings create-gmail-4`** - Creates and configures a client-side encryption identity that's authorized to send mail from the user account. Google publishes the S/MIME certificate to a shared domain-wide directory so that people within a Google Workspace organization can encrypt and send mail to the identity.
- **`workspace-admin-pp-cli gmail-settings create-gmail-5`** - Creates and uploads a client-side encryption S/MIME public key certificate chain and private key metadata for the authenticated user.
- **`workspace-admin-pp-cli gmail-settings delete`** - Removes the specified delegate (which can be of any verification status), and revokes any verification that may have been required for using it. Note that a delegate user must be referred to by their primary email address, and not an email alias. This method is only available to service account clients that have been delegated domain-wide authority.
- **`workspace-admin-pp-cli gmail-settings delete-gmail`** - Immediately and permanently deletes the specified filter.
- **`workspace-admin-pp-cli gmail-settings delete-gmail-2`** - Deletes the specified forwarding address and revokes any verification that may have been required. This method is only available to service account clients that have been delegated domain-wide authority.
- **`workspace-admin-pp-cli gmail-settings delete-gmail-3`** - Deletes the specified send-as alias. Revokes any verification that may have been required for using it. This method is only available to service account clients that have been delegated domain-wide authority.
- **`workspace-admin-pp-cli gmail-settings delete-gmail-4`** - Deletes a client-side encryption identity. The authenticated user can no longer use the identity to send encrypted messages. You cannot restore the identity after you delete it. Instead, use the CreateCseIdentity method to create another identity with the same configuration.
- **`workspace-admin-pp-cli gmail-settings delete-gmail-5`** - Deletes the specified S/MIME config for the specified send-as alias.
- **`workspace-admin-pp-cli gmail-settings disable`** - Turns off a client-side encryption key pair. The authenticated user can no longer use the key pair to decrypt incoming CSE message texts or sign outgoing CSE mail. To regain access, use the EnableCseKeyPair to turn on the key pair. After 30 days, you can permanently delete the key pair by using the ObliterateCseKeyPair method.
- **`workspace-admin-pp-cli gmail-settings enable`** - Turns on a client-side encryption key pair that was turned off. The key pair becomes active again for any associated client-side encryption identities.
- **`workspace-admin-pp-cli gmail-settings get`** - Gets the specified delegate. Note that a delegate user must be referred to by their primary email address, and not an email alias. This method is only available to service account clients that have been delegated domain-wide authority.
- **`workspace-admin-pp-cli gmail-settings get-auto-forwarding`** - Gets the auto-forwarding setting for the specified account.
- **`workspace-admin-pp-cli gmail-settings get-gmail`** - Gets a filter.
- **`workspace-admin-pp-cli gmail-settings get-gmail-2`** - Gets the specified forwarding address.
- **`workspace-admin-pp-cli gmail-settings get-gmail-3`** - Gets the specified send-as alias. Fails with an HTTP 404 error if the specified address is not a member of the collection.
- **`workspace-admin-pp-cli gmail-settings get-gmail-4`** - Retrieves a client-side encryption identity configuration.
- **`workspace-admin-pp-cli gmail-settings get-gmail-5`** - Retrieves an existing client-side encryption key pair.
- **`workspace-admin-pp-cli gmail-settings get-gmail-6`** - Gets the specified S/MIME config for the specified send-as alias.
- **`workspace-admin-pp-cli gmail-settings get-imap`** - Gets IMAP settings.
- **`workspace-admin-pp-cli gmail-settings get-language`** - Gets language settings.
- **`workspace-admin-pp-cli gmail-settings get-pop`** - Gets POP settings.
- **`workspace-admin-pp-cli gmail-settings get-vacation`** - Gets vacation responder settings.
- **`workspace-admin-pp-cli gmail-settings insert`** - Insert (upload) the given S/MIME config for the specified send-as alias. Note that pkcs12 format is required for the key.
- **`workspace-admin-pp-cli gmail-settings list`** - Lists the delegates for the specified account. This method is only available to service account clients that have been delegated domain-wide authority.
- **`workspace-admin-pp-cli gmail-settings list-gmail`** - Lists the message filters of a Gmail user.
- **`workspace-admin-pp-cli gmail-settings list-gmail-2`** - Lists the forwarding addresses for the specified account.
- **`workspace-admin-pp-cli gmail-settings list-gmail-3`** - Lists the send-as aliases for the specified account. The result includes the primary send-as address associated with the account as well as any custom "from" aliases.
- **`workspace-admin-pp-cli gmail-settings list-gmail-4`** - Lists the client-side encrypted identities for an authenticated user.
- **`workspace-admin-pp-cli gmail-settings list-gmail-5`** - Lists client-side encryption key pairs for an authenticated user.
- **`workspace-admin-pp-cli gmail-settings list-gmail-6`** - Lists S/MIME configs for the specified send-as alias.
- **`workspace-admin-pp-cli gmail-settings obliterate`** - Deletes a client-side encryption key pair permanently and immediately. You can only permanently delete key pairs that have been turned off for more than 30 days. To turn off a key pair, use the DisableCseKeyPair method. Gmail can't restore or decrypt any messages that were encrypted by an obliterated key. Authenticated users and Google Workspace administrators lose access to reading the encrypted messages.
- **`workspace-admin-pp-cli gmail-settings patch`** - Patch the specified send-as alias.
- **`workspace-admin-pp-cli gmail-settings patch-gmail`** - Associates a different key pair with an existing client-side encryption identity. The updated key pair must validate against Google's [S/MIME certificate profiles](https://support.google.com/a/answer/7300887).
- **`workspace-admin-pp-cli gmail-settings set-default`** - Sets the default S/MIME config for the specified send-as alias.
- **`workspace-admin-pp-cli gmail-settings update`** - Updates a send-as alias. If a signature is provided, Gmail will sanitize the HTML before saving it with the alias. Addresses other than the primary address for the account can only be updated by service account clients that have been delegated domain-wide authority.
- **`workspace-admin-pp-cli gmail-settings update-auto-forwarding`** - Updates the auto-forwarding setting for the specified account. A verified forwarding address must be specified when auto-forwarding is enabled. This method is only available to service account clients that have been delegated domain-wide authority.
- **`workspace-admin-pp-cli gmail-settings update-imap`** - Updates IMAP settings.
- **`workspace-admin-pp-cli gmail-settings update-language`** - Updates language settings. If successful, the return object contains the `displayLanguage` that was saved for the user, which may differ from the value passed into the request. This is because the requested `displayLanguage` may not be directly supported by Gmail but have a close variant that is, and so the variant may be chosen and saved instead.
- **`workspace-admin-pp-cli gmail-settings update-pop`** - Updates POP settings.
- **`workspace-admin-pp-cli gmail-settings update-vacation`** - Updates vacation responder settings.
- **`workspace-admin-pp-cli gmail-settings verify`** - Sends a verification email to the specified send-as alias address. The verification status must be `pending`. This method is only available to service account clients that have been delegated domain-wide authority.

### history

Manage history

- **`workspace-admin-pp-cli history <userId>`** - Lists the history of all changes to the given mailbox. History results are returned in chronological order (increasing `historyId`).

### labels

Manage labels

- **`workspace-admin-pp-cli labels create`** - Creates a new label.
- **`workspace-admin-pp-cli labels delete`** - Immediately and permanently deletes the specified label and removes it from any messages and threads that it is applied to.
- **`workspace-admin-pp-cli labels get`** - Gets the specified label.
- **`workspace-admin-pp-cli labels list`** - Lists all labels in the user's mailbox.
- **`workspace-admin-pp-cli labels patch`** - Patch the specified label.
- **`workspace-admin-pp-cli labels update`** - Updates the specified label.

### messages

Manage messages

- **`workspace-admin-pp-cli messages batch-delete`** - Deletes many messages by message ID. Provides no guarantees that messages were not already deleted or even existed at all.
- **`workspace-admin-pp-cli messages batch-modify`** - Modifies the labels on the specified messages.
- **`workspace-admin-pp-cli messages delete`** - Immediately and permanently deletes the specified message. This operation cannot be undone. Prefer `messages.trash` instead.
- **`workspace-admin-pp-cli messages get`** - Gets the specified message.
- **`workspace-admin-pp-cli messages import`** - Imports a message into only this user's mailbox, with standard email delivery scanning and classification similar to receiving via SMTP. This method doesn't perform SPF checks, so it might not work for some spam messages, such as those attempting to perform domain spoofing. This method does not send a message. Note: This function doesn't trigger forwarding rules or filters set up by the user.
- **`workspace-admin-pp-cli messages insert`** - Directly inserts a message into only this user's mailbox similar to `IMAP APPEND`, bypassing most scanning and classification. Does not send a message.
- **`workspace-admin-pp-cli messages list`** - Lists the messages in the user's mailbox.
- **`workspace-admin-pp-cli messages send`** - Sends the specified message to the recipients in the `To`, `Cc`, and `Bcc` headers. For example usage, see [Sending email](https://developers.google.com/gmail/api/guides/sending).

### settings

Manage settings

- **`workspace-admin-pp-cli settings alertcenter-get`** - Returns customer-level settings.
- **`workspace-admin-pp-cli settings alertcenter-update`** - Updates the customer-level settings.

### stop

Manage stop

- **`workspace-admin-pp-cli stop <userId>`** - Stop receiving push notifications for the given user mailbox.

### teamdrives

Manage teamdrives

- **`workspace-admin-pp-cli teamdrives create`** - Deprecated use drives.create instead.
- **`workspace-admin-pp-cli teamdrives delete`** - Deprecated use drives.delete instead.
- **`workspace-admin-pp-cli teamdrives get`** - Deprecated use drives.get instead.
- **`workspace-admin-pp-cli teamdrives list`** - Deprecated use drives.list instead.
- **`workspace-admin-pp-cli teamdrives update`** - Deprecated use drives.update instead

### threads

Manage threads

- **`workspace-admin-pp-cli threads delete`** - Immediately and permanently deletes the specified thread. Any messages that belong to the thread are also deleted. This operation cannot be undone. Prefer `threads.trash` instead.
- **`workspace-admin-pp-cli threads get`** - Gets the specified thread.
- **`workspace-admin-pp-cli threads list`** - Lists the threads in the user's mailbox.

### users_profile

Manage users profile

- **`workspace-admin-pp-cli users-profile <userId>`** - Gets the current user's Gmail profile.

### watch

Manage watch

- **`workspace-admin-pp-cli watch <userId>`** - Set up or update a push notification watch on the given user mailbox.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
workspace-admin-pp-cli alerts list

# JSON for scripting and agents
workspace-admin-pp-cli alerts list --json

# Filter to specific fields
workspace-admin-pp-cli alerts list --json --select id,name,status

# Dry run — show the request without sending
workspace-admin-pp-cli alerts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
workspace-admin-pp-cli alerts list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `WORKSPACE_ADMIN_USER_ID` resolves `{userId}`

Base URL: `https://admin.googleapis.com`

## Health Check

```bash
workspace-admin-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `workspace-admin-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/workspace-admin-pp-cli/config.toml`; `--home`, `WORKSPACE_ADMIN_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WORKSPACE_ADMIN_USERID` | endpoint | Yes |  |
| `GOOGLE_WORKSPACE_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `workspace-admin-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `workspace-admin-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GOOGLE_WORKSPACE_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 insufficient permission / scope not authorized** — Authorize the service account's scopes in Admin Console > Security > API Controls > Domain-wide Delegation, then run `workspace-admin-pp-cli doctor` to confirm each scope passes.
- **401 unauthorized on Directory/Reports/Alert Center** — Impersonate a super admin: `auth service-account --key sa.json --impersonate admin@example.com`; admin APIs reject non-admin subjects.
- **Empty results from an audit command** — Run `sync --resources users,drive_files,drive_permissions,tokens,activities` first; audit commands read the local store, not the live API.
- **429 rate limit / quota exceeded on bulk sync** — Re-run sync; it is resumable and rate-limit aware. Narrow scope with `--resources` or `--since` to reduce calls.
- **Cannot read another user's Drive or Gmail** — Per-user data requires domain-wide delegation; impersonate that user with `--impersonate user@example.com`.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**gws (official Google Workspace CLI)**](https://github.com/googleworkspace/cli) — Rust (28500 stars)
- [**GAM**](https://github.com/GAM-team/GAM) — Python (4200 stars)
- [**google_workspace_mcp**](https://github.com/taylorwilsdon/google_workspace_mcp) — Python (2700 stars)
- [**GAMADV-XTD3**](https://github.com/taers232c/GAMADV-XTD3) — Python (800 stars)
- [**orvice/google-workspace-mcp**](https://github.com/orvice/google-workspace-mcp) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
