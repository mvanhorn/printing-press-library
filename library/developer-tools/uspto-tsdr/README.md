# Uspto Tsdr CLI

Beginning on October 2, 2020, you will need an API key to access the TSDR REST API See https://account.uspto.gov/api-manager and the uspto's TSDR Data API webpage for more information on retrieving bulk data.

 Click on the Authorize box and enter your api key.  It is required and will be sent on all requests.

 This uses the uspto's swagger object with a number of changes.  The uspto's api does not allow browser request (CORS issues) so requests from this page will not actually work.  The generated curl commands will work and the modified swagger object can be imported into postman.

## Install

The recommended path installs both the `uspto-tsdr-pp-cli` binary and the `pp-uspto-tsdr` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install uspto-tsdr
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install uspto-tsdr --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/uspto-tsdr-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-uspto-tsdr --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-uspto-tsdr --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-uspto-tsdr skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-uspto-tsdr. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export TSDR_APIKEY_HEADER="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/tsdr-pp-cli/config.toml`.

### 3. Verify Setup

```bash
uspto-tsdr-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
uspto-tsdr-pp-cli case-multi-status --ids example-value
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Trademark intelligence
- **`trademark status`** — Full current state of a trademark in one command — mark text, status, owner, classes, filing/registration dates, attorney, and prosecution event count

  _Agents evaluating trademark status need the complete picture in one call instead of parsing XML manually_

  ```bash
  uspto-tsdr-pp-cli trademark status 97123456 --json
  ```
- **`trademark timeline`** — Every prosecution event in chronological order — office actions, examiner reviews, publication events, and registration milestones

  _Trademark attorneys need the full event history to evaluate prosecution strength and identify potential issues_

  ```bash
  uspto-tsdr-pp-cli trademark timeline 97123456 --json
  ```
- **`trademark docs`** — List all documents in the prosecution file — office actions, responses, specimens, registration certificates — with type and date filtering

  _Litigation prep and due diligence require reviewing every document in a trademark file without clicking through the TSDR web UI_

  ```bash
  uspto-tsdr-pp-cli trademark docs 97123456 --filter-type SPE --json
  ```

### Portfolio management
- **`trademark deadlines`** — Calculate Section 8, 9, and 15 maintenance deadlines with window-open dates and days-away countdown

  _Missing a maintenance deadline means losing the registration — this is the #1 pain point for trademark portfolio managers_

  ```bash
  uspto-tsdr-pp-cli trademark deadlines 97123456 --json
  ```
- **`trademark watch`** — Monitor multiple trademarks for status changes — caches previous statuses locally and flags any changes since last check

  _Agents monitoring trademark portfolios need change detection, not full status dumps they have to diff themselves_

  ```bash
  uspto-tsdr-pp-cli trademark watch 97123456 97654321 --json
  ```
- **`trademark batch`** — Batch status lookup for multiple trademarks using the multi-case endpoint or individual fallback with rate-limit throttling

  _IP paralegals managing hundreds of marks need batch status without manually checking each one_

  ```bash
  uspto-tsdr-pp-cli trademark batch 97123456 97654321 --json
  ```

## Usage

Run `uspto-tsdr-pp-cli --help` for the full command reference and flag list.

## Commands

### case-multi-status

Manage case multi status

- **`uspto-tsdr-pp-cli case-multi-status get-list`** - Parameters can be one of the following: rnXXXXXXX for US registration number, snXXXXXXXX for US serial number, refXXXXXXXX and irXXXXXXX for Madrid numbers. Example: https://tsdrapi.uspto.gov/ts/cd/caseMultiStatus/sn?ids=78787878,76767676

### casedoc

Manage casedoc


### casedocs

Manage casedocs

- **`uspto-tsdr-pp-cli casedocs get-bundle-info-pdf`** - Digits can be entered in one of the first four parameters. rnXXXXXXX for US registration number, snXXXXXXXX for US serial number, refXXXXXXXX and irXXXXXXX for Madrid numbers. Examples: https://tsdrapi.uspto.gov/ts/cd/casedocs/bundle.pdf?sn=75757575,78787878 or https://tsdrapi.uspto.gov/ts/cd/casedocs/bundle.pdf?sn=72131351,76515878&type=SPE Documents sent/received on Nov 30th, 2003 for Serial Number 72-131351 as a PDF https://tsdrapi.uspto.gov/ts/cd/casedocs/bundle.pdf?sn=72131351&date=2003-11-30  

Note: exactly one of rn,sn,ref,ir must be specified, like sn in the examples
- **`uspto-tsdr-pp-cli casedocs get-bundle-info-xml`** - Digits can be entered in one of the first four parameters. rnXXXXXXX for US registration number, snXXXXXXXX for US serial number, refXXXXXXXX and irXXXXXXX for Madrid numbers. Examples: https://tsdrapi.uspto.gov/ts/cd/casedocs/bundle.xml?sn=75757575,78787878 Metadata (in XML) about all documents for Serial Number 75008897 sent/received during 2006 https://tsdrapi.uspto.gov/ts/cd/casedocs/bundle.xml?sn=75008897&fromdate=2006-01-01&todate=2006-12-31 Metadata (in XML) about all documents for International Registration Number 0835690 sorted from earliest to latest https://tsdrapi.uspto.gov/ts/cd/casedocs/bundle.xml?ir=0835690&sort=date:A

Note: exactly one of rn,sn,ref,ir must be specified, like sn or ir in the examples
- **`uspto-tsdr-pp-cli casedocs get-bundle-info-zip`** - Parameters can be one of the following: rnXXXXXXX for US registration number, snXXXXXXXX for US serial number, refXXXXXXXX and irXXXXXXX for Madrid numbers. Example: https://tsdrapi.uspto.gov/ts/cd/casedocs/bundle.zip?sn=75757575,78787878

Note: only one parameter can be specified, like sn in the example

### casestatus

Manage casestatus


### raw-image

Manage raw image

- **`uspto-tsdr-pp-cli raw-image get-image`** - Parameter is the digits only of the serial number, no leading sn. Example: https://tsdrapi.uspto.gov/ts/cd/rawImage/78787878


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
uspto-tsdr-pp-cli case-multi-status --ids example-value

# JSON for scripting and agents
uspto-tsdr-pp-cli case-multi-status --ids example-value --json

# Filter to specific fields
uspto-tsdr-pp-cli case-multi-status --ids example-value --json --select id,name,status

# Dry run — show the request without sending
uspto-tsdr-pp-cli case-multi-status --ids example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
uspto-tsdr-pp-cli case-multi-status --ids example-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-uspto-tsdr -g
```

Then invoke `/pp-uspto-tsdr <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add uspto-tsdr uspto-tsdr-pp-mcp -e TSDR_APIKEY_HEADER=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/uspto-tsdr-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TSDR_APIKEY_HEADER` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "uspto-tsdr": {
      "command": "uspto-tsdr-pp-mcp",
      "env": {
        "TSDR_APIKEY_HEADER": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
uspto-tsdr-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/tsdr-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TSDR_APIKEY_HEADER` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `uspto-tsdr-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TSDR_APIKEY_HEADER`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
