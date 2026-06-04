# NASA Image and Video Library CLI

**Every NASA Image and Video Library endpoint, plus a local SQLite mirror, FTS search, resumable bulk download, caption text extraction, and an agent-native MCP surface no other tool offers.**

The official NASA Image and Video Library exposes five endpoints but no Go CLI covers all of them, and none of the existing wrappers go past returning JSON. nasa-images-pp-cli adds a local SQLite mirror with FTS5 search, byte-range resumable album downloads, caption text extraction (not just URLs), deterministic best-variant picking for agents, and a typed MCP server — every command works offline once synced, and every command pipes cleanly to jq.

Created by [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `nasa-images-pp-cli` binary and the `pp-nasa-images` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install nasa-images
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install nasa-images --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install nasa-images --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install nasa-images --agent claude-code
npx -y @mvanhorn/printing-press-library install nasa-images --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/nasa-images/cmd/nasa-images-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nasa-images-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-nasa-images --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-nasa-images --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install nasa-images --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nasa-images-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "nasa-images": {
      "command": "nasa-images-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# search the live API with all the filters NASA exposes
nasa-images-pp-cli media --q "perseverance rover" --media-type image --year-start 2024 --json

# get exactly one URL for the best variant under a size cap — no manifest parsing
nasa-images-pp-cli assets best PIA26345 --prefer orig,large --max-bytes 5000000

# bulk-download a curated album with byte-range resume; re-runs pick up where they left off
nasa-images-pp-cli download album Mars-Perseverance --variant orig --resume --out ./mars

# populate the local SQLite mirror for offline FTS search and timeline queries
nasa-images-pp-cli mirror search --q "webb" --year-start 2022

# chronologically-sorted FTS search NASA's upstream API doesn't offer
nasa-images-pp-cli recent --q "deep field" --sort date-desc --limit 10

# actual caption text, not the .srt URL every other wrapper stops at
nasa-images-pp-cli captions fetch jsc2022m000123 --format text

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`download album`** — Bulk-download every asset in a curated NASA album with byte-range resume — re-runs pick up exactly where the last failed transfer left off.

  _When you need 200 NASA images for a deck or archive, this is the one command that finishes — flaky wifi, sleep-resume, mid-run cancellation all become non-events._

  ```bash
  nasa-images-pp-cli download album Apollo-at-50 --variant orig --resume --out ./apollo
  ```
- **`recent`** — FTS5 search over the synced local mirror (title, description, description_508, keywords, album), with chronological sort the upstream API doesn't expose.

  _When breaking news hits, you can pull the most recent NASA images on a topic in milliseconds — without paging through decades of archival results._

  ```bash
  nasa-images-pp-cli recent --q "perseverance" --sort date-desc --limit 20 --json
  ```
- **`center profile`** — Local aggregation over the synced cache: counts by media_type, year-bucket histogram, top keywords, and top photographers for one of the 11 NASA centers.

  _Journalists answer 'which center publishes this kind of image' before they search — saving search time on stories where a center is the obvious source._

  ```bash
  nasa-images-pp-cli center profile JPL --json
  ```
- **`unused-in`** — Lists nasa_ids in a curated album that haven't been downloaded yet locally — anti-join against the downloads ledger.

  _Plan next week's slides without picking the same image twice — without keeping a manual list of nasa_ids you've already used._

  ```bash
  nasa-images-pp-cli unused-in Apollo-at-50 --json
  ```
- **`timeline`** — Local GROUP BY strftime('%Y-%m', date_created) over FTS-matched rows; prints month-bucket counts showing when a topic got coverage.

  _Spot publishing patterns — quiet months, new-release bursts — that surface story angles a keyword search hides._

  ```bash
  nasa-images-pp-cli timeline --q "perseverance" --bucket month --json
  ```

### API indirections we follow for you
- **`captions fetch`** — Returns the captions file CONTENT (.srt or .vtt), not just the URL — with an optional `--format text` mode that strips cue numbers and timecodes for readable transcripts.

  _Agents pulling NASA video captions for analysis, search, or quote-finding skip an extra HTTP call and a parse step._

  ```bash
  nasa-images-pp-cli captions fetch jsc2022m000123 --format text --agent
  ```
- **`metadata fetch`** — Follows the /metadata/{id} indirection, fetches the metadata.json sidecar, flattens AVAIL:* and EXIF fields, and drops internal-path leak fields (SourceFile, File:Directory, AVAIL:Owner).

  _Journalists and educators get the bylined photo credit (photographer + center + date) without scrubbing noise fields by hand._

  ```bash
  nasa-images-pp-cli metadata fetch PIA24439 --json --select AVAIL:Title,AVAIL:Photographer,AVAIL:DateCreated
  ```

### Agent-native plumbing
- **`assets best`** — Parses an asset's rendition manifest, classifies each file by variant (orig/large/medium/small/thumb), applies a caller preference order with optional byte ceiling, and prints exactly one URL.

  _Claude agents in MCP can ask for 'the best version of nasa_id X under 5 MB' and get exactly one URL — no token spend on parsing filenames._

  ```bash
  nasa-images-pp-cli assets best as11-40-5874 --prefer orig,large,medium --max-bytes 5000000
  ```

### Domain-specific shortcuts
- **`citation`** — Generates a ready-to-paste citation string (APA / MLA / Chicago) from cached metadata (photographer, date, center, nasa_id, URL).

  _Drop a NASA image in a piece and paste the citation underneath — no formatting by hand._

  ```bash
  nasa-images-pp-cli citation PIA24439 --style apa
  ```

## Usage

Run `nasa-images-pp-cli --help` for the full command reference and flag list.

## Commands

### albums

Retrieve the contents of a curated NASA album (e.g. Apollo-at-50, Mars-Perseverance)

- **`nasa-images-pp-cli albums <album_name>`** - Return paginated asset listing for a curated album. Album names are case-sensitive (Apollo-at-50 works, apollo-at-50 returns 404).

### assets

Retrieve the rendition manifest for an asset (every file URL: original/large/medium/small/thumb for images; mp4 variants for video; mp3/m4a for audio)

- **`nasa-images-pp-cli assets <nasa_id>`** - Return the list of every file URL for an asset

### captions

Retrieve the location URL of a video asset's captions file (.srt or .vtt)

- **`nasa-images-pp-cli captions <nasa_id>`** - Return the location URL of the captions file. Video assets only — 400 for images, 404 for video without captions. Use `captions fetch` (novel command) to follow the indirection and fetch the caption text.

### media

Search the NASA Image and Video Library catalog by free text and filters

- **`nasa-images-pp-cli media`** - Search the catalog by free text or filters. At least one query parameter is required.

### metadata

Retrieve the location URL of an asset's metadata.json sidecar (AVAIL editorial + ExifTool fields)

- **`nasa-images-pp-cli metadata <nasa_id>`** - Return the location URL of the metadata.json sidecar for an asset. Use `metadata fetch` (novel command) to follow the indirection and fetch the cleaned metadata content.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
nasa-images-pp-cli albums mock-value

# JSON for scripting and agents
nasa-images-pp-cli albums mock-value --json

# Filter to specific fields
nasa-images-pp-cli albums mock-value --json --select id,name,status

# Dry run — show the request without sending
nasa-images-pp-cli albums mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
nasa-images-pp-cli albums mock-value --agent
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
nasa-images-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/nasa-images-pp-cli/config.json` (or `$XDG_CONFIG_HOME/nasa-images-pp-cli/config.json` when set).

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the NASA ID is spelled correctly (case-sensitive: `as11-40-5874`, not `AS11-40-5874`).
- Discover NASA IDs with `nasa-images-pp-cli media --q <term> --page-size 5 --json`, or populate the local mirror via `mirror search --q <topic>` and browse with `recent --q <term>`.

### API-specific

- **Empty results from media list** — Check spelling and try a broader --q; the upstream API uses keyword matching only. For breaking-news content, run `mirror search --q <topic>` first then use `recent --sort date-desc`.
- **404 from album get** — Album names are exactly case-sensitive — `Apollo-at-50` works, `apollo-at-50` does not. Discover album names from search results' `data[].album` field.
- **400 from captions fetch with mediatype=image error** — Captions only exist for video assets. Run `assets get <nasa_id>` first to confirm media_type=video.
- **metadata fetch returns AVAIL:Owner or SourceFile fields** — Upgrade — current version filters these leak fields by default. The upstream sidecar still leaks them, but our metadata fetch strips them.
- **Download interrupted mid-album** — Re-run the same `download album` command with `--resume`. The local downloads ledger tracks per-file progress, and in-flight files resume from the last completed byte.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**ProgramComputer/NASA-MCP-server**](https://github.com/ProgramComputer/NASA-MCP-server) — TypeScript (88 stars)
- [**AJFunk/nasa-sdk**](https://github.com/AJFunk/nasa-sdk) — JavaScript (49 stars)
- [**peteretelej/nasa**](https://github.com/peteretelej/nasa) — Go (17 stars)
- [**jezweb/nasa-mcp-server**](https://github.com/jezweb/nasa-mcp-server) — Python (8 stars)
- [**scientific-dev/python-nasa-api**](https://github.com/scientific-dev/python-nasa-api) — Python (4 stars)
- [**abh80/nasa-api-wrapper**](https://github.com/abh80/nasa-api-wrapper) — TypeScript (3 stars)
- [**DanielLMcGuire/nasa-images-cli**](https://github.com/DanielLMcGuire/nasa-images-cli) — Python (1 stars)
- [**kaitoy/nasa-images-mcp-server**](https://github.com/kaitoy/nasa-images-mcp-server) — TypeScript (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
