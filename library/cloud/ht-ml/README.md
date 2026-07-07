# ht-ml.app CLI

**The only ht-ml.app tool that remembers what you published, a local registry of every site, its once-only update_key, and full version history, plus one-command publish-with-assets.**

ht-ml.app is deliberately accountless: you POST one HTML document, get a public URL, and a write key shown exactly once. That makes publishing frictionless and management impossible. This CLI is the missing memory layer. Every publish is captured to a local SQLite store, so you can list and audit everything you've shipped, update a site by id without ever touching its key, auto-upload referenced assets in one pass, roll back to any prior version, and export a passphrase-sealed vault of your keys for disaster recovery.

## Install

The recommended path installs both the `ht-ml-pp-cli` binary and the `pp-ht-ml` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ht-ml
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ht-ml --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ht-ml --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ht-ml --agent claude-code
npx -y @mvanhorn/printing-press-library install ht-ml --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/cmd/ht-ml-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ht-ml-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ht-ml --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ht-ml --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ht-ml --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ht-ml --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ht-ml-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/cmd/ht-ml-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ht-ml": {
      "command": "ht-ml-pp-mcp"
    }
  }
}
```

</details>

## Authentication

ht-ml.app has no global API key. Creating and reading sites needs no credential at all. Each site mints a high-entropy update_key once at creation — the only write credential, with no recovery endpoint. This CLI captures that key into a local store at create time, so update, asset, and password commands resolve it automatically by site_id. Keep the store safe and back it up with `keys export`; losing an update_key with no backup orphans the site forever.

## Quick Start

```bash
# health check; works with no setup or credentials
ht-ml-pp-cli doctor --dry-run

# publish a local HTML file; prints the public URL and stores the site + key
ht-ml-pp-cli publish ./deck.html

# publish and auto-upload every referenced image/video in one pass
ht-ml-pp-cli publish ./report.html --assets

# list every site you've published (the inventory ht-ml.app can't give you)
ht-ml-pp-cli list

# replace a site's HTML by id; the update_key is resolved for you
ht-ml-pp-cli update <site_id> ./deck.html

# back up your once-only write keys for disaster recovery
ht-ml-pp-cli keys export --out ht-ml-keys.vault

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Registry & recovery
- **`list`** — See every site you've ever published, with health flags for orphaned sites and broken assets.

  _Reach for this to recall or audit what you've shipped; it's the only way to enumerate sites the accountless API won't list._

  ```bash
  ht-ml-pp-cli list --agent --select url,title,status
  ```
- **`keys export`** — Reveal a single write key, or export/import a passphrase-sealed vault of all your update_keys for disaster recovery and a second machine.

  _Use after publishing or before switching machines; without a vault, a lost key orphans the site forever._

  ```bash
  ht-ml-pp-cli keys export --out ht-ml-keys.vault
  ```

### Asset reconciliation
- **`assets sync`** — Parse a site's HTML, find every referenced-but-missing image or video, and upload them all in one pass.

  _Pick this whenever a published page has broken images; it fixes all of them at once instead of one upload per file._

  ```bash
  ht-ml-pp-cli assets sync <site_id> --root ./public
  ```
- **`assets audit`** — Across all your sites, list the ones with publicly-visible broken or missing images.

  _Run during a review to catch broken client-facing pages before someone else does._

  ```bash
  ht-ml-pp-cli assets audit --missing-only --agent
  ```

### Versioning & living docs
- **`rollback`** — Revert a live site to any prior HTML version stored locally, with the update_key resolved for you.

  _Use when a republish shipped bad data; it restores the last-good HTML in one command._

  ```bash
  ht-ml-pp-cli rollback <site_id>
  ```
- **`republish`** — Publish a recurring document under a stable local alias: update in place if the alias exists, or create it once and bind it.

  _Pick this for scheduled or daily publishes so the public URL never churns._

  ```bash
  ht-ml-pp-cli republish --as status-report ./status.html
  ```

### Publish safety
- **`scan`** — Mechanically scan HTML for leaked secrets and PII before it becomes a public, permanent URL.

  _Run before any publish on a person's behalf; it returns a typed exit code so it can gate a pipeline._

  ```bash
  ht-ml-pp-cli scan ./page.html
  ```

## Recipes


### Publish a deck and get just the URL

```bash
ht-ml-pp-cli publish ./deck.html --agent --select url,site_id
```

Publish and return only the live URL and id, ready to hand to a downstream step.

### Inspect a site's referenced assets compactly

```bash
ht-ml-pp-cli sites <site_id> --agent --select status,assets.relative_path,assets.status
```

Use dotted --select to narrow the verbose site-plus-assets payload to just status and each asset's path and status.

### Audit every site for broken images

```bash
ht-ml-pp-cli assets audit --missing-only --agent
```

Cross-site join that lists publicly-visible missing assets the API cannot surface in one call.

### List your published sites by title

```bash
ht-ml-pp-cli list --sort title --agent --select site_id,title,url
```

The local registry is the only inventory of what you have shipped (the API has no list endpoint); sort by title to find a page fast.

### Recover keys on a second machine

```bash
ht-ml-pp-cli keys import ht-ml-keys.vault
```

Import a passphrase-sealed vault so update and rollback work from another machine.

## Usage

Run `ht-ml-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `HT_ML_CONFIG_DIR`, `HT_ML_DATA_DIR`, `HT_ML_STATE_DIR`, or `HT_ML_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `HT_ML_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export HT_ML_HOME=/srv/ht-ml
ht-ml-pp-cli doctor
```

Under `HT_ML_HOME=/srv/ht-ml`, the four dirs resolve to `/srv/ht-ml/config`, `/srv/ht-ml/data`, `/srv/ht-ml/state`, and `/srv/ht-ml/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "ht-ml": {
      "command": "ht-ml-pp-mcp",
      "env": {
        "HT_ML_HOME": "/srv/ht-ml"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `HT_ML_DATA_DIR` overrides an explicit `--home` for that kind. Use `HT_ML_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `HT_ML_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `ht-ml-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### sites

Inspect ht-ml.app sites

- **`ht-ml-pp-cli sites <site_id>`** - Get a site's status and the assets its HTML references (no auth; public read)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ht-ml-pp-cli sites mock-value

# JSON for scripting and agents
ht-ml-pp-cli sites mock-value --json

# Filter to specific fields
ht-ml-pp-cli sites mock-value --json --select id,name,status

# Dry run — show the request without sending
ht-ml-pp-cli sites mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ht-ml-pp-cli sites mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - list and audit commands use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
ht-ml-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `ht-ml-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/ht-ml-pp-cli/config.toml`; `--home`, `HT_ML_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized when updating a site** — The site's update_key isn't in your local store. Run `ht-ml-pp-cli keys import <vault>` if you have a backup, or `ht-ml-pp-cli list` to confirm the site_id.
- **403 Forbidden when uploading an asset** — The asset path must appear in the site's HTML first. Reference it in the HTML, then run `ht-ml-pp-cli assets sync <site_id>`.
- **422 Unprocessable Entity on publish** — The HTML failed ht-ml.app's safety scan. Read the returned message field and revise the HTML.
- **I lost a site's update_key** — There is no recovery endpoint; without a backup the site is read-only forever. Run `ht-ml-pp-cli keys export` after publishing so this can't happen again.
