# art-goat CLI

**One contemplative practice, every major open-access museum and astronomy API.**

art-goat is an OmniMuseum aggregator for a daily contemplative art practice. Eight open-access sources — Art Institute of Chicago, NASA APOD, Metropolitan Museum of Art, Cleveland Museum of Art, Rijksmuseum, Smithsonian Open Access, Te Papa Tongarewa, and the National Palace Museum Taiwan — collapse into one local SQLite corpus. Sit with one piece for a fixed period, capture your reflection, and let an opinionated `today` pick rotate against your recent practice. The contemplative spine — `sit`, `today`, `path`, `journal` — is the soul; federated `browse`, `similar`, `compare`, and `artist --arc` ride alongside as substrate.

Where similar tools wrap one museum, art-goat treats the eight as a single curated corpus: `today` rotates across sources, `similar` and `artist --arc` step laterally across them, and `path --theme` walks a theme through the whole federation. The point isn't completeness of any one museum's catalog; it's a daily practice grounded in real, public-domain collections.

Printed by [@justinwfu](https://github.com/justinwfu) (justinwfu).

## Data sources

| Source | Slug | Auth | License |
| --- | --- | --- | --- |
| Art Institute of Chicago | `aic` | none | CC0 / CC-By |
| NASA Astronomy Picture of the Day | `apod` | DEMO_KEY ok; free key for higher rate | Public domain (NASA) |
| Metropolitan Museum of Art | `met` | none | CC0 (public domain items) |
| Cleveland Museum of Art | `cleveland` | none | CC0 |
| Harvard Art Museums | `harvard` | required (`HARVARD_API_KEY` or `ART_GOAT_HARVARD_KEY`) | Mixed; public-domain subset only |
| Smithsonian Open Access | `smithsonian` | DEMO_KEY ok via api.data.gov; better with your own key (`SMITHSONIAN_API_KEY` or `ART_GOAT_API_KEY`) | CC0 |
| National Palace Museum Taiwan | `npmtw` | none (curated static subset of ~9 famous works pending a public API) | Public domain |

> **Removed 2026-05-22:** Rijksmuseum and Te Papa Tongarewa adapters were removed — Rijksmuseum's documented signup URL 404s and its keyless alternatives are either an XML rewrite (OAI-PMH) or 100× the request count (LOD identifiers); Te Papa's adapter dropped 100% of records past the search-query fix, sending the sync into a runaway pagination loop. Restore from git history if reviving either.

Pull all of them with `art-goat-pp-cli sources sync`, or scope to one with `--source <slug>`. Sync is incremental — re-running it updates the local corpus rather than rebuilding it.

## Deferred to v2

- **NPM Taiwan live API** — currently shipped as a small static-curated subset because the museum's open-data portal does not yet expose a queryable JSON endpoint. Will be promoted to a live source once one is published.

## Install

The recommended path installs both the `art-goat-pp-cli` binary and the `pp-art-goat` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install art-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install art-goat --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/art-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-art-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-art-goat --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-art-goat skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-art-goat. The skill defines how its required CLI can be installed.
```

## Authentication

Anonymous out of the box. AIC, Met, Cleveland, and NPM Taiwan need no key. NASA APOD and Smithsonian Open Access work via DEMO_KEY; supply your own key (`NASA_API_KEY`, `SMITHSONIAN_API_KEY`, or the umbrella `ART_GOAT_API_KEY`) to lift the 30 req/hr DEMO_KEY ceiling. Rijksmuseum and Te Papa require a free signup key — see the source table above. The CLI surfaces the exact env var name in its error message when a required key is missing.

## Quick Start

```bash
# Populate the local corpus across all configured sources (AIC, APOD, Met, Cleveland, Harvard, Smithsonian, NPM Taiwan). Anonymous; no setup wall.
art-goat-pp-cli sources sync


# Today's curated piece — opinionated against your practice, with a one-line 'why this today'.
art-goat-pp-cli today


# Sit with a random piece. Image opens in your browser; reflection captured in terminal.
art-goat-pp-cli sit


# See breadth, variety, region coverage, mood drift. Streak available at the bottom if you want to know.
art-goat-pp-cli journal stats


# Step laterally from one piece to another across the corpus.
art-goat-pp-cli similar aic:24645 --json --select source,title,creator

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Contemplative practice
- **`sit`** — Sit with one piece of art for a fixed period. Opens the image in your browser, shows the curator's description, runs a quiet timer, and captures your reflection in the journal.

  _Use this when you want to spend deliberate time with one piece instead of skimming a feed. The reflection is persisted locally for later search._

  ```bash
  art-goat-pp-cli sit --duration 10 --dry-run
  ```
- **`today`** — Today's curated piece, chosen using anti-repeat against your sits and journal-aware diversity against your recent medium and culture choices. Includes a one-line 'why this today'.

  _Use today as the default daily entry point. It avoids showing you what you already sat with and rotates against your recent medium and region choices._

  ```bash
  art-goat-pp-cli today
  ```

### Practice journal
- **`journal stats`** — Practice statistics emphasising source breadth, medium variety, region coverage, and mood drift, with streak available at the bottom labeled 'if you want to know'.

  _Use this to see breadth instead of streak length. Practice quality is captured as how widely you've ranged, not how many days in a row you've sat._

  ```bash
  art-goat-pp-cli journal stats
  ```
- **`journal search`** — FTS5 over your reflection history. Surfaces every past sit whose text matches the query.

  _Use this to find prior reflections by token. Years of practice become queryable._

  ```bash
  art-goat-pp-cli journal search "solitude"
  ```

## Recipes

Workflows that combine several commands for a complete practice loop.

### Start a daily practice

```bash
art-goat-pp-cli sources sync && art-goat-pp-cli today
```

Populate the local corpus across all eight sources, then enter today's curated piece in a single command.

### Discover what you've been missing

```bash
art-goat-pp-cli gaps --limit 5
art-goat-pp-cli coverage --json --select sources_total,sources_sat
```

`gaps` surfaces the regions and mediums in your corpus you've never sat with; `coverage` quantifies practice breadth as a percentage.

### Follow one piece across museums

```bash
art-goat-pp-cli similar met:436532 --json --select source,title,creator
art-goat-pp-cli artist "hokusai" --arc
```

Use `similar` to step laterally from a seed work, or `artist --arc` to render a creator's stylistic periods across sources as a short career narrative.

### Walk a theme through every museum

```bash
art-goat-pp-cli path --theme "impermanence"
art-goat-pp-cli path --theme "solitude" --steps 7
```

`path` runs the theme as an FTS5 search across the federated corpus and returns a diversity-ordered walk — no two consecutive steps from the same source/region/medium when the corpus allows.

### Bridge across moods

```bash
art-goat-pp-cli today --mode bridge-from-last
```

Reads your last sit's mood and picks toward the opposite half of the scale. Cultivates rotation of energy across days, not just visuals.

### Revisit your past practice

```bash
art-goat-pp-cli journal revisit --age 1y
art-goat-pp-cli journal revisit --age 6mo --json
art-goat-pp-cli journal compare 12 47
```

`revisit` surfaces the sit closest to `today - <age>` (within ±7 days). `compare` renders two sits side-by-side with their works and reflections — a longitudinal log instead of a flat list.

### Export your practice for safekeeping

```bash
ART_GOAT_JOURNAL_PATH=~/Obsidian/art-goat art-goat-pp-cli journal export
```

Mirror the SQLite journal to per-sit Markdown files. SQLite stays canonical; the Markdown is your tool-agnostic backup.

## Usage

Run `art-goat-pp-cli --help` for the full command reference and flag list.

## Commands

### planetary

Manage planetary

- **`art-goat-pp-cli planetary apod-get`** - Fetch the Astronomy Picture of the Day for a given date or date range. Anonymous via DEMO_KEY.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
art-goat-pp-cli planetary

# JSON for scripting and agents
art-goat-pp-cli planetary --json

# Filter to specific fields
art-goat-pp-cli planetary --json --select id,name,status

# Dry run — show the request without sending
art-goat-pp-cli planetary --dry-run

# Agent mode — JSON + compact + no prompts in one flag
art-goat-pp-cli planetary --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-art-goat -g
```

Then invoke `/pp-art-goat <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add art-goat art-goat-pp-mcp -e ART_GOAT_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/art-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ART_GOAT_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "art-goat": {
      "command": "art-goat-pp-mcp",
      "env": {
        "ART_GOAT_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
art-goat-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/art-goat-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ART_GOAT_API_KEY` | per_call | Yes | Set to your API credential. |
| `NASA_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `art-goat-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ART_GOAT_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **sit opens nothing in browser** — Run with --no-image to fall back to description mode, or check that $BROWSER is set on your platform.
- **NASA APOD rate limit hit** — Run `art-goat-pp-cli auth set apod` and paste a free key from https://api.nasa.gov/. The DEMO_KEY ships with a ~30/hour cap.
- **Empty results from search or browse** — Run `art-goat-pp-cli sources sync` first; novel commands read from the local store, which `sources sync` populates.
- **Journal entries not showing in Obsidian** — Set ART_GOAT_JOURNAL_PATH to your Obsidian vault path before journal write; the Markdown mirror writes there.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**art-institute-of-chicago/api**](https://github.com/art-institute-of-chicago/api) — PHP
- [**nasa/apod-api**](https://github.com/nasa/apod-api) — Python
- [**metmuseum/openaccess**](https://github.com/metmuseum/openaccess) — CSV

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
