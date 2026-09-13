# Immoweb CLI

**Search Immoweb from the terminal with every site filter, then get the answers the site can't give: price cuts, days on market, commune medians and rental yield.**

Talks to the same JSON endpoints the immoweb.be website uses, with no login and no browser, and exposes every search filter as a flag. Everything you look at lands in a local SQLite history, so `watch` shows what is new, cheaper or gone since last time, `deal` judges one listing against its neighbours, and `market` rebuilds the commune price statistics Immoweb no longer publishes.

Learn more at [Immoweb](https://www.immoweb.be).

Created by [@sambassio](https://github.com/sambassio) (sambassio).

## Install

The recommended path installs both the `immoweb-pp-cli` binary and the `pp-immoweb` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install immoweb
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install immoweb --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install immoweb --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install immoweb --agent claude-code
npx -y @mvanhorn/printing-press-library install immoweb --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/immoweb/cmd/immoweb-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/immoweb-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install immoweb --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-immoweb --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-immoweb --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install immoweb --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/immoweb-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/immoweb/cmd/immoweb-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "immoweb": {
      "command": "immoweb-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Check the CLI can reach immoweb.be (no account or key needed).
immoweb-pp-cli doctor

# Resolve a commune name to the postcodes Immoweb filters on.
immoweb-pp-cli locations --query ixelles

# Search live listings with friendly filters; results are also stored locally.
immoweb-pp-cli find --type apartment --deal rent --commune ixelles --max-price 1500 --min-bedrooms 2 --sort newest

# Open one listing as a readable card: price, surfaces, EPC, agency contact, link.
immoweb-pp-cli show 21828249

# Save the search under a name so it can be watched and triaged.
immoweb-pp-cli saved add ixelles-2bed --type apartment --deal rent --commune ixelles --max-price 1500 --min-bedrooms 2

# See what is new, cheaper or gone since the last run.
immoweb-pp-cli watch ixelles-2bed

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Market intelligence
- **`market`** — Median asking price, €/m², price-cut share and recent disappearances for one or several communes, broken down by bedrooms, EPC, type or postcode. Student rooms and per-room lets are left out. Pulls missing areas on first use (--no-pull to stay offline).

  _Reach for this when an agent needs neighbourhood price levels or wants to compare communes before searching._

  ```bash
  immoweb-pp-cli market ixelles saint-gilles --type apartment --deal rent --by bedrooms --agent
  ```
- **`yield`** — Estimates gross rental yield for a sale listing or a whole commune from comparable rents, and refuses when there are too few comparables. Pulls missing areas on first use (--no-pull to stay offline).

  _Use for investor questions about rental return before looking at individual listings._

  ```bash
  immoweb-pp-cli yield --commune liege --type apartment --agent
  ```

### Decision support
- **`deal`** — Tells you whether one listing is cheap or expensive: €/m² percentile against comparable listings, days on market, price-cut history, demand and EPC gap. Pulls missing areas on first use (--no-pull to stay offline).

  _Use before advising on an offer or deciding whether a specific listing is worth a visit._

  ```bash
  immoweb-pp-cli deal 21828249 --agent
  ```
- **`triage`** — Ranks the current matches of a saved search (or ad-hoc filters) by a transparent score: EUR/m2 percentile within the same postal code, freshness, price cut, private seller and EPC (with --enrich) so you know who to call first.

  _Use when a search returns too many listings and the user needs a prioritised short list with reasons._

  ```bash
  immoweb-pp-cli triage --type apartment --deal rent --commune ixelles --max-price 1500 --min-bedrooms 2 --agent
  ```

### Local state that compounds
- **`drops`** — Lists stored listings whose asking price fell (from prices recorded by find/watch/pull and Immoweb's old-price field), with first and latest price, number of cuts, total % cut and days listed.

  _Use to find motivated sellers or negotiation leverage across everything the user has tracked._

  ```bash
  immoweb-pp-cli drops --commune liege --since 30d --agent
  ```

## Recipes

### Who to call first for a rental search

```bash
immoweb-pp-cli triage --type apartment --deal rent --commune ixelles --max-price 1500 --min-bedrooms 2 --agent
```

Ranks current matches with the per-factor breakdown (EUR/m2 vs the same postal code, freshness, cut, private seller); add --enrich 5 to score EPC for the top five.

### Is this house fairly priced?

```bash
immoweb-pp-cli deal 21828249 --agent
```

Returns the €/m² percentile against comparables, days on market and any recorded price cuts.

### Key facts from a listing, agent-sized

```bash
immoweb-pp-cli listings get 21828249 --agent --select classified.price.mainValue,classified.property.netHabitableSurface,classified.transaction.certificates.epcScore,classified.statistics.viewCount
```

The raw detail JSON is about 20 KB; --select keeps only the dotted fields the agent needs.

### Compare commune rents

```bash
immoweb-pp-cli market ixelles saint-gilles etterbeek --type apartment --deal rent --by bedrooms --agent
```

Median rent and €/m² per bedroom count, side by side, with sample sizes.

### Recent price cuts in what you have tracked

```bash
immoweb-pp-cli drops --since 30d --agent
```

Reads the local store only: cuts come from prices recorded by find, watch and pull plus Immoweb's own old-price field, so pull an area first (immoweb-pp-cli pull --commune liege --type apartment --deal sale).

## Usage

Run `immoweb-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `IMMOWEB_CONFIG_DIR`, `IMMOWEB_DATA_DIR`, `IMMOWEB_STATE_DIR`, or `IMMOWEB_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `IMMOWEB_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export IMMOWEB_HOME=/srv/immoweb
immoweb-pp-cli doctor
```

Under `IMMOWEB_HOME=/srv/immoweb`, the four dirs resolve to `/srv/immoweb/config`, `/srv/immoweb/data`, `/srv/immoweb/state`, and `/srv/immoweb/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "immoweb": {
      "command": "immoweb-pp-mcp",
      "env": {
        "IMMOWEB_HOME": "/srv/immoweb"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `IMMOWEB_DATA_DIR` overrides an explicit `--home` for that kind. Use `IMMOWEB_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `IMMOWEB_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `immoweb-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### listings

Immoweb classifieds: search, count, map results, full details and similar listings

- **`immoweb-pp-cli listings count`** - Exact number of listings matching the filters (search pages cap at 9,969)
- **`immoweb-pp-cli listings get`** - Full details of one listing: EPC, cadastral income, surfaces, heating, agency contact, photos, views and bookmarks
- **`immoweb-pp-cli listings map`** - Up to 200 listings per call with GPS coordinates (the bulk path used by the Immoweb map view)
- **`immoweb-pp-cli listings search`** - Search listings with every Immoweb filter (30 per page, raw Immoweb result objects)
- **`immoweb-pp-cli listings similar`** - Listings Immoweb considers similar to a given listing

### locations

Resolve commune names and postal codes to Immoweb location filters

- **`immoweb-pp-cli locations`** - Look up a commune, locality or postal code; returns the postalCodes value to pass to search filters

### Search, track and analyse

- **`immoweb-pp-cli find`** - Live search with friendly filters (`--type`, `--deal`, `--commune`/`--postcode`/`--province`, price, bedrooms, surfaces, `--epc`, `--garden`...), `--url` to start from a pasted Immoweb search, `--pages`, `--private-only`, `--hide-under-option`, `--near lat,lng --radius-km`. Results are stored locally.
- **`immoweb-pp-cli show <id|url>`** - Readable listing card: price, EUR/m2, EPC, cadastral income, surfaces, days online, demand, agency contact, recorded price history.
- **`immoweb-pp-cli photos <id|url>`** - Download photos (`--dir`, `--size small|medium|large|xl`) or only list their URLs (`--list`).
- **`immoweb-pp-cli saved add|list|remove`** - Named local searches (same filters as `find`, or `--url`).
- **`immoweb-pp-cli watch <saved>`** / **`watch --all`** - New listings, price changes (with the old price) and listings gone since the last run. The first run records a silent baseline.
- **`immoweb-pp-cli pull`** - Harvest every listing of an area into the local store (`--commune`/`--postcode`, `--type`, `--deal`); splits the area into price bands of up to 200 listings so it needs only a few requests, and marks listings that disappeared as gone.
- **`immoweb-pp-cli hide <id...>`** (`--undo`) - Dismiss listings so `find`, `watch` and `triage` skip them.
- **`immoweb-pp-cli shortlist add|list|remove`** - Local shortlist with notes.
- **`immoweb-pp-cli dump`** - Export stored listings as `--format csv|geojson|jsonl` (a JSON array with `--json`).
- **`market`, `deal`, `triage`, `drops`, `yield`** - see Unique Features.

`find`, `show`, `watch`, `pull` and the automatic pulls of `market`/`deal`/`yield`/`triage` fill the local store; `drops`, `dump`, `search` and `--no-pull` runs read it. Do not use the generic `sync` command to fill the store: it has no area filter and would page through listings for all of Belgium. Use `pull` instead.

### Known behaviours

- The 19 Brussels communes resolve to their own postal codes (Ixelles = 1050), because Immoweb's "(all localities)" grouping adds neighbouring codes (Ixelles would include 1000, the City of Brussels). Elsewhere a commune follows Immoweb's grouping: check it with `immoweb-pp-cli locations --query <name>`, and use `--postcode` or `market --by postcode` for a clean split.
- Student rooms (kots) and per-room lets are left out of `market`, `yield` and `triage`, and `deal` compares rooms only with rooms; `--include-rooms` keeps them.
- The first `market`, `deal` or `yield` on an area pulls it from Immoweb (a few seconds, up to ~20 s for large communes on a slow connection). The pull is reused for 24 h (`--refresh-after`); `--no-pull` or `--data-source local` stay offline.
- Days on market come from Immoweb's publication date, which only the listing detail carries: they appear after `show`, `deal` or `triage --enrich`. Without it, `market` leaves the median out and says why, and `triage` scores freshness on the last update date.
- Listings without a published price ("price on request") cannot be reached through price bands; `pull` reports them as `without_price`.
- Immoweb soft-throttles by returning empty pages; the CLI turns that into exit code 7 instead of an empty result. Wait a minute and retry.
- `triage` leaves out hidden listings and listings under option (`--include-under-option` keeps them).


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`immoweb-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`immoweb-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`immoweb-pp-cli learnings list`** - Inspect taught rows
- **`immoweb-pp-cli learnings forget <query>`** - Undo a teach
- **`immoweb-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`immoweb-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`immoweb-pp-cli teach-pattern`** - Install a query/resource template up front
- **`immoweb-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `IMMOWEB_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `immoweb-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
immoweb-pp-cli listings get 21828249

# JSON for scripting and agents
immoweb-pp-cli listings get 21828249 --json
# Filter to specific fields
immoweb-pp-cli listings get 21828249 --json --select classified

# Dry run — show the request without sending
immoweb-pp-cli listings get 21828249 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
immoweb-pp-cli listings get 21828249 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only toward Immoweb** - it never contacts agencies, posts or changes anything on immoweb.be; its only writes are the local store (saved searches, hidden listings, shortlist) and downloaded photos
- **Offline-friendly** - `drops`, `dump`, `search` and `--no-pull` runs read the local SQLite store filled by `find`, `show`, `watch` and `pull`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `1` unexpected failure, `2` usage error, `3` not found, `4` Immoweb refused the request (bot protection), `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
immoweb-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `immoweb-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/immoweb-pp-cli/config.toml`; `--home`, `IMMOWEB_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the listing ID is correct: a removed, sold or rented listing returns exit 3
- For saved searches, run `immoweb-pp-cli saved list` to see the available names

### API-specific
- **A search suddenly returns an empty page before the end of the results** — Immoweb soft-throttles with empty pages; the CLI reports a rate-limit error. Wait a minute or lower --pages, then re-run.
- **'unknown commune' error from find or saved add** — Get the postal code from immoweb-pp-cli locations --query <name>, then search with immoweb-pp-cli find --postcode <code> (plus your other filters).
- **market, deal or yield say there are not enough comparables** — Pull more data for the area first: immoweb-pp-cli pull --commune <name> --type <type> --deal <sale|rent>.
- **HTML page instead of JSON (non-JSON error)** — Either a filter value is invalid (check --help: --epc A,B, --type house,apartment) or Immoweb served a bot challenge; fix the value, or wait a few minutes and retry with fewer requests (--rate-limit 2).
- **market for a commune outside Brussels includes a neighbouring postcode** — Outside Brussels a commune follows Immoweb's own grouping, which can span postcodes; market warns when two communes share one. Use market --by postcode to see the split, or pass --postcode.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.immoweb.be/en/search-results
- Capture coverage: 12 API entries from 12 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence)

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**immoweb-keeper**](https://github.com/lawrensylvan/immoweb-keeper) — JavaScript (29 stars)
- [**immoweb-be-scraper**](https://github.com/pierodellagiustina/immoweb-be-scraper) — Python (7 stars)
- [**ImmoWeb**](https://github.com/g-coomans/ImmoWeb) — Python (4 stars)
- [**immoweb-zimmo-scraper**](https://github.com/vincentcox/immoweb-zimmo-scraper) — Python (4 stars)
- [**immoweb-telegram-bot**](https://github.com/othellodesutter/immoweb-telegram-bot) — Python (3 stars)
- [**Homiio**](https://github.com/OxyHQ/Homiio) — TypeScript (3 stars)
- [**belgium-rental-watcher**](https://github.com/simon324/belgium-rental-watcher) — TypeScript (2 stars)
- [**immoweb_bot**](https://github.com/adamkorompai/immoweb_bot) — Python (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
