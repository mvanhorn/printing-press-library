# Q-SYS CLI

**Q-SYS specs, configuration, wiring, compatibility, and fault articles in one offline index - answering equipment-list questions no QSC website can take as input.**

QSC splits every integrator answer across three sites: qsys.com carries the spec-sheet PDFs where the electrical numbers live, help.qsys.com carries configuration and compatibility with zero electrical specs, and support.qsys.com carries the FAQ, application notes, and fault articles. None of them accepts a list of models. This CLI harvests all three into local SQLite, joins them per product, and answers spec, configuration, wiring, compatibility, and fault questions offline - including from a job site with no usable network.

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

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

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

> **Concurrency warning:** run only ONE qsys-pp-cli process at a time. Two or more
> concurrent invocations against the same store can crash with SIGBUS (~50% at just
> 2 readers) due to an upstream SQLite WAL-index mmap defect
> ([cli-printing-press#4349](https://github.com/mvanhorn/cli-printing-press/issues/4349)).
> Serial access is 100% safe. Do not fan out parallel MCP tool calls or `&` jobs.

```bash
# confirm the CLI is healthy before harvesting anything
qsys-pp-cli doctor --dry-run

# build the local corpus from all three vendor sites; this is the one slow step
qsys-pp-cli harvest --timeout 900s

# confirm spec-sheet PDFs were text-extracted, not just linked
qsys-pp-cli coverage

# the unified card: specs, config, wiring, gotchas
qsys-pp-cli product get CX-Q

# the pre-quote sweep across an equipment list
qsys-pp-cli bom verify CX-Q TSC-70-G3 --qds 10.0

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### One record, three sources
- **`product get`** — See a Q-SYS product's specs, configuration pages, wiring guidance, known gotchas, and factory-reset procedure in one record.

  _Reach for this first for any question about a single model; it answers spec, config, wiring, and gotcha questions in one call instead of four._

  ```bash
  qsys-pp-cli product get CX-Q --agent
  ```
- **`connect`** — Get the networking, wiring, and I/O guidance that actually applies to a given model, including third-party application notes.

  _Use this for how-do-I-wire-this-in questions instead of reading an entire networking section._

  ```bash
  qsys-pp-cli connect TSC-70-G3 --agent
  ```

### Equipment-list answers no vendor site can give
- **`bom verify`** — One report per model in an equipment list: Designer-version support, end-of-life status, LTS carry date, and spec-sheet availability.

  _Use this before quoting to catch an unsupported or end-of-life part while a swap is still free._

  ```bash
  qsys-pp-cli bom verify CX-Q TSC-70-G3 NL-C4 --qds 10.0 --agent
  ```
- **`bom risks`** — Surface every known issue, awareness note, and troubleshooting article that touches any model on an equipment list, filtered to a Designer release.

  _Use this alongside bom verify to find the problems that are documented but not reflected in the compatibility matrix._

  ```bash
  qsys-pp-cli bom risks CX-Q TSC-70-G3 --qds 10.0 --agent
  ```
- **`compat check`** — Check a whole equipment list against a Q-SYS Designer version and get back what is supported and what is not.

  _Use this for the fast supported/not-supported answer when the client has standardized on a Designer version._

  ```bash
  qsys-pp-cli compat check CX-Q TSC-70-G3 NL-C4 --qds 10.0 --agent
  ```

### Release and fault intelligence
- **`qds`** — For one Q-SYS Designer release: known issues, LTS status and end date, and which hardware was removed.

  _Use this when deciding whether to standardize a site on a Designer release, or before recommending an upgrade._

  ```bash
  qsys-pp-cli qds 10.0 --agent
  ```
- **`fault`** — Paste the literal fault or status string Q-SYS Designer displays and get the article that explains it, plus the models it applies to.

  _Use this on site when Designer shows a fault and the job network cannot reach a search engine._

  ```bash
  qsys-pp-cli fault "LAN A Streaming Error - Not Connected" --agent
  ```

### Trust the local copy
- **`coverage`** — Report per source how many pages parsed, how many spec-sheet PDFs were linked versus actually text-extracted, and how many support articles were indexed.

  _Run this after a harvest; a drop in extracted-PDF count means specs silently stopped being searchable._

  ```bash
  qsys-pp-cli coverage --agent
  ```

## Recipes

### First run: build the corpus

```bash
qsys-pp-cli harvest --timeout 900s
qsys-pp-cli coverage
```

`harvest` walks all three vendor sitemaps (help.qsys.com, qsys.com, and support.qsys.com) into the local corpus; every other command reads it. `coverage` then reports how many products resolved a spec sheet, how many pages parsed, and how many support articles were indexed, so an incomplete harvest is visible instead of silent. Narrow a first pass with `--only products --limit 25`, add `--with-pdfs` when spec-sheet text is needed, and use `--only support` for the knowledge base that `fault`, `bom risks`, and `qds` read.

### Pre-quote sweep across an equipment list

```bash
qsys-pp-cli bom verify CX-Q TSC-70-G3 NL-C4 --qds 10.0 --agent
```

Returns one row per model with version support, end-of-life status, LTS carry date, and spec-sheet URL.

### Find the documented problems on a list

```bash
qsys-pp-cli bom risks CX-Q TSC-70-G3 --qds 10.0 --agent
```

Surfaces known-issue, awareness, and troubleshooting articles touching any model on the list.

### Resolve a fault string from a Designer screen

```bash
qsys-pp-cli fault "LAN A Streaming Error - Not Connected" --agent
```

Matches the literal string against error/status and troubleshooting article titles and bodies.

### Narrow a verbose product record for an agent

```bash
qsys-pp-cli product get CX-Q --agent --select model,family,spec_pdf_url,discontinued
```

Product records carry full spec-sheet text; --select trims the payload so an agent does not burn context on prose.

### Decide whether to standardize on a Designer release

```bash
qsys-pp-cli qds 10.0 --agent
```

Known issues, LTS status and end date, and hardware removed in that release.

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

### harvest — build the local corpus (run this first)

Walks all three vendor sitemaps and builds the local corpus that every other command reads. The full harvest fetches roughly 750 help pages, 270 product pages, and 1,900 support articles, rate limited to be polite to the vendor servers.

- **`qsys-pp-cli harvest`** - Build the whole corpus from help.qsys.com, qsys.com, and support.qsys.com
- **`qsys-pp-cli harvest --only pages|products|compat|support`** - Harvest one source instead of all three
- **`qsys-pp-cli harvest --only support`** - Harvest support.qsys.com; required for `fault`, `bom risks`, and `qds`
- **`qsys-pp-cli harvest --limit 25`** - Cap items per source
- **`qsys-pp-cli harvest --with-pdfs`** - Also download and text-extract spec-sheet PDFs (slower; needs `pdftotext`)

> `harvest` is not the same command as the top-level `sync`. Top-level `sync` walks the generated endpoint resources and refreshes entity lookups; it does not build the corpus. Run `qsys-pp-cli coverage` after a harvest to confirm how much of each site actually parsed.

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

### support

Q-SYS support knowledge base: FAQ, application notes, awareness, troubleshooting, error/status messages

- **`qsys-pp-cli support article`** - Fetch a Q-SYS support article as clean text
- **`qsys-pp-cli support index`** - Fetch the support.qsys.com sitemap listing every knowledge-base article


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
# Filter to specific fields by name
qsys-pp-cli networking mock-value --json --select <field>[,<field>...]

# Dry run — show the request without sending
qsys-pp-cli networking mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
qsys-pp-cli networking mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
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
- **coverage reports PDFs linked but 0 text-extracted** — re-run 'qsys-pp-cli harvest --only products --with-pdfs --timeout 900s'; text extraction needs pdftotext on PATH
- **harvest dies at the FTS rebuild step** — pass --timeout 900s; the 60s default kills a full product harvest before the search index finishes
- **doctor reports 0 rows but coverage reports thousands** — expected - harvest fills the domain tables and does not write sync_state; judge the corpus by 'coverage', not by doctor's cache section
- **a product returns no spec text** — about a third of Q-SYS products publish no spec-sheet PDF; check 'coverage' for the linked-vs-extracted split before assuming a bug

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**q-sys-mcp**](https://github.com/tomsfaire/q-sys-mcp) — JavaScript
- [**qrc-client-js**](https://github.com/qsys-tools/qrc-client-js) — JavaScript
- [**QSC-QSYS-Launcher**](https://github.com/mckay115/QSC-QSYS-Launcher) — C#

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
