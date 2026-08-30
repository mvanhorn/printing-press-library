# The Lancet CLI

**Search Lancet articles offline and analyze author impact, institutional networks, and editorial trends — powered by the free OpenAlex scholarly graph.**

The Lancet CLI gives researchers and institutions offline access to The Lancet family of journals via OpenAlex. Beyond search and fetch, it ranks researchers by citation impact within a specialty (rank-authors), maps institutional co-authorship networks (mesh), tracks which institutions are publishing more over time (affiliation-growth), and detects how a journal's editorial focus shifts (drift) — insights impossible from single API queries or paywalled tools.

Learn more at [The Lancet](https://docs.openalex.org/).

## Install

The recommended path installs both the `thelancet-pp-cli` binary and the `pp-thelancet` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install thelancet
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install thelancet --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install thelancet --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install thelancet --agent claude-code
npx -y @mvanhorn/printing-press-library install thelancet --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/thelancet/cmd/thelancet-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/thelancet-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install thelancet --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-thelancet --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-thelancet --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install thelancet --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/thelancet-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/thelancet/cmd/thelancet-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "thelancet": {
      "command": "thelancet-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify the CLI is working and OpenAlex is reachable (no auth required)
thelancet-pp-cli doctor --dry-run

# Find recent Lancet articles on a research topic (scoped to Lancet journals by default)
thelancet-pp-cli works search --search 'CRISPR gene therapy' --per-page 5

# Build the local database from OpenAlex so the analytics commands below can run offline
thelancet-pp-cli refresh --journal lancet

# Rank the most-cited authors in a specific Lancet journal
thelancet-pp-cli rank-authors --journal lancet-oncology --json

# See how the journal's editorial focus has shifted between two periods
thelancet-pp-cli drift --journal lancet-oncology --window1 2018:2020 --window2 2023:2024

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local data compounds
- **`rank-authors`** — Rank researchers at your institution by citation impact within a specific Lancet journal or topic.

  _Grant managers and department heads use this to identify who is leading at your institution in a specific field; agents can use it to find high-impact collaborators._

  ```bash
  thelancet-pp-cli rank-authors --institution 'Harvard' --journal lancet-oncology --json
  ```
- **`mesh`** — Find researchers at your institution who co-author Lancet papers and quantify how their work connects.

  _Librarians and research directors use this to uncover collaboration opportunities and map how an institution's researchers connect through Lancet._

  ```bash
  thelancet-pp-cli mesh --org 'Stanford University' --json
  ```
- **`affiliation-growth`** — Track institutions gaining publication velocity in Lancet journals; identify rising research centers.

  _Strategy teams track this to understand competitive research landscapes and identify emerging partner institutions._

  ```bash
  thelancet-pp-cli affiliation-growth --journal lancet-neurology --years 5 --json
  ```
- **`visibility-gap`** — Find authors whose citation impact is out of step with the prestige of the Lancet journals they publish in.

  _Grant managers and research directors use this to identify under-recognized talent for promotion or collaboration._

  ```bash
  thelancet-pp-cli visibility-gap --institution 'Oxford' --json
  ```

### Offline analysis
- **`drift`** — Compare how a journal's topic distribution shifts between two time windows; spot emerging and fading specialties.

  _Research strategists use this to align their work with journal editorial priorities and detect when journal scope shifts._

  ```bash
  thelancet-pp-cli drift --journal lancet-oncology --window1 2018:2020 --window2 2023:2024 --json
  ```
- **`curate`** — Auto-generate ranked reading lists for a topic, sorted by date/citations/relevance, exportable as Markdown or BibTeX.

  _Librarians and researchers use this to quickly assemble authoritative reading lists without manual screening._

  ```bash
  thelancet-pp-cli curate --topic 'gene therapy' --sort citations --output bibtex
  ```

## Recipes


### Build a topic reading list for a literature review

```bash
thelancet-pp-cli curate --topic 'machine learning diagnosis' --sort citations --output bibtex > review.bib
```

Generate a citation-ranked, BibTeX-exportable reading list on a topic across Lancet journals in one command.

### Narrow a deeply nested works response for an agent

```bash
thelancet-pp-cli works search --search 'immunotherapy' --per-page 10 --agent --select results.title,results.cited_by_count,results.authorships.author.display_name
```

OpenAlex works are large and deeply nested; pair --agent with dotted --select paths to return only the fields you need and save context.

### Spot editorial drift in a specialty journal

```bash
thelancet-pp-cli drift --journal lancet-oncology --window1 2018:2020 --window2 2023:2024 --top-n 15
```

Compare topic distributions across two multi-year windows to see which specialties the journal now publishes more or less of.

## Local JSON API

The `serve` command exposes the analytics engine as a local read-only JSON API — useful for the Lancet web portal or any local tool that prefers HTTP over shelling out to the CLI:

```bash
# Default: loopback only
thelancet-pp-cli serve

# Custom port (keep it on 127.0.0.1)
thelancet-pp-cli serve --listen 127.0.0.1:9090
```

Two endpoints, mirroring the CLI commands' parameters, defaults, and JSON shapes:

| Endpoint | Mirrors | Query params |
|----------|---------|--------------|
| `GET /affiliations` | `affiliation-growth` | `journal` (slug, default all), `years` (default 5), `threshold` (default 2), `limit` (default 25) |
| `GET /authors` | `rank-authors` | `journal`, `institution` (substring), `limit` (default 25) |

```bash
curl 'http://127.0.0.1:8080/affiliations?journal=lancet-neurology&years=5'
curl 'http://127.0.0.1:8080/authors?institution=Oxford&limit=10'
```

Responses are the same JSON arrays the commands emit with `--json`. Invalid parameters return `400` with `{"error": "..."}`; engine failures return `500` with a generic structured error (no internal detail). Use `--db` to point at a non-default database path; stop with Ctrl+C (graceful shutdown).

**Security:** binds to loopback (`127.0.0.1:8080`) by default, serves only read queries against the local mirror, and has no authentication — do not expose it on a public interface.

## Usage

Run `thelancet-pp-cli --help` for the full command reference and flag list.

## Commands

### authors

Search and retrieve authors

- **`thelancet-pp-cli authors get`** - Get a single author by OpenAlex ID
- **`thelancet-pp-cli authors search`** - Search authors

### sources

Journal (source) metadata

- **`thelancet-pp-cli sources <id>`** - Get a journal (source) by ISSN or OpenAlex ID

### works

Search and retrieve Lancet articles

- **`thelancet-pp-cli works get`** - Get a single article by OpenAlex ID or DOI
- **`thelancet-pp-cli works search`** - Search Lancet articles (works)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
thelancet-pp-cli authors get mock-value

# JSON for scripting and agents
thelancet-pp-cli authors get mock-value --json

# Filter to specific fields
thelancet-pp-cli authors get mock-value --json --select id,name,status

# Dry run — show the request without sending
thelancet-pp-cli authors get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
thelancet-pp-cli authors get mock-value --agent
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
thelancet-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/lancet-via-openalex-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **analytics command says 'no local mirror'** — Run refresh first: thelancet-pp-cli refresh --journal lancet (analytics read the local OpenAlex-derived store)
- **rank-authors or mesh returns empty for an institution** — Institution names match OpenAlex display names; try a distinctive substring like 'Oxford' rather than a full legal name or acronym
- **OpenAlex returns 429 (rate limited)** — OpenAlex allows ~10 req/s; lower --rate-limit or set a contact email via OPENALEX_MAILTO to join the polite pool

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**pyalex**](https://github.com/J535D165/pyalex) — Python (250 stars)
- [**pubmed-cli**](https://github.com/tpapp/pubmed-cli) — Python (156 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

