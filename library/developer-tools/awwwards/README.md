# Awwwards CLI

**Query Awwwards jury scores, palettes, and tech stacks from a local SQLite mirror - multi-filter search, trend deltas, and one-shot design briefings the site itself can't do.**

Awwwards jury-scores the best web design in the world, but that intelligence is locked in server-rendered HTML with one-filter-at-a-time browsing. This CLI mirrors cards, scores, palettes, and tags into local SQLite, then answers questions the site cannot: multi-filter intersections via 'find', trend deltas via 'trends', and one-shot agent design briefings via 'context-pack'.

Learn more at [Awwwards](https://www.awwwards.com).

## Install

The recommended path installs both the `awwwards-pp-cli` binary and the `pp-awwwards` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install awwwards
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install awwwards --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install awwwards --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install awwwards --agent claude-code
npx -y @mvanhorn/printing-press-library install awwwards --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/cmd/awwwards-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/awwwards-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install awwwards --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-awwwards --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-awwwards --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install awwwards --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/awwwards-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/cmd/awwwards-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "awwwards": {
      "command": "awwwards-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Sanity-check the CLI plumbing (drop --dry-run to also probe awwwards.com)
awwwards-pp-cli doctor --dry-run

# Today's newest award entries with tags, dates, and thumbnail URLs
awwwards-pp-cli latest --json

# Mirror recent winners plus jury scores and palettes into local SQLite - powers every analytics command
awwwards-pp-cli mirror --pages 5 --details

# Multi-filter AND intersection the site itself cannot do (one filter at a time server-side)
awwwards-pp-cli find --tag clean --tech gsap --json

# Deep-dive one winner: jury scores by dimension, color palette, tech stack
awwwards-pp-cli inspect monolog --json

# One-shot design briefing: reference sites + palettes + tech + score benchmarks
awwwards-pp-cli context-pack --category portfolio --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Trend and profile analytics
- **`trends`** — Quantify what's rising and falling in award-winning design: tag, color-family, and tech frequency over a time window, with window-over-window deltas.

  _Reach for this when you need current design-language evidence ("is dark mode still dominant?") instead of training-data taste. Prerequisite: run 'awwwards-pp-cli mirror --pages 10' first (add --details for the color axis)._

  ```bash
  awwwards-pp-cli trends --by tag --since 90d --agent
  ```
- **`studio`** — Aggregated award profile of one agency or studio: wins by tier, average dimension scores, and dominant tags and tech.

  _Use to study how a specific top studio wins: their score profile and signature techniques in one call. Prerequisite: credits come from detail pages - run 'awwwards-pp-cli mirror --details' first._

  ```bash
  awwwards-pp-cli studio obys --json
  ```

### Design context for agents
- **`context-pack`** — One-shot design briefing for a build: top-scoring reference sites, dominant palettes, recurring tech, co-occurring style tags, and jury-score benchmarks for a category or style.

  _Run this first when designing a site: it turns "what does great look like for this kind of page" into machine-readable input. Prerequisite: run 'awwwards-pp-cli mirror --pages 10 --details' first; palettes and score benchmarks come from detail data._

  ```bash
  awwwards-pp-cli context-pack --category e-commerce --agent
  ```
- **`palette-match`** — Find award-winning sites whose palette contains a color near a target hex, ranked by RGB distance.

  _Use when a brand color is fixed and you need proof of how top-rated designs deploy colors like it. Prerequisite: palette rows come from detail pages - run 'awwwards-pp-cli mirror --details' first._

  ```bash
  awwwards-pp-cli palette-match "#0F4C81" --distance 25 --json
  ```
- **`elements-top`** — Section-level inspiration ranked by quality: heroes, footers, or 404 pages from sites whose jury score clears your bar.

  _Use before building a specific page section: it returns only the sections from provably high-scoring sites. Prerequisite: run 'awwwards-pp-cli mirror --elements <type> --details' first._

  ```bash
  awwwards-pp-cli elements-top hero --dim design --min 8 --json
  ```

## Recipes

### Prime the local design mirror (run once before any analytics)

```bash
awwwards-pp-cli mirror --pages 5 --details --elements hero
```

Mirrors cards, jury scores, palettes, credits, and hero elements into local SQLite - every analytics recipe below reads this mirror.

### Ground an agent before designing a landing page

```bash
awwwards-pp-cli context-pack --category e-commerce --agent
```

Returns top reference sites, dominant palettes, recurring tech, and score benchmarks as one JSON document an agent can design from.

### Check what's trending in award-winning design

```bash
awwwards-pp-cli trends --by tech --since 90d --vs 90d --json
```

Tech frequency this quarter vs last - cite actual counts, not vibes.

### Narrow winners across filters, agent-shaped

```bash
awwwards-pp-cli find --tag dark --tech gsap --agent --select items.slug,items.title,items.tags,items.thumbnail_url
```

Client-side AND-intersection across filters with dotted-path field narrowing so the agent parses only what it needs.

### Steal the palette strategy of a fixed brand color

```bash
awwwards-pp-cli palette-match "#0F4C81" --distance 25 --json
```

Finds winners whose extracted palette contains a near-match, ranked by RGB distance.

### Study only the best heroes before building one

```bash
awwwards-pp-cli elements-top hero --dim design --min 8 --json
```

Section screenshots joined to parent-site jury scores - inspiration filtered by proof of quality.

## Usage

Run `awwwards-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as runtime state |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `AWWWARDS_CONFIG_DIR`, `AWWWARDS_DATA_DIR`, `AWWWARDS_STATE_DIR`, or `AWWWARDS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `AWWWARDS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export AWWWARDS_HOME=/srv/awwwards
awwwards-pp-cli doctor
```

Under `AWWWARDS_HOME=/srv/awwwards`, the four dirs resolve to `/srv/awwwards/config`, `/srv/awwwards/data`, `/srv/awwwards/state`, and `/srv/awwwards/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "awwwards": {
      "command": "awwwards-pp-mcp",
      "env": {
        "AWWWARDS_HOME": "/srv/awwwards"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `AWWWARDS_DATA_DIR` overrides an explicit `--home` for that kind. Use `AWWWARDS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `AWWWARDS_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `awwwards-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### collections

Curated theme boards (dark-mode, hot-right-now, ai-powered-web-projects...)

- **`awwwards-pp-cli collections get`** - Fetch one curated collection's site grid (owner username + collection slug)
- **`awwwards-pp-cli collections list`** - List curated collections

### directory

Agencies and freelancers directory with country/specialty filters

- **`awwwards-pp-cli directory browse`** - Browse the directory by one filter: specialty (freelance, agency-studio, art-direction, graphic-design, interactive) or country
- **`awwwards-pp-cli directory list`** - List top agencies and freelancers

### elements

Section-level design inspiration (heroes, footers, 404 pages, navigation...)

- **`awwwards-pp-cli elements <type>`** - Browse tagged screenshots of individual page sections by type: hero, footer, 404_page, about_us, animation, branding, contact, forms, gallery, header, icons, illustration, interaction, layout, loading, login_and_sign_up, maps

### sites

Individual award-winning site detail pages (scores, jury notes, palette, tags)

- **`awwwards-pp-cli sites content`** - Fetch the lightweight content partial for a site (same data, less page chrome)
- **`awwwards-pp-cli sites get`** - Fetch a site's detail page: overall + per-dimension scores, jury votes, color palette, tags, tech stack

### websites

Browse award-winning websites (listings with embedded card data)

- **`awwwards-pp-cli websites browse`** - Browse websites by one filter: award tier (sites_of_the_day, sites_of_the_month, sites_of_the_year, nominees, honorable-mentions, developer-award), category (e-commerce, architecture), tag/tech (gsap, webflow, three-js, clean), country (france), color (%23FFFFFF), or font (Aeonik)
- **`awwwards-pp-cli websites list`** - List the latest websites; supports text search and pagination


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
awwwards-pp-cli collections list

# JSON for scripting and agents
awwwards-pp-cli collections list --json

# Filter to specific fields
awwwards-pp-cli collections list --json --select id,name,status

# Dry run — show the request without sending
awwwards-pp-cli collections list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
awwwards-pp-cli collections list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
awwwards-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `awwwards-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/awwwards-pp-cli/config.toml`; `--home`, `AWWWARDS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Analytics commands (trends, context-pack, palette-match) return empty results** — Run 'awwwards-pp-cli mirror --pages 10 --details' first - these commands read the local design mirror, not the live site
- **HTTP 429 or slow responses during mirror** — Lower the page count ('mirror --pages 3') and re-run later; the built-in rate limiter backs off automatically
- **A filter returns no results ('find --tag <x>')** — Tags are exact strings from Awwwards cards (e.g. 'Three.js', 'GSAP', 'Clean'); run 'awwwards-pp-cli trends --by tag' to see the live tag vocabulary
- **'sites get <slug>' returns page metadata but you expected scores** — Generated endpoint commands return raw page data; use 'inspect <slug>' for parsed scores, jury votes, and palette
- **elements-top returns items but none ranked, or all unjoined** — Elements rank against their parent site's scores, which come from detail pages. Feed both first: run 'awwwards-pp-cli mirror --elements hero --details'

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.awwwards.com/
- Capture coverage: 0 API entries from 14 total network entries
- Reachability: standard_http (95% confidence)
- Protocols: html_scrape (55% confidence)
- Auth signals: none

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
