# ImmoScout24 CLI

**Search ImmoScout24 through the replayable mobile JSON API, not brittle blocked HTML.**

Use the mobile JSON surface behind ImmoScout24 to count searches, list fresh listing cards, retrieve map markers, and inspect expose details. The CLI keeps responses agent-friendly with JSON, selection, bounded paging, and local sync conventions.

## Install

The recommended path installs both the `immoscout24-pp-cli` binary and the `pp-immoscout24` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install immoscout24
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install immoscout24 --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install immoscout24 --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install immoscout24 --agent claude-code
npx -y @mvanhorn/printing-press-library install immoscout24 --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/immoscout24/cmd/immoscout24-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/immoscout24-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install immoscout24 --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-immoscout24 --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-immoscout24 --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install immoscout24 --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/immoscout24-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/immoscout24/cmd/immoscout24-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "immoscout24": {
      "command": "immoscout24-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify the CLI is installed without making a network call
immoscout24-pp-cli doctor --dry-run

```

## Recipes

### Check install health

```bash
immoscout24-pp-cli doctor --dry-run --agent
```

Run a no-network health check in agent-friendly mode.

### Search around Kriftel, Hattersheim, and Hofheim

```bash
immoscout24-pp-cli immoscout24-mobile-search list --search-type radius --geocoordinates '50.078;8.445;8' --realestatetype apartmentrent --pricetype calculatedtotalrent --price=-2500 --numberofrooms=3- --livingspace=100- --equipment=parking --fulltext modern --pagesize 20 --json --select results.resultListItems.item.id,results.resultListItems.item.title,results.resultListItems.item.attributes,results.resultListItems.item.web_url
```

Use this for a rental apartment search near Kriftel/Hattersheim/Hofheim with at least 3 rooms, at least 100 sqm, warm rent below 2500 EUR, and a fixed parking-space filter. Then inspect candidate exposes for kitchen layout, floorplans, and modern condition with: immoscout24-pp-cli expose inspect <id> --json --select results.web_url,results.text_count,results.image_count,results.floorplan_count,results.floorplan_document_count,results.texts.title,results.texts.items.value,results.images.render_url,results.floorplans.render_url,results.floorplan_documents.url

## Unique Features

These capabilities aren't available in any other tool for this API.

### Mobile JSON search
- **`immoscout24-mobile-search total`** — Count matching ImmoScout24 mobile listings before paging through results.

  _Use this first to understand query size and avoid unnecessarily paging broad searches._

  ```bash
  immoscout24-pp-cli immoscout24-mobile-search total --geocodes /de/berlin/berlin --realestatetype apartmentrent --pricetype calculatedtotalrent --json --select results.totalResults
  ```
- **`immoscout24-mobile-search list`** — Fetch listing cards with IDs, titles, address snippets, prices, room counts, thumbnails, and shareable expose links.

  _Use this when an agent needs real listing IDs and summaries to inspect or compare._

  ```bash
  immoscout24-pp-cli immoscout24-mobile-search list --geocodes /de/berlin/berlin --realestatetype apartmentrent --pricetype calculatedtotalrent --pagesize 1 --json --select results.resultListItems.item.id,results.resultListItems.item.title,results.resultListItems.item.web_url
  ```
- **`immoscout24-mobile-search map`** — Retrieve map markers and paging metadata for broad regional searches.

  _Use this to estimate geographic spread and map hit counts before fetching details._

  ```bash
  immoscout24-pp-cli immoscout24-mobile-search map --geocodes /de/berlin/berlin --real-estate-type apartmentrent --pagesize 2 --json --select results.paging.numberOfHits,results.markers.type
  ```

### Expose details
- **`expose`** — Inspect detailed mobile expose sections for a real listing ID.

  _Use this after listing search when an agent needs detail sections for one property._

  ```bash
  immoscout24-pp-cli expose 168983602 --json --compact
  ```
- **`expose inspect`** — Extract grouped expose text sections plus deduplicated property image/media URLs and separately detected floorplans for careful property review.

  _Use this when an agent must inspect wording, amenities, captions, pictures, floorplans, and other media before recommending a property._

  ```bash
  immoscout24-pp-cli expose inspect 168983602 --json --select results.id,results.web_url,results.text_count,results.texts.title,results.texts.items.value,results.image_count,results.floorplan_count,results.floorplan_document_count,results.images.render_url,results.floorplans.render_url,results.floorplan_documents.url
  ```

### Link translation
- **`translate-url`** — Convert between ImmoScout24 web links, mobile API URLs, expose IDs, and CLI commands.

  _Use this to turn a shared web link into replayable mobile API calls, or turn CLI search parameters back into links you can send._

  ```bash
  immoscout24-pp-cli translate-url --search-type radius --geocoordinates '50.078;8.445;8' --realestatetype apartmentrent --pricetype calculatedtotalrent --price=-2500 --numberofrooms=3- --livingspace=100- --equipment=parking --fulltext modern --json --select web_url,mobile_urls.list,cli_commands.list
  ```

## Usage

Run `immoscout24-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `IMMOSCOUT24_CONFIG_DIR`, `IMMOSCOUT24_DATA_DIR`, `IMMOSCOUT24_STATE_DIR`, or `IMMOSCOUT24_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `IMMOSCOUT24_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export IMMOSCOUT24_HOME=/srv/immoscout24
immoscout24-pp-cli doctor
```

Under `IMMOSCOUT24_HOME=/srv/immoscout24`, the four dirs resolve to `/srv/immoscout24/config`, `/srv/immoscout24/data`, `/srv/immoscout24/state`, and `/srv/immoscout24/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "immoscout24": {
      "command": "immoscout24-pp-mcp",
      "env": {
        "IMMOSCOUT24_HOME": "/srv/immoscout24"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `IMMOSCOUT24_DATA_DIR` overrides an explicit `--home` for that kind. Use `IMMOSCOUT24_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `IMMOSCOUT24_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `immoscout24-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### expose

Retrieve expose detail records.

- **`immoscout24-pp-cli expose <id>`** - Returns detailed sections, media, contact metadata, map information, and tracking fields for an expose ID.
- **`immoscout24-pp-cli expose inspect <id>`** - Extracts grouped expose text sections, image/media URLs, separately detected floorplans, and non-image links for detailed review.

### immoscout24-mobile-search

Manage immoscout24 mobile search

- **`immoscout24-pp-cli immoscout24-mobile-search list`** - Retrieves listing cards for a mobile search query. Use a small pagesize and respect rate limits.
- **`immoscout24-pp-cli immoscout24-mobile-search map`** - Returns map markers and paging metadata for broad searches.
- **`immoscout24-pp-cli immoscout24-mobile-search total`** - Returns the total number of listings matching a mobile search query.

### translate-url

Translate ImmoScout24 links and CLI search parameters.

- **`immoscout24-pp-cli translate-url [url-or-id]`** - Converts between ImmoScout24 web links, mobile API URLs, expose IDs, and CLI commands.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
immoscout24-pp-cli expose 168983602

# JSON for scripting and agents
immoscout24-pp-cli expose 168983602 --json

# Filter to specific fields
immoscout24-pp-cli expose inspect 168983602 --json --select results.id,results.web_url,results.texts.title,results.texts.items.value

# Dry run — show the request without sending
immoscout24-pp-cli expose 168983602 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
immoscout24-pp-cli expose inspect 168983602 --agent --max-texts 20
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
immoscout24-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `immoscout24-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/immoscout24-mobile-pp-cli/config.toml`; `--home`, `IMMOSCOUT24_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Direct website requests return CloudFront 401** — Use the generated mobile API commands; do not scrape the HTML website transport.
- **A search returns no listings** — Check that the region path starts with /de/ and that realestatetype matches one of apartmentrent, apartmentbuy, houserent, or housebuy.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Fredy**](https://github.com/orangecoding/fredy) — JavaScript
- [**ImmoScout**](https://github.com/wiestju/ImmoScout) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
