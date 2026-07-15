# TED (Tenders Electronic Daily) CLI

**The entire EU public procurement corpus — €815B/year — searchable offline, with B2B lead generation for construction contract winners.**

TED publishes every significant EU public contract (€676K notices/year) but the web interface makes analysis nearly impossible. This CLI syncs the corpus to SQLite, then layers market intelligence nobody else offers: win rates, concentration scores, dark-buyer detection, and opportunity scoring — free, composable, agent-native.

Created by [@m91michel](https://github.com/m91michel) (Mathias Michel).

## Install

The recommended path installs both the `eu-tenders-pp-cli` binary and the `pp-eu-tenders` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install eu-tenders
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install eu-tenders --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install eu-tenders --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install eu-tenders --agent claude-code
npx -y @mvanhorn/printing-press-library install eu-tenders --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/eu-tenders/cmd/eu-tenders-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/eu-tenders-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install eu-tenders --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-eu-tenders --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-eu-tenders --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install eu-tenders --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/eu-tenders-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/eu-tenders/cmd/eu-tenders-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "eu-tenders": {
      "command": "eu-tenders-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Find recent construction contract winners who might need rented machinery
eu-tenders-pp-cli leads --cpv 45 --country DEU --min-value 500000 --days 30

# Find open IT contracts in Germany
eu-tenders-pp-cli notices search --country DEU --cpv 72000000 --scope ACTIVE --limit 10

# Pull matching notices into local SQLite for offline analysis
eu-tenders-pp-cli sync --country DEU --cpv 72000000 --since 2023-01-01

# Rank open opportunities by urgency, value, and fit
eu-tenders-pp-cli score --keywords "cloud migration" --country DEU --max-days 30

# Morning briefing: highest-priority expiring tenders
eu-tenders-pp-cli deadline-heat --country DEU --cpv 72 --days 14

# Is this market open? See incumbent repeat rates before writing a proposal
eu-tenders-pp-cli win-rate --cpv 72000000 --country DEU --min-calls 5

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Lead generation
- **`leads`** — Surface recent construction contract award winners as B2B outreach candidates — company name, project location, contract value, and construction type — so you can contact winners who need construction machinery.

  _Reach for this when prospecting construction companies for B2B outreach — it identifies firms that just won large projects and will need equipment, subcontractors, and services in the near term._

  ```bash
  eu-tenders-pp-cli leads --cpv 45 --country DEU --region NW --min-value 500000 --days 30 --agent --select leads.winner_name,leads.project_location,leads.contract_value,leads.contact_url
  ```

### Market intelligence
- **`win-rate`** — See what fraction of contract competitions in a market go to new winners vs. incumbents — your real odds before writing a proposal.

  _Reach for this when deciding whether a procurement market is worth entering — high repeat-winner rate means the contract is effectively pre-determined._

  ```bash
  eu-tenders-pp-cli win-rate --cpv 72000000 --country FRA --min-calls 5 --show-winners --agent
  ```
- **`concentration`** — Compute which companies capture what share of awarded contract value in a sector and country, with HHI score and year-over-year trend.

  _Reach for this when researching market structure, competitive dynamics, or potential regulatory concentration issues in EU procurement._

  ```bash
  eu-tenders-pp-cli concentration --cpv 72000000 --country DEU --top 5 --since 2022-01-01 --agent
  ```
- **`velocity`** — See whether a procurement market is heating up, cooling off, or spiking — weekly notice count trends over rolling windows vs. same period last year.

  _Use this to spot demand surges in your sector before competitors react — especially useful after policy announcements or budget cycles._

  ```bash
  eu-tenders-pp-cli velocity --country DEU --cpv 72000000 --window 90d --compare 1y --agent
  ```
- **`cpv-drift`** — See which procurement categories are growing or shrinking in a country's spending mix year-over-year — essential for platform builders and policy researchers.

  _Use this to identify which procurement markets are expanding and worth investing in — particularly useful after major policy shifts like digital transformation mandates._

  ```bash
  eu-tenders-pp-cli cpv-drift --country DEU --since 2020-01-01 --top 20 --metric value --agent
  ```

### Bid intelligence
- **`score`** — Get a ranked shortlist of open tenders scored by deadline urgency, contract value, keyword fit, and market openness — your morning briefing, prioritized.

  _Use this when you need to decide which tenders deserve proposal effort today — it surfaces fit + urgency + openness in a single ranked list._

  ```bash
  eu-tenders-pp-cli score --keywords "cloud migration" --country DEU --cpv 72 --min-value 500000 --max-days 30 --agent
  ```
- **`buyer`** — Build a full procurement dossier on any contracting authority: their spending cadence, CPV mix, typical contract values, and repeat winner patterns.

  _Reach for this when preparing a bid to a specific buyer — understand their preferences, typical timelines, and incumbent relationships before writing._

  ```bash
  eu-tenders-pp-cli buyer --name "Bundesagentur für Arbeit" --since 2020-01-01 --show-winners --agent
  ```
- **`deadline-heat`** — A ranked calendar of expiring tenders weighted by urgency × value / competition density — your daily prioritized view of what needs attention now.

  _Use this as a daily morning briefing command — it surfaces high-urgency, high-value, low-competition opportunities that a simple deadline sort misses._

  ```bash
  eu-tenders-pp-cli deadline-heat --country DEU --cpv 72 --days 14 --min-value 200000 --agent
  ```

### Compliance & integrity
- **`dark-buyers`** — Surface contracting authorities whose calls-for-tender rarely produce public awards, or whose awards show suspiciously low winner diversity — a compliance and integrity signal.

  _Reach for this when investigating procurement compliance, building risk scores for contracting authorities, or preparing investigative journalism._

  ```bash
  eu-tenders-pp-cli dark-buyers --country POL --cpv 45000000 --since 2022-01-01 --min-calls 3 --agent
  ```

## Usage

Run `eu-tenders-pp-cli --help` for the full command reference and flag list.

## Commands

### notices

Manage notices

- **`eu-tenders-pp-cli notices search`** - Search for notices using expert search query. More information about the query format and field names can be found on [this page](https://ted.europa.eu/en/search/expert-search))

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
eu-tenders-pp-cli notices

# JSON for scripting and agents
eu-tenders-pp-cli notices --json

# Filter to specific fields
eu-tenders-pp-cli notices --json --select id,name,status

# Dry run — show the request without sending
eu-tenders-pp-cli notices --dry-run

# Agent mode — JSON + compact + no prompts in one flag
eu-tenders-pp-cli notices --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
eu-tenders-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/eu-tenders-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **timedOut: true in response** — Add --limit 50 or narrow query with more filters; TED search times out at 30s
- **Empty results on narrow query** — Try --scope ALL (not ACTIVE) — the notice may be published but deadline passed
- **sync takes very long** — Add --since 2024-01-01 to scope the sync window; iteration mode pulls unlimited but takes time for large corpora
- **CPV code not found** — Run 'eu-tenders-pp-cli cpv search <keyword>' to find the right 8-digit code

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**OP-TED/eForms-SDK**](https://github.com/OP-TED/eForms-SDK) — Java (67 stars)
- [**pudo/ted**](https://github.com/pudo/ted) — Python (45 stars)
- [**ONSBigData/ExtracTED**](https://github.com/ONSBigData/ExtracTED) — Python (22 stars)
- [**fbuchner/ted-mcp**](https://github.com/fbuchner/ted-mcp) — TypeScript (12 stars)
- [**flexponsive/tap-eu-ted**](https://github.com/flexponsive/tap-eu-ted) — Python (8 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
