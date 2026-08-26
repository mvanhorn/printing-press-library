# Agoda CLI

**Search Agoda hotels with the true all-in price, and re-rank by what you will actually pay.**

Agoda returns both the advertised price and the true all-in price in the same response, but only ever shows you the advertised one. This CLI surfaces both, breaks out the hidden tax-and-fee delta, and re-sorts by real cost - which routinely changes which hotel is cheapest. It talks to Agoda over plain HTTP with no browser, no rendering service, and no API key, and it keeps a local price history so it can answer questions a stateless scraper cannot.

Learn more at [Agoda](https://www.agoda.com).

Created by [@devacto](https://github.com/devacto) (Victor Wibisono).

## Install

The recommended path installs both the `agoda-pp-cli` binary and the `pp-agoda` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install agoda
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install agoda --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install agoda --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install agoda --agent claude-code
npx -y @mvanhorn/printing-press-library install agoda --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/agoda/cmd/agoda-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/agoda-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install agoda --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-agoda --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-agoda --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install agoda --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/agoda-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/agoda/cmd/agoda-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "agoda": {
      "command": "agoda-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Public hotel search, destination lookup, property detail, and reviews need no credentials at all - they replay over ordinary HTTP. Only member-priced and account surfaces (saved properties, AgodaVIP tier, vip delta) need a logged-in session: copy the Cookie header from a signed-in agoda.com browser tab and export it as AGODA_COOKIE (AGODA_SESSION_COOKIE is also accepted), and subsequent authenticated calls replay with it.

## Quick Start

```bash
# confirm reachability and configuration before spending a real call
agoda-pp-cli doctor --dry-run

# Agoda addresses destinations by opaque numeric id, so resolve the name first
agoda-pp-cli destinations --search-text Tokyo

# search with true all-in pricing on by default
agoda-pp-cli hotels search Tokyo --checkin 2026-10-15 --nights 2 --adults 2 --currency USD

# re-rank by real cost - the order usually differs from the advertised-price order
agoda-pp-cli hotels rank Tokyo --checkin 2026-10-15 --nights 2 --limit 10

# expose properties whose fee load is an outlier for the destination
agoda-pp-cli hotels fees Tokyo --checkin 2026-10-15 --nights 2

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Honest pricing
- **`hotels search`** — Shows what you will actually pay, not the teaser rate, with the hidden tax-and-fee delta broken out per property.

  _Reach for this instead of any scraped Agoda price. Quote the inclusive figure to a user; the advertised rate is not what they will be charged._

  ```bash
  agoda-pp-cli hotels search Tokyo --checkin 2026-10-15 --nights 2 --adults 2 --currency USD --agent
  ```
- **`hotels rank`** — Re-sorts a destination's results by true all-in price instead of Agoda's teaser-price ranking.

  _Use this whenever the decision is about price. Ordinary search ordering inherits Agoda's advertised-price ranking and will mislead._

  ```bash
  agoda-pp-cli hotels rank Tokyo --checkin 2026-10-15 --nights 2 --limit 10 --agent
  ```
- **`hotels fees`** — Flags properties whose tax-and-fee ratio is an outlier against the destination median.

  _Use before recommending a property. A hotel with a below-median advertised price and an above-median fee ratio is the classic bait pattern._

  ```bash
  agoda-pp-cli hotels fees Tokyo --checkin 2026-10-15 --nights 2 --agent
  ```

### Local state that compounds
- **`prices cheapest`** — Returns the cheapest check-in dates across a flexible window for a destination.

  _Use for flexible-date travelers. Returns the price floor across a window rather than a single-date quote._

  ```bash
  agoda-pp-cli prices cheapest Tokyo --window 2026-10-01..2026-11-30 --nights 3 --agent
  ```
- **`vip delta`** — Runs the same search signed-in and anonymous, then diffs per property to show what your VIP tier is actually worth.

  _Use when a user asks whether signing in or chasing a VIP tier is worth it. Reports the measured discount on a real search instead of marketing copy._

  ```bash
  agoda-pp-cli vip delta Tokyo --checkin 2026-10-15 --nights 2 --agent
  ```
- **`watch run`** — Surfaces only watched properties whose latest true all-in price dropped meaningfully below their trailing median.

  _Schedule it. Returns empty most days and returns something worth acting on when a watched property actually drops._

  ```bash
  agoda-pp-cli watch run --min-pct 7 --agent
  ```
- **`search`** — Full-text search over every property this CLI has already seen, with no network call.

  _Use after a few live searches to answer property questions without spending a request or waiting on the network._

  ```bash
  agoda-pp-cli search "shinjuku" --agent
  ```

### Agent-native plumbing
- **`compare`** — Puts finalist properties side by side on true all-in price, hidden fee share, review score, star rating, and free-cancellation deadline.

  _Use once the choice is narrowed to finalists, instead of re-reading two detail pages and eyeballing the difference._

  ```bash
  agoda-pp-cli compare 936623 788273 --destination Tokyo --checkin 2026-10-15 --nights 2 --agent
  ```

## Recipes

### What will this actually cost

```bash
agoda-pp-cli hotels search Tokyo --checkin 2026-10-15 --nights 2 --adults 2 --currency USD --agent --select results.name,results.price_all_in,results.price_advertised,results.hidden_pct
```

Returns just the four fields that matter for a cost decision, keeping the deeply nested Agoda payload out of the agent's context.

### The cheapest hotel is not the one listed cheapest

```bash
agoda-pp-cli hotels rank Tokyo --checkin 2026-10-15 --nights 2 --limit 10 --agent
```

Re-sorts by all-in cost; properties with above-average fee loads drop down the list and genuinely cheaper stays surface.

### Find the price floor for a flexible trip

```bash
agoda-pp-cli prices cheapest Tokyo --window 2026-10-01..2026-11-30 --nights 3 --agent
```

Sweeps a two-month window in one pass and returns the cheapest check-in dates rather than a single-date quote.

### Spot the resort-fee trap

```bash
agoda-pp-cli hotels fees Tokyo --checkin 2026-10-15 --nights 2 --agent
```

Ranks properties by how much of their true cost is tax and fees, flagging outliers against the destination median.

### Is signing in worth anything here

```bash
agoda-pp-cli vip delta Tokyo --checkin 2026-10-15 --nights 2 --agent
```

Issues the same search authenticated and anonymous and reports the measured per-property discount.

## Usage

Run `agoda-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `AGODA_CONFIG_DIR`, `AGODA_DATA_DIR`, `AGODA_STATE_DIR`, or `AGODA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `AGODA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export AGODA_HOME=/srv/agoda
agoda-pp-cli doctor
```

Under `AGODA_HOME=/srv/agoda`, the four dirs resolve to `/srv/agoda/config`, `/srv/agoda/data`, `/srv/agoda/state`, and `/srv/agoda/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "agoda": {
      "command": "agoda-pp-mcp",
      "env": {
        "AGODA_HOME": "/srv/agoda"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `AGODA_DATA_DIR` overrides an explicit `--home` for that kind. Use `AGODA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `AGODA_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `agoda-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### destinations

Resolve a destination name to the numeric city id every Agoda search requires

- **`agoda-pp-cli destinations`** - Resolve a free-text destination (city, area, landmark) to an Agoda city id

### reviews

Guest reviews for a property, paginated and sortable

- **`agoda-pp-cli reviews`** - List guest reviews for a property by Agoda hotel id


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`agoda-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`agoda-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`agoda-pp-cli learnings list`** - Inspect taught rows
- **`agoda-pp-cli learnings forget <query>`** - Undo a teach
- **`agoda-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`agoda-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`agoda-pp-cli teach-pattern`** - Install a query/resource template up front
- **`agoda-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `AGODA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `agoda-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
agoda-pp-cli reviews

# JSON for scripting and agents
agoda-pp-cli reviews --json
# Filter to specific fields
agoda-pp-cli reviews --json --select hotelReviewId,rating,reviewComments

# Dry run — show the request without sending
agoda-pp-cli reviews --dry-run

# Agent mode — JSON + compact + no prompts in one flag
agoda-pp-cli reviews --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
agoda-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `agoda-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/agoda-pp-cli/config.toml`; `--home`, `AGODA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Prices come back in an unexpected currency** — Pass --currency explicitly. Agoda derives currency from request geography unless both currency fields are set, which this CLI does for you.
- **Destination name is rejected or matches the wrong city** — Resolve the name first with 'agoda-pp-cli destinations --search-text <name>', then pass the numeric id you get back to 'agoda-pp-cli hotels search --city-id <id>'.
- **Rate limited, or results come back empty during a wide sweep** — Lower --max-scan-pages and retry. The client backs off adaptively and reports a typed rate-limit error rather than returning empty results.
- **vip delta reports no difference** — Confirm the session imported with 'agoda-pp-cli doctor'. If cookies are valid, a zero delta is a real result - your tier does not discount that search.
- **Authenticated commands return exit code 4** — Export a fresh AGODA_COOKIE taken from a signed-in agoda.com browser tab; Agoda session cookies expire. Run 'agoda-pp-cli doctor' to confirm the session is detected.
- **compare reports a property id as not found even though it exists** — Agoda returns a rotating subset of a city's inventory on each call. compare already re-searches across several sort orders; re-run it, or confirm the property is bookable for those exact dates with 'agoda-pp-cli hotels search <destination>'.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**agoda-review-mcp**](https://github.com/birariro/agoda-review-mcp) — Java
- [**hotelrate-crawl**](https://github.com/seanbabalala/hotelrate-crawl) — Python
- [**agoda-agent**](https://github.com/jiaweing/agoda-agent) — TypeScript
- [**agoda-property-listing-scraper**](https://github.com/ScraperHub/agoda-property-listing-scraper) — Python
- [**data-collection-pipeline**](https://github.com/TyW-98/data-collection-pipeline) — Python
- [**agodaparser**](https://github.com/egeland/agodaparser) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
