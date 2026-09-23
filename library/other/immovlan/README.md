# Immovlan CLI

**Search Immovlan from the terminal with every site filter, keep a local history, and cross it with Immoweb: exclusives, re-listings, rented PEB F/G traps and division candidates.**

Reads the same server-rendered pages the immovlan.be site serves, with no login, and stores every result in SQLite. Field names mirror immoweb-pp-cli so a sourcing job can merge both portals, and same-as tells you which listings exist only on Immovlan. enrich fills the detail fields the search cards never show, and peb-trap, split-candidates and relisted answer the questions a property trader asks before calling an agency.

Learn more at [Immovlan](https://immovlan.be).

Created by [@sambassio](https://github.com/sambassio) (sambassio).

## Install

The recommended path installs both the `immovlan-pp-cli` binary and the `pp-immovlan` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install immovlan
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install immovlan --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install immovlan --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install immovlan --agent claude-code
npx -y @mvanhorn/printing-press-library install immovlan --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/immovlan/cmd/immovlan-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/immovlan-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install immovlan --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-immovlan --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-immovlan --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install immovlan --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/immovlan-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/immovlan/cmd/immovlan-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "immovlan": {
      "command": "immovlan-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirm immovlan.be answers plain HTTP from this machine
immovlan-pp-cli doctor --dry-run

# Search Schaerbeek and Saint-Josse for bad-PEB sales; every result lands in the local store
immovlan-pp-cli find --type maison,appartement --deal sale --postcode 1030,1210 --epc F,G --min-price 500000 --max-price 2500000 --pages 3 --agent

# Read one listing: PEB letter, street, surfaces, year, cadastral income, seller, datePosted
immovlan-pp-cli show vbe69761 --agent

# Fill detail fields for the stored listings that lack them
immovlan-pp-cli enrich --missing epc,rented --top 100 --agent

# Rented F/G properties: owners who cannot re-let since 2026
immovlan-pp-cli peb-trap --postcode 1030,1210 --agent

# Listings that are not on Immoweb (reads immoweb-pp-cli's store)
immovlan-pp-cli same-as --all --unmatched --postcode 1030 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-portal intelligence
- **`same-as`** — Tell whether a stored Immovlan listing is also on Immoweb, at what price, or is Immovlan-only.

  _Use it to dedupe a cross-portal sourcing run or to find Immovlan exclusives before calling an agency. Requires immoweb-pp-cli's local store (`--immoweb-db` or `$IMMOWEB_PP_DB`); run `enrich --missing street` first so addresses can match. Every row carries flat `immoweb_id` (0 when Immovlan-only), `immoweb_url`, `immoweb_price`, `delta_pct`, `match_status` (address / price_surface_agency / none) and `immoweb_gone_status`; the nested `immoweb` object exists on matched rows only and is dropped by `--agent` compaction, so agents read the flat fields._

  ```bash
  immovlan-pp-cli same-as --all --unmatched --postcode 1030 --agent
  ```
- **`agencies`** — Which agencies list in a zone, their PEB F/G share, private-seller share and the CRM software behind each feed.

  _Use it to see which feeds are syndicated (likely also on Immoweb) versus Rossel-exclusive. The software column stays `?` until `enrich` has read the listing pages._

  ```bash
  immovlan-pp-cli agencies --postcode 1030 --epc F,G --agent
  ```

### Motivated-seller signals
- **`peb-trap`** — List PEB F or G properties that are currently rented: owners who can no longer re-let in Brussels since 2026.

  _Use it when the question is which sellers are structurally motivated, not merely cheap. Needs `enrich --missing epc,rented` first; `--include-unknown` also lists F/G rows not yet enriched._

  ```bash
  immovlan-pp-cli peb-trap --postcode 1030,1210 --max-price 2500000 --agent
  ```
- **`split-candidates`** — Houses large enough to divide, ranked by €/m² within their postcode, with land, year, PEB and rented flags.

  _Use it for a trader who divides houses into apartments; find cannot filter by surface._

  ```bash
  immovlan-pp-cli split-candidates --min-surface 200 --postcode 1030 --agent
  ```

### Local state that compounds
- **`enrich`** — Fill missing detail fields (PEB letter, street, geo, rented, cadastral income, agency software) for many stored listings in one resumable pass.

  _Run it after find and before peb-trap, same-as or split-candidates so their inputs exist._

  ```bash
  immovlan-pp-cli enrich --missing epc,rented --top 150 --agent
  ```
- **`relisted`** — Listings that came back under a new reference, with their true first-seen date and price at each reference.

  _Use it to expose a fake Nouveau before negotiating on ancienneté._

  ```bash
  immovlan-pp-cli relisted --since 90d --postcode 1030 --agent
  ```

## Recipes

### Bad-PEB houses for sale in two communes, newest first

```bash
immovlan-pp-cli find --type maison --deal sale --postcode 1030,1210 --epc F,G --pages 5 --agent --select id,url,price,surface_m2,price_per_m2,epc,locality
```

One line per listing with the fields the sourcing job needs; --select keeps the payload small.

### Rented PEB traps after enrichment

```bash
immovlan-pp-cli enrich --missing epc,rented --top 200 --agent && immovlan-pp-cli peb-trap --postcode 1030 --agent
```

enrich reads the detail pages once; peb-trap then filters locally.

### Immovlan exclusives

```bash
immovlan-pp-cli same-as --all --unmatched --postcode 1030 --agent
```

Joins with immoweb-pp-cli's store; unmatched rows are listings only Immovlan carries.

### What changed since the last run of a saved search

```bash
immovlan-pp-cli saved add sch-fg --type maison,appartement --deal sale --postcode 1030 && immovlan-pp-cli watch sch-fg --agent
```

First run is a baseline; later runs report new, price-changed and gone listings since the previous run.

### Notary sales on Immovlan

```bash
immovlan-pp-cli find --deal public-sale --postcode 1030,1080 --agent
```

Immovlan's own public-sale feed, next to Biddit.

## Usage

Run `immovlan-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as `teach.log` and the learn journal |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `IMMOVLAN_CONFIG_DIR`, `IMMOVLAN_DATA_DIR`, `IMMOVLAN_STATE_DIR`, or `IMMOVLAN_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `IMMOVLAN_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export IMMOVLAN_HOME=/srv/immovlan
immovlan-pp-cli doctor
```

Under `IMMOVLAN_HOME=/srv/immovlan`, the four dirs resolve to `/srv/immovlan/config`, `/srv/immovlan/data`, `/srv/immovlan/state`, and `/srv/immovlan/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "immovlan": {
      "command": "immovlan-pp-mcp",
      "env": {
        "IMMOVLAN_HOME": "/srv/immovlan"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `IMMOVLAN_DATA_DIR` overrides an explicit `--home` for that kind. Use `IMMOVLAN_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `IMMOVLAN_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `immovlan-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### listings

Immovlan listings: server-rendered search pages (20 cards per page) and detail pages

- **`immovlan-pp-cli listings get`** - Fetch one listing page by reference (e.g. vbe69761); Immovlan redirects to the canonical URL. Returns the schema.org JSON-LD RealEstateListing.
- **`immovlan-pp-cli listings search`** - Search listings; returns the detail links of one result page (20 per page). Use the promoted 'find' command for parsed cards.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`immovlan-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`immovlan-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`immovlan-pp-cli learnings list`** - Inspect taught rows
- **`immovlan-pp-cli learnings forget <query>`** - Undo a teach
- **`immovlan-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`immovlan-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`immovlan-pp-cli teach-pattern`** - Install a query/resource template up front
- **`immovlan-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `IMMOVLAN_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `immovlan-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
immovlan-pp-cli listings get vbe69761

# JSON for scripting and agents
immovlan-pp-cli listings get vbe69761 --json
# Filter to specific fields
immovlan-pp-cli listings get vbe69761 --json --select url,name,datePosted

# Dry run — show the request without sending
immovlan-pp-cli listings get vbe69761 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
immovlan-pp-cli listings get vbe69761 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **One JSON shape** - `find`, `watch`, `drops`, `relisted`, `peb-trap`, `split-candidates`, `agencies` and `same-as` return an object with `results` plus counters and an optional `note` (an empty store is a note, not an error); `dump --agent` returns the bare array. Photo URLs are always `pictures` (`show`, `dump`, `photos --list`).
- **Offline-friendly** - `find`, `watch` and `enrich` store every page in SQLite; `drops`, `peb-trap`, `split-candidates`, `relisted`, `agencies`, `same-as`, `dump` and `shortlist` read it offline; `show --data-source local` prints the stored row
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` blocked (HTTP 401/403 on the generic `listings` commands or `doctor`: bot wall, geography or rate block; there are no credentials to configure), `5` API error (including a 403 or bot challenge met by `find`, `show`, `watch` or `enrich`), `7` rate limited, `10` config error.

## Health Check

```bash
immovlan-pp-cli doctor
```

Verifies configuration and connectivity to immovlan.be (plain-HTTP GET of a search page).

## Configuration

Run `immovlan-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/immovlan-pp-cli/config.toml`; `--home`, `IMMOVLAN_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Use the reference from the listing URL (vbe…, rbu…, rbw…) or run `find` and read `results[].id`

### API-specific
- **HTTP 403 on every command** — Immovlan blocks headless browsers, not plain HTTP: keep the default Chrome User-Agent (do not override --user-agent) and run from a normal network; check with immovlan-pp-cli doctor
- **find returns fewer listings than the site shows** — The result count is rendered client-side; raise --pages (20 per page) and watch total_matching, which is estimated from the pagination
- **epc is a band (bad/poor) instead of a letter** — Cards carry the band only when the watermark is missing; run immovlan-pp-cli enrich --missing epc to read the letter from the detail page
- **same-as reports no immoweb store** — Install immoweb-pp-cli and run a find or pull there first, or point --immoweb-db at its SQLite file
- **show cannot find a reference** — Use the reference from the listing URL (vbe…, rbu…, rbw…) or paste the full URL; withdrawn listings return 404
