# Pexels CLI

**Every Pexels photo, video, and collection endpoint — plus a local store that adds quota forecasting, dedup-aware bulk download, and one-shot attribution exports no other Pexels tool has.**

Created by [@vcolombo](https://github.com/vcolombo) (Vincent Colombo).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow).

A single binary for the full Pexels API: search and curated photos, video search and popular feeds, and collections including your own. It mirrors results into a local SQLite store so you can re-search offline, dedup downloads across sessions, forecast your rate budget, pick the best-fit resolution for a target, and export license-compliant attribution in one command.

## Install

The recommended path installs both the `pexels-pp-cli` binary and the `pp-pexels` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install pexels
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install pexels --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install pexels --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install pexels --agent claude-code
npx -y @mvanhorn/printing-press-library install pexels --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/cmd/pexels-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pexels-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install pexels --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pexels --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pexels --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install pexels --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pexels-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PEXELS_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/cmd/pexels-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pexels": {
      "command": "pexels-pp-mcp",
      "env": {
        "PEXELS_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Pexels uses a free API key sent as a raw `Authorization: <key>` header — there is NO `Bearer` prefix (the single most common 401 in the ecosystem). Set `PEXELS_API_KEY` or run `pexels-pp-cli auth login`. Every request also sends a User-Agent, since Pexels' edge returns 403 to header-less clients.

## Quick Start

```bash
# Confirm the CLI is wired and see whether your API key is detected
pexels-pp-cli doctor --dry-run

# Core photo search with Pexels' filters
pexels-pp-cli photos search --query "golden retriever puppy" --orientation square --per-page 5

# Dedup-aware download that writes attribution sidecars
pexels-pp-cli download "mountain lake" --type photo --limit 3 --size large --sidecar

# Emit SOURCES.md crediting everything you've downloaded
pexels-pp-cli attribution export --csv

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Rate-limit intelligence
- **`quota forecast`** — Check before a bulk pull whether it fits your remaining hourly/monthly Pexels quota, with a reset ETA.

  _Reach for this before any large download to avoid burning the 200/hour budget mid-batch._

  ```bash
  pexels-pp-cli quota forecast --resources photos,videos --max-pages 10 --agent
  ```

### Resolution & download intelligence
- **`resolve`** — Pick the smallest photo size or video file that meets a target dimension without upscaling.

  _Use when you know the media id and need exactly one URL sized for a target, instead of parsing eight src variants._

  ```bash
  pexels-pp-cli resolve 2014422 --target-width 1280 --target-height 720 --agent
  ```
- **`download`** — Bulk-download a query, skipping media already in your ledger and checkpointing each page so a hit quota stops you gracefully.

  _Use for repeated or large harvests that must not re-download files or exceed the rate budget._

  ```bash
  pexels-pp-cli download "mountain lake" --type photo --limit 30 --max-pages 3 --size large --sidecar
  ```

### License compliance
- **`attribution export`** — Emit a SOURCES.md and per-file .meta.json sidecars crediting every photographer in your local download ledger.

  _Reach for this to satisfy Pexels' attribution guideline in one shot rather than hand-building credit lists._

  ```bash
  pexels-pp-cli attribution export --resources photos,videos --csv
  ```

### Local store that compounds
- **`search`** — Full-text search media you already synced, with stable ordering and no API call.

  _Reach for this to re-find media you pulled earlier instead of spending quota on a fresh live search._

  ```bash
  pexels-pp-cli search "sunset" --type photos --limit 10 --agent --select id,photographer,alt
  ```
- **`analytics`** — Group your synced photos by photographer to review credit balance and licensing coverage.

  _Use during a licensing or credit review to see which photographers dominate your pulled set._

  ```bash
  pexels-pp-cli analytics --type photos --group-by photographer --limit 20
  ```

## Recipes


### Agent-native photo search, narrowed

```bash
pexels-pp-cli photos search --query "foggy forest" --orientation portrait --per-page 8 --agent --select photos.id,photos.photographer,photos.src.large2x,photos.alt
```

Returns only the four fields an agent needs from a deeply-nested response, instead of all eight src sizes per photo.

### Forecast then harvest

```bash
pexels-pp-cli quota forecast --resources photos --max-pages 5 && pexels-pp-cli download "city skyline night" --type photo --limit 50 --max-pages 5 --size large --sidecar
```

Check the rate budget, then run a dedup-aware bulk pull that writes attribution sidecars.

### Best-fit resolution for a known id

```bash
pexels-pp-cli resolve 2014422 --target-width 1920 --target-height 1080 --agent
```

Picks the smallest src/video_file that covers 1080p without upscaling.

### Browse your own collections

```bash
pexels-pp-cli collections list --per-page 20 --json
```

Lists the authenticated user's collections — a surface only two ecosystem tools expose.

### Re-find synced media offline

```bash
pexels-pp-cli sync --resources photos --max-pages 3 && pexels-pp-cli search "sunset" --type photos --limit 10
```

Mirror a few pages locally, then full-text search them without spending more quota.

## Usage

Run `pexels-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PEXELS_CONFIG_DIR`, `PEXELS_DATA_DIR`, `PEXELS_STATE_DIR`, or `PEXELS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PEXELS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PEXELS_HOME=/srv/pexels
pexels-pp-cli doctor
```

Under `PEXELS_HOME=/srv/pexels`, the four dirs resolve to `/srv/pexels/config`, `/srv/pexels/data`, `/srv/pexels/state`, and `/srv/pexels/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "pexels": {
      "command": "pexels-pp-mcp",
      "env": {
        "PEXELS_HOME": "/srv/pexels"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `PEXELS_DATA_DIR` overrides an explicit `--home` for that kind. Use `PEXELS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PEXELS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `pexels-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### collections

Browse featured collections, your own collections, and collection media

- **`pexels-pp-cli collections featured`** - List Pexels featured collections
- **`pexels-pp-cli collections list`** - List the authenticated user's own collections (requires PEXELS_API_KEY)
- **`pexels-pp-cli collections media`** - List the media (photos and/or videos) inside a collection

### photos

Search, curate, and fetch Pexels photos

- **`pexels-pp-cli photos curated`** - Browse the Pexels editor-curated photo feed (use --random for a random page)
- **`pexels-pp-cli photos get`** - Get a single photo by its numeric id
- **`pexels-pp-cli photos search`** - Search photos by keyword with orientation/size/color/locale filters

### videos

Search, browse popular, and fetch Pexels videos

- **`pexels-pp-cli videos get`** - Get a single video by its numeric id
- **`pexels-pp-cli videos popular`** - Browse popular videos with optional dimension/duration filters
- **`pexels-pp-cli videos search`** - Search videos by keyword with orientation/size/locale filters


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pexels-pp-cli collections list

# JSON for scripting and agents
pexels-pp-cli collections list --json

# Filter to specific fields
pexels-pp-cli collections list --json --select id,name,status

# Dry run — show the request without sending
pexels-pp-cli collections list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pexels-pp-cli collections list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
pexels-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `pexels-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/pexels-pp-cli/config.toml`; `--home`, `PEXELS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PEXELS_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `pexels-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `pexels-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PEXELS_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every call** — Your key is likely sent with a Bearer prefix by another tool; this CLI sends the raw key. Verify with `pexels-pp-cli doctor` and re-set via `pexels-pp-cli auth login`.
- **403 from api.pexels.com** — Header-less clients get Cloudflare-403'd; this CLI sets a User-Agent automatically. If you still see it, your key may be gated — check status.pexels.com.
- **Bulk download stops early** — You hit the 200/hour budget. Run `pexels-pp-cli quota forecast` first, then resume with --max-pages once the reset passes.
- **search returns nothing** — Offline search reads the local store; run `pexels-pp-cli sync --resources photos` first, or use `photos search` for a live query.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**pexels (official SDK)**](https://github.com/pexels/pexels-javascript) — JavaScript (171 stars)
- [**pypexels**](https://github.com/salvoventura/pypexels) — Python (76 stars)
- [**pexels-image-downloader**](https://github.com/AguilarLagunasArturo/pexels-image-downloader) — Python (44 stars)
- [**stock-images-mcp**](https://github.com/Zulelee/stock-images-mcp) — Python (37 stars)
- [**garylab pexels-mcp-server**](https://github.com/garylab/pexels-mcp-server) — Python (17 stars)
- [**CaullenOmdahl pexels-mcp-server**](https://github.com/CaullenOmdahl/pexels-mcp-server) — TypeScript (7 stars)
- [**afshinator mcp-server-pexels**](https://github.com/afshinator/mcp-server-pexels) — TypeScript (1 stars)
- [**developer-ishan mcp-pexels**](https://github.com/developer-ishan/mcp-pexels) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
