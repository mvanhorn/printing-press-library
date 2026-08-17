# Disaster Shelter CLI (FEMA National Shelter System)

**Credible, real-time disaster shelter information for AI agents and people: which FEMA-tracked shelters are open right now, where they are, which take pets, and which are at capacity. Sourced from FEMA's National Shelter System OpenShelters feed, honest about missing data.**

Gives agents and people the most credible open-shelter picture straight from FEMA's National Shelter System (NSS) OpenShelters feed: open shelters across all states and territories, who takes pets, who is wheelchair / ADA accessible, and who is filling up. It answers the questions people actually ask in a disaster, like 'the closest open shelter to me that allows pets' and 'which shelters are at capacity', geocoding addresses when the feed omits coordinates and never inventing a number it does not have. Deep thanks to all first responders, emergency management practitioners, and relief nonprofit organizations for the work you do in communities when disaster strikes. This is an unofficial tool; in a life-threatening emergency call 911 and follow the official guidance and evacuation orders from FEMA, your local emergency management, and your local authorities.

## Install from source

This repository builds with the Go toolchain (1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/shelters/cmd/shelters-pp-cli@latest
```

Or clone and build the CLI plus the bundled MCP server:

```bash
git clone https://github.com/mvanhorn/printing-press-library
cd shelters-pp-cli
make build       # binaries in ./bin
go test ./...    # parsers verified against real + clearly-labeled synthetic fixtures in internal/cli/testdata
```

Then run `shelters-pp-cli doctor` (no API key needed) and `shelters-pp-cli brief --markdown` for a one-call situational briefing.

## Install via the Printing Press library

Once this CLI is published to the public library, one command installs both the `shelters-pp-cli` binary and the `pp-shelters` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install shelters
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install shelters --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install shelters --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install shelters --agent claude-code
npx -y @mvanhorn/printing-press-library install shelters --agent claude-code --agent codex
```

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shelters-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install shelters --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-shelters --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-shelters --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install shelters --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shelters-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/shelters/cmd/shelters-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "shelters": {
      "command": "shelters-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirm the CLI is wired up; no API key needed.
shelters-pp-cli doctor

# What shelters are open right now (few or none when no disaster is active).
shelters-pp-cli shelters

# The closest open shelters to me that allow pets.
shelters-pp-cli near "2400 W Bradley Ave, Champaign, IL" --pets

# One-call situational briefing with the gratitude and safety footer.
shelters-pp-cli brief --markdown

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Closest shelter
- **`near`** — Ranks open shelters by straight-line distance from a lat,lon, ZIP, or street address; geocodes shelters that are missing coordinates and reports any it cannot locate rather than dropping them.

  _Answers 'the closest shelter to me that allows pets' in one call; add --pets, --ada, or --wheelchair._

  ```bash
  shelters-pp-cli near 78566 --pets --limit 3
  ```

### Capacity
- **`capacity`** — Computes utilization only where both a population and a capacity are reported, labeling the denominator (evacuation vs post-impact), and surfaces shelters reported FULL without asserting it as ground truth.

  _Answers 'which shelter is at capacity?' honestly, marking unknowns as unknown._

  ```bash
  shelters-pp-cli capacity --state TX
  ```

### One-call situational awareness
- **`brief`** — One command returns the open count, a breakdown by state, pet-friendly and accessible counts, and the capacity picture, with an optional human briefing.

  _Reach for this first when an agent asks 'what is the shelter situation right now'._

  ```bash
  shelters-pp-cli brief --markdown
  ```

### Listings and detail
- **`shelters`** — Open shelters flattened from the feed and filterable by state, pets, ADA, wheelchair, managing org, and status.

  _Use to narrow open shelters to the ones that match a person's needs._

  ```bash
  shelters-pp-cli shelters --state FL --ada --json
  ```
- **`shelter`** — Full detail for one shelter joined on the stable shelter_id rather than the churning objectid, with unreported fields as explicit null.

  _Use when you have a shelter_id and need its full record._

  ```bash
  shelters-pp-cli shelter 368133
  ```

## Recipes

### Closest pet-friendly shelter as compact JSON for an agent

```bash
shelters-pp-cli near 29.76,-95.37 --pets --limit 1 --json --select data.shelters
```

### Shelters confirmed wheelchair accessible in one state

```bash
shelters-pp-cli shelters --state TX --wheelchair --json
```

### Which shelters are at or over capacity

```bash
shelters-pp-cli capacity --json --select data.shelters
```

### One-call situational briefing

```bash
shelters-pp-cli brief --markdown
```

## Output Formats

```bash
# Human-readable listing (default in terminal, JSON when piped)
shelters-pp-cli shelters

# JSON for scripting and agents
shelters-pp-cli shelters --json

# Filter the inner data to specific fields
shelters-pp-cli shelters --json --select data.count

# Dry run — short-circuit without sending a request
shelters-pp-cli shelters --dry-run

# Agent mode — JSON + compact + no prompts in one flag
shelters-pp-cli shelters --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select` returns only the fields you need
- **Honest** - missing data is null or "unknown", never invented
- **Read-only** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Every response carries `source` and a client-side `fetched_at` (UTC), because the feed has no server timestamp.

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Data, freshness, and honesty

- **Source:** FEMA National Shelter System OpenShelters feed (`gis.fema.gov`, ArcGIS Feature Service). No API key.
- **Freshness:** NSS reports roughly twice a day and only changes when an emergency manager updates a record, so status can lag reality. The feed carries no timestamp; this CLI stamps each response with the client fetch time.
- **Empty is normal:** a near-empty list means no disaster is active, not a failure. Counts spike during named events.
- **Coordinates:** frequently null even for open shelters; `near` geocodes from the street address and reports anything it cannot locate.
- **Capacity:** computed only where population and a capacity both exist; the denominator is labeled. Never assumed.
- **Full NSS** (beyond the public OpenShelters feed) requires an MOU with FEMA and is out of scope; `gis-links` points to it.

## Health Check

```bash
shelters-pp-cli doctor
```

Verifies configuration and connectivity to the feed.

## Configuration

Config file: `~/.config/shelters-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting

- **`shelters` is empty** — That is the normal quiet-state contract (no disaster active). It is not an error.
- **`near` says a shelter could not be located** — The feed omitted coordinates and the address would not geocode; those shelters are reported in a count and excluded from the ranking. Pass a precise `lat,lon` origin for the most accurate distances.
- **`capacity` shows "unknown" for a shelter** — The feed did not report both a population and a capacity for it; this tool will not invent a denominator.

### API-specific
- **shelters is empty** — That is the normal quiet-state contract (no disaster active). It is not an error.
- **near says a shelter could not be located** — The feed omitted coordinates and the address would not geocode; those shelters are reported in a count and excluded from the ranking. Pass a precise lat,lon origin for the most accurate distances.
- **capacity shows unknown for a shelter** — The feed did not report both a population and a capacity for it; this tool will not invent a denominator.

## Gratitude and safety

Thank you to all first responders, emergency management practitioners, and relief nonprofit organizations for the work you do in communities when disaster strikes.

This is an unofficial tool. FEMA's National Shelter System and your local emergency management are the authoritative sources. In a life-threatening emergency call 911 and follow official guidance and evacuation orders.
