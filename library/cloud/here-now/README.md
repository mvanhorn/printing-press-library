# here.now CLI

**Publish a folder to a live URL in one command, mirror your Drives offline, and never lose an anonymous site to the 24-hour clock.**

here.now lets agents publish static Sites to live URLs and keep private files in cloud Drives. This CLI is the missing layer on top: `publish dir ./site` orchestrates the whole inline-upload-finalize dance for you, `drives sync` pushes only what changed, and a local claim-token vault keeps your free-tier anonymous sites from silently expiring. Built free-plan-first — anonymous publishing needs no account, and paid-only analytics fails soft with a clear message instead of a raw error.

Learn more at [here.now](https://here.now).

Created by [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `here-now-pp-cli` binary and the `pp-here-now` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install here-now
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install here-now --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install here-now --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install here-now --agent claude-code
npx -y @mvanhorn/printing-press-library install here-now --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/here-now/cmd/here-now-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/here-now-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-here-now --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-here-now --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install here-now --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/here-now-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `HERENOW_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/here-now/cmd/here-now-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "here-now": {
      "command": "here-now-pp-mcp",
      "env": {
        "HERENOW_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Most onboarding is key-less: anonymous publish, public site reads, and Site Data writes work with no credential at all. When you want permanent sites and your own Drives, run `here-now-pp-cli auth login` to do the email-code flow (a one-time code is emailed, you paste it back) and the API key is stored locally, or set HERENOW_API_KEY in your environment. Drive share tokens also authenticate Drive-scoped reads.

## Quick Start

```bash
# Health check — confirms reachability and whether you're on the free or paid plan; works with no key.
here-now-pp-cli doctor --dry-run

# Preview an anonymous publish of a local folder; no account needed.
here-now-pp-cli publish dir ./site --anon --dry-run

# See the anonymous sites you've published and their claim tokens before they expire.
here-now-pp-cli claims --json

# Check how close you are to the free-plan limits.
here-now-pp-cli usage --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Anonymous publish lifecycle
- **`claims`** — Every anonymous publish records its slug, claim token, and 24h expiry into a local vault, so you can make an expiring site permanent later without hunting for the token in terminal scrollback.

  _Reach for this when an agent publishes anonymously and needs to keep the site — the token to do so exists nowhere else._

  ```bash
  here-now-pp-cli claims --json
  ```
- **`claims expiring`** — Lists anonymous sites expiring within a time window so you can claim them before they vanish.

  _Run this before anonymous sites silently expire at 24h to catch the ones worth keeping._

  ```bash
  here-now-pp-cli claims expiring --within 6h --json
  ```
- **`publish resume`** — Detects a locally-recorded publish that uploaded its files but never finalized, and completes it instead of re-publishing from scratch.

  _Use this when a publish half-failed (uploads done, finalize missed) to finish it without burning publish-rate budget on a redo._

  ```bash
  here-now-pp-cli publish resume my-site --dry-run
  ```

### Drive as a filesystem
- **`drives sync`** — Rsync-style sync of a local directory to a Drive: compares local sha256 against the synced file checksums and uploads only what changed, new, or deleted.

  _Reach for this instead of re-uploading a whole folder — it only sends drift, saving rate budget and time._

  ```bash
  here-now-pp-cli drives sync ./assets --drive drv_example --dry-run
  ```
- **`drives diff`** — Shows which local files differ from a Drive (added, changed, deleted) without uploading anything.

  _Use this to preview exactly what a sync would change before committing to the upload._

  ```bash
  here-now-pp-cli drives diff ./assets --drive drv_example --json
  ```

### Free-plan health
- **`usage`** — Local rollup of site count, drive bytes, and recent publish cadence against the free-plan ceilings (500 sites, 10GB, 60 publishes/hour).

  _Use this to stay under the free-plan caps without paying for analytics; it's the free user's only proactive limit signal._

  ```bash
  here-now-pp-cli usage --json
  ```
- **`sites stale`** — Lists sites not updated in N days from the local mirror, so you can reclaim free-plan slots by deleting dead ones.

  _Run this near the 500-site free cap to find which sites to prune._

  ```bash
  here-now-pp-cli sites stale --days 30 --json
  ```

## Recipes

### Publish a folder and keep it permanently

```bash
here-now-pp-cli publish dir ./site --anon && here-now-pp-cli claims expiring --within 24h
```

Publish anonymously for a live URL, then see the claim window so you can `claims redeem` before it expires.

### Sync a working folder to your Drive

```bash
here-now-pp-cli drives sync ./project --drive drv_example
```

Uploads only files that changed since the last sync, computed from local sha256 vs the mirror.

### Read form submissions from a site's collection

```bash
here-now-pp-cli publishes data list-site-records my-site signups --json --select id,data,createdAt
```

List a site's Site Data records with field selection so the agent's context stays small on deeply nested records.

### Find sites to prune before hitting the free cap

```bash
here-now-pp-cli sites stale --days 30 --json --select slug,url,updatedAt
```

Lists the least-recently-updated sites so you can reclaim free-plan slots.

### Check free-plan headroom

```bash
here-now-pp-cli usage --json
```

Local rollup of site count, drive bytes, and publish cadence against free-tier limits — no paid analytics required.

## Usage

Run `here-now-pp-cli --help` for the full command reference and flag list.

## Commands

### domains

Custom domains, subdomain handles, and links.

- **`here-now-pp-cli domains create`** - Add a custom domain
- **`here-now-pp-cli domains delete`** - Remove a custom domain
- **`here-now-pp-cli domains get`** - Get custom domain status
- **`here-now-pp-cli domains list`** - List custom domains

### drives

Private cloud storage for agent files.

- **`here-now-pp-cli drives create`** - Create a Drive
- **`here-now-pp-cli drives delete`** - Soft-delete a Drive
- **`here-now-pp-cli drives get`** - Get Drive details
- **`here-now-pp-cli drives get-default`** - Get or create the default Drive
- **`here-now-pp-cli drives list`** - List account Drives
- **`here-now-pp-cli drives patch`** - Patch Drive metadata

### handle

Manage handle

- **`here-now-pp-cli handle create`** - Create account subdomain handle
- **`here-now-pp-cli handle delete`** - Delete account subdomain handle
- **`here-now-pp-cli handle get`** - Get account subdomain handle
- **`here-now-pp-cli handle update`** - Update account subdomain handle

### here-now-analytics

Manage here.now analytics

- **`here-now-pp-cli here-now-analytics`** - Returns aggregate analytics across all Sites owned by the authenticated paid account.

### here-now-auth

Manage here.now auth

- **`here-now-pp-cli here-now-auth request-agent-code`** - Starts the agent-assisted API key flow by emailing a one-time code to the user.
- **`here-now-pp-cli here-now-auth verify-agent-code`** - Completes agent-assisted sign-in. If the email is new, the account is created.

### links

Manage links

- **`here-now-pp-cli links create`** - Create a link from a subdomain handle/domain path to a Site
- **`here-now-pp-cli links delete`** - Delete a link
- **`here-now-pp-cli links get`** - Get a link
- **`here-now-pp-cli links list`** - List subdomain handle or domain links
- **`here-now-pp-cli links update`** - Update a link

### me

Manage me

- **`here-now-pp-cli me delete-variable`** - Delete a service variable
- **`here-now-pp-cli me list-variables`** - List service variables
- **`here-now-pp-cli me set-variable`** - Create or update a service variable

### profile_resource

Manage profile resource

- **`here-now-pp-cli profile-resource add-profile-site`** - Shows an active owned Site on the authenticated user's public profile. Password-protected Sites cannot be shown on a profile. Adding a Site already on the profile is idempotent.
- **`here-now-pp-cli profile-resource get-profile`** - Returns the authenticated user's public profile settings and Sites shown on the profile.
- **`here-now-pp-cli profile-resource list-profile-sites`** - Lists the authenticated user's Sites currently shown on their public profile.
- **`here-now-pp-cli profile-resource patch-profile`** - Turns the public profile on or off and controls whether future Sites are added to the profile automatically.
- **`here-now-pp-cli profile-resource patch-profile-username`** - Changes the authenticated user's profile username and profile URL.
- **`here-now-pp-cli profile-resource remove-profile-site`** - Removes a Site from the authenticated user's public profile without deleting the Site.

### publish

Manage publish

- **`here-now-pp-cli publish create-site`** - Creates a pending Site version and returns presigned upload URLs. Anonymous requests are allowed and create temporary Sites that expire after 24 hours.
- **`here-now-pp-cli publish delete-site`** - Delete a Site
- **`here-now-pp-cli publish from-drive`** - Publish a Drive version as a Site
- **`here-now-pp-cli publish get-site`** - Get Site details
- **`here-now-pp-cli publish update-site`** - Creates a pending replacement version for an existing Site. Authenticated Sites require API key ownership. Anonymous Sites require claimToken.

### publishes

Manage publishes

- **`here-now-pp-cli publishes list-sites`** - List account Sites
- **`here-now-pp-cli publishes search-sites`** - Searches the authenticated user's active owned Sites by slug, URL/domain, viewer metadata, file path, and indexed text content. Password-protected Sites are included for the owner because search reads stored publish files, not public URLs.

### support

Authenticated support requests.

- **`here-now-pp-cli support`** - Send an authenticated support request

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
here-now-pp-cli domains list

# JSON for scripting and agents
here-now-pp-cli domains list --json

# Filter to specific fields
here-now-pp-cli domains list --json --select id,name,status

# Dry run — show the request without sending
here-now-pp-cli domains list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
here-now-pp-cli domains list --agent
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
here-now-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/here-now-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `HERENOW_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `here-now-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `here-now-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $HERENOW_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **analytics command returns a paid-plan error** — Analytics is a paid feature; on the free plan use `here-now-pp-cli usage` for local site/drive/publish-rate stats instead.
- **an anonymous site disappeared** — Anonymous sites expire after 24h. Run `here-now-pp-cli claims expiring --within 6h` regularly and `here-now-pp-cli sites claim <slug>` to make them permanent.
- **publish failed partway with files uploaded but no live URL** — Run `here-now-pp-cli publish resume <slug>` to finalize the existing upload instead of republishing.
- **presigned upload URL expired during a large publish** — Run `here-now-pp-cli sites refresh-uploads <slug>` to get fresh upload targets, then resume.
