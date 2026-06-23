---
name: pp-plane
description: "Printing Press CLI for Plane. The Plane REST API Visit our quick start guide and full API documentation at [developers.plane.so](https://developers."
author: "Anton Sidorov aka anticodeguy"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - plane-pp-cli
    install:
      - kind: go
        bins: [plane-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/project-management/plane/cmd/plane-pp-cli
---

# Plane — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `plane-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill** (`plane-pp-cli --version`). If it is missing, install it by ONE of the paths below. Pick the first one that fits your environment.

**1. Pre-built binary — no toolchain (best for agents, CI, and sandboxes).** Needs neither Node nor Go. Download the asset for your OS/arch from the rolling `plane-current` release and put it on `$PATH`:

```bash
# Linux x86_64 — for other platforms swap the asset name:
#   plane-pp-cli-{linux,darwin}-{amd64,arm64} | plane-pp-cli-windows-{amd64,arm64}.exe
curl -fsSL -o plane-pp-cli \
  https://github.com/mvanhorn/printing-press-library/releases/download/plane-current/plane-pp-cli-linux-amd64
chmod +x plane-pp-cli && sudo mv plane-pp-cli /usr/local/bin/   # or any dir on $PATH
```

On macOS also clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine plane-pp-cli`.

**2. Full install (CLI + this skill in one shot) — needs Node AND Go.** The `npx` installer shells into `go install` under the hood, so a Go toolchain (1.26.3+) must already be present; it does **not** download the pre-built binary. Binaries default to `$HOME/.local/bin` (macOS/Linux) or `%LOCALAPPDATA%\Programs\PrintingPress\bin` (Windows):

```bash
npx -y @mvanhorn/printing-press-library install plane --cli-only
```

**3. Direct Go install — no Node, but needs Go 1.26.4 or newer:**

```bash
go install github.com/mvanhorn/printing-press-library/library/project-management/plane/cmd/plane-pp-cli@latest
```

After any path, confirm with `plane-pp-cli --version` and ensure the install directory is on `$PATH` for the agent/runtime that will invoke this skill. If `--version` reports "command not found", the runtime cannot see the binary directory on `$PATH` — fix that before proceeding with skill commands.

## Command Reference

**assets** — **File Upload & Presigned URLs**

Generate presigned URLs for direct file uploads to cloud storage. Handle user avatars, cover images, and generic project assets with secure upload workflows.

*Key Features:*
- Generate presigned URLs for S3 uploads
- Support for user avatars and cover images
- Generic asset upload for projects
- File validation and size limits

*Use Cases:* User profile images, project file uploads, secure direct-to-cloud uploads.

- `plane-pp-cli assets create-generic-upload` — Generate presigned URL for generic asset upload
- `plane-pp-cli assets create-user-upload` — Generate presigned URL for user asset upload
- `plane-pp-cli assets delete-user` — Delete user asset. Delete a user profile asset (avatar or cover image) and remove its reference from the user profile.
- `plane-pp-cli assets get-generic` — Get presigned URL for asset download
- `plane-pp-cli assets update-generic` — Update generic asset after upload completion
- `plane-pp-cli assets update-user` — Mark user asset as uploaded

**invitations** — Manage invitations

- `plane-pp-cli invitations workspaces-create` — Create a workspace invite
- `plane-pp-cli invitations workspaces-destroy` — Delete a workspace invite
- `plane-pp-cli invitations workspaces-list` — List all workspace invites for a workspace
- `plane-pp-cli invitations workspaces-partial-update` — Update a workspace invite
- `plane-pp-cli invitations workspaces-retrieve` — Get a workspace invite by ID

**issues** — Manage issues

- `plane-pp-cli issues get-workspace-work-item` — Retrieve a specific work item using workspace slug, project identifier, and issue identifier.
- `plane-pp-cli issues search-work-items` — Perform semantic search across issue names, sequence IDs, and project identifiers.

**members** — **Team Member Management**

Manage team members, roles, and permissions within projects and workspaces. Control access levels and track member participation.

*Key Features:*
- Invite and manage team members
- Assign roles and permissions
- Control project and workspace access
- Track member activity and participation

*Use Cases:* Team setup, access control, role management, collaboration.

- `plane-pp-cli members` — Retrieve all users who are members of the specified workspace.

**projects** — **Project Management**

Create and manage projects to organize your development work. Configure project settings, manage team access, and control project visibility.

*Key Features:*
- Create, update, and delete projects
- Configure project settings and preferences
- Manage team access and permissions
- Control project visibility and sharing

*Use Cases:* Project setup, team collaboration, access control, project configuration.

- `plane-pp-cli projects create` — Create a new project in the workspace with default states and member assignments.
- `plane-pp-cli projects delete` — Permanently remove a project and all its associated data from the workspace.
- `plane-pp-cli projects list` — Retrieve all projects in a workspace or get details of a specific project.
- `plane-pp-cli projects retrieve` — Retrieve details of a specific project.
- `plane-pp-cli projects update` — Partially update an existing project's properties like name, description, or settings.

**stickies** — Manage stickies

- `plane-pp-cli stickies create-sticky` — Create a new sticky in the workspace
- `plane-pp-cli stickies delete-sticky` — Delete a sticky by its ID
- `plane-pp-cli stickies list` — List all stickies in the workspace
- `plane-pp-cli stickies retrieve-sticky` — Retrieve a sticky by its ID
- `plane-pp-cli stickies update-sticky` — Update a sticky by its ID

**users** — **Current User Information**

Get information about the currently authenticated user including profile details and account settings.

*Key Features:*
- Retrieve current user profile
- Access user account information
- View user preferences and settings
- Get authentication context

*Use Cases:* Profile display, user context, account information, authentication status.

- `plane-pp-cli users` — Retrieve the authenticated user's profile information including basic details.

**work-items** — **Work Items & Tasks**

Create and manage work items like tasks, bugs, features, and user stories. The core entities for tracking work in your projects.

*Key Features:*
- Create, update, and manage work items
- Assign to team members and set priorities
- Track progress through workflow states
- Set due dates, estimates, and relationships

*Use Cases:* Bug tracking, task management, feature development, sprint planning.

- `plane-pp-cli work-items get-workspace-2` — Retrieve a specific work item using workspace slug, project identifier, and issue identifier.
- `plane-pp-cli work-items search-2` — Perform semantic search across issue names, sequence IDs, and project identifiers.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
plane-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `plane-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export PLANE_API_KEY_AUTHENTICATION="<your-key>"
```

Or persist it in `~/.config/plane-pp-cli/config.toml`.

Run `plane-pp-cli doctor` to verify setup.

## Workspace targeting

Plane's REST API is workspace-scoped: every request goes to `…/api/v1/workspaces/<slug>/…`. The public API **cannot enumerate** a user's workspaces from an API key, so the slug is user-supplied — take it from the browser URL (`app.plane.so/<slug>/`). Keep `base_url` templated as `https://<host>/api/v1/workspaces/{slug}` (literal `{slug}`); do **not** bake a concrete slug into it (that pins the CLI to one workspace and is flagged by `doctor`).

The active workspace is chosen by precedence: **`--workspace <slug>` flag > `PLANE_SLUG` env > `default_workspace` (config)**.

> **Hitting an unexpected `403`/empty result?** Run `echo $PLANE_SLUG` first. A process-wide `PLANE_SLUG` exported in your shell **silently overrides** `default_workspace`, so commands target that workspace and items in any other one come back as `403 "You do not have permission"` — a wrong-workspace symptom masquerading as a permissions problem, not a key issue. Either unset it, set it per-project (e.g. `PLANE_SLUG=<slug>` in the project's env), or pass `--workspace <slug>` explicitly (the flag always wins). `plane-pp-cli workspaces current` shows the active slug **and where it was resolved from**.

```bash
# One-time onboarding: probe + enroll your slug(s), write a templated base_url
plane-pp-cli init --host https://api.plane.so acme bravo --default acme

# Or manage the local registry directly
plane-pp-cli workspaces add acme bravo   # access-probes each before saving
plane-pp-cli workspaces use acme         # probe + set as the default
plane-pp-cli workspaces list             # show enrolled workspaces (the API can't list them for you)
plane-pp-cli workspaces current          # show active slug + where it was resolved from

# Target a specific workspace for one command (overrides env + default)
plane-pp-cli members --workspace bravo --agent --select display_name
```

Enrollment is local-only (a `[[workspaces]]` registry in `config.toml`) because the API can't enumerate workspaces by key. A bad slug fails loudly (the probe rejects it with a non-zero exit) rather than silently returning the wrong workspace.

**Via the MCP server:** the `plane_execute` tool takes an optional top-level `workspace` argument — the MCP twin of `--workspace`, same top precedence over `PLANE_SLUG`/`default_workspace`. Pass it to target one call at a specific workspace; omit it to use the configured default. (Without it the server resolves the slug from `PLANE_SLUG`/`default_workspace` at load time, so the same `403`-as-wrong-workspace trap above applies to MCP calls.)

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  plane-pp-cli projects list --agent --select id,name,status
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
plane-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
plane-pp-cli feedback --stdin < notes.txt
plane-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/plane-pp-cli/feedback.jsonl`. They are never POSTed unless `PLANE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PLANE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
plane-pp-cli profile save briefing --json
plane-pp-cli --profile briefing projects list
plane-pp-cli profile list --json
plane-pp-cli profile show briefing
plane-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `plane-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/project-management/plane/cmd/plane-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add plane-pp-mcp -- plane-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which plane-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   plane-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `plane-pp-cli <command> --help`.
