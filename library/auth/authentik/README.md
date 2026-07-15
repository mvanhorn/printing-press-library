# authentik CLI

**Agent-native admin CLI for authentik identity provider with offline SQLite cache and an MCP server for Claude Desktop.**

Inspect users, groups, applications, flows, tokens, and providers from the terminal or from Claude. Sync once, query offline, audit with a single command.

Printed by [@CarlF01](https://github.com/CarlF01) (CFinney).

## Install

The recommended path installs both the `authentik-pp-cli` binary and the `pp-authentik` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install authentik
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install authentik --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install authentik --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install authentik --agent claude-code
npx -y @mvanhorn/printing-press-library install authentik --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/auth/authentik/cmd/authentik-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/authentik-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-authentik --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-authentik --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-authentik skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-authentik. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/authentik-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `AUTHENTIK_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/auth/authentik/cmd/authentik-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "authentik": {
      "command": "authentik-pp-mcp",
      "env": {
        "AUTHENTIK_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

authentik uses Bearer token auth. Create an API token in the admin UI (Directory > Tokens) and set `AUTHENTIK_TOKEN` in your environment.

## Quick Start

```bash
# Verify auth and reachability
authentik-pp-cli doctor


# One-shot operator health snapshot
authentik-pp-cli health --json


# Cache users, groups, apps, flows locally
authentik-pp-cli sync --full


# List users with selected fields
authentik-pp-cli core users_list --json --select results.username,results.is_active


# Audit unused API tokens
authentik-pp-cli tokens stale --days 90 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`health`** — Joins admin/system + tasks + workers + version into a single agent-readable summary
- **`tokens stale`** — Find API tokens whose owners have not used them in N days
- **`apps unused`** — List applications with no successful login in N days
- **`users groups`** — Recursively expand a user's group memberships including inherited roles
- **`flows map`** — Render a flow with its ordered stage bindings as a tree

## Usage

Run `authentik-pp-cli --help` for the full command reference and flag list.

## Commands

### admin

Manage admin

- **`authentik-pp-cli admin apps-list`** - Read-only view list all installed apps
- **`authentik-pp-cli admin models-list`** - Read-only view list all installed models
- **`authentik-pp-cli admin settings-partial-update`** - Settings view
- **`authentik-pp-cli admin settings-retrieve`** - Settings view
- **`authentik-pp-cli admin settings-update`** - Settings view
- **`authentik-pp-cli admin system-create`** - Get system information.
- **`authentik-pp-cli admin system-retrieve`** - Get system information.
- **`authentik-pp-cli admin version-history-list`** - VersionHistory Viewset
- **`authentik-pp-cli admin version-history-retrieve`** - VersionHistory Viewset
- **`authentik-pp-cli admin version-retrieve`** - Get running and latest version.

### authenticators

Manage authenticators

- **`authentik-pp-cli authenticators admin-all-list`** - Get all devices for current user
- **`authentik-pp-cli authenticators admin-duo-create`** - Viewset for Duo authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-duo-destroy`** - Viewset for Duo authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-duo-list`** - Viewset for Duo authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-duo-partial-update`** - Viewset for Duo authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-duo-retrieve`** - Viewset for Duo authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-duo-update`** - Viewset for Duo authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-email-create`** - Viewset for email authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-email-destroy`** - Viewset for email authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-email-list`** - Viewset for email authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-email-partial-update`** - Viewset for email authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-email-retrieve`** - Viewset for email authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-email-update`** - Viewset for email authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-endpoint-create`** - Viewset for Endpoint authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-endpoint-destroy`** - Viewset for Endpoint authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-endpoint-list`** - Viewset for Endpoint authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-endpoint-partial-update`** - Viewset for Endpoint authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-endpoint-retrieve`** - Viewset for Endpoint authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-endpoint-update`** - Viewset for Endpoint authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-sms-create`** - Viewset for sms authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-sms-destroy`** - Viewset for sms authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-sms-list`** - Viewset for sms authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-sms-partial-update`** - Viewset for sms authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-sms-retrieve`** - Viewset for sms authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-sms-update`** - Viewset for sms authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-static-create`** - Viewset for static authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-static-destroy`** - Viewset for static authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-static-list`** - Viewset for static authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-static-partial-update`** - Viewset for static authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-static-retrieve`** - Viewset for static authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-static-update`** - Viewset for static authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-totp-create`** - Viewset for totp authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-totp-destroy`** - Viewset for totp authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-totp-list`** - Viewset for totp authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-totp-partial-update`** - Viewset for totp authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-totp-retrieve`** - Viewset for totp authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-totp-update`** - Viewset for totp authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-webauthn-create`** - Viewset for WebAuthn authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-webauthn-destroy`** - Viewset for WebAuthn authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-webauthn-list`** - Viewset for WebAuthn authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-webauthn-partial-update`** - Viewset for WebAuthn authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-webauthn-retrieve`** - Viewset for WebAuthn authenticator devices (for admins)
- **`authentik-pp-cli authenticators admin-webauthn-update`** - Viewset for WebAuthn authenticator devices (for admins)
- **`authentik-pp-cli authenticators all-list`** - Get all devices for current user
- **`authentik-pp-cli authenticators duo-destroy`** - Viewset for Duo authenticator devices
- **`authentik-pp-cli authenticators duo-list`** - Viewset for Duo authenticator devices
- **`authentik-pp-cli authenticators duo-partial-update`** - Viewset for Duo authenticator devices
- **`authentik-pp-cli authenticators duo-retrieve`** - Viewset for Duo authenticator devices
- **`authentik-pp-cli authenticators duo-update`** - Viewset for Duo authenticator devices
- **`authentik-pp-cli authenticators duo-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli authenticators email-destroy`** - Viewset for email authenticator devices
- **`authentik-pp-cli authenticators email-list`** - Viewset for email authenticator devices
- **`authentik-pp-cli authenticators email-partial-update`** - Viewset for email authenticator devices
- **`authentik-pp-cli authenticators email-retrieve`** - Viewset for email authenticator devices
- **`authentik-pp-cli authenticators email-update`** - Viewset for email authenticator devices
- **`authentik-pp-cli authenticators email-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli authenticators endpoint-list`** - Viewset for Endpoint authenticator devices
- **`authentik-pp-cli authenticators endpoint-retrieve`** - Viewset for Endpoint authenticator devices
- **`authentik-pp-cli authenticators endpoint-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli authenticators sms-destroy`** - Viewset for sms authenticator devices
- **`authentik-pp-cli authenticators sms-list`** - Viewset for sms authenticator devices
- **`authentik-pp-cli authenticators sms-partial-update`** - Viewset for sms authenticator devices
- **`authentik-pp-cli authenticators sms-retrieve`** - Viewset for sms authenticator devices
- **`authentik-pp-cli authenticators sms-update`** - Viewset for sms authenticator devices
- **`authentik-pp-cli authenticators sms-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli authenticators static-destroy`** - Viewset for static authenticator devices
- **`authentik-pp-cli authenticators static-list`** - Viewset for static authenticator devices
- **`authentik-pp-cli authenticators static-partial-update`** - Viewset for static authenticator devices
- **`authentik-pp-cli authenticators static-retrieve`** - Viewset for static authenticator devices
- **`authentik-pp-cli authenticators static-update`** - Viewset for static authenticator devices
- **`authentik-pp-cli authenticators static-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli authenticators totp-destroy`** - Viewset for totp authenticator devices
- **`authentik-pp-cli authenticators totp-list`** - Viewset for totp authenticator devices
- **`authentik-pp-cli authenticators totp-partial-update`** - Viewset for totp authenticator devices
- **`authentik-pp-cli authenticators totp-retrieve`** - Viewset for totp authenticator devices
- **`authentik-pp-cli authenticators totp-update`** - Viewset for totp authenticator devices
- **`authentik-pp-cli authenticators totp-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli authenticators webauthn-destroy`** - Viewset for WebAuthn authenticator devices
- **`authentik-pp-cli authenticators webauthn-list`** - Viewset for WebAuthn authenticator devices
- **`authentik-pp-cli authenticators webauthn-partial-update`** - Viewset for WebAuthn authenticator devices
- **`authentik-pp-cli authenticators webauthn-retrieve`** - Viewset for WebAuthn authenticator devices
- **`authentik-pp-cli authenticators webauthn-update`** - Viewset for WebAuthn authenticator devices
- **`authentik-pp-cli authenticators webauthn-used-by-list`** - Get a list of all objects that use this object

### core

Manage core

- **`authentik-pp-cli core application-entitlements-create`** - ApplicationEntitlement Viewset
- **`authentik-pp-cli core application-entitlements-destroy`** - ApplicationEntitlement Viewset
- **`authentik-pp-cli core application-entitlements-list`** - ApplicationEntitlement Viewset
- **`authentik-pp-cli core application-entitlements-partial-update`** - ApplicationEntitlement Viewset
- **`authentik-pp-cli core application-entitlements-retrieve`** - ApplicationEntitlement Viewset
- **`authentik-pp-cli core application-entitlements-update`** - ApplicationEntitlement Viewset
- **`authentik-pp-cli core application-entitlements-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli core applications-check-access-retrieve`** - Check access to a single application by slug
- **`authentik-pp-cli core applications-create`** - Application Viewset
- **`authentik-pp-cli core applications-destroy`** - Application Viewset
- **`authentik-pp-cli core applications-list`** - Custom list method that checks Policy based access instead of guardian
- **`authentik-pp-cli core applications-partial-update`** - Application Viewset
- **`authentik-pp-cli core applications-retrieve`** - Application Viewset
- **`authentik-pp-cli core applications-set-icon-create`** - Set application icon
- **`authentik-pp-cli core applications-set-icon-url-create`** - Set application icon (as URL)
- **`authentik-pp-cli core applications-update`** - Application Viewset
- **`authentik-pp-cli core applications-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli core authenticated-sessions-destroy`** - AuthenticatedSession Viewset
- **`authentik-pp-cli core authenticated-sessions-list`** - AuthenticatedSession Viewset
- **`authentik-pp-cli core authenticated-sessions-retrieve`** - AuthenticatedSession Viewset
- **`authentik-pp-cli core authenticated-sessions-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli core brands-create`** - Brand Viewset
- **`authentik-pp-cli core brands-current-retrieve`** - Get current brand
- **`authentik-pp-cli core brands-destroy`** - Brand Viewset
- **`authentik-pp-cli core brands-list`** - Brand Viewset
- **`authentik-pp-cli core brands-partial-update`** - Brand Viewset
- **`authentik-pp-cli core brands-retrieve`** - Brand Viewset
- **`authentik-pp-cli core brands-update`** - Brand Viewset
- **`authentik-pp-cli core brands-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli core groups-add-user-create`** - Add user to group
- **`authentik-pp-cli core groups-create`** - Group Viewset
- **`authentik-pp-cli core groups-destroy`** - Group Viewset
- **`authentik-pp-cli core groups-list`** - Group Viewset
- **`authentik-pp-cli core groups-partial-update`** - Group Viewset
- **`authentik-pp-cli core groups-remove-user-create`** - Remove user from group
- **`authentik-pp-cli core groups-retrieve`** - Group Viewset
- **`authentik-pp-cli core groups-update`** - Group Viewset
- **`authentik-pp-cli core groups-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli core tokens-create`** - Token Viewset
- **`authentik-pp-cli core tokens-destroy`** - Token Viewset
- **`authentik-pp-cli core tokens-list`** - Token Viewset
- **`authentik-pp-cli core tokens-partial-update`** - Token Viewset
- **`authentik-pp-cli core tokens-retrieve`** - Token Viewset
- **`authentik-pp-cli core tokens-set-key-create`** - Set token key. Action is logged as event. `authentik_core.set_token_key` permission
is required.
- **`authentik-pp-cli core tokens-update`** - Token Viewset
- **`authentik-pp-cli core tokens-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli core tokens-view-key-retrieve`** - Return token key and log access
- **`authentik-pp-cli core transactional-applications-update`** - Convert data into a blueprint, validate it and apply it
- **`authentik-pp-cli core user-consent-destroy`** - UserConsent Viewset
- **`authentik-pp-cli core user-consent-list`** - UserConsent Viewset
- **`authentik-pp-cli core user-consent-retrieve`** - UserConsent Viewset
- **`authentik-pp-cli core user-consent-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli core users-create`** - User Viewset
- **`authentik-pp-cli core users-destroy`** - User Viewset
- **`authentik-pp-cli core users-impersonate-create`** - Impersonate a user
- **`authentik-pp-cli core users-impersonate-end-retrieve`** - End Impersonation a user
- **`authentik-pp-cli core users-list`** - User Viewset
- **`authentik-pp-cli core users-me-retrieve`** - Get information about current user
- **`authentik-pp-cli core users-partial-update`** - User Viewset
- **`authentik-pp-cli core users-paths-retrieve`** - Get all user paths
- **`authentik-pp-cli core users-recovery-create`** - Create a temporary link that a user can use to recover their account
- **`authentik-pp-cli core users-recovery-email-create`** - Send an email with a temporary link that a user can use to recover their account
- **`authentik-pp-cli core users-retrieve`** - User Viewset
- **`authentik-pp-cli core users-service-account-create`** - Create a new user account that is marked as a service account
- **`authentik-pp-cli core users-set-password-create`** - Set password for user
- **`authentik-pp-cli core users-update`** - User Viewset
- **`authentik-pp-cli core users-used-by-list`** - Get a list of all objects that use this object

### enterprise

Manage enterprise

- **`authentik-pp-cli enterprise license-create`** - License Viewset
- **`authentik-pp-cli enterprise license-destroy`** - License Viewset
- **`authentik-pp-cli enterprise license-forecast-retrieve`** - Forecast how many users will be required in a year
- **`authentik-pp-cli enterprise license-install-id-retrieve`** - Get install_id
- **`authentik-pp-cli enterprise license-list`** - License Viewset
- **`authentik-pp-cli enterprise license-partial-update`** - License Viewset
- **`authentik-pp-cli enterprise license-retrieve`** - License Viewset
- **`authentik-pp-cli enterprise license-summary-retrieve`** - Get the total license status
- **`authentik-pp-cli enterprise license-update`** - License Viewset
- **`authentik-pp-cli enterprise license-used-by-list`** - Get a list of all objects that use this object

### events

Manage events

- **`authentik-pp-cli events actions-list`** - Get all actions
- **`authentik-pp-cli events create`** - Event Read-Only Viewset
- **`authentik-pp-cli events destroy`** - Event Read-Only Viewset
- **`authentik-pp-cli events list`** - Event Read-Only Viewset
- **`authentik-pp-cli events notifications-destroy`** - Notification Viewset
- **`authentik-pp-cli events notifications-list`** - Notification Viewset
- **`authentik-pp-cli events notifications-mark-all-seen-create`** - Mark all the user's notifications as seen
- **`authentik-pp-cli events notifications-partial-update`** - Notification Viewset
- **`authentik-pp-cli events notifications-retrieve`** - Notification Viewset
- **`authentik-pp-cli events notifications-update`** - Notification Viewset
- **`authentik-pp-cli events notifications-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli events partial-update`** - Event Read-Only Viewset
- **`authentik-pp-cli events retrieve`** - Event Read-Only Viewset
- **`authentik-pp-cli events rules-create`** - NotificationRule Viewset
- **`authentik-pp-cli events rules-destroy`** - NotificationRule Viewset
- **`authentik-pp-cli events rules-list`** - NotificationRule Viewset
- **`authentik-pp-cli events rules-partial-update`** - NotificationRule Viewset
- **`authentik-pp-cli events rules-retrieve`** - NotificationRule Viewset
- **`authentik-pp-cli events rules-update`** - NotificationRule Viewset
- **`authentik-pp-cli events rules-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli events top-per-user-list`** - Get the top_n events grouped by user count
- **`authentik-pp-cli events transports-create`** - NotificationTransport Viewset
- **`authentik-pp-cli events transports-destroy`** - NotificationTransport Viewset
- **`authentik-pp-cli events transports-list`** - NotificationTransport Viewset
- **`authentik-pp-cli events transports-partial-update`** - NotificationTransport Viewset
- **`authentik-pp-cli events transports-retrieve`** - NotificationTransport Viewset
- **`authentik-pp-cli events transports-test-create`** - Send example notification using selected transport. Requires
Modify permissions.
- **`authentik-pp-cli events transports-update`** - NotificationTransport Viewset
- **`authentik-pp-cli events transports-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli events update`** - Event Read-Only Viewset
- **`authentik-pp-cli events volume-list`** - Get event volume for specified filters and timeframe

### flows

Manage flows

- **`authentik-pp-cli flows bindings-create`** - FlowStageBinding Viewset
- **`authentik-pp-cli flows bindings-destroy`** - FlowStageBinding Viewset
- **`authentik-pp-cli flows bindings-list`** - FlowStageBinding Viewset
- **`authentik-pp-cli flows bindings-partial-update`** - FlowStageBinding Viewset
- **`authentik-pp-cli flows bindings-retrieve`** - FlowStageBinding Viewset
- **`authentik-pp-cli flows bindings-update`** - FlowStageBinding Viewset
- **`authentik-pp-cli flows bindings-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli flows executor-get`** - Get the next pending challenge from the currently active flow.
- **`authentik-pp-cli flows executor-solve`** - Solve the previously retrieved challenge and advanced to the next stage.
- **`authentik-pp-cli flows inspector-get`** - Get current flow state and record it
- **`authentik-pp-cli flows instances-cache-clear-create`** - Clear flow cache
- **`authentik-pp-cli flows instances-cache-info-retrieve`** - Info about cached flows
- **`authentik-pp-cli flows instances-create`** - Flow Viewset
- **`authentik-pp-cli flows instances-destroy`** - Flow Viewset
- **`authentik-pp-cli flows instances-diagram-retrieve`** - Return diagram for flow with slug `slug`, in the format used by flowchart.js
- **`authentik-pp-cli flows instances-execute-retrieve`** - Execute flow for current user
- **`authentik-pp-cli flows instances-export-retrieve`** - Export flow to .yaml file
- **`authentik-pp-cli flows instances-import-create`** - Import flow from .yaml file
- **`authentik-pp-cli flows instances-list`** - Flow Viewset
- **`authentik-pp-cli flows instances-partial-update`** - Flow Viewset
- **`authentik-pp-cli flows instances-retrieve`** - Flow Viewset
- **`authentik-pp-cli flows instances-set-background-create`** - Set Flow background
- **`authentik-pp-cli flows instances-set-background-url-create`** - Set Flow background (as URL)
- **`authentik-pp-cli flows instances-update`** - Flow Viewset
- **`authentik-pp-cli flows instances-used-by-list`** - Get a list of all objects that use this object

### oauth2

Manage oauth2

- **`authentik-pp-cli oauth2 access-tokens-destroy`** - AccessToken Viewset
- **`authentik-pp-cli oauth2 access-tokens-list`** - AccessToken Viewset
- **`authentik-pp-cli oauth2 access-tokens-retrieve`** - AccessToken Viewset
- **`authentik-pp-cli oauth2 access-tokens-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli oauth2 authorization-codes-destroy`** - AuthorizationCode Viewset
- **`authentik-pp-cli oauth2 authorization-codes-list`** - AuthorizationCode Viewset
- **`authentik-pp-cli oauth2 authorization-codes-retrieve`** - AuthorizationCode Viewset
- **`authentik-pp-cli oauth2 authorization-codes-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli oauth2 refresh-tokens-destroy`** - RefreshToken Viewset
- **`authentik-pp-cli oauth2 refresh-tokens-list`** - RefreshToken Viewset
- **`authentik-pp-cli oauth2 refresh-tokens-retrieve`** - RefreshToken Viewset
- **`authentik-pp-cli oauth2 refresh-tokens-used-by-list`** - Get a list of all objects that use this object

### policies

Manage policies

- **`authentik-pp-cli policies all-cache-clear-create`** - Clear policy cache
- **`authentik-pp-cli policies all-cache-info-retrieve`** - Info about cached policies
- **`authentik-pp-cli policies all-destroy`** - Policy Viewset
- **`authentik-pp-cli policies all-list`** - Policy Viewset
- **`authentik-pp-cli policies all-retrieve`** - Policy Viewset
- **`authentik-pp-cli policies all-test-create`** - Test policy
- **`authentik-pp-cli policies all-types-list`** - Get all creatable types
- **`authentik-pp-cli policies all-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli policies bindings-create`** - PolicyBinding Viewset
- **`authentik-pp-cli policies bindings-destroy`** - PolicyBinding Viewset
- **`authentik-pp-cli policies bindings-list`** - PolicyBinding Viewset
- **`authentik-pp-cli policies bindings-partial-update`** - PolicyBinding Viewset
- **`authentik-pp-cli policies bindings-retrieve`** - PolicyBinding Viewset
- **`authentik-pp-cli policies bindings-update`** - PolicyBinding Viewset
- **`authentik-pp-cli policies bindings-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli policies dummy-create`** - Dummy Viewset
- **`authentik-pp-cli policies dummy-destroy`** - Dummy Viewset
- **`authentik-pp-cli policies dummy-list`** - Dummy Viewset
- **`authentik-pp-cli policies dummy-partial-update`** - Dummy Viewset
- **`authentik-pp-cli policies dummy-retrieve`** - Dummy Viewset
- **`authentik-pp-cli policies dummy-update`** - Dummy Viewset
- **`authentik-pp-cli policies dummy-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli policies event-matcher-create`** - Event Matcher Policy Viewset
- **`authentik-pp-cli policies event-matcher-destroy`** - Event Matcher Policy Viewset
- **`authentik-pp-cli policies event-matcher-list`** - Event Matcher Policy Viewset
- **`authentik-pp-cli policies event-matcher-partial-update`** - Event Matcher Policy Viewset
- **`authentik-pp-cli policies event-matcher-retrieve`** - Event Matcher Policy Viewset
- **`authentik-pp-cli policies event-matcher-update`** - Event Matcher Policy Viewset
- **`authentik-pp-cli policies event-matcher-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli policies expression-create`** - Source Viewset
- **`authentik-pp-cli policies expression-destroy`** - Source Viewset
- **`authentik-pp-cli policies expression-list`** - Source Viewset
- **`authentik-pp-cli policies expression-partial-update`** - Source Viewset
- **`authentik-pp-cli policies expression-retrieve`** - Source Viewset
- **`authentik-pp-cli policies expression-update`** - Source Viewset
- **`authentik-pp-cli policies expression-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli policies geoip-create`** - GeoIP Viewset
- **`authentik-pp-cli policies geoip-destroy`** - GeoIP Viewset
- **`authentik-pp-cli policies geoip-iso3166-list`** - Get all countries in ISO-3166-1
- **`authentik-pp-cli policies geoip-list`** - GeoIP Viewset
- **`authentik-pp-cli policies geoip-partial-update`** - GeoIP Viewset
- **`authentik-pp-cli policies geoip-retrieve`** - GeoIP Viewset
- **`authentik-pp-cli policies geoip-update`** - GeoIP Viewset
- **`authentik-pp-cli policies geoip-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli policies password-create`** - Password Policy Viewset
- **`authentik-pp-cli policies password-destroy`** - Password Policy Viewset
- **`authentik-pp-cli policies password-expiry-create`** - Password Expiry Viewset
- **`authentik-pp-cli policies password-expiry-destroy`** - Password Expiry Viewset
- **`authentik-pp-cli policies password-expiry-list`** - Password Expiry Viewset
- **`authentik-pp-cli policies password-expiry-partial-update`** - Password Expiry Viewset
- **`authentik-pp-cli policies password-expiry-retrieve`** - Password Expiry Viewset
- **`authentik-pp-cli policies password-expiry-update`** - Password Expiry Viewset
- **`authentik-pp-cli policies password-expiry-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli policies password-list`** - Password Policy Viewset
- **`authentik-pp-cli policies password-partial-update`** - Password Policy Viewset
- **`authentik-pp-cli policies password-retrieve`** - Password Policy Viewset
- **`authentik-pp-cli policies password-update`** - Password Policy Viewset
- **`authentik-pp-cli policies password-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli policies reputation-create`** - Reputation Policy Viewset
- **`authentik-pp-cli policies reputation-destroy`** - Reputation Policy Viewset
- **`authentik-pp-cli policies reputation-list`** - Reputation Policy Viewset
- **`authentik-pp-cli policies reputation-partial-update`** - Reputation Policy Viewset
- **`authentik-pp-cli policies reputation-retrieve`** - Reputation Policy Viewset
- **`authentik-pp-cli policies reputation-scores-destroy`** - Reputation Viewset
- **`authentik-pp-cli policies reputation-scores-list`** - Reputation Viewset
- **`authentik-pp-cli policies reputation-scores-retrieve`** - Reputation Viewset
- **`authentik-pp-cli policies reputation-scores-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli policies reputation-update`** - Reputation Policy Viewset
- **`authentik-pp-cli policies reputation-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli policies unique-password-create`** - Password Uniqueness Policy Viewset
- **`authentik-pp-cli policies unique-password-destroy`** - Password Uniqueness Policy Viewset
- **`authentik-pp-cli policies unique-password-list`** - Password Uniqueness Policy Viewset
- **`authentik-pp-cli policies unique-password-partial-update`** - Password Uniqueness Policy Viewset
- **`authentik-pp-cli policies unique-password-retrieve`** - Password Uniqueness Policy Viewset
- **`authentik-pp-cli policies unique-password-update`** - Password Uniqueness Policy Viewset
- **`authentik-pp-cli policies unique-password-used-by-list`** - Get a list of all objects that use this object

### propertymappings

Manage propertymappings

- **`authentik-pp-cli propertymappings all-destroy`** - PropertyMapping Viewset
- **`authentik-pp-cli propertymappings all-list`** - PropertyMapping Viewset
- **`authentik-pp-cli propertymappings all-retrieve`** - PropertyMapping Viewset
- **`authentik-pp-cli propertymappings all-test-create`** - Test Property Mapping
- **`authentik-pp-cli propertymappings all-types-list`** - Get all creatable types
- **`authentik-pp-cli propertymappings all-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings notification-create`** - NotificationWebhookMapping Viewset
- **`authentik-pp-cli propertymappings notification-destroy`** - NotificationWebhookMapping Viewset
- **`authentik-pp-cli propertymappings notification-list`** - NotificationWebhookMapping Viewset
- **`authentik-pp-cli propertymappings notification-partial-update`** - NotificationWebhookMapping Viewset
- **`authentik-pp-cli propertymappings notification-retrieve`** - NotificationWebhookMapping Viewset
- **`authentik-pp-cli propertymappings notification-update`** - NotificationWebhookMapping Viewset
- **`authentik-pp-cli propertymappings notification-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings provider-google-workspace-create`** - GoogleWorkspaceProviderMapping Viewset
- **`authentik-pp-cli propertymappings provider-google-workspace-destroy`** - GoogleWorkspaceProviderMapping Viewset
- **`authentik-pp-cli propertymappings provider-google-workspace-list`** - GoogleWorkspaceProviderMapping Viewset
- **`authentik-pp-cli propertymappings provider-google-workspace-partial-update`** - GoogleWorkspaceProviderMapping Viewset
- **`authentik-pp-cli propertymappings provider-google-workspace-retrieve`** - GoogleWorkspaceProviderMapping Viewset
- **`authentik-pp-cli propertymappings provider-google-workspace-update`** - GoogleWorkspaceProviderMapping Viewset
- **`authentik-pp-cli propertymappings provider-google-workspace-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings provider-microsoft-entra-create`** - MicrosoftEntraProviderMapping Viewset
- **`authentik-pp-cli propertymappings provider-microsoft-entra-destroy`** - MicrosoftEntraProviderMapping Viewset
- **`authentik-pp-cli propertymappings provider-microsoft-entra-list`** - MicrosoftEntraProviderMapping Viewset
- **`authentik-pp-cli propertymappings provider-microsoft-entra-partial-update`** - MicrosoftEntraProviderMapping Viewset
- **`authentik-pp-cli propertymappings provider-microsoft-entra-retrieve`** - MicrosoftEntraProviderMapping Viewset
- **`authentik-pp-cli propertymappings provider-microsoft-entra-update`** - MicrosoftEntraProviderMapping Viewset
- **`authentik-pp-cli propertymappings provider-microsoft-entra-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings provider-rac-create`** - RACPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-rac-destroy`** - RACPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-rac-list`** - RACPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-rac-partial-update`** - RACPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-rac-retrieve`** - RACPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-rac-update`** - RACPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-rac-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings provider-radius-create`** - RadiusProviderPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-radius-destroy`** - RadiusProviderPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-radius-list`** - RadiusProviderPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-radius-partial-update`** - RadiusProviderPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-radius-retrieve`** - RadiusProviderPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-radius-update`** - RadiusProviderPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-radius-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings provider-saml-create`** - SAMLPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-saml-destroy`** - SAMLPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-saml-list`** - SAMLPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-saml-partial-update`** - SAMLPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-saml-retrieve`** - SAMLPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-saml-update`** - SAMLPropertyMapping Viewset
- **`authentik-pp-cli propertymappings provider-saml-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings provider-scim-create`** - SCIMMapping Viewset
- **`authentik-pp-cli propertymappings provider-scim-destroy`** - SCIMMapping Viewset
- **`authentik-pp-cli propertymappings provider-scim-list`** - SCIMMapping Viewset
- **`authentik-pp-cli propertymappings provider-scim-partial-update`** - SCIMMapping Viewset
- **`authentik-pp-cli propertymappings provider-scim-retrieve`** - SCIMMapping Viewset
- **`authentik-pp-cli propertymappings provider-scim-update`** - SCIMMapping Viewset
- **`authentik-pp-cli propertymappings provider-scim-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings provider-scope-create`** - ScopeMapping Viewset
- **`authentik-pp-cli propertymappings provider-scope-destroy`** - ScopeMapping Viewset
- **`authentik-pp-cli propertymappings provider-scope-list`** - ScopeMapping Viewset
- **`authentik-pp-cli propertymappings provider-scope-partial-update`** - ScopeMapping Viewset
- **`authentik-pp-cli propertymappings provider-scope-retrieve`** - ScopeMapping Viewset
- **`authentik-pp-cli propertymappings provider-scope-update`** - ScopeMapping Viewset
- **`authentik-pp-cli propertymappings provider-scope-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings source-kerberos-create`** - KerberosSource PropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-kerberos-destroy`** - KerberosSource PropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-kerberos-list`** - KerberosSource PropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-kerberos-partial-update`** - KerberosSource PropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-kerberos-retrieve`** - KerberosSource PropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-kerberos-update`** - KerberosSource PropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-kerberos-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings source-ldap-create`** - LDAP PropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-ldap-destroy`** - LDAP PropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-ldap-list`** - LDAP PropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-ldap-partial-update`** - LDAP PropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-ldap-retrieve`** - LDAP PropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-ldap-update`** - LDAP PropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-ldap-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings source-oauth-create`** - OAuthSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-oauth-destroy`** - OAuthSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-oauth-list`** - OAuthSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-oauth-partial-update`** - OAuthSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-oauth-retrieve`** - OAuthSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-oauth-update`** - OAuthSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-oauth-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings source-plex-create`** - PlexSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-plex-destroy`** - PlexSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-plex-list`** - PlexSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-plex-partial-update`** - PlexSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-plex-retrieve`** - PlexSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-plex-update`** - PlexSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-plex-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings source-saml-create`** - SAMLSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-saml-destroy`** - SAMLSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-saml-list`** - SAMLSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-saml-partial-update`** - SAMLSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-saml-retrieve`** - SAMLSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-saml-update`** - SAMLSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-saml-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings source-scim-create`** - SCIMSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-scim-destroy`** - SCIMSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-scim-list`** - SCIMSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-scim-partial-update`** - SCIMSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-scim-retrieve`** - SCIMSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-scim-update`** - SCIMSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-scim-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli propertymappings source-telegram-create`** - TelegramSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-telegram-destroy`** - TelegramSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-telegram-list`** - TelegramSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-telegram-partial-update`** - TelegramSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-telegram-retrieve`** - TelegramSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-telegram-update`** - TelegramSourcePropertyMapping Viewset
- **`authentik-pp-cli propertymappings source-telegram-used-by-list`** - Get a list of all objects that use this object

### providers

Manage providers

- **`authentik-pp-cli providers all-destroy`** - Provider Viewset
- **`authentik-pp-cli providers all-list`** - Provider Viewset
- **`authentik-pp-cli providers all-retrieve`** - Provider Viewset
- **`authentik-pp-cli providers all-types-list`** - Get all creatable types
- **`authentik-pp-cli providers all-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers google-workspace-create`** - GoogleWorkspaceProvider Viewset
- **`authentik-pp-cli providers google-workspace-destroy`** - GoogleWorkspaceProvider Viewset
- **`authentik-pp-cli providers google-workspace-groups-create`** - GoogleWorkspaceProviderGroup Viewset
- **`authentik-pp-cli providers google-workspace-groups-destroy`** - GoogleWorkspaceProviderGroup Viewset
- **`authentik-pp-cli providers google-workspace-groups-list`** - GoogleWorkspaceProviderGroup Viewset
- **`authentik-pp-cli providers google-workspace-groups-retrieve`** - GoogleWorkspaceProviderGroup Viewset
- **`authentik-pp-cli providers google-workspace-groups-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers google-workspace-list`** - GoogleWorkspaceProvider Viewset
- **`authentik-pp-cli providers google-workspace-partial-update`** - GoogleWorkspaceProvider Viewset
- **`authentik-pp-cli providers google-workspace-retrieve`** - GoogleWorkspaceProvider Viewset
- **`authentik-pp-cli providers google-workspace-sync-object-create`** - Sync/Re-sync a single user/group object
- **`authentik-pp-cli providers google-workspace-sync-status-retrieve`** - Get provider's sync status
- **`authentik-pp-cli providers google-workspace-update`** - GoogleWorkspaceProvider Viewset
- **`authentik-pp-cli providers google-workspace-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers google-workspace-users-create`** - GoogleWorkspaceProviderUser Viewset
- **`authentik-pp-cli providers google-workspace-users-destroy`** - GoogleWorkspaceProviderUser Viewset
- **`authentik-pp-cli providers google-workspace-users-list`** - GoogleWorkspaceProviderUser Viewset
- **`authentik-pp-cli providers google-workspace-users-retrieve`** - GoogleWorkspaceProviderUser Viewset
- **`authentik-pp-cli providers google-workspace-users-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers ldap-create`** - LDAPProvider Viewset
- **`authentik-pp-cli providers ldap-destroy`** - LDAPProvider Viewset
- **`authentik-pp-cli providers ldap-list`** - LDAPProvider Viewset
- **`authentik-pp-cli providers ldap-partial-update`** - LDAPProvider Viewset
- **`authentik-pp-cli providers ldap-retrieve`** - LDAPProvider Viewset
- **`authentik-pp-cli providers ldap-update`** - LDAPProvider Viewset
- **`authentik-pp-cli providers ldap-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers microsoft-entra-create`** - MicrosoftEntraProvider Viewset
- **`authentik-pp-cli providers microsoft-entra-destroy`** - MicrosoftEntraProvider Viewset
- **`authentik-pp-cli providers microsoft-entra-groups-create`** - MicrosoftEntraProviderGroup Viewset
- **`authentik-pp-cli providers microsoft-entra-groups-destroy`** - MicrosoftEntraProviderGroup Viewset
- **`authentik-pp-cli providers microsoft-entra-groups-list`** - MicrosoftEntraProviderGroup Viewset
- **`authentik-pp-cli providers microsoft-entra-groups-retrieve`** - MicrosoftEntraProviderGroup Viewset
- **`authentik-pp-cli providers microsoft-entra-groups-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers microsoft-entra-list`** - MicrosoftEntraProvider Viewset
- **`authentik-pp-cli providers microsoft-entra-partial-update`** - MicrosoftEntraProvider Viewset
- **`authentik-pp-cli providers microsoft-entra-retrieve`** - MicrosoftEntraProvider Viewset
- **`authentik-pp-cli providers microsoft-entra-sync-object-create`** - Sync/Re-sync a single user/group object
- **`authentik-pp-cli providers microsoft-entra-sync-status-retrieve`** - Get provider's sync status
- **`authentik-pp-cli providers microsoft-entra-update`** - MicrosoftEntraProvider Viewset
- **`authentik-pp-cli providers microsoft-entra-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers microsoft-entra-users-create`** - MicrosoftEntraProviderUser Viewset
- **`authentik-pp-cli providers microsoft-entra-users-destroy`** - MicrosoftEntraProviderUser Viewset
- **`authentik-pp-cli providers microsoft-entra-users-list`** - MicrosoftEntraProviderUser Viewset
- **`authentik-pp-cli providers microsoft-entra-users-retrieve`** - MicrosoftEntraProviderUser Viewset
- **`authentik-pp-cli providers microsoft-entra-users-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers oauth2-create`** - OAuth2Provider Viewset
- **`authentik-pp-cli providers oauth2-destroy`** - OAuth2Provider Viewset
- **`authentik-pp-cli providers oauth2-list`** - OAuth2Provider Viewset
- **`authentik-pp-cli providers oauth2-partial-update`** - OAuth2Provider Viewset
- **`authentik-pp-cli providers oauth2-preview-user-retrieve`** - Preview user data for provider
- **`authentik-pp-cli providers oauth2-retrieve`** - OAuth2Provider Viewset
- **`authentik-pp-cli providers oauth2-setup-urls-retrieve`** - Get Providers setup URLs
- **`authentik-pp-cli providers oauth2-update`** - OAuth2Provider Viewset
- **`authentik-pp-cli providers oauth2-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers proxy-create`** - ProxyProvider Viewset
- **`authentik-pp-cli providers proxy-destroy`** - ProxyProvider Viewset
- **`authentik-pp-cli providers proxy-list`** - ProxyProvider Viewset
- **`authentik-pp-cli providers proxy-partial-update`** - ProxyProvider Viewset
- **`authentik-pp-cli providers proxy-retrieve`** - ProxyProvider Viewset
- **`authentik-pp-cli providers proxy-update`** - ProxyProvider Viewset
- **`authentik-pp-cli providers proxy-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers rac-create`** - RACProvider Viewset
- **`authentik-pp-cli providers rac-destroy`** - RACProvider Viewset
- **`authentik-pp-cli providers rac-list`** - RACProvider Viewset
- **`authentik-pp-cli providers rac-partial-update`** - RACProvider Viewset
- **`authentik-pp-cli providers rac-retrieve`** - RACProvider Viewset
- **`authentik-pp-cli providers rac-update`** - RACProvider Viewset
- **`authentik-pp-cli providers rac-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers radius-create`** - RadiusProvider Viewset
- **`authentik-pp-cli providers radius-destroy`** - RadiusProvider Viewset
- **`authentik-pp-cli providers radius-list`** - RadiusProvider Viewset
- **`authentik-pp-cli providers radius-partial-update`** - RadiusProvider Viewset
- **`authentik-pp-cli providers radius-retrieve`** - RadiusProvider Viewset
- **`authentik-pp-cli providers radius-update`** - RadiusProvider Viewset
- **`authentik-pp-cli providers radius-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers saml-create`** - SAMLProvider Viewset
- **`authentik-pp-cli providers saml-destroy`** - SAMLProvider Viewset
- **`authentik-pp-cli providers saml-import-metadata-create`** - Create provider from SAML Metadata
- **`authentik-pp-cli providers saml-list`** - SAMLProvider Viewset
- **`authentik-pp-cli providers saml-metadata-retrieve`** - Return metadata as XML string
- **`authentik-pp-cli providers saml-partial-update`** - SAMLProvider Viewset
- **`authentik-pp-cli providers saml-preview-user-retrieve`** - Preview user data for provider
- **`authentik-pp-cli providers saml-retrieve`** - SAMLProvider Viewset
- **`authentik-pp-cli providers saml-update`** - SAMLProvider Viewset
- **`authentik-pp-cli providers saml-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers scim-create`** - SCIMProvider Viewset
- **`authentik-pp-cli providers scim-destroy`** - SCIMProvider Viewset
- **`authentik-pp-cli providers scim-groups-create`** - SCIMProviderGroup Viewset
- **`authentik-pp-cli providers scim-groups-destroy`** - SCIMProviderGroup Viewset
- **`authentik-pp-cli providers scim-groups-list`** - SCIMProviderGroup Viewset
- **`authentik-pp-cli providers scim-groups-retrieve`** - SCIMProviderGroup Viewset
- **`authentik-pp-cli providers scim-groups-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers scim-list`** - SCIMProvider Viewset
- **`authentik-pp-cli providers scim-partial-update`** - SCIMProvider Viewset
- **`authentik-pp-cli providers scim-retrieve`** - SCIMProvider Viewset
- **`authentik-pp-cli providers scim-sync-object-create`** - Sync/Re-sync a single user/group object
- **`authentik-pp-cli providers scim-sync-status-retrieve`** - Get provider's sync status
- **`authentik-pp-cli providers scim-update`** - SCIMProvider Viewset
- **`authentik-pp-cli providers scim-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers scim-users-create`** - SCIMProviderUser Viewset
- **`authentik-pp-cli providers scim-users-destroy`** - SCIMProviderUser Viewset
- **`authentik-pp-cli providers scim-users-list`** - SCIMProviderUser Viewset
- **`authentik-pp-cli providers scim-users-retrieve`** - SCIMProviderUser Viewset
- **`authentik-pp-cli providers scim-users-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli providers ssf-create`** - SSFProvider Viewset
- **`authentik-pp-cli providers ssf-destroy`** - SSFProvider Viewset
- **`authentik-pp-cli providers ssf-list`** - SSFProvider Viewset
- **`authentik-pp-cli providers ssf-partial-update`** - SSFProvider Viewset
- **`authentik-pp-cli providers ssf-retrieve`** - SSFProvider Viewset
- **`authentik-pp-cli providers ssf-update`** - SSFProvider Viewset
- **`authentik-pp-cli providers ssf-used-by-list`** - Get a list of all objects that use this object

### rac

Manage rac

- **`authentik-pp-cli rac connection-tokens-destroy`** - ConnectionToken Viewset
- **`authentik-pp-cli rac connection-tokens-list`** - ConnectionToken Viewset
- **`authentik-pp-cli rac connection-tokens-partial-update`** - ConnectionToken Viewset
- **`authentik-pp-cli rac connection-tokens-retrieve`** - ConnectionToken Viewset
- **`authentik-pp-cli rac connection-tokens-update`** - ConnectionToken Viewset
- **`authentik-pp-cli rac connection-tokens-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli rac endpoints-create`** - Endpoint Viewset
- **`authentik-pp-cli rac endpoints-destroy`** - Endpoint Viewset
- **`authentik-pp-cli rac endpoints-list`** - List accessible endpoints
- **`authentik-pp-cli rac endpoints-partial-update`** - Endpoint Viewset
- **`authentik-pp-cli rac endpoints-retrieve`** - Endpoint Viewset
- **`authentik-pp-cli rac endpoints-update`** - Endpoint Viewset
- **`authentik-pp-cli rac endpoints-used-by-list`** - Get a list of all objects that use this object

### rbac

Manage rbac

- **`authentik-pp-cli rbac initial-permissions-create`** - InitialPermissions viewset
- **`authentik-pp-cli rbac initial-permissions-destroy`** - InitialPermissions viewset
- **`authentik-pp-cli rbac initial-permissions-list`** - InitialPermissions viewset
- **`authentik-pp-cli rbac initial-permissions-partial-update`** - InitialPermissions viewset
- **`authentik-pp-cli rbac initial-permissions-retrieve`** - InitialPermissions viewset
- **`authentik-pp-cli rbac initial-permissions-update`** - InitialPermissions viewset
- **`authentik-pp-cli rbac initial-permissions-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli rbac permissions-assigned-by-roles-assign`** - Assign permission(s) to role. When `object_pk` is set, the permissions
are only assigned to the specific object, otherwise they are assigned globally.
- **`authentik-pp-cli rbac permissions-assigned-by-roles-list`** - Get assigned object permissions for a single object
- **`authentik-pp-cli rbac permissions-assigned-by-roles-unassign-partial-update`** - Unassign permission(s) to role. When `object_pk` is set, the permissions
are only assigned to the specific object, otherwise they are assigned globally.
- **`authentik-pp-cli rbac permissions-assigned-by-users-assign`** - Assign permission(s) to user
- **`authentik-pp-cli rbac permissions-assigned-by-users-list`** - Get assigned object permissions for a single object
- **`authentik-pp-cli rbac permissions-assigned-by-users-unassign-partial-update`** - Unassign permission(s) to user. When `object_pk` is set, the permissions
are only assigned to the specific object, otherwise they are assigned globally.
- **`authentik-pp-cli rbac permissions-list`** - Read-only list of all permissions, filterable by model and app
- **`authentik-pp-cli rbac permissions-retrieve`** - Read-only list of all permissions, filterable by model and app
- **`authentik-pp-cli rbac permissions-roles-destroy`** - Get a role's assigned object permissions
- **`authentik-pp-cli rbac permissions-roles-list`** - Get a role's assigned object permissions
- **`authentik-pp-cli rbac permissions-roles-partial-update`** - Get a role's assigned object permissions
- **`authentik-pp-cli rbac permissions-roles-retrieve`** - Get a role's assigned object permissions
- **`authentik-pp-cli rbac permissions-roles-update`** - Get a role's assigned object permissions
- **`authentik-pp-cli rbac permissions-users-destroy`** - Get a users's assigned object permissions
- **`authentik-pp-cli rbac permissions-users-list`** - Get a users's assigned object permissions
- **`authentik-pp-cli rbac permissions-users-partial-update`** - Get a users's assigned object permissions
- **`authentik-pp-cli rbac permissions-users-retrieve`** - Get a users's assigned object permissions
- **`authentik-pp-cli rbac permissions-users-update`** - Get a users's assigned object permissions
- **`authentik-pp-cli rbac roles-create`** - Role viewset
- **`authentik-pp-cli rbac roles-destroy`** - Role viewset
- **`authentik-pp-cli rbac roles-list`** - Role viewset
- **`authentik-pp-cli rbac roles-partial-update`** - Role viewset
- **`authentik-pp-cli rbac roles-retrieve`** - Role viewset
- **`authentik-pp-cli rbac roles-update`** - Role viewset
- **`authentik-pp-cli rbac roles-used-by-list`** - Get a list of all objects that use this object

### root

Manage root

- **`authentik-pp-cli root`** - Retrieve public configuration options

### schema

Manage schema

- **`authentik-pp-cli schema`** - OpenApi3 schema for this API. Format can be selected via content negotiation.

- YAML: application/vnd.oai.openapi
- JSON: application/vnd.oai.openapi+json

### sources

Manage sources

- **`authentik-pp-cli sources all-destroy`** - Prevent deletion of built-in sources
- **`authentik-pp-cli sources all-list`** - Source Viewset
- **`authentik-pp-cli sources all-retrieve`** - Source Viewset
- **`authentik-pp-cli sources all-set-icon-create`** - Set source icon
- **`authentik-pp-cli sources all-set-icon-url-create`** - Set source icon (as URL)
- **`authentik-pp-cli sources all-types-list`** - Get all creatable types
- **`authentik-pp-cli sources all-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources all-user-settings-list`** - Get all sources the user can configure
- **`authentik-pp-cli sources group-connections-all-destroy`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-all-list`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-all-partial-update`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-all-retrieve`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-all-update`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-all-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources group-connections-kerberos-create`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-kerberos-destroy`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-kerberos-list`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-kerberos-partial-update`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-kerberos-retrieve`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-kerberos-update`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-kerberos-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources group-connections-ldap-create`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-ldap-destroy`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-ldap-list`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-ldap-partial-update`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-ldap-retrieve`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-ldap-update`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-ldap-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources group-connections-oauth-create`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-oauth-destroy`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-oauth-list`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-oauth-partial-update`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-oauth-retrieve`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-oauth-update`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-oauth-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources group-connections-plex-create`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-plex-destroy`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-plex-list`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-plex-partial-update`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-plex-retrieve`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-plex-update`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-plex-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources group-connections-saml-create`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-saml-destroy`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-saml-list`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-saml-partial-update`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-saml-retrieve`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-saml-update`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-saml-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources group-connections-telegram-create`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-telegram-destroy`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-telegram-list`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-telegram-partial-update`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-telegram-retrieve`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-telegram-update`** - Group-source connection Viewset
- **`authentik-pp-cli sources group-connections-telegram-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources kerberos-create`** - Kerberos Source Viewset
- **`authentik-pp-cli sources kerberos-destroy`** - Kerberos Source Viewset
- **`authentik-pp-cli sources kerberos-list`** - Kerberos Source Viewset
- **`authentik-pp-cli sources kerberos-partial-update`** - Kerberos Source Viewset
- **`authentik-pp-cli sources kerberos-retrieve`** - Kerberos Source Viewset
- **`authentik-pp-cli sources kerberos-sync-status-retrieve`** - Get provider's sync status
- **`authentik-pp-cli sources kerberos-update`** - Kerberos Source Viewset
- **`authentik-pp-cli sources kerberos-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources ldap-create`** - LDAP Source Viewset
- **`authentik-pp-cli sources ldap-debug-retrieve`** - Get raw LDAP data to debug
- **`authentik-pp-cli sources ldap-destroy`** - LDAP Source Viewset
- **`authentik-pp-cli sources ldap-list`** - LDAP Source Viewset
- **`authentik-pp-cli sources ldap-partial-update`** - LDAP Source Viewset
- **`authentik-pp-cli sources ldap-retrieve`** - LDAP Source Viewset
- **`authentik-pp-cli sources ldap-sync-status-retrieve`** - Get provider's sync status
- **`authentik-pp-cli sources ldap-update`** - LDAP Source Viewset
- **`authentik-pp-cli sources ldap-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources oauth-create`** - Source Viewset
- **`authentik-pp-cli sources oauth-destroy`** - Source Viewset
- **`authentik-pp-cli sources oauth-list`** - Source Viewset
- **`authentik-pp-cli sources oauth-partial-update`** - Source Viewset
- **`authentik-pp-cli sources oauth-retrieve`** - Source Viewset
- **`authentik-pp-cli sources oauth-types-list`** - Get all creatable source types. If ?name is set, only returns the type for <name>.
If <name> isn't found, returns the default type.
- **`authentik-pp-cli sources oauth-update`** - Source Viewset
- **`authentik-pp-cli sources oauth-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources plex-create`** - Plex source Viewset
- **`authentik-pp-cli sources plex-destroy`** - Plex source Viewset
- **`authentik-pp-cli sources plex-list`** - Plex source Viewset
- **`authentik-pp-cli sources plex-partial-update`** - Plex source Viewset
- **`authentik-pp-cli sources plex-redeem-token-authenticated-create`** - Redeem a plex token for an authenticated user, creating a connection
- **`authentik-pp-cli sources plex-redeem-token-create`** - Redeem a plex token, check it's access to resources against what's allowed
for the source, and redirect to an authentication/enrollment flow.
- **`authentik-pp-cli sources plex-retrieve`** - Plex source Viewset
- **`authentik-pp-cli sources plex-update`** - Plex source Viewset
- **`authentik-pp-cli sources plex-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources saml-create`** - SAMLSource Viewset
- **`authentik-pp-cli sources saml-destroy`** - SAMLSource Viewset
- **`authentik-pp-cli sources saml-list`** - SAMLSource Viewset
- **`authentik-pp-cli sources saml-metadata-retrieve`** - Return metadata as XML string
- **`authentik-pp-cli sources saml-partial-update`** - SAMLSource Viewset
- **`authentik-pp-cli sources saml-retrieve`** - SAMLSource Viewset
- **`authentik-pp-cli sources saml-update`** - SAMLSource Viewset
- **`authentik-pp-cli sources saml-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources scim-create`** - SCIMSource Viewset
- **`authentik-pp-cli sources scim-destroy`** - SCIMSource Viewset
- **`authentik-pp-cli sources scim-groups-create`** - SCIMSourceGroup Viewset
- **`authentik-pp-cli sources scim-groups-destroy`** - SCIMSourceGroup Viewset
- **`authentik-pp-cli sources scim-groups-list`** - SCIMSourceGroup Viewset
- **`authentik-pp-cli sources scim-groups-partial-update`** - SCIMSourceGroup Viewset
- **`authentik-pp-cli sources scim-groups-retrieve`** - SCIMSourceGroup Viewset
- **`authentik-pp-cli sources scim-groups-update`** - SCIMSourceGroup Viewset
- **`authentik-pp-cli sources scim-groups-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources scim-list`** - SCIMSource Viewset
- **`authentik-pp-cli sources scim-partial-update`** - SCIMSource Viewset
- **`authentik-pp-cli sources scim-retrieve`** - SCIMSource Viewset
- **`authentik-pp-cli sources scim-update`** - SCIMSource Viewset
- **`authentik-pp-cli sources scim-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources scim-users-create`** - SCIMSourceUser Viewset
- **`authentik-pp-cli sources scim-users-destroy`** - SCIMSourceUser Viewset
- **`authentik-pp-cli sources scim-users-list`** - SCIMSourceUser Viewset
- **`authentik-pp-cli sources scim-users-partial-update`** - SCIMSourceUser Viewset
- **`authentik-pp-cli sources scim-users-retrieve`** - SCIMSourceUser Viewset
- **`authentik-pp-cli sources scim-users-update`** - SCIMSourceUser Viewset
- **`authentik-pp-cli sources scim-users-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources telegram-create`** - Mixin to add a used_by endpoint to return a list of all objects using this object
- **`authentik-pp-cli sources telegram-destroy`** - Mixin to add a used_by endpoint to return a list of all objects using this object
- **`authentik-pp-cli sources telegram-list`** - Mixin to add a used_by endpoint to return a list of all objects using this object
- **`authentik-pp-cli sources telegram-partial-update`** - Mixin to add a used_by endpoint to return a list of all objects using this object
- **`authentik-pp-cli sources telegram-retrieve`** - Mixin to add a used_by endpoint to return a list of all objects using this object
- **`authentik-pp-cli sources telegram-update`** - Mixin to add a used_by endpoint to return a list of all objects using this object
- **`authentik-pp-cli sources telegram-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources user-connections-all-destroy`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-all-list`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-all-partial-update`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-all-retrieve`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-all-update`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-all-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources user-connections-kerberos-create`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-kerberos-destroy`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-kerberos-list`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-kerberos-partial-update`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-kerberos-retrieve`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-kerberos-update`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-kerberos-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources user-connections-ldap-create`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-ldap-destroy`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-ldap-list`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-ldap-partial-update`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-ldap-retrieve`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-ldap-update`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-ldap-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources user-connections-oauth-create`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-oauth-destroy`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-oauth-list`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-oauth-partial-update`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-oauth-retrieve`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-oauth-update`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-oauth-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources user-connections-plex-create`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-plex-destroy`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-plex-list`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-plex-partial-update`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-plex-retrieve`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-plex-update`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-plex-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources user-connections-saml-create`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-saml-destroy`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-saml-list`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-saml-partial-update`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-saml-retrieve`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-saml-update`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-saml-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli sources user-connections-telegram-create`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-telegram-destroy`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-telegram-list`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-telegram-partial-update`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-telegram-retrieve`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-telegram-update`** - User-source connection Viewset
- **`authentik-pp-cli sources user-connections-telegram-used-by-list`** - Get a list of all objects that use this object

### ssf

Manage ssf

- **`authentik-pp-cli ssf streams-list`** - SSFStream Viewset
- **`authentik-pp-cli ssf streams-retrieve`** - SSFStream Viewset

### stages

Manage stages

- **`authentik-pp-cli stages all-destroy`** - Stage Viewset
- **`authentik-pp-cli stages all-list`** - Stage Viewset
- **`authentik-pp-cli stages all-retrieve`** - Stage Viewset
- **`authentik-pp-cli stages all-types-list`** - Get all creatable types
- **`authentik-pp-cli stages all-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages all-user-settings-list`** - Get all stages the user can configure
- **`authentik-pp-cli stages authenticator-duo-create`** - AuthenticatorDuoStage Viewset
- **`authentik-pp-cli stages authenticator-duo-destroy`** - AuthenticatorDuoStage Viewset
- **`authentik-pp-cli stages authenticator-duo-enrollment-status-create`** - Check enrollment status of user details in current session
- **`authentik-pp-cli stages authenticator-duo-import-device-manual-create`** - Import duo devices into authentik
- **`authentik-pp-cli stages authenticator-duo-import-devices-automatic-create`** - Import duo devices into authentik
- **`authentik-pp-cli stages authenticator-duo-list`** - AuthenticatorDuoStage Viewset
- **`authentik-pp-cli stages authenticator-duo-partial-update`** - AuthenticatorDuoStage Viewset
- **`authentik-pp-cli stages authenticator-duo-retrieve`** - AuthenticatorDuoStage Viewset
- **`authentik-pp-cli stages authenticator-duo-update`** - AuthenticatorDuoStage Viewset
- **`authentik-pp-cli stages authenticator-duo-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages authenticator-email-create`** - AuthenticatorEmailStage Viewset
- **`authentik-pp-cli stages authenticator-email-destroy`** - AuthenticatorEmailStage Viewset
- **`authentik-pp-cli stages authenticator-email-list`** - AuthenticatorEmailStage Viewset
- **`authentik-pp-cli stages authenticator-email-partial-update`** - AuthenticatorEmailStage Viewset
- **`authentik-pp-cli stages authenticator-email-retrieve`** - AuthenticatorEmailStage Viewset
- **`authentik-pp-cli stages authenticator-email-update`** - AuthenticatorEmailStage Viewset
- **`authentik-pp-cli stages authenticator-email-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages authenticator-endpoint-gdtc-create`** - AuthenticatorEndpointGDTCStage Viewset
- **`authentik-pp-cli stages authenticator-endpoint-gdtc-destroy`** - AuthenticatorEndpointGDTCStage Viewset
- **`authentik-pp-cli stages authenticator-endpoint-gdtc-list`** - AuthenticatorEndpointGDTCStage Viewset
- **`authentik-pp-cli stages authenticator-endpoint-gdtc-partial-update`** - AuthenticatorEndpointGDTCStage Viewset
- **`authentik-pp-cli stages authenticator-endpoint-gdtc-retrieve`** - AuthenticatorEndpointGDTCStage Viewset
- **`authentik-pp-cli stages authenticator-endpoint-gdtc-update`** - AuthenticatorEndpointGDTCStage Viewset
- **`authentik-pp-cli stages authenticator-endpoint-gdtc-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages authenticator-sms-create`** - AuthenticatorSMSStage Viewset
- **`authentik-pp-cli stages authenticator-sms-destroy`** - AuthenticatorSMSStage Viewset
- **`authentik-pp-cli stages authenticator-sms-list`** - AuthenticatorSMSStage Viewset
- **`authentik-pp-cli stages authenticator-sms-partial-update`** - AuthenticatorSMSStage Viewset
- **`authentik-pp-cli stages authenticator-sms-retrieve`** - AuthenticatorSMSStage Viewset
- **`authentik-pp-cli stages authenticator-sms-update`** - AuthenticatorSMSStage Viewset
- **`authentik-pp-cli stages authenticator-sms-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages authenticator-static-create`** - AuthenticatorStaticStage Viewset
- **`authentik-pp-cli stages authenticator-static-destroy`** - AuthenticatorStaticStage Viewset
- **`authentik-pp-cli stages authenticator-static-list`** - AuthenticatorStaticStage Viewset
- **`authentik-pp-cli stages authenticator-static-partial-update`** - AuthenticatorStaticStage Viewset
- **`authentik-pp-cli stages authenticator-static-retrieve`** - AuthenticatorStaticStage Viewset
- **`authentik-pp-cli stages authenticator-static-update`** - AuthenticatorStaticStage Viewset
- **`authentik-pp-cli stages authenticator-static-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages authenticator-totp-create`** - AuthenticatorTOTPStage Viewset
- **`authentik-pp-cli stages authenticator-totp-destroy`** - AuthenticatorTOTPStage Viewset
- **`authentik-pp-cli stages authenticator-totp-list`** - AuthenticatorTOTPStage Viewset
- **`authentik-pp-cli stages authenticator-totp-partial-update`** - AuthenticatorTOTPStage Viewset
- **`authentik-pp-cli stages authenticator-totp-retrieve`** - AuthenticatorTOTPStage Viewset
- **`authentik-pp-cli stages authenticator-totp-update`** - AuthenticatorTOTPStage Viewset
- **`authentik-pp-cli stages authenticator-totp-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages authenticator-validate-create`** - AuthenticatorValidateStage Viewset
- **`authentik-pp-cli stages authenticator-validate-destroy`** - AuthenticatorValidateStage Viewset
- **`authentik-pp-cli stages authenticator-validate-list`** - AuthenticatorValidateStage Viewset
- **`authentik-pp-cli stages authenticator-validate-partial-update`** - AuthenticatorValidateStage Viewset
- **`authentik-pp-cli stages authenticator-validate-retrieve`** - AuthenticatorValidateStage Viewset
- **`authentik-pp-cli stages authenticator-validate-update`** - AuthenticatorValidateStage Viewset
- **`authentik-pp-cli stages authenticator-validate-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages authenticator-webauthn-create`** - AuthenticatorWebAuthnStage Viewset
- **`authentik-pp-cli stages authenticator-webauthn-destroy`** - AuthenticatorWebAuthnStage Viewset
- **`authentik-pp-cli stages authenticator-webauthn-device-types-list`** - WebAuthnDeviceType Viewset
- **`authentik-pp-cli stages authenticator-webauthn-device-types-retrieve`** - WebAuthnDeviceType Viewset
- **`authentik-pp-cli stages authenticator-webauthn-list`** - AuthenticatorWebAuthnStage Viewset
- **`authentik-pp-cli stages authenticator-webauthn-partial-update`** - AuthenticatorWebAuthnStage Viewset
- **`authentik-pp-cli stages authenticator-webauthn-retrieve`** - AuthenticatorWebAuthnStage Viewset
- **`authentik-pp-cli stages authenticator-webauthn-update`** - AuthenticatorWebAuthnStage Viewset
- **`authentik-pp-cli stages authenticator-webauthn-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages captcha-create`** - CaptchaStage Viewset
- **`authentik-pp-cli stages captcha-destroy`** - CaptchaStage Viewset
- **`authentik-pp-cli stages captcha-list`** - CaptchaStage Viewset
- **`authentik-pp-cli stages captcha-partial-update`** - CaptchaStage Viewset
- **`authentik-pp-cli stages captcha-retrieve`** - CaptchaStage Viewset
- **`authentik-pp-cli stages captcha-update`** - CaptchaStage Viewset
- **`authentik-pp-cli stages captcha-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages consent-create`** - ConsentStage Viewset
- **`authentik-pp-cli stages consent-destroy`** - ConsentStage Viewset
- **`authentik-pp-cli stages consent-list`** - ConsentStage Viewset
- **`authentik-pp-cli stages consent-partial-update`** - ConsentStage Viewset
- **`authentik-pp-cli stages consent-retrieve`** - ConsentStage Viewset
- **`authentik-pp-cli stages consent-update`** - ConsentStage Viewset
- **`authentik-pp-cli stages consent-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages deny-create`** - DenyStage Viewset
- **`authentik-pp-cli stages deny-destroy`** - DenyStage Viewset
- **`authentik-pp-cli stages deny-list`** - DenyStage Viewset
- **`authentik-pp-cli stages deny-partial-update`** - DenyStage Viewset
- **`authentik-pp-cli stages deny-retrieve`** - DenyStage Viewset
- **`authentik-pp-cli stages deny-update`** - DenyStage Viewset
- **`authentik-pp-cli stages deny-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages dummy-create`** - DummyStage Viewset
- **`authentik-pp-cli stages dummy-destroy`** - DummyStage Viewset
- **`authentik-pp-cli stages dummy-list`** - DummyStage Viewset
- **`authentik-pp-cli stages dummy-partial-update`** - DummyStage Viewset
- **`authentik-pp-cli stages dummy-retrieve`** - DummyStage Viewset
- **`authentik-pp-cli stages dummy-update`** - DummyStage Viewset
- **`authentik-pp-cli stages dummy-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages email-create`** - EmailStage Viewset
- **`authentik-pp-cli stages email-destroy`** - EmailStage Viewset
- **`authentik-pp-cli stages email-list`** - EmailStage Viewset
- **`authentik-pp-cli stages email-partial-update`** - EmailStage Viewset
- **`authentik-pp-cli stages email-retrieve`** - EmailStage Viewset
- **`authentik-pp-cli stages email-templates-list`** - Get all available templates, including custom templates
- **`authentik-pp-cli stages email-update`** - EmailStage Viewset
- **`authentik-pp-cli stages email-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages identification-create`** - IdentificationStage Viewset
- **`authentik-pp-cli stages identification-destroy`** - IdentificationStage Viewset
- **`authentik-pp-cli stages identification-list`** - IdentificationStage Viewset
- **`authentik-pp-cli stages identification-partial-update`** - IdentificationStage Viewset
- **`authentik-pp-cli stages identification-retrieve`** - IdentificationStage Viewset
- **`authentik-pp-cli stages identification-update`** - IdentificationStage Viewset
- **`authentik-pp-cli stages identification-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages invitation-create`** - InvitationStage Viewset
- **`authentik-pp-cli stages invitation-destroy`** - InvitationStage Viewset
- **`authentik-pp-cli stages invitation-invitations-create`** - Invitation Viewset
- **`authentik-pp-cli stages invitation-invitations-destroy`** - Invitation Viewset
- **`authentik-pp-cli stages invitation-invitations-list`** - Invitation Viewset
- **`authentik-pp-cli stages invitation-invitations-partial-update`** - Invitation Viewset
- **`authentik-pp-cli stages invitation-invitations-retrieve`** - Invitation Viewset
- **`authentik-pp-cli stages invitation-invitations-update`** - Invitation Viewset
- **`authentik-pp-cli stages invitation-invitations-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages invitation-list`** - InvitationStage Viewset
- **`authentik-pp-cli stages invitation-partial-update`** - InvitationStage Viewset
- **`authentik-pp-cli stages invitation-retrieve`** - InvitationStage Viewset
- **`authentik-pp-cli stages invitation-update`** - InvitationStage Viewset
- **`authentik-pp-cli stages invitation-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages mtls-create`** - MutualTLSStage Viewset
- **`authentik-pp-cli stages mtls-destroy`** - MutualTLSStage Viewset
- **`authentik-pp-cli stages mtls-list`** - MutualTLSStage Viewset
- **`authentik-pp-cli stages mtls-partial-update`** - MutualTLSStage Viewset
- **`authentik-pp-cli stages mtls-retrieve`** - MutualTLSStage Viewset
- **`authentik-pp-cli stages mtls-update`** - MutualTLSStage Viewset
- **`authentik-pp-cli stages mtls-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages password-create`** - PasswordStage Viewset
- **`authentik-pp-cli stages password-destroy`** - PasswordStage Viewset
- **`authentik-pp-cli stages password-list`** - PasswordStage Viewset
- **`authentik-pp-cli stages password-partial-update`** - PasswordStage Viewset
- **`authentik-pp-cli stages password-retrieve`** - PasswordStage Viewset
- **`authentik-pp-cli stages password-update`** - PasswordStage Viewset
- **`authentik-pp-cli stages password-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages prompt-create`** - PromptStage Viewset
- **`authentik-pp-cli stages prompt-destroy`** - PromptStage Viewset
- **`authentik-pp-cli stages prompt-list`** - PromptStage Viewset
- **`authentik-pp-cli stages prompt-partial-update`** - PromptStage Viewset
- **`authentik-pp-cli stages prompt-prompts-create`** - Prompt Viewset
- **`authentik-pp-cli stages prompt-prompts-destroy`** - Prompt Viewset
- **`authentik-pp-cli stages prompt-prompts-list`** - Prompt Viewset
- **`authentik-pp-cli stages prompt-prompts-partial-update`** - Prompt Viewset
- **`authentik-pp-cli stages prompt-prompts-preview-create`** - Preview a prompt as a challenge, just like a flow would receive
- **`authentik-pp-cli stages prompt-prompts-retrieve`** - Prompt Viewset
- **`authentik-pp-cli stages prompt-prompts-update`** - Prompt Viewset
- **`authentik-pp-cli stages prompt-prompts-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages prompt-retrieve`** - PromptStage Viewset
- **`authentik-pp-cli stages prompt-update`** - PromptStage Viewset
- **`authentik-pp-cli stages prompt-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages redirect-create`** - RedirectStage Viewset
- **`authentik-pp-cli stages redirect-destroy`** - RedirectStage Viewset
- **`authentik-pp-cli stages redirect-list`** - RedirectStage Viewset
- **`authentik-pp-cli stages redirect-partial-update`** - RedirectStage Viewset
- **`authentik-pp-cli stages redirect-retrieve`** - RedirectStage Viewset
- **`authentik-pp-cli stages redirect-update`** - RedirectStage Viewset
- **`authentik-pp-cli stages redirect-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages source-create`** - SourceStage Viewset
- **`authentik-pp-cli stages source-destroy`** - SourceStage Viewset
- **`authentik-pp-cli stages source-list`** - SourceStage Viewset
- **`authentik-pp-cli stages source-partial-update`** - SourceStage Viewset
- **`authentik-pp-cli stages source-retrieve`** - SourceStage Viewset
- **`authentik-pp-cli stages source-update`** - SourceStage Viewset
- **`authentik-pp-cli stages source-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages user-delete-create`** - UserDeleteStage Viewset
- **`authentik-pp-cli stages user-delete-destroy`** - UserDeleteStage Viewset
- **`authentik-pp-cli stages user-delete-list`** - UserDeleteStage Viewset
- **`authentik-pp-cli stages user-delete-partial-update`** - UserDeleteStage Viewset
- **`authentik-pp-cli stages user-delete-retrieve`** - UserDeleteStage Viewset
- **`authentik-pp-cli stages user-delete-update`** - UserDeleteStage Viewset
- **`authentik-pp-cli stages user-delete-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages user-login-create`** - UserLoginStage Viewset
- **`authentik-pp-cli stages user-login-destroy`** - UserLoginStage Viewset
- **`authentik-pp-cli stages user-login-list`** - UserLoginStage Viewset
- **`authentik-pp-cli stages user-login-partial-update`** - UserLoginStage Viewset
- **`authentik-pp-cli stages user-login-retrieve`** - UserLoginStage Viewset
- **`authentik-pp-cli stages user-login-update`** - UserLoginStage Viewset
- **`authentik-pp-cli stages user-login-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages user-logout-create`** - UserLogoutStage Viewset
- **`authentik-pp-cli stages user-logout-destroy`** - UserLogoutStage Viewset
- **`authentik-pp-cli stages user-logout-list`** - UserLogoutStage Viewset
- **`authentik-pp-cli stages user-logout-partial-update`** - UserLogoutStage Viewset
- **`authentik-pp-cli stages user-logout-retrieve`** - UserLogoutStage Viewset
- **`authentik-pp-cli stages user-logout-update`** - UserLogoutStage Viewset
- **`authentik-pp-cli stages user-logout-used-by-list`** - Get a list of all objects that use this object
- **`authentik-pp-cli stages user-write-create`** - UserWriteStage Viewset
- **`authentik-pp-cli stages user-write-destroy`** - UserWriteStage Viewset
- **`authentik-pp-cli stages user-write-list`** - UserWriteStage Viewset
- **`authentik-pp-cli stages user-write-partial-update`** - UserWriteStage Viewset
- **`authentik-pp-cli stages user-write-retrieve`** - UserWriteStage Viewset
- **`authentik-pp-cli stages user-write-update`** - UserWriteStage Viewset
- **`authentik-pp-cli stages user-write-used-by-list`** - Get a list of all objects that use this object

### tasks

Manage tasks

- **`authentik-pp-cli tasks list`** - List
- **`authentik-pp-cli tasks retrieve`** - Retrieve
- **`authentik-pp-cli tasks retry-create`** - Retry task
- **`authentik-pp-cli tasks schedules-list`** - Schedules list
- **`authentik-pp-cli tasks schedules-partial-update`** - Schedules partial update
- **`authentik-pp-cli tasks schedules-retrieve`** - Schedules retrieve
- **`authentik-pp-cli tasks schedules-send-create`** - Trigger this schedule now
- **`authentik-pp-cli tasks schedules-update`** - Schedules update
- **`authentik-pp-cli tasks status-retrieve`** - Global status summary for all tasks
- **`authentik-pp-cli tasks workers-list`** - Get currently connected worker count.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
authentik-pp-cli events list

# JSON for scripting and agents
authentik-pp-cli events list --json

# Filter to specific fields
authentik-pp-cli events list --json --select id,name,status

# Dry run — show the request without sending
authentik-pp-cli events list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
authentik-pp-cli events list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
authentik-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/authentik-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `AUTHENTIK_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `authentik-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $AUTHENTIK_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized** — Confirm AUTHENTIK_TOKEN is set and the token has admin scope: `echo $AUTHENTIK_TOKEN | head -c 8`
- **Connection refused** — Confirm AUTHENTIK_BASE_URL is reachable: `curl -I $AUTHENTIK_BASE_URL/-/health/live/`

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**goauthentik/terraform-provider-authentik**](https://github.com/goauthentik/terraform-provider-authentik) — Go (200 stars)
- [**goauthentik/client-python**](https://github.com/goauthentik/client-python) — Python (30 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
