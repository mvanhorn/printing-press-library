# Q-SYS CLI

**Q-SYS product specs, configuration procedures, and connection guides in one local index - with equipment-list compatibility checks neither QSC website can do.**

Q-SYS documentation is split across two sites: qsys.com carries the product spec sheets as PDFs, and help.qsys.com carries the configuration and networking guidance. Neither one can tell you whether a list of equipment runs on a given Designer version. This CLI harvests both into local SQLite, joins them into one record per product, and answers spec, configuration, wiring, and compatibility questions offline - including from a job site with no usable network.

## Install

The recommended path installs both the `qsys-pp-cli` binary and the `pp-qsys` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install qsys
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install qsys --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install qsys --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install qsys --agent claude-code
npx -y @mvanhorn/printing-press-library install qsys --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/qsys/cmd/qsys-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/qsys-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install qsys --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-qsys --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-qsys --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install qsys --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/qsys-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/qsys/cmd/qsys-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "qsys": {
      "command": "qsys-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the CLI is healthy before syncing anything
qsys-pp-cli doctor --dry-run

# build the local corpus from both vendor sites; this is the one slow step
qsys-pp-cli harvest

# full-text search across every synced page
qsys-pp-cli search "dante clocking"

# the unified card: overview, specs, config pages, wiring
qsys-pp-cli product get CX-Q

# check an equipment list against a Designer version
qsys-pp-cli compat check CX-Q TSC-70-G3 --qds 9.4

# confirm how much of each site actually parsed
qsys-pp-cli coverage

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### One record per product
- **`product get`** — See a Q-SYS product's overview, spec-sheet text, configuration pages, and connection guidance in one record.

  _Reach for this first for any question about a specific model; it answers spec, config, and wiring questions in one call instead of three._

  ```bash
  qsys-pp-cli product get CX-Q --agent
  ```

### Design-time safety checks
- **`compat check`** — Check a whole equipment list against a Q-SYS Designer version and get back what is supported and what is not.

  _Use this before quoting or commissioning to catch an unsupported part while it is still cheap to swap._

  ```bash
  qsys-pp-cli compat check CX-Q TSC-70-G3 NL-C4 --qds 9.4 --agent
  ```
- **`compat deprecated`** — Flag which models in a list are deprecated or discontinued before they reach a quote.

  _Use this to sanity-check a parts list; an end-of-life part caught at design time costs nothing, caught at order time costs a redesign._

  ```bash
  qsys-pp-cli compat deprecated CX-Q CXD-Q --agent
  ```
- **`bom verify`** — One report per model in an equipment list: version support, EOL status, and spec-sheet availability in a single pass.

  _Use this for the complete pre-quote check on a parts list instead of three separate lookups per part._

  ```bash
  qsys-pp-cli bom verify CX-Q TSC-70-G3 NL-C4 --qds 9.4 --agent
  ```

### Field answers
- **`connect`** — Get the networking, wiring, and I/O guidance that actually applies to a given model.

  _Use this for how-do-I-wire-this-in questions instead of reading the whole networking section._

  ```bash
  qsys-pp-cli connect TSC-70-G3 --agent
  ```
- **`integrations`** — Find which UC platforms (Teams, Zoom, Meet) a device is certified or integrated with.

  _Use this when a room design must match the client's chosen UC platform._

  ```bash
  qsys-pp-cli integrations TSC-70-G3 --agent
  ```

### Version-aware reads
- **`page get`** — Read a help page as of a specific Q-SYS Designer version from the versioned doc tree.

  _Use this when commissioning a system that runs an older Designer than today's docs describe._

  ```bash
  qsys-pp-cli page get control_router --version 9.4 --agent
  ```

### Trust the local copy
- **`coverage`** — Report how many products resolved a spec sheet and how many pages parsed, so extraction gaps are visible.

  _Run this after a harvest; a silent drop in coverage means the vendor changed their HTML and results are now incomplete._

  ```bash
  qsys-pp-cli coverage --agent
  ```

## Recipes

### Check a whole BOM against a Designer version

```bash
qsys-pp-cli bom verify --qds 9.4 --agent < bom.txt
```

Reads an equipment list from a file on stdin and returns a per-model report: version support, EOL status, and spec-sheet availability.

### Narrow a verbose product record for an agent

```bash
qsys-pp-cli product get CX-Q --agent --select model,family,spec_pdf_url,discontinued
```

Product records carry full spec-sheet text; --select trims the payload to just the fields needed so an agent does not burn context on prose.

### Read docs as an older Designer version saw them

```bash
qsys-pp-cli page get control_router --version 9.4
```

A site commissioned on 9.4 reads the 9.4 tree instead of silently getting today's 10.x behavior.

### Get wiring guidance for a touchscreen

```bash
qsys-pp-cli connect TSC-70-G3
```

Resolves the model to its family and returns only the networking and wiring pages that apply to it.

### Verify the local copy is complete

```bash
qsys-pp-cli coverage --agent
```

Reports spec-sheet match rate and page parse rate so a silent extraction regression is visible.

## Usage

Run `qsys-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `QSYS_CONFIG_DIR`, `QSYS_DATA_DIR`, `QSYS_STATE_DIR`, or `QSYS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `QSYS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export QSYS_HOME=/srv/qsys
qsys-pp-cli doctor
```

Under `QSYS_HOME=/srv/qsys`, the four dirs resolve to `/srv/qsys/config`, `/srv/qsys/data`, `/srv/qsys/state`, and `/srv/qsys/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "qsys": {
      "command": "qsys-pp-mcp",
      "env": {
        "QSYS_HOME": "/srv/qsys"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `QSYS_DATA_DIR` overrides an explicit `--home` for that kind. Use `QSYS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `QSYS_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `qsys-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### compat

Hardware and software compatibility matrices

- **`qsys-pp-cli compat by-product`** - List the Q-SYS Designer versions and compatibility notes for a hardware product, per the compatibility matrix
- **`qsys-pp-cli compat by-version`** - List hardware support by Q-SYS Designer version: which hardware was added or removed in each release of the compatibility matrix
- **`qsys-pp-cli compat deprecations`** - List deprecated hardware and feature notices with the release in which each item was deprecated
- **`qsys-pp-cli compat upgrade-path`** - Show firmware and Q-SYS Designer upgrade path requirements, including the supported upgrade sequences

### networking

Connection, wiring, and network setup guidance

- **`qsys-pp-cli networking <topic>`** - Fetch a Q-SYS networking or connection guidance page

### page

Q-SYS Help documentation pages (configuration, networking, hardware)

- **`qsys-pp-cli page get`** - Fetch a Q-SYS Help documentation page as clean text
- **`qsys-pp-cli page index`** - Fetch the Q-SYS Help sitemap listing every documentation page

### product

Q-SYS product pages and spec sheets on qsys.com

- **`qsys-pp-cli product index`** - Fetch the qsys.com sitemap listing every product page
- **`qsys-pp-cli product page`** - Fetch a qsys.com product page as clean text
- **`qsys-pp-cli product resources`** - List spec-sheet and manual PDF links for a product


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`qsys-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`qsys-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`qsys-pp-cli learnings list`** - Inspect taught rows
- **`qsys-pp-cli learnings forget <query>`** - Undo a teach
- **`qsys-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`qsys-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`qsys-pp-cli teach-pattern`** - Install a query/resource template up front
- **`qsys-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `QSYS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `qsys-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
qsys-pp-cli networking mock-value

# JSON for scripting and agents
qsys-pp-cli networking mock-value --json

# Filter to specific fields
qsys-pp-cli networking mock-value --json --select id,name,status

# Dry run — show the request without sending
qsys-pp-cli networking mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
qsys-pp-cli networking mock-value --agent
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
qsys-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `qsys-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/qsys-pp-cli/config.toml`; `--home`, `QSYS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **product get returns no spec text** — Run `qsys-pp-cli coverage` - the product page may not link a spec sheet, and the source PDF URL is still returned.
- **search returns nothing after install** — Run `qsys-pp-cli harvest` first; the corpus is empty until the initial harvest completes.
- **compat check reports a model as unknown** — Model naming varies between the spec sheets and the compatibility matrix; try the series name (CX-Q) rather than a specific SKU (CX-Q 8K8).
- **harvest is slow** — Expected - the initial sync walks both sitemaps and fetches spec PDFs. Use `qsys-pp-cli harvest --only products --limit 25` to narrow it.
- **page get --version returns a 404 for a version** — Only released version trees are served (9.4, 9.6, and 10.0 verified); a misspelled or unreleased version 404s from help.qsys.com.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**qrwc**](https://github.com/qsys-sd/qrwc) — JavaScript
- [**qrc-client-js**](https://github.com/qsys-tools/qrc-client-js) — JavaScript
- [**qsys-qrc-py**](https://github.com/VideoGameRoulette/qsys-qrc-py) — Python
- [**qsys**](https://github.com/gagehelton/qsys) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
