# Openfda CLI

The FDA safety data terminal. Every drug adverse event, device recall, food contamination, and product label the FDA publishes — synced locally, searchable offline, with trend analysis no other tool offers.


## Install

The recommended path installs both the `openfda-pp-cli` binary and the `pp-openfda` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install openfda
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install openfda --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openfda-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-openfda --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-openfda --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-openfda skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-openfda. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export FDA_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/openfda-pp-cli/config.json`.

### 3. Verify Setup

```bash
openfda-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
openfda-pp-cli animal-events
```

## Usage

Run `openfda-pp-cli --help` for the full command reference and flag list.

## Commands

### animal-events

Animal drug and device adverse event reports.

- **`openfda-pp-cli animal-events list`** - Search animal adverse event reports

### device-510k

Premarket notification submissions demonstrating substantial equivalence.

- **`openfda-pp-cli device-510k list`** - Search 510(k) clearance records

### device-classification

Medical device product codes, specialty areas, and regulatory class.

- **`openfda-pp-cli device-classification list`** - Search device classifications

### device-covid19

COVID-19 serological testing evaluation data.

- **`openfda-pp-cli device-covid19 list`** - Search COVID-19 serology test evaluations

### device-events

Medical device adverse event reports (MAUDE/MDR) — injuries, deaths, malfunctions.

- **`openfda-pp-cli device-events count`** - Count device events by field
- **`openfda-pp-cli device-events list`** - Search device adverse event reports

### device-pma

Class III medical device premarket approval decisions.

- **`openfda-pp-cli device-pma list`** - Search premarket approval records

### device-recall-detail

Detailed device recall actions addressing defects or health risks.

- **`openfda-pp-cli device-recall-detail list`** - Search device recall action details

### device-recalls

Medical device recall enforcement reports.

- **`openfda-pp-cli device-recalls list`** - Search device recall enforcement reports

### device-registration

Medical device manufacturing establishment registrations and product listings.

- **`openfda-pp-cli device-registration list`** - Search device registrations and listings

### device-udi

Global Unique Device Identification Database (GUDID).

- **`openfda-pp-cli device-udi list`** - Search unique device identifiers

### drug-approvals

FDA-approved drug products since 1939 — applications, submissions, and marketing status.

- **`openfda-pp-cli drug-approvals list`** - Search approved drug products

### drug-events

Reports of drug side effects, medication errors, product quality problems (FAERS). 4.9M+ reports since 2003.

- **`openfda-pp-cli drug-events count`** - Count adverse events by field
- **`openfda-pp-cli drug-events list`** - Search drug adverse event reports

### drug-labels

Structured product information including prescribing info, black box warnings, indications.

- **`openfda-pp-cli drug-labels list`** - Search drug product labels

### drug-ndc

National Drug Code directory — product identifiers, packaging, and classification.

- **`openfda-pp-cli drug-ndc list`** - Search NDC directory

### drug-recalls

Drug product recall enforcement reports.

- **`openfda-pp-cli drug-recalls count`** - Count drug recalls by field
- **`openfda-pp-cli drug-recalls list`** - Search drug recall enforcement reports

### drug-shortages

Current and historical drug shortages from manufacturing issues, delays, and discontinuations.

- **`openfda-pp-cli drug-shortages list`** - Search drug shortages

### food-events

CAERS reports — food, dietary supplement, and cosmetic adverse events.

- **`openfda-pp-cli food-events list`** - Search food/supplement adverse event reports

### food-recalls

Food product recall enforcement reports.

- **`openfda-pp-cli food-recalls count`** - Count food recalls by field
- **`openfda-pp-cli food-recalls list`** - Search food recall enforcement reports

### nsde

Non-Standardized Drug Entities — drug names that don't map to standard terminology.

- **`openfda-pp-cli nsde list`** - Search non-standardized drug entities

### substance

Substance data from the FDA substance registration system.

- **`openfda-pp-cli substance list`** - Search substance records

### tobacco-problems

Tobacco product problem reports.

- **`openfda-pp-cli tobacco-problems list`** - Search tobacco problem reports


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
openfda-pp-cli animal-events

# JSON for scripting and agents
openfda-pp-cli animal-events --json

# Filter to specific fields
openfda-pp-cli animal-events --json --select id,name,status

# Dry run — show the request without sending
openfda-pp-cli animal-events --dry-run

# Agent mode — JSON + compact + no prompts in one flag
openfda-pp-cli animal-events --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-openfda -g
```

Then invoke `/pp-openfda <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add openfda openfda-pp-mcp -e FDA_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openfda-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FDA_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "openfda": {
      "command": "openfda-pp-mcp",
      "env": {
        "FDA_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
openfda-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/openfda-pp-cli/config.json`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FDA_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `openfda-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FDA_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
