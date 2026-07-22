# Ticketmaster CLI

**Every Discovery v2 endpoint plus offline search, multi-venue watchlists, residency dedup, and on-sale tracking no API call exposes.**

ticketmaster-pp-cli is the first single-binary CLI for the Ticketmaster Discovery API. It absorbs the full read-only surface (events, venues, attractions, classifications, suggest) and adds a local SQLite store with FTS search, named watchlists, residency collapse, tour-view with on-sale flags, and markdown briefs — the workflows real users built scripts to handle.

Learn more at [Ticketmaster](http://developer.ticketmaster.com/support/contact-us/).

Created by [@omarshahine](https://github.com/omarshahine) (Omar Shahine).

## Install

The recommended path installs both the `ticketmaster-pp-cli` binary and the `pp-ticketmaster` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ticketmaster
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ticketmaster --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ticketmaster --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ticketmaster --agent claude-code
npx -y @mvanhorn/printing-press-library install ticketmaster --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/cmd/ticketmaster-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ticketmaster-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ticketmaster --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ticketmaster --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ticketmaster --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ticketmaster --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ticketmaster-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TICKETMASTER_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/cmd/ticketmaster-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ticketmaster": {
      "command": "ticketmaster-pp-mcp",
      "env": {
        "TICKETMASTER_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authentication is a single Ticketmaster Discovery API consumer key, passed as the `apikey` query parameter on every request. Register at https://developer-acct.ticketmaster.com and copy the Consumer Key from your My Apps dashboard. Set TICKETMASTER_API_KEY in your shell environment. The free tier allows 5000 requests/day at 5/second.

## Quick Start

```bash
# Verifies the API key is set, the Discovery API is reachable, and the local store is initialized.
ticketmaster-pp-cli doctor

# Sync the API surface into the local SQLite store for offline search and aggregation.
ticketmaster-pp-cli sync

# Fan out across multiple venues and return one merged event list.
ticketmaster-pp-cli events upcoming --venue-ids KovZ917Ahkk,KovZpZAFkvEA --days 60 --json

# Collapse Broadway/opera residencies into single rows with first/last date and night count.
ticketmaster-pp-cli events residency --window 28 --json

# Render this week's events as a paste-ready markdown brief.
ticketmaster-pp-cli events brief --dma 383 --window 7

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local-store aggregations
- **`events upcoming`** — Fan out across a venue ID file or list and return one merged, deduplicated, date-sorted event list — the watchlist primitive behind any curated 'what's on at my venues' workflow.

  _When the user has a curated list of venues they care about and wants one merged feed; replaces hand-rolled per-venue fan-out scripts._

  ```bash
  ticketmaster-pp-cli events upcoming --venue-ids KovZ917Ahkk,KovZpZAFkvEA --days 60 --json
  ```
- **`events residency`** — Collapse runs of same-name + same-venue events into one row per residency with first_date, last_date, night_count, and id_list — so a 16-night opera season shows as one entry, not 16.

  _When listing upcoming events would otherwise show many near-duplicate rows for Broadway tours, opera seasons, or comedy residencies._

  ```bash
  ticketmaster-pp-cli events residency --window 28 --json
  ```
- **`events by-classification`** — Local join of events × classifications, grouped by segment and genre, with event count and three example events per leaf — the bucketed view newsletter authors and local-scene trackers reach for.

  _When summarizing 'what's on this month' broken down by music vs theatre vs comedy vs sports._

  ```bash
  ticketmaster-pp-cli events by-classification --dma 383 --window 60 --json
  ```
- **`events watchlist`** — Save, list, run, and remove named filter sets (venue IDs, attraction IDs, segments, DMA IDs) that persist across runs in the local SQLite store — the generic primitive any curated 'my venues' workflow composes from.

  _When the same curated venue/artist/genre filter recurs across many queries._

  ```bash
  ticketmaster-pp-cli events watchlist save seattle --venue-ids KovZ917Ahkk,KovZpZAFkvEA,KovZpZA1klkA
  ```
- **`events price-bands`** — Bucket events by priceRanges.min into <$50 / $50-100 / $100-200 / $200+ bands and report count + sample events per band, grouped by classification.

  _When the user wants to know where the affordable shows are this month, or how a venue's pricing skews._

  ```bash
  ticketmaster-pp-cli events price-bands --dma 383 --window 30 --json
  ```

### Tour & on-sale tracking
- **`events tour`** — For a given attraction (artist/team/touring show), return every upcoming event sorted by date, with city, venue, on-sale status, and a flag for events going on-sale within 7 days.

  _When tracking an artist across cities or watching for presale windows._

  ```bash
  ticketmaster-pp-cli events tour KovZ917Ahkk --on-sale-window 7 --json
  ```
- **`events on-sale-soon`** — Local query for events whose public on-sale falls in the next N days, sorted ascending — the canonical 'presale watch' view that no API endpoint provides.

  _When the user wants to be alerted to upcoming on-sale dates without polling each artist manually._

  ```bash
  ticketmaster-pp-cli events on-sale-soon --window 7 --classification rock --json
  ```

### Agent-native plumbing
- **`events dedup`** — Read an event JSON array from stdin or the local store, apply a deduplication strategy (name+venue+date, or tour-leg), and write the deduped stream to stdout — composes with any upstream command.

  _When merging results from multiple queries or sources and the duplicates need to be removed before agent processing._

  ```bash
  ticketmaster-pp-cli events list --keyword phish --json | ticketmaster-pp-cli events dedup --strategy tour-leg
  ```
- **`events brief`** — Render a markdown 'what's on' report grouped by night → venue → events with classification labels and price bands, suitable for newsletter, Obsidian, iMessage, or agent context.

  _When the user needs a paste-ready event summary for a chat thread, newsletter, or LLM context._

  ```bash
  ticketmaster-pp-cli events brief --dma 383 --window 7
  ```

## Recipes

### Seattle watchlist composition (no Seattle-specific code in the CLI)

```bash
ticketmaster-pp-cli events watchlist save seattle --venue-ids KovZ917Ahkk,KovZpZAFkvEA,KovZpZA1klkA,KovZpZAFEdeA,KovZpZAFkv1A
```

Save a named watchlist of Seattle venue IDs. Then run "events watchlist run seattle --days 60 --json" to apply the filter across upcoming events; pipe through "events dedup --strategy name-venue-date" for the final cleaned feed. Replaces a 595-line bash script.

### Track an artist across all upcoming dates with on-sale flags

```bash
ticketmaster-pp-cli events tour KovZ917Ahkk --on-sale-window 7 --json --select 'name,dates.start.localDate,_embedded.venues[0].name,_embedded.venues[0].city.name,sales.public.startDateTime'
```

Returns every upcoming tour stop with city, venue, and a flag for stops going on-sale within a week. The --select narrows the deeply-nested Discovery payload to just what an agent needs.

### Weekend brief for a metro

```bash
ticketmaster-pp-cli events brief --dma 383 --window 3 --classification music
```

Render a markdown brief of the next 3 days of music events in Seattle-Tacoma (DMA 383) - paste-ready for Obsidian or an iMessage thread.

### On-sale watch for rock shows

```bash
ticketmaster-pp-cli events on-sale-soon --window 14 --classification rock --json
```

Two-week-out scan for rock events going on public sale; pipe to an alerting script.

## Usage

Run `ticketmaster-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TICKETMASTER_CONFIG_DIR`, `TICKETMASTER_DATA_DIR`, `TICKETMASTER_STATE_DIR`, or `TICKETMASTER_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TICKETMASTER_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TICKETMASTER_HOME=/srv/ticketmaster
ticketmaster-pp-cli doctor
```

Under `TICKETMASTER_HOME=/srv/ticketmaster`, the four dirs resolve to `/srv/ticketmaster/config`, `/srv/ticketmaster/data`, `/srv/ticketmaster/state`, and `/srv/ticketmaster/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "ticketmaster": {
      "command": "ticketmaster-pp-mcp",
      "env": {
        "TICKETMASTER_HOME": "/srv/ticketmaster"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TICKETMASTER_DATA_DIR` overrides an explicit `--home` for that kind. Use `TICKETMASTER_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TICKETMASTER_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `ticketmaster-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### attractions

Manage attractions

- **`ticketmaster-pp-cli attractions find`** - Find attractions (artists, sports, packages, plays and so on) and filter your search by name, and much more.
- **`ticketmaster-pp-cli attractions get`** - Get details for a specific attraction using the unique identifier for the attraction.

### classifications

Manage classifications

- **`ticketmaster-pp-cli classifications get`** - Get details for a specific segment, genre, or sub-genre using its unique identifier.
- **`ticketmaster-pp-cli classifications get-genre`** - Get details for a specific genre using its unique identifier.
- **`ticketmaster-pp-cli classifications get-segment`** - Get details for a specific segment using its unique identifier.
- **`ticketmaster-pp-cli classifications get-subgenre`** - Get details for a specific sub-genre using its unique identifier.
- **`ticketmaster-pp-cli classifications list`** - Find classifications and filter your search by name, and much more. Classifications help define the nature of attractions and events.

### events

Manage events

- **`ticketmaster-pp-cli events get`** - Get details for a specific event using the unique identifier for the event. This includes the venue and location, the attraction(s), and the Ticketmaster Website URL for purchasing tickets for the event.
- **`ticketmaster-pp-cli events list`** - Find events and filter your search by location, date, availability, and much more.

### suggest

Manage suggest

- **`ticketmaster-pp-cli suggest`** - Find search suggestions and filter your suggestions by location, source, etc.

### venues

Manage venues

- **`ticketmaster-pp-cli venues get`** - Get details for a specific venue using the unique identifier for the venue.
- **`ticketmaster-pp-cli venues list`** - Find venues and filter your search by name, and much more.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`ticketmaster-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`ticketmaster-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`ticketmaster-pp-cli learnings list`** - Inspect taught rows
- **`ticketmaster-pp-cli learnings forget <query>`** - Undo a teach
- **`ticketmaster-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`ticketmaster-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`ticketmaster-pp-cli teach-pattern`** - Install a query/resource template up front
- **`ticketmaster-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `TICKETMASTER_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `ticketmaster-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ticketmaster-pp-cli attractions get mock-value

# JSON for scripting and agents
ticketmaster-pp-cli attractions get mock-value --json

# Filter to specific fields
ticketmaster-pp-cli attractions get mock-value --json --select id,name,status

# Dry run — show the request without sending
ticketmaster-pp-cli attractions get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ticketmaster-pp-cli attractions get mock-value --agent
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
ticketmaster-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `ticketmaster-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/ticketmaster-pp-cli/config.toml`; `--home`, `TICKETMASTER_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TICKETMASTER_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `ticketmaster-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ticketmaster-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TICKETMASTER_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 401 / 'Invalid API Key'** — Confirm TICKETMASTER_API_KEY matches the Consumer Key (not Consumer Secret) at developer-acct.ticketmaster.com/products-and-docs/user/me. Re-source your shell.
- **HTTP 429 'Quota exceeded'** — Free tier is 5000 req/day, 5/sec. Run `ticketmaster-pp-cli doctor` to see today's call count, or upgrade the plan.
- **Empty results for a venue you know exists** — Venue may not be Ticketmaster-primary; try a DMA query with classification filter instead. Use `suggest <venue-name>` to discover the canonical venue ID.
- **priceRanges missing on event get** — Resale and dynamic-priced events often omit priceRanges; this is upstream behavior, not a CLI bug.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**delorenj/mcp-server-ticketmaster**](https://github.com/delorenj/mcp-server-ticketmaster) — TypeScript
- [**mochow13/ticketmaster-mcp-server**](https://github.com/mochow13/ticketmaster-mcp-server) — TypeScript
- [**arcward/ticketpy**](https://github.com/arcward/ticketpy) — Python
- [**arcward/picketer**](https://github.com/arcward/picketer) — Go
- [**npm:ticketmaster**](https://www.npmjs.com/package/ticketmaster) — JavaScript
- [**konfig-sdks/ticketmaster-python-sdk**](https://github.com/konfig-sdks/ticketmaster-python-sdk) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
