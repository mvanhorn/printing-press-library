# Crestron CLI

**Every Crestron product, spec sheet, and firmware release — searchable offline, with fleet-wide currency checks the website cannot do.**

Crestron.com has no public API and no way to ask a question across more than one product at a time. This CLI mirrors the product catalog, the documentation library, and the firmware release history into local SQLite, then answers the questions integrators actually have: what firmware covers my installed models, what changed between two versions, which parts are discontinued and what replaced them. Sign in and it also unlocks release notes and firmware downloads.

Learn more at [Crestron](https://www.crestron.com).

Created by [@Drummerms](https://github.com/Drummerms) (drummerms).

## Install

The recommended path installs both the `crestron-pp-cli` binary and the `pp-crestron` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install crestron
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install crestron --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install crestron --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install crestron --agent claude-code
npx -y @mvanhorn/printing-press-library install crestron --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/crestron/cmd/crestron-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/crestron-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install crestron --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-crestron --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-crestron --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install crestron --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
crestron-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/crestron-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/crestron/cmd/crestron-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "crestron": {
      "command": "crestron-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Most of this CLI works with no account at all: the product catalog, specifications, spec sheets, manuals, certificates, CAD and Revit files, and firmware version numbers and release dates are all public. A Crestron account unlocks two more things — firmware release notes and the firmware binaries themselves. Run `crestron-pp-cli auth login --chrome` and the CLI imports your existing Crestron.com session cookies straight from Chrome; it never asks for or stores your password.

## Quick Start

```bash
# confirm the CLI can reach Crestron.com before syncing anything
crestron-pp-cli doctor --dry-run

# build the local catalog mirror; this is what makes offline search possible
crestron-pp-cli sync --resources products,categories

# look up a model by number to confirm the mirror is populated
crestron-pp-cli product get DM-NVX-360

# read the full specification table as structured output
crestron-pp-cli specs show DM-NVX-360 --agent

# check whether a part is still sellable and what replaced it
crestron-pp-cli lifecycle UC-FCM-Z --agent

```

## Known Gaps

- **`tail` has nothing to stream.** Crestron.com serves server-rendered HTML
  with no JSON list endpoints, so the generic polling command ships with an
  empty resource list and reports `resources: []` for every invocation. Use
  `sync` on a schedule and diff the mirror instead.
- **`--max-age` is accepted but inert.** The global flag parses and is
  documented, but no staleness hint is emitted, so raising or lowering it
  changes nothing. Check mirror age with `doctor` until this is wired up.
- **Discontinued products are not in the mirror.** Crestron's sitemap does not
  expose the Inactive/Discontinued catalog tree, so `sync` stores zero
  discontinued products and `lifecycle` silently falls back to a live lookup for
  them (its `note` field says so when it does).
- **Every MCP tool reports as auth-required.** The spec declares cookie auth
  globally, so all 13 tools advertise `auth-required` even though only the
  firmware pages are actually gated. Read-only catalog and spec tools still work
  unauthenticated; the label is wrong, not the behavior.
- **Mirrors synced before this release carry page footer text in release
  notes.** The extractor now trims the site footer, and `firmware diff` trims it
  on read, but the full-text index still contains it until you re-run
  `sync --notes`. Until then a search for a footer word (for example
  "Sustainability") matches nearly every release.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Fleet lifecycle intelligence
- **`fleet status`** — Check every model in your installed fleet against current firmware in one command.

  _Reach for this instead of checking models one at a time; it also catches releases that a per-model search would miss because the release is titled under a sibling model._

  ```bash
  crestron-pp-cli fleet status --file fleet.txt --agent
  ```
- **`lifecycle`** — Report whether a model is still sellable and trace its replacement chain.

  _Reach for this when triaging an as-built list to find which parts can still be ordered and what replaced the rest._

  ```bash
  crestron-pp-cli lifecycle UC-FCM-Z --agent
  ```

### Firmware knowledge base
- **`search`** — Search every firmware release note and change log at once for a term.

  _Use this to answer 'which version fixed X' without opening a dozen version pages._

  ```bash
  crestron-pp-cli search "HDCP" --type firmware_release --agent
  ```
- **`firmware diff`** — Show what changed between two firmware versions for a model.

  _Pick this when deciding whether an upgrade is worth scheduling on a live site._

  ```bash
  crestron-pp-cli firmware diff DM-NVX-384 7.3.5149.23092 7.4.0255.22319 --agent
  ```

### Design and submittal workflow
- **`submittal`** — Download every documentation asset for a list of models into per-model folders with a coverage report.

  _Use this to assemble a CSI submittal package in one step instead of hundreds of individual downloads._

  ```bash
  crestron-pp-cli submittal DM-NVX-384 --agent
  ```
- **`specs compare`** — Compare two models field by field across the full specification table.

  _Use this when choosing between sibling models in the same series, which often differ in only a few spec rows._

  ```bash
  crestron-pp-cli specs compare DM-NVX-360 DM-NVX-363 --agent
  ```

## Recipes

### First run: build the local mirror

```bash
crestron-pp-cli sync
crestron-pp-cli product get DM-NVX-360
```

`sync` crawls Crestron.com into the local mirror; `search`, `specs`, `fleet`, `lifecycle`, and `submittal` all read it and return nothing until it has run. Add `--notes` with a signed-in session to make release notes and change logs full-text searchable. Bound a long crawl with `--max-duration` — the root `--timeout` applies per request, not to the whole sync.

### Audit a whole fleet for firmware currency

```bash
crestron-pp-cli fleet status --file fleet.txt --agent --select model,installed,latest,days_behind
```

Reads a plain list of model numbers and reports which are behind, resolving family-scoped releases so nothing is missed.

### Find which firmware version fixed something

```bash
crestron-pp-cli search "Dante" --type firmware_release --limit 10 --agent
```

Full-text searches every synced release note and change log at once.

### Narrow a verbose spec table to the fields you care about

```bash
crestron-pp-cli specs show DM-NVX-360 --agent --select sections.name,sections.rows.key,sections.rows.value
```

Specification tables run to dozens of rows across a dozen sections; selecting dotted paths keeps agent context small.

### Assemble a submittal package for a project

```bash
crestron-pp-cli submittal DM-NVX-384 TSW-1070 CP4N --out ./submittal --agent
```

Downloads every documentation asset for each model into its own folder and reports which asset classes were missing.

### Triage an as-built list for discontinued parts

```bash
crestron-pp-cli lifecycle UC-FCM-Z --agent --select model,status,replaced_by
```

Reports sellable status and the successor chain so a refresh estimate can be priced.

## Usage

Run `crestron-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CRESTRON_CONFIG_DIR`, `CRESTRON_DATA_DIR`, `CRESTRON_STATE_DIR`, or `CRESTRON_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CRESTRON_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CRESTRON_HOME=/srv/crestron
crestron-pp-cli doctor
```

Under `CRESTRON_HOME=/srv/crestron`, the four dirs resolve to `/srv/crestron/config`, `/srv/crestron/data`, `/srv/crestron/state`, and `/srv/crestron/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "crestron": {
      "command": "crestron-pp-mcp",
      "env": {
        "CRESTRON_HOME": "/srv/crestron"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CRESTRON_DATA_DIR` overrides an explicit `--home` for that kind. Use `CRESTRON_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CRESTRON_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `crestron-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### sync — build the local mirror (run this first)

Crawl Crestron.com into a local SQLite mirror so products, categories, and firmware releases can be queried offline. Crestron publishes no API and no product sitemap, so the catalog is walked the way the website itself does. Nothing local — `search`, `specs`, `fleet`, `lifecycle`, `submittal` — has data until this runs.

- **`crestron-pp-cli sync`** - Build the whole mirror: categories, products, and firmware releases
- **`crestron-pp-cli sync --resources categories,products`** - Build just the catalog half
- **`crestron-pp-cli sync --resources releases --notes --max-notes 50`** - Also pull release notes and change logs so `search` can full-text them (requires a signed-in session; without one the version and date are still recorded)
- **`crestron-pp-cli sync --max-duration 1h --concurrency 3`** - Bound the whole crawl and the number of parallel category walks

> The root `--timeout` applies per request, not to the whole sync. Use `--max-duration` (default 30m) for the overall crawl budget.

### account

Crestron.com sign-in state

- **`crestron-pp-cli account`** - Check whether the stored Crestron.com session is still signed in

### asset

Download Crestron documentation and firmware files

- **`crestron-pp-cli asset <guid> <filename>`** - Download a public documentation asset such as a spec sheet, manual, certificate, CAD drawing, or Revit family

### catalog

Browse the Crestron product catalog taxonomy

- **`crestron-pp-cli catalog category`** - Open a catalog category page and read its subcategories and product counts
- **`crestron-pp-cli catalog products`** - List the products in a catalog category (needs the category's document and node ids)
- **`crestron-pp-cli catalog tree`** - List every product category path in the catalog

### firmware

Crestron firmware and software releases

- **`crestron-pp-cli firmware release`** - Read a firmware release page including its version, date, release notes, and change log (requires sign-in)
- **`crestron-pp-cli firmware search`** - Find firmware and software releases for a model or family

### product

Look up Crestron products, specifications, and their documentation

- **`crestron-pp-cli product accessories`** - List optional accessories for a product
- **`crestron-pp-cli product page`** - Fetch a product detail page including its JSON-LD, specification table, and document id
- **`crestron-pp-cli product replacements`** - List replacement products for a discontinued item
- **`crestron-pp-cli product resources`** - List every documentation asset for a product by its document id
- **`crestron-pp-cli product variants`** - List the member models of a product series

### resource

Search Crestron's documentation and firmware resource library

- **`crestron-pp-cli resource`** - Search spec sheets, manuals, firmware, certificates, and drawings


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`crestron-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`crestron-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`crestron-pp-cli learnings list`** - Inspect taught rows
- **`crestron-pp-cli learnings forget <query>`** - Undo a teach
- **`crestron-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`crestron-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`crestron-pp-cli teach-pattern`** - Install a query/resource template up front
- **`crestron-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CRESTRON_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `crestron-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
crestron-pp-cli asset mock-value mock-value

# JSON for scripting and agents
crestron-pp-cli asset mock-value mock-value --json

# Filter to specific fields
crestron-pp-cli asset mock-value mock-value --json --select id,name,status

# Dry run — show the request without sending
crestron-pp-cli asset mock-value mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
crestron-pp-cli asset mock-value mock-value --agent
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

## Health Check

```bash
crestron-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `crestron-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/crestron-pp-cli/config.toml`; `--home`, `CRESTRON_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `crestron-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Firmware release notes come back empty or a download redirects to a sign-in page** — Run `crestron-pp-cli auth login --chrome` — release notes and binaries require a Crestron account session.
- **`fleet status` reports a model as unknown** — Run `crestron-pp-cli sync --resources products,releases` so the series-to-model mapping is populated.
- **Local results look out of date** — Run `crestron-pp-cli sync` again to refresh the local mirror; re-running is always safe.
- **A search returns nothing for a model you know exists** — Crestron groups firmware by family, so search the series instead, or use `crestron-pp-cli fleet status` which resolves the mapping for you.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**CrestronMasterTool**](https://github.com/alastairWH/CrestronMasterTool) — C#
- [**Crestron-EDK-Superscripts**](https://github.com/JaytheSpazz/Crestron-EDK-Superscripts) — PowerShell
- [**CrestronScripts**](https://github.com/intellectualrockstar/CrestronScripts) — PowerShell
- [**Crestron-FTP-Scripts**](https://github.com/oniointeractive/Crestron-FTP-Scripts) — PowerShell
- [**crestron-mcp**](https://github.com/Desluca/crestron-mcp) — TypeScript
- [**Crestron-Documentation**](https://github.com/StephenGenusa/Crestron-Documentation) — Markdown

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
