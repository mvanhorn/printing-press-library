# CDC Pakistan CLI

**The only machine-readable view of Pakistani custody data — per-security CDS penetration and a twenty-year eligibility record, from a source no tool has ever automated.**

CDC publishes the sole per-security custody-penetration figures in the Pakistani market as split PDFs behind a bot challenge, with no API and no history endpoint. This CLI mints a browser clearance once, replays it over plain HTTP, and turns the downloads corpus into a queryable local store. `coverage map` proves what is mirrored, `identity ledger` stops symbol recycling from corrupting your return series, and `eligibility state` folds two decades of notices into a per-security lifecycle.

## Install

The recommended path installs both the `cdc-pakistan-pp-cli` binary and the `pp-cdc-pakistan` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install cdc-pakistan
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install cdc-pakistan --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install cdc-pakistan --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install cdc-pakistan --agent claude-code
npx -y @mvanhorn/printing-press-library install cdc-pakistan --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/cmd/cdc-pakistan-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cdc-pakistan-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install cdc-pakistan --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cdc-pakistan --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cdc-pakistan --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install cdc-pakistan --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
cdc-pakistan-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cdc-pakistan-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/cmd/cdc-pakistan-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "cdc-pakistan": {
      "command": "cdc-pakistan-pp-mcp"
    }
  }
}
```

</details>

## Authentication

CDC sits behind a Cloudflare JS challenge on every path. Run `auth login --chrome` once: it opens a real browser, clears the challenge, and stores the clearance cookie together with the exact User-Agent that minted it, because the cookie is bound to that User-Agent. The clearance has a measured hard lifetime of roughly thirty minutes regardless of traffic — the cookie's own expiry field claims a year and is not to be trusted. Every command checks the remaining margin before starting work and refuses early rather than dying midway through a fan-out.

## Quick Start

```bash
# confirm the binary, config path and store are healthy before touching the network
cdc-pakistan-pp-cli doctor --dry-run

# build the document index first; nothing else can be trusted as complete until this runs
cdc-pakistan-pp-cli coverage map --category notices --timeout 30m

# read one security's CDS-eligibility lifecycle out of the local store
cdc-pakistan-pp-cli eligibility state --isin PK0069501016 --agent

# check whether this symbol was ever reused by a different issuer
cdc-pakistan-pp-cli identity ledger --symbol LOTCHEM --agent

# prove the penetration extraction before using the numbers
cdc-pakistan-pp-cli verify rows --vintage 2025-11-30 --agent

```

## Known Gaps

- **The PDF-backed features need macOS.** `verify rows`, `verify schema`, `gop stake` and
  `float triangulate` all read the penetration reports, and extracting text from them uses
  Apple's PDFKit through a `swift` subprocess. Three pure-Go PDF libraries were tried against
  the live vintage first: all three either returned empty text or panicked on its xref stream.
  A pure-Go fallback ships and is what runs off Darwin, but it has not been shown to read the
  current reports, so treat those four commands as macOS-only until it is. Every other command,
  including the whole eligibility and identity surface, is platform-independent.
- **The free-float panel is one vintage, not a series.** CDC overwrites the penetration report
  in place, so only the current month is retrievable and there is no archive to backfill from.
  `float triangulate` and `gop stake` give you a cross-section; no month-over-month delta can be
  computed, and no such command is offered rather than one that silently interpolates. History
  accrues only forward, from your first sync onward.
- **The clearance cookie lasts about thirty minutes.** Measured, not advertised: the cookie's own
  `expires` field claims a year and is wrong. Under continuous use it returned HTTP 200 at
  t+1500s and 403 at t+1800s from mint, so it is a hard server-side window and not an idle
  timeout. Long fan-outs must be resumable, and every command refuses up front when the
  remaining margin is too small rather than dying midway. Re-minting needs a real browser.
- **Read CDC's `robots.txt` before a wide sweep.** It is Cloudflare-managed and, as of
  2026-09-08, gives the wildcard agent `Content-Signal: search=yes,ai-train=no,use=reference`
  followed by `Allow: /` — so no path this CLI reads is disallowed — while `Disallow: /` is set
  for nine named AI crawlers (`ClaudeBot`, `GPTBot`, `CCBot`, `Google-Extended`,
  `Applebot-Extended`, `Amazonbot`, `Bytespider`, `meta-externalagent`,
  `CloudflareBrowserRenderingCrawler`). This CLI is an operator-run client fetching specific
  documents on demand under its minting browser's User-Agent: it is not one of those crawlers,
  it does not train on what it fetches, and `use=reference` covers feeding an agent a document
  you asked it for. The signals are an express reservation of rights under Article 4 of EU
  Directive 2019/790, so check the current file and satisfy yourself before sweeping the corpus.
- **JSON output HTML-escapes `&`, `<` and `>`.** Strings come back with `\u0026` where a literal
  `&` belongs, including document titles and copy-paste command hints. The table and CSV renderings
  are unaffected; decode the JSON before displaying it.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`coverage map`** — See exactly which document buckets are mirrored, which are genuinely absent at source, and which were never asked for.

  _Run this before trusting any other command's completeness; it is the only command that writes the document index._

  ```bash
  cdc-pakistan-pp-cli coverage map --category notices --agent
  ```
- **`identity ledger`** — Reconstruct a security's symbol, name and ISIN history so you never splice two different issuers into one return series.

  _Reach for this before joining CDC data to any symbol-keyed price series; symbol recycling silently corrupts factor research._

  ```bash
  cdc-pakistan-pp-cli identity ledger --symbol LOTCHEM --agent
  ```
- **`stats history`** — Track the sixteen CDS aggregate metrics forward from your first sync, with archived rows labelled as such.

  _Use to measure retail-participation growth; archived rows are non-contiguous and must never be read as a continuous series._

  ```bash
  cdc-pakistan-pp-cli stats history --metric sub_accounts_individual --agent
  ```

### Twenty years of regulatory events
- **`eligibility state`** — Get a security's full CDS-eligibility history folded from twenty years of notices into a six-state lifecycle.

  _Use for CDS-eligibility history, but note CDS eligibility is not the same as PSX trading suspension and is not a tradability gate._

  ```bash
  cdc-pakistan-pp-cli eligibility state --isin PK0069501016 --agent
  ```

### Extraction you can trust
- **`verify rows`** — Prove the PDF extraction is trustworthy by asserting the report's own percentage column against its own share and capital columns.

  _Run this before any analysis that depends on the penetration numbers; residual signatures tell you which column drifted._

  ```bash
  cdc-pakistan-pp-cli verify rows --vintage 2025-11-30 --agent
  ```
- **`verify schema`** — Refuse to blend vintages whose column sets are incompatible, and refuse a month that is missing one of its parts.

  _Run this whenever a new vintage lands; it blocks silently-incompatible data instead of averaging it._

  ```bash
  cdc-pakistan-pp-cli verify schema --vintage 2025-11-30 --agent
  ```

### Cross-source joins
- **`float triangulate`** — Put CDC custody penetration beside PSX free-float shares and NCCPL free-float percent, with each denominator named.

  _Use this to see where the three float definitions disagree; it deliberately never blends them into one number._

  ```bash
  cdc-pakistan-pp-cli float triangulate --symbol OGDC --agent
  ```
- **`gop stake`** — Derive state-held capital per security from the difference between the including-GoP and excluding-GoP capital columns.

  _Use for a current cross-section of state ownership; there is only one vintage, so no time series exists._

  ```bash
  cdc-pakistan-pp-cli gop stake --min-pct 25 --agent
  ```

## Recipes

### Prove the mirror before trusting it

```bash
cdc-pakistan-pp-cli coverage map --agent --select buckets.category,buckets.year,buckets.state
```

Returns the tri-state coverage per bucket so you can tell a genuine source gap from an unasked question.

### Narrow a wide eligibility history for an agent

```bash
cdc-pakistan-pp-cli eligibility state --isin PK0069501016 --agent --select events.effective_date,events.state,events.notice_url
```

The full event payload is large; selecting three dotted paths keeps an agent's context small while preserving the audit trail.

### Find state-owned concentration

```bash
cdc-pakistan-pp-cli gop stake --min-pct 25 --agent --select rows.symbol,rows.gop_shares,rows.gop_pct
```

Lists securities where the Government of Pakistan holds at least a quarter of paid-up capital.

### Catch a symbol collision before a join

```bash
cdc-pakistan-pp-cli identity resolve --symbol LOTCHEM --as-of 2019-06-30 --agent
```

Returns the identity in force on that date, or NOT_IN_VINTAGE rather than guessing.

### Audit an extraction end to end

```bash
cdc-pakistan-pp-cli verify rows --vintage 2025-11-30 --agent --select findings.check_name,findings.severity,findings.isin
```

Surfaces only the failing checks and the ISINs they belong to.

## Usage

Run `cdc-pakistan-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CDC_PAKISTAN_CONFIG_DIR`, `CDC_PAKISTAN_DATA_DIR`, `CDC_PAKISTAN_STATE_DIR`, or `CDC_PAKISTAN_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CDC_PAKISTAN_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CDC_PAKISTAN_HOME=/srv/cdc-pakistan
cdc-pakistan-pp-cli doctor
```

Under `CDC_PAKISTAN_HOME=/srv/cdc-pakistan`, the four dirs resolve to `/srv/cdc-pakistan/config`, `/srv/cdc-pakistan/data`, `/srv/cdc-pakistan/state`, and `/srv/cdc-pakistan/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "cdc-pakistan": {
      "command": "cdc-pakistan-pp-mcp",
      "env": {
        "CDC_PAKISTAN_HOME": "/srv/cdc-pakistan"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CDC_PAKISTAN_DATA_DIR` overrides an explicit `--home` for that kind. Use `CDC_PAKISTAN_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CDC_PAKISTAN_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `cdc-pakistan-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### assets

Static PDF report files under /assets/uploads/YYYY/MM/.

- **`cdc-pakistan-pp-cli assets`** - Fetch one report PDF. Also challenged by Cloudflare, so the same clearance cookie is required.

### downloads

CDC downloads corpus — 7,956 documents across 15 categories, 2007-2026.

- **`cdc-pakistan-pp-cli downloads`** - List download items for one (category, year, page). THE ONLY VALID ENUMERATOR — the /downloads-category/<cat>/page/N/ URL pagination is decorative and returns an identical block for every N.

### statistics

CDS aggregate statistics — a 16-row HTML table, overwritten monthly. Current-state only.

- **`cdc-pakistan-pp-cli statistics`** - Fetch the current CDS aggregate statistics table. Row labels DRIFT between vintages (Sahulat row deleted; Securities split into Listed/Unlisted), so key on normalised label with an explicit unknown bucket.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`cdc-pakistan-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`cdc-pakistan-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`cdc-pakistan-pp-cli learnings list`** - Inspect taught rows
- **`cdc-pakistan-pp-cli learnings forget <query>`** - Undo a teach
- **`cdc-pakistan-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`cdc-pakistan-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`cdc-pakistan-pp-cli teach-pattern`** - Install a query/resource template up front
- **`cdc-pakistan-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CDC_PAKISTAN_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `cdc-pakistan-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
cdc-pakistan-pp-cli downloads

# JSON for scripting and agents
cdc-pakistan-pp-cli downloads --json
# Filter to specific fields by name
cdc-pakistan-pp-cli downloads --json --select <field>[,<field>...]

# Dry run — show the request without sending
cdc-pakistan-pp-cli downloads --dry-run

# Agent mode — JSON + compact + no prompts in one flag
cdc-pakistan-pp-cli downloads --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
cdc-pakistan-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `cdc-pakistan-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/cdc-pakistan-pp-cli/config.toml`; `--home`, `CDC_PAKISTAN_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `cdc-pakistan-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Every command returns a clearance-expired error** — Run `auth login --chrome` again; the clearance has a hard ~30-minute lifetime and cannot be renewed without a browser.
- **A command refuses immediately with 'clearance margin too small'** — That is deliberate — re-mint with `auth login --chrome` rather than starting work that cannot finish.
- **coverage map stopped partway with chunks remaining** — Re-run `auth login --chrome` then re-run the same command; it re-derives what is missing from the store and resumes.
- **A vintage load is refused for schema drift** — Inspect with `verify schema --vintage <date>`; CDC has changed columns before, so acknowledge the new fingerprint deliberately rather than blending.
- **float triangulate returns rows with an empty CDC leg** — Part B of the report covers mutual-fund units and carries no paid-up capital, so no penetration percentage exists for those ISINs.
