# Dropbox CLI

**Find duplicates, sync conflicts, and stale shared links across a whole Dropbox, then clean up with plans you preview first.**

dropbox-pp-cli copies the metadata of every file in your Dropbox into a local SQLite index, so finding where the space went or which files are byte-identical takes seconds and downloads nothing. The cleanup commands write plan files that you check and preview before anything changes; apply runs them in Dropbox batch jobs and records each change in a journal that undo can reverse. File deletes are Dropbox soft deletes that undo can restore, revoking a shared link is permanent, and code folders like node_modules and .git are skipped unless you opt in.

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `dropbox-pp-cli` binary and the `pp-dropbox` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install dropbox
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install dropbox --cli-only
```

For the skill only, install it into the same agents as the default command above. This skips the CLI binary and can update or reinstall the skill:

```bash
npx -y @mvanhorn/printing-press-library install dropbox --skill-only
```

To constrain the skill install to specific agents, repeat `--agent` with names from the [`skills`](https://github.com/vercel-labs/skills) CLI:

```bash
npx -y @mvanhorn/printing-press-library install dropbox --agent claude-code
npx -y @mvanhorn/printing-press-library install dropbox --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/dropbox/cmd/dropbox-pp-cli@latest
```

This installs the CLI without the skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dropbox-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install dropbox --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-dropbox --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-dropbox --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install dropbox --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle for Claude Desktop installation without JSON configuration.

The bundle reuses your local OAuth tokens. Authenticate first if needed:

```bash
dropbox-pp-cli auth login
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dropbox-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/dropbox/cmd/dropbox-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "dropbox": {
      "command": "dropbox-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Dropbox uses OAuth with PKCE, so you only need an app key, not a secret. Create a free app at dropbox.com/developers/apps with Scoped access and Full Dropbox, add http://127.0.0.1:8085/callback as a redirect URI, and on the Permissions tab enable account_info.read, files.metadata.read and write, files.content.read and write, sharing.read and write, and file_requests.read and write, then click Submit. Then run `dropbox-pp-cli auth login --client-id <your app key>` (or set DROPBOX_CLIENT_ID) and approve in the browser. Do not pass --client-secret. Access tokens last four hours and refresh on their own. If you change permissions later, run auth login again, because existing tokens keep their old scopes.

Without a client ID flag, `DROPBOX_CLIENT_ID`, or a saved client ID, `auth login` prompts for the app key. `--no-input` and `--agent` disable that prompt.

## Quick Start

```bash
# Shows a no-network preview of the health check.
dropbox-pp-cli doctor --dry-run


# One-time browser sign-in with the app key from your Dropbox app.
dropbox-pp-cli auth login --client-id YOUR_APP_KEY


# Verifies credentials and Dropbox connectivity.
dropbox-pp-cli doctor


# Copies file metadata into the local index. The first run on an account with a few million entries can take close to an hour and resumes if interrupted; later runs only fetch changes.
dropbox-pp-cli index


# Shows where the space goes before you decide what to clean.
dropbox-pp-cli overview --agent


# Plans removal of sync-conflict copies that add nothing over their originals.
dropbox-pp-cli conflicts --plan conflicts.json


# Validates the plan against the current index.
dropbox-pp-cli plan check conflicts.json


# Previews the batches. Add --yes to run them; journal and undo cover the result.
dropbox-pp-cli apply conflicts.json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Find the mess

- **`conflicts`**: Find Dropbox conflicted copies and Selective Sync Conflict folders, compare each one to its original file by file, and plan removal only for copies that add nothing.

  _Use after device syncs leave conflicted copies or Selective Sync Conflict folders. Only identical copies and copies whose files all exist in the original go into the plan._

  ```bash
  dropbox-pp-cli conflicts --plan conflicts.json --agent
  ```
- **`links audit`**: List every shared link you own with its visibility and expiry, flag links to files that no longer exist, and plan bulk revokes.

  _Use for a privacy pass over old shared links. Nothing changes until apply runs with --yes, and revoked links cannot be restored._

  ```bash
  dropbox-pp-cli links audit --agent
  ```
- **`overview`**: See quota, space by top-level folder, file type, and year, real duplicate totals, conflict copies, and how much space code folders like node_modules take.

  _Run it first in any cleanup session, after index, to decide where to look. Duplicate and conflict totals skip code folders; pass --include-dev-dirs to the finder commands to include them._

  ```bash
  dropbox-pp-cli overview --agent
  ```
- **`mess`**: List empty folders, single-file folders, loose files at the root, 'Copy of' and '(1)' names, very deep paths, and near-duplicate folder names like Taxes and Taxes 2.

  _Use it to turn a vague 'clean this up' into a concrete list. Its plan only removes empty folders._

  ```bash
  dropbox-pp-cli mess --agent
  ```

### Change safely

- **`organize`**: Turn a glob and a destination template like /Photos/{year}/{month} into a move plan, with missing folders and name collisions worked out before anything moves.

  _Use when someone describes a filing rule. It writes a plan for plan check and apply instead of moving files directly. Dates use local time unless you pass --tz._

  ```bash
  dropbox-pp-cli organize --match '*.jpg' --under '/Camera Uploads' --to '/Photos/{year}/{month}' --plan photos.json
  ```
- **`apply`**: Preview a plan, then run it with --yes in Dropbox batch jobs of up to 1000 items that retry on lock contention and record every change in a local journal.

  _Run every cleanup plan through apply. Without --yes it only previews and lists any extra flags the plan needs (--allow-dev-dirs, --allow-cross-share, --allow-nonempty-delete, --allow-unshare). It refuses plans whose files changed since planning._

  ```bash
  dropbox-pp-cli apply photos.json
  ```
- **`plan check`**: Check a plan against the index before it runs: files that changed since planning, name collisions, case-only renames, code folders, and moves that would change who can see a file.

  _Run it on every plan, especially plans an agent wrote by hand. Exit code 2 means the plan has errors._

  ```bash
  dropbox-pp-cli plan check photos.json --agent
  ```
- **`undo`**: Reverse an applied batch: move files back and restore soft-deleted files from the revision recorded when they were deleted.

  _Use when an applied plan looks wrong. Restores work within Dropbox's restore window, at least 30 days on personal plans. Folders created by apply stay in place, and revoked links cannot be restored._

  ```bash
  dropbox-pp-cli undo 20261006-153012-abcd --yes
  ```
- **`journal`**: List applied batches with their results and how many days remain to restore deleted files.

  _Check it after apply to confirm results and to get the batch ID for undo._

  ```bash
  dropbox-pp-cli journal --agent
  ```

## Cookbook


### Find where the space goes

```bash
dropbox-pp-cli overview --agent --select top_folders,by_type,dev_dirs
```

Folder, file type, and code-folder breakdowns from the local index.

### Clean up sync conflict copies

```bash
dropbox-pp-cli conflicts --plan conflicts.json
```

Plans removal only for copies identical to their original or fully contained in it; everything else is listed for review.

### File Camera Uploads by month

```bash
dropbox-pp-cli organize --match '*.jpg' --under '/Camera Uploads' --to '/Photos/{year}/{month}' --plan photos.json
```

Builds a move plan from each file's client_modified date for plan check and apply.

### Review shared links

```bash
dropbox-pp-cli links audit --plan revoke.json --agent
```

Lists every shared link with visibility and expiry and plans revokes for links whose files are gone.

### Undo the last change

```bash
dropbox-pp-cli journal --agent --select batches.id,batches.created_at
```

Find the batch ID, then pass it to undo.

## Usage

Run `dropbox-pp-cli --help` for the full command reference and flag list.

## Safety model

- Run `index` before cleanup. In testing, a few million entries took close to an hour; interrupted runs resume, later runs fetch changes, and the local index can reach a few GB on very large accounts.
- Cleanup finders can write plans, and `plan check` validates them. `apply` and `undo` preview without `--yes`; executing either requires `--yes`.
- Finders skip development folders such as `node_modules`, `.git`, and virtual environments unless `--include-dev-dirs` is set. Applying changes there requires `--allow-dev-dirs`.
- In apply plans, cross-share moves require `--allow-cross-share`; deleting nonempty or shared folders requires `--allow-nonempty-delete` or `--allow-unshare`, respectively.
- `apply` and `undo` refuse to execute under the Printing Press test harness.
- File deletes are soft deletes. `undo` can restore them within Dropbox's restore window, at least 30 days on personal plans.
- Link revokes and `file-requests delete-all-closed` are permanent.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | Settings in `config.json` and saved profiles |
| `data` | Credentials in `credentials.toml`, the local `data.db` store, and `feedback.jsonl` |
| `state` | Local invocation records and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `DROPBOX_CONFIG_DIR`, `DROPBOX_DATA_DIR`, `DROPBOX_STATE_DIR`, or `DROPBOX_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `DROPBOX_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export DROPBOX_HOME=/srv/dropbox
dropbox-pp-cli doctor
```

Under `DROPBOX_HOME=/srv/dropbox`, the four dirs resolve to `/srv/dropbox/config`, `/srv/dropbox/data`, `/srv/dropbox/state`, and `/srv/dropbox/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "dropbox": {
      "command": "dropbox-pp-mcp",
      "env": {
        "DROPBOX_HOME": "/srv/dropbox"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `DROPBOX_DATA_DIR` overrides an explicit `--home` for that kind. Use `DROPBOX_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `DROPBOX_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave legacy `config.json` and are consolidated into `credentials.toml` under the data directory. Run `dropbox-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### Hand-written commands that ship

- `dropbox-pp-cli index`: Build or refresh the local metadata index.
- `dropbox-pp-cli overview`: Show space by folder, file type, and year.
- `dropbox-pp-cli tree [path]`: Show indexed folder sizes as a tree.
- `dropbox-pp-cli dupes`: Find byte-identical files and optionally write a cleanup plan.
- `dropbox-pp-cli conflicts`: Find conflict copies and optionally write a cleanup plan.
- `dropbox-pp-cli mess`: Find structural clutter and optionally write an empty-folder plan.
- `dropbox-pp-cli organize`: Build a rule-based move plan.
- `dropbox-pp-cli search <query>`: Search indexed names and paths offline.
- `dropbox-pp-cli plan check <file>`: Validate a plan against the index.
- `dropbox-pp-cli apply <file>`: Preview a plan; add `--yes` to execute it and journal results.
- `dropbox-pp-cli undo <batch-id>`: Preview an undo; add `--yes` to execute it.
- `dropbox-pp-cli journal [batch-id]`: List batches or inspect one batch.
- `dropbox-pp-cli links audit`: Audit shared links and optionally write a revoke plan.
- `dropbox-pp-cli files download <path> --output <file|->`: Download to a local file or stdout.
- `dropbox-pp-cli files upload <local-file> <dropbox-path>`: Upload a file up to 150 MiB. Upload sessions are unavailable.

### file-requests

Inspect and manage file requests.

- **`dropbox-pp-cli file-requests count`**: Return the total number of open and closed file requests.
- **`dropbox-pp-cli file-requests delete-all-closed`**: Delete all closed file requests owned by the current user.
- **`dropbox-pp-cli file-requests get`**: Return a specified file request.
- **`dropbox-pp-cli file-requests list`**: Return file requests owned by the current user.
- **`dropbox-pp-cli file-requests list-continue`**: Continue paginating file requests using a cursor.

### files

Inspect and manage Dropbox files and folders.

- **`dropbox-pp-cli files copy`**: Copy a file or folder to another location in the user's Dropbox.
- **`dropbox-pp-cli files copy-batch`**: Copy multiple files or folders to different locations in the user's Dropbox.
- **`dropbox-pp-cli files copy-batch-check`**: Return the status and per-entry results of an asynchronous copy batch.
- **`dropbox-pp-cli files create-folder`**: Create a folder at a given path.
- **`dropbox-pp-cli files create-folder-batch`**: Create multiple folders at once.
- **`dropbox-pp-cli files create-folder-batch-check`**: Return the status of an asynchronous create-folder batch.
- **`dropbox-pp-cli files delete`**: Delete a file or folder at a given path.
- **`dropbox-pp-cli files delete-batch`**: Delete multiple files or folders at once.
- **`dropbox-pp-cli files delete-batch-check`**: Return the status and per-entry results of an asynchronous delete batch.
- **`dropbox-pp-cli files get-latest-cursor`**: Get a cursor for the current folder state without returning entries.
- **`dropbox-pp-cli files get-metadata`**: Return metadata for a file or folder.
- **`dropbox-pp-cli files get-temporary-link`**: Get a temporary file content link that expires in four hours.
- **`dropbox-pp-cli files get-thumbnail-batch`**: Get thumbnails for up to 25 images.
- **`dropbox-pp-cli files list-folder`**: Start returning the contents of a folder.
- **`dropbox-pp-cli files list-folder-continue`**: Continue listing a folder or retrieve changes using its cursor.
- **`dropbox-pp-cli files list-revisions`**: Return revisions for a file path or file ID.
- **`dropbox-pp-cli files move`**: Move a file or folder to another location in the user's Dropbox.
- **`dropbox-pp-cli files move-batch`**: Move multiple files or folders to different locations in the user's Dropbox.
- **`dropbox-pp-cli files move-batch-check`**: Return the status and per-entry results of an asynchronous move batch.
- **`dropbox-pp-cli files restore`**: Restore a specific revision of a file to the given path.
- **`dropbox-pp-cli files search`**: Search for files and folders.
- **`dropbox-pp-cli files search-continue`**: Fetch the next page of search results.
- **`dropbox-pp-cli files tags-add`**: Add a tag to an item.
- **`dropbox-pp-cli files tags-get`**: Get tags assigned to items.
- **`dropbox-pp-cli files tags-remove`**: Remove a tag from an item.

### sharing

Inspect and manage shared links and shared folders.

- **`dropbox-pp-cli sharing create-shared-link`**: Create a shared link with custom settings.
- **`dropbox-pp-cli sharing get-shared-link-metadata`**: Get metadata for a shared link.
- **`dropbox-pp-cli sharing list-folder-members`**: Return shared folder membership by folder ID.
- **`dropbox-pp-cli sharing list-folders`**: Return shared folders accessible to the current user.
- **`dropbox-pp-cli sharing list-folders-continue`**: Continue paginating shared folders using a cursor.
- **`dropbox-pp-cli sharing list-received-files`**: Return files shared with the current user.
- **`dropbox-pp-cli sharing list-shared-links`**: List shared links owned by the current user.
- **`dropbox-pp-cli sharing modify-shared-link-settings`**: Modify the settings of an existing shared link.
- **`dropbox-pp-cli sharing revoke-shared-link`**: Revoke a shared link.

### users

Read account and space information.

- **`dropbox-pp-cli users get-current-account`**: Get information about the current user's account.
- **`dropbox-pp-cli users get-space-usage`**: Get space usage for the current user's account.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`dropbox-pp-cli recall <query>`**: Look up cached resources for a query before running discovery
- **`dropbox-pp-cli teach`**: Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`dropbox-pp-cli learnings list`**: Inspect taught rows
- **`dropbox-pp-cli learnings forget <query>`**: Undo a teach
- **`dropbox-pp-cli learnings candidates`**: List auto-captured candidates awaiting confirm/reject
- **`dropbox-pp-cli learnings stats`**: Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`dropbox-pp-cli teach-pattern`**: Install a query/resource template up front
- **`dropbox-pp-cli teach-lookup`**: Add an entity alias, such as a folder nickname, for pattern substitution

Pass `--no-learn` or set `DROPBOX_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way. Once this version of `dropbox-pp-cli` opens the database, older binaries refuse it with a version error. Upgrade the binary before using that store again.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
dropbox-pp-cli file-requests list

# JSON for scripting and agents
dropbox-pp-cli file-requests list --json
# Filter to specific fields by name
dropbox-pp-cli overview --json --select top_folders,by_type

# Dry run: show the HTTP request without sending it
dropbox-pp-cli file-requests list --dry-run

# Agent mode: JSON, compact output, and no prompts
dropbox-pp-cli file-requests list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Prompt control**: `--no-input` and `--agent` disable prompts. `auth login` can prompt for a missing client ID in interactive mode
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable**: `--select top_folders,by_type` on `overview` returns those fields
- **Previewable**: `--dry-run` shows the HTTP request for endpoint commands; run `apply` or `undo` without `--yes` to preview its operations
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success; `1` for `apply` or `undo` when any operation failed or ended unknown; `2` for usage errors and `plan check` errors; `3` not found; `4` auth error; `5` API error; `6` partial failure; `7` rate limited; `10` config error.

## Health Check

```bash
dropbox-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `dropbox-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/dropbox-pp-cli/config.json`; `--home`, `DROPBOX_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting

**Authentication errors (exit code 4)**

- Run `dropbox-pp-cli doctor` to check credentials.

**Not found errors (exit code 3)**

- Check the resource ID.
- Use the relevant list command to find available items.

### API-specific

- **auth login says an OAuth2 client ID is required**: Pass your Dropbox app key with `dropbox-pp-cli auth login --client-id YOUR_APP_KEY`, or set `DROPBOX_CLIENT_ID`.
- **Error says the app or token lacks a scope such as files.metadata.read**: Enable the scope on the App Console Permissions tab, click Submit, then run `dropbox-pp-cli auth login` again.
- **auth login fails with redirect_uri mismatch**: Add http://127.0.0.1:8085/callback to the app's Redirect URIs, or pass --port to match one you registered.
- **First index takes a long time**: It is a one-time crawl (close to an hour for a few million entries in testing) that resumes if interrupted. Later runs of `dropbox-pp-cli index` only fetch changes.
- **apply refuses an op with rev_mismatch**: Files changed after the plan was written. Run `dropbox-pp-cli index`, then rebuild the plan.
- **apply refuses delete_nonempty_folder**: Review the folder's indexed descendants. Use `--allow-nonempty-delete` only if deleting them is intended.
- **apply stops and says to check the journal**: Do not re-apply. Run `dropbox-pp-cli index`, then `dropbox-pp-cli journal <batch-id> --ops` to see which operations finished.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Dropbox-Uploader**](https://github.com/andreafabrizi/Dropbox-Uploader): Shell
- [**dbxcli**](https://github.com/dropbox/dbxcli): Go
- [**dbx-mcp-server**](https://github.com/amgadabdelhafez/dbx-mcp-server): TypeScript
- [**ngs/dropbox-mcp-server**](https://github.com/ngs/dropbox-mcp-server): Go
- [**rclone**](https://github.com/rclone/rclone): Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
