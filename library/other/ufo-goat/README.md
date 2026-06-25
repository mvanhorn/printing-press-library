# War.gov UFO CLI

**The declassified UAP file archive in your terminal — browse, search, and download 162+ files from the PURSUE initiative**

The first CLI for the War.gov/UFO declassified files portal. Search across every contributing agency (DoD, FBI, NASA, State, CIA, DOE, ODNI, and more), download files with resume support, track new release tranches, and discover video-PDF pairings — all from a single binary with offline SQLite storage.

## Install

The recommended path installs both the `ufo-goat-pp-cli` binary and the `pp-ufo` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install ufo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install ufo --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/ufo-goat/cmd/ufo-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ufo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ufo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ufo --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-ufo skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-ufo. The skill defines how its required CLI can be installed.
```

## Authentication

No authentication required for browsing and searching the file manifest. The CSV manifest is fetched from GitHub (public). File downloads from war.gov may encounter Akamai bot protection (HTTP 403).

Environment variables:

| Variable | Description |
|----------|-------------|
| `UFO_BASE_URL` | Override the base URL (default: `https://raw.githubusercontent.com/DenisSergeevitch/UFO-USA/main/metadata`) |
| `UFO_CONFIG` | Override config file path (default: `~/.config/ufo-goat-pp-cli/config.json`) |

## Quick Start

```bash
# Check connectivity and local store status
ufo-goat-pp-cli doctor

# Fetch the latest file manifest from the PURSUE archive
ufo-goat-pp-cli sync

# Search across titles, descriptions, and locations
ufo-goat-pp-cli search "Apollo"

# View the chronological incident timeline
ufo-goat-pp-cli timeline --after 1960-01-01 --before 1970-12-31

# Find video-PDF pairings for cross-referencing
ufo-goat-pp-cli pairs
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`sync`** — Automatically detect and fetch new file tranches as the government releases them on a rolling basis

  _Agents monitoring the PURSUE release schedule get notified of new files without manual checking_

  ```bash
  ufo-goat-pp-cli sync
  ```
- **`new`** — Show the files in the latest government release tranche — the 'what just dropped' command, scoped to the batch rather than your sync timing

  _When an agent needs to see exactly what the most recent PURSUE batch contained, regardless of when it last synced_

  ```bash
  ufo-goat-pp-cli new            # latest tranche
  ufo-goat-pp-cli new --release 1
  ufo-goat-pp-cli new --since 7d # sync-timing fallback
  ```
- **`releases`** — Treat the PURSUE batch as a first-class lens: list every tranche, compare two, and detect when a new one lands

  _The government declassifies files in batches (release_1, release_2, …). `releases` summarizes each batch's date, file count, and agency/type mix; `releases check` is the cron-friendly "did a new batch drop?" probe._

  ```bash
  ufo-goat-pp-cli releases              # summarize every tranche
  ufo-goat-pp-cli releases diff 1 2     # compare two tranches
  ufo-goat-pp-cli releases check --exit-code   # exit 3 when nothing new
  ```

### Cross-agency intelligence
- **`timeline`** — View a chronological incident timeline spanning 1944-2025 across every contributing agency

  _Researchers need to see the full picture: FBI case from 1947 next to a DoD mission report from 2024_

  ```bash
  ufo-goat-pp-cli timeline --after 1940-01-01 --before 1949-12-31
  ```
- **`pairs`** — Find video-PDF pairings so researchers can locate the document that accompanies a video and vice versa

  _41 videos have paired documents — this command surfaces the connections instantly_

  ```bash
  ufo-goat-pp-cli pairs --agent
  ```
- **`agencies`** — See which agency contributed what: file counts, types, date ranges, and coverage analysis

  _Quick answer to 'what did the FBI release vs NASA vs DoD'_

  ```bash
  ufo-goat-pp-cli agencies --json
  ```
- **`locations`** — Aggregate incidents by geographic location for mapping and spatial analysis

  _Spatial patterns emerge from aggregation across all agencies_

  ```bash
  ufo-goat-pp-cli locations --json
  ```

### Agent-native plumbing
- **`download`** — Download files with resume support, verification, and progress tracking for the 2.3 GB archive

  _The archive is 2.3 GB of PDFs alone — agents need reliable batch downloads with state tracking_

  ```bash
  ufo-goat-pp-cli download --agency FBI --resume
  ```

## Commands

### File Archive

| Command | Description |
|---------|-------------|
| `files list` | List all declassified UAP files from local store (filter by `--agency`, `--type`, `--location`, `--release`) |
| `files get` | Get details of a specific file by ID or title |
| `files search` | Full-text search across titles, descriptions, and locations (`--release` to scope to a tranche) |
| `search` | Top-level shortcut for `files search` |
| `download` | Download files from war.gov with resume support |

### Cross-Agency Analysis

| Command | Description |
|---------|-------------|
| `agencies` | Agency breakdown with file counts and type coverage |
| `timeline` | Chronological incident timeline, 1944-2025 |
| `pairs` | Video-PDF pairings for cross-referencing |
| `locations` | Incidents aggregated by geographic location |

### Data Management

| Command | Description |
|---------|-------------|
| `sync` | Sync the UAP file manifest to local SQLite from a configurable source (reports newly-landed release tranches) |
| `sources` | List the manifest sources the CLI can sync from (`--source`, `--manifest-url`, `UFO_SOURCE`, `UFO_MANIFEST_URL`) |
| `new` | Show files in the latest release tranche (`--release N`, or `--since`/`--since-sync` for sync-timing) |
| `releases` | Summarize every release tranche; `releases diff <from> <to>` compares two; `releases check` detects a new batch |
| `analytics` | Run analytics queries on locally synced data |
| `export` | Export data to JSONL or JSON |
| `import` | Import data from JSONL file |

### Utilities

| Command | Description |
|---------|-------------|
| `doctor` | Check CLI health and connectivity |
| `workflow archive` | Sync all resources for offline access |
| `workflow status` | Show local archive status and sync state |
| `profile` | Named sets of flags saved for reuse |
| `which` | Find the command that implements a capability |
| `agent-context` | Emit structured JSON describing this CLI for agents |
| `api` | Browse all API endpoints by interface name |
| `tail` | Stream live changes by polling the API |
| `feedback` | Record feedback about this CLI |

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ufo-goat-pp-cli agencies

# JSON for scripting and agents
ufo-goat-pp-cli agencies --json

# Filter to specific fields
ufo-goat-pp-cli files list --json --select id,title,agency

# CSV output
ufo-goat-pp-cli files list --csv

# Compact output (key fields only, minimal tokens)
ufo-goat-pp-cli files list --compact

# Dry run — show the request without sending
ufo-goat-pp-cli agencies --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ufo-goat-pp-cli agencies --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** — never prompts, every input is a flag
- **Pipeable** — `--json` output to stdout, errors to stderr
- **Filterable** — `--select id,title` returns only fields you need
- **Previewable** — `--dry-run` shows the request without sending
- **Read-only by default** — this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** — sync/search commands use the local SQLite store
- **Agent-safe by default** — no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-ufo -g
```

Then invoke `/pp-ufo <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/ufo-goat/cmd/ufo-goat-pp-mcp@latest
```

Then register it:

```bash
claude mcp add ufo ufo-goat-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ufo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/ufo-goat/cmd/ufo-goat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ufo": {
      "command": "ufo-goat-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
ufo-goat-pp-cli doctor
```

Example output:

```
  OK Config: ok
  OK Auth: not required
  OK API: reachable
  config_path: ~/.config/ufo-goat-pp-cli/config.json
  base_url: https://raw.githubusercontent.com/DenisSergeevitch/UFO-USA/main/metadata
  version: 1.0.0
  OK Cache: fresh
    db_path: ~/.local/share/ufo-goat-pp-cli/data.db
```

## Cookbook

```bash
# Sync and check for new files in one pass
ufo-goat-pp-cli sync && ufo-goat-pp-cli new

# Summarize every government release tranche
ufo-goat-pp-cli releases --json

# See everything in a specific release tranche
ufo-goat-pp-cli files list --release 1 --agent

# Compare the composition of two tranches
ufo-goat-pp-cli releases diff 1 2 --json

# Scheduled "did a new batch drop?" check (exit 3 when nothing new)
ufo-goat-pp-cli releases check --exit-code --agent

# List all FBI PDFs from the 1950s
ufo-goat-pp-cli files list --agency FBI --type PDF --after 1950-01-01 --before 1959-12-31

# Search for incidents in New Mexico
ufo-goat-pp-cli search "New Mexico" --json

# Export the full archive as JSONL for external analysis (resource: files)
ufo-goat-pp-cli export <resource> --format jsonl --output ufo-archive.jsonl

# Download only NASA videos
ufo-goat-pp-cli download --agency NASA --type VID --output-dir ~/ufo-videos

# Resume an interrupted download
ufo-goat-pp-cli download --resume

# Get agency breakdown as CSV for a spreadsheet
ufo-goat-pp-cli agencies --csv

# View timeline of 1940s incidents as JSON for an agent
ufo-goat-pp-cli timeline --after 1940-01-01 --before 1949-12-31 --agent

# Find all video-PDF pairs and pipe to jq
ufo-goat-pp-cli pairs --json | jq '.[].video_title'

# Run analytics on synced data grouped by agency
ufo-goat-pp-cli analytics --type files --group-by agency --json

# Check what files were added in the last week
ufo-goat-pp-cli new --since 1w --json

# Full resync after a data correction
ufo-goat-pp-cli sync --full
```

## Manifest Sources

The PURSUE archive is published in rolling release tranches (Release 1 on
2026-05-08, Release 2 on 2026-05-22, Release 3 on 2026-06-12, ...). war.gov
blocks programmatic access, so the CLI syncs from a community mirror. The source
is **configurable** — run `ufo-goat-pp-cli sources` to list them:

| Source | Status | Notes |
|--------|--------|-------|
| `community` | available (default) | Tracks every release tranche (`abigailhaddad/ufo-releases`) |
| `legacy` | available | Original Release 1 mirror (`DenisSergeevitch/UFO-USA`), frozen 2026-05-08 |
| `wargov` | planned | Reserved for a future sanctioned, direct-from-war.gov feed |

Select a source per-sync, or set it via environment variable. Resolution
precedence (highest first): `--manifest-url`, `--source`, `UFO_MANIFEST_URL`,
`UFO_SOURCE`, built-in default.

```bash
ufo-goat-pp-cli sources                                   # list sources
ufo-goat-pp-cli sync --source legacy                      # named source
ufo-goat-pp-cli sync --manifest-url https://host/uap.csv  # custom mirror
UFO_SOURCE=community ufo-goat-pp-cli sync                 # via env
```

The `wargov` entry is a deliberate placeholder: when a direct, sanctioned feed
exists, it becomes selectable with `--source wargov` (with auth if required) and
no other command changes — see `internal/manifest/source.go`.

## Configuration

Config file: `~/.config/ufo-goat-pp-cli/config.json`

| Variable | Description | Default |
|----------|-------------|---------|
| `UFO_SOURCE` | Named manifest source to sync from | `community` |
| `UFO_MANIFEST_URL` | Custom manifest CSV URL (overrides `UFO_SOURCE`) | (unset) |
| `UFO_CONFIG` | Override config file path | `~/.config/ufo-goat-pp-cli/config.json` |

Database: `~/.local/share/ufo-goat-pp-cli/data.db` (SQLite, created on first `sync`)

## Troubleshooting

**Config errors (exit code 10)**
- Run `ufo-goat-pp-cli doctor` to verify configuration
- Check that `~/.config/ufo-goat-pp-cli/config.json` is valid JSON (if it exists)

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run `ufo-goat-pp-cli files list` to see available items
- Run `ufo-goat-pp-cli sync` if the local store is empty

**API errors (exit code 5)**
- Check internet connectivity — the manifest is fetched from GitHub
- Run `ufo-goat-pp-cli doctor` to verify API reachability

**Rate limited (exit code 7)**
- GitHub rate limits raw content at 5000 req/hr
- Wait and retry, or use `--rate-limit 1` to throttle

### API-specific

- **403 Forbidden when downloading files** — war.gov is behind Akamai CDN and may block direct HTTP requests. Try downloading fewer files at a time.
- **Empty results from sync** — Check internet connectivity. The manifest is fetched from GitHub. Run `ufo-goat-pp-cli doctor` to verify.
- **No new files found** — New tranches are released periodically. Run `ufo-goat-pp-cli sync` to check for updates.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**UFO-USA**](https://github.com/DenisSergeevitch/UFO-USA) — Python
- [**UFOSINT Explorer**](https://github.com/UFOSINT/ufosint-explorer) — Python
- [**nuforc_sightings_data**](https://github.com/timothyrenner/nuforc_sightings_data) — Python
- [**uap-data-vis-tool**](https://github.com/jamsoft/uap-data-vis-tool) — C#

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
