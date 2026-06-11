# Plane CLI

A terminal CLI for [Plane](https://plane.so) — open-source project management for issues, cycles, modules, sub-issues, and workspaces; a self-hostable Jira / Linear / ClickUp alternative. Ships with offline SQLite sync for instant search and analytics, plus an MCP server (`plane-pp-mcp`) so agents can drive Plane directly.

Pre-built binaries for Linux, macOS, and Windows ship in the [`plane-current`](https://github.com/mvanhorn/printing-press-library/releases/tag/plane-current) release, so you can install with no Go or Node toolchain — see [Install](#install).

Visit the quick start guide and full API documentation at [developers.plane.so](https://developers.plane.so/api-reference/introduction).

Learn more at [Plane](https://plane.so).

Created by [@sidorovanthon](https://github.com/sidorovanthon) (Anton Sidorov aka anticodeguy).

## Install

The recommended path installs both the `plane-pp-cli` binary and the `pp-plane` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install plane
```

> **This path needs Go, not just Node.** The installer shells into `go install` under the hood, so a Go toolchain (1.26.3+) must be present — it does not download a pre-built binary. If you have no toolchain (an agent, CI, a sandbox), use the [pre-built binary](#pre-built-binary-no-node-no-go) below instead.

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install plane --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install plane --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install plane --agent claude-code
npx -y @mvanhorn/printing-press-library install plane --agent claude-code --agent codex
```

### Pre-built binary (no Node, no Go)

The simplest install when you have no toolchain — also the right path for agents, CI, and sandboxes. Download the asset for your platform from the rolling [`plane-current` release](https://github.com/mvanhorn/printing-press-library/releases/tag/plane-current) and put it on your `$PATH`. Assets are rebuilt on every push to `main`, so the URLs are stable:

```
plane-pp-cli-linux-amd64     plane-pp-cli-darwin-amd64     plane-pp-cli-windows-amd64.exe
plane-pp-cli-linux-arm64     plane-pp-cli-darwin-arm64     plane-pp-cli-windows-arm64.exe
```

```bash
# Example: Linux x86_64 (swap the asset name for your OS/arch)
curl -fsSL -o plane-pp-cli \
  https://github.com/mvanhorn/printing-press-library/releases/download/plane-current/plane-pp-cli-linux-amd64
chmod +x plane-pp-cli && sudo mv plane-pp-cli /usr/local/bin/   # or any dir on $PATH
plane-pp-cli --version
```

On macOS, also clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine plane-pp-cli`. This installs the CLI only — no skill.

### Direct Go install (no Node, needs Go)

If you have a Go toolchain (1.26.3+) but no Node, install the CLI directly — no skill:

```bash
go install github.com/mvanhorn/printing-press-library/library/project-management/plane/cmd/plane-pp-cli@latest
```

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install plane --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-plane --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-plane --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install plane --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/plane-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PLANE_API_KEY_AUTHENTICATION` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/project-management/plane/cmd/plane-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "plane": {
      "command": "plane-pp-mcp",
      "env": {
        "PLANE_SLUG": "<slug>",
        "PLANE_API_KEY_AUTHENTICATION": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Set the endpoint variables for the tenant, workspace, or API version you want this CLI to use:

```bash
export PLANE_SLUG="<slug>"
```

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export PLANE_API_KEY_AUTHENTICATION="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/plane-pp-cli/config.toml`.

### 3. Verify Setup

```bash
plane-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
plane-pp-cli projects list
```

## Usage

Run `plane-pp-cli --help` for the full command reference and flag list.

## Commands

### assets

**File Upload & Presigned URLs**

Generate presigned URLs for direct file uploads to cloud storage. Handle user avatars, cover images, and generic project assets with secure upload workflows.

*Key Features:*
- Generate presigned URLs for S3 uploads
- Support for user avatars and cover images
- Generic asset upload for projects
- File validation and size limits

*Use Cases:* User profile images, project file uploads, secure direct-to-cloud uploads.

- **`plane-pp-cli assets create-generic-upload`** - Generate presigned URL for generic asset upload
- **`plane-pp-cli assets create-user-upload`** - Generate presigned URL for user asset upload
- **`plane-pp-cli assets delete-user`** - Delete user asset.

Delete a user profile asset (avatar or cover image) and remove its reference from the user profile.
This performs a soft delete by marking the asset as deleted and updating the user's profile.
- **`plane-pp-cli assets get-generic`** - Get presigned URL for asset download
- **`plane-pp-cli assets update-generic`** - Update generic asset after upload completion
- **`plane-pp-cli assets update-user`** - Mark user asset as uploaded

### invitations

Manage invitations

- **`plane-pp-cli invitations workspaces-create`** - Create a workspace invite
- **`plane-pp-cli invitations workspaces-destroy`** - Delete a workspace invite
- **`plane-pp-cli invitations workspaces-list`** - List all workspace invites for a workspace
- **`plane-pp-cli invitations workspaces-partial-update`** - Update a workspace invite
- **`plane-pp-cli invitations workspaces-retrieve`** - Get a workspace invite by ID

### issues

Manage issues

- **`plane-pp-cli issues get-workspace-work-item`** - Retrieve a specific work item using workspace slug, project identifier, and issue identifier.
- **`plane-pp-cli issues search-work-items`** - Perform semantic search across issue names, sequence IDs, and project identifiers.

### members

**Team Member Management**

Manage team members, roles, and permissions within projects and workspaces. Control access levels and track member participation.

*Key Features:*
- Invite and manage team members
- Assign roles and permissions
- Control project and workspace access
- Track member activity and participation

*Use Cases:* Team setup, access control, role management, collaboration.

- **`plane-pp-cli members`** - Retrieve all users who are members of the specified workspace.

### projects

**Project Management**

Create and manage projects to organize your development work. Configure project settings, manage team access, and control project visibility.

*Key Features:*
- Create, update, and delete projects
- Configure project settings and preferences
- Manage team access and permissions
- Control project visibility and sharing

*Use Cases:* Project setup, team collaboration, access control, project configuration.

- **`plane-pp-cli projects create`** - Create a new project in the workspace with default states and member assignments.
- **`plane-pp-cli projects delete`** - Permanently remove a project and all its associated data from the workspace.
- **`plane-pp-cli projects list`** - Retrieve all projects in a workspace or get details of a specific project.
- **`plane-pp-cli projects retrieve`** - Retrieve details of a specific project.
- **`plane-pp-cli projects update`** - Partially update an existing project's properties like name, description, or settings.

### stickies

Manage stickies

- **`plane-pp-cli stickies create-sticky`** - Create a new sticky in the workspace
- **`plane-pp-cli stickies delete-sticky`** - Delete a sticky by its ID
- **`plane-pp-cli stickies list`** - List all stickies in the workspace
- **`plane-pp-cli stickies retrieve-sticky`** - Retrieve a sticky by its ID
- **`plane-pp-cli stickies update-sticky`** - Update a sticky by its ID

### users

**Current User Information**

Get information about the currently authenticated user including profile details and account settings.

*Key Features:*
- Retrieve current user profile
- Access user account information
- View user preferences and settings
- Get authentication context

*Use Cases:* Profile display, user context, account information, authentication status.

- **`plane-pp-cli users`** - Retrieve the authenticated user's profile information including basic details.

### work-items

**Work Items & Tasks**

Create and manage work items like tasks, bugs, features, and user stories. The core entities for tracking work in your projects.

*Key Features:*
- Create, update, and manage work items
- Assign to team members and set priorities
- Track progress through workflow states
- Set due dates, estimates, and relationships

*Use Cases:* Bug tracking, task management, feature development, sprint planning.

- **`plane-pp-cli work-items get-workspace-2`** - Retrieve a specific work item using workspace slug, project identifier, and issue identifier.
- **`plane-pp-cli work-items search-2`** - Perform semantic search across issue names, sequence IDs, and project identifiers.

### relations

**Issue Relations** *(novel command)*

Manage relationships between work items — list, create, and remove blocking, blocked_by, duplicate, relates_to, and temporal (start/finish before/after) links without hand-driving the relations endpoint.

- **`plane-pp-cli relations list <issue> --project <uuid>`** - List all relations for a work item, grouped by relation type.
- **`plane-pp-cli relations set <issue> --project <uuid> --type blocked_by --related <uuid>`** - Create a relation; Plane stores the inverse automatically.
- **`plane-pp-cli relations unset <issue> --project <uuid> --type <type> --related <uuid>`** - Remove a relation.

### module

**Module Membership & Sync Enrichment** *(novel command)*

Plane's issue API never returns module membership, so a plain `sync` leaves `module_ids` null. These commands surface and manage it; the enrichment also runs automatically at the tail of `sync`.

- **`plane-pp-cli module sync`** - Walk modules → module-issues, populate a junction table, and patch each issue's `module_ids` (also runs automatically inside `sync`).
- **`plane-pp-cli module of <issue>`** - Show which modules an issue belongs to (from the local cache).
- **`plane-pp-cli module create-issue <module> <project> <slug> --name "..."`** - Create a work item and add it to a module in one step. The positional `<slug>` is overridden by the global `--workspace` flag when both are given.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
plane-pp-cli projects list

# JSON for scripting and agents
plane-pp-cli projects list --json

# Filter to specific fields
plane-pp-cli projects list --json --select id,name,status

# Dry run — show the request without sending
plane-pp-cli projects list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
plane-pp-cli projects list --agent
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

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `PLANE_SLUG` resolves `{slug}`

Base URL: `https://api.plane.so/api/v1/workspaces/{slug}`

## Health Check

```bash
plane-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/plane-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PLANE_SLUG` | endpoint | Yes |  |
| `PLANE_API_KEY_AUTHENTICATION` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `plane-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `plane-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PLANE_API_KEY_AUTHENTICATION`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
