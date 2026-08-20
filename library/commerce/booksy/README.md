# Booksy CLI

**Book a haircut in Poland from the terminal — search barbers, compare prices and reviews, check open slots, and book, with a dry-run-by-default booking guard.**

A CLI over Booksy's Polish marketplace. Public commands (search, business, reviews, suggest) need no login; add BOOKSY_ACCESS_TOKEN for your account and the guided `book` command. `book` previews the exact appointment and only commits with --confirm, so agents can plan safely and you approve the real booking.

## Install

The recommended path installs both the `booksy-pp-cli` binary and the `pp-booksy` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install booksy
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install booksy --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install booksy --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install booksy --agent claude-code
npx -y @mvanhorn/printing-press-library install booksy --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/booksy/cmd/booksy-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/booksy-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install booksy --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-booksy --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-booksy --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install booksy --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/booksy-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BOOKSY_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/booksy/cmd/booksy-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "booksy": {
      "command": "booksy-pp-mcp",
      "env": {
        "BOOKSY_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Public discovery works out of the box. Authenticated actions (your profile, open slots, booking) use your Booksy web session token: copy the `x-access-token` request header value from booksy.com (DevTools -> Network -> any /customer_api/me request) and set it via `booksy-pp-cli auth set-token <token>` or the BOOKSY_ACCESS_TOKEN env var. The public x-api-key and a device fingerprint are built in.

## Quick Start

```bash
# Confirm the CLI is wired and reachable (works without a token).
booksy-pp-cli doctor --dry-run

# Resolve your city to a location id.
booksy-pp-cli discover locations --query Warszawa

# Find barbershops, sorted by score.
booksy-pp-cli businesses search --query barber --location-id 47905 --per-page 10

# See haircut services and the --service-variant id to book.
booksy-pp-cli services 297360 --query haircut

# Check open slots (no login needed).
booksy-pp-cli availability 297360 --service-variant 20193554 --from 2026-08-19 --to 2026-08-31

# Preview the booking; add --confirm to actually book.
booksy-pp-cli book 297360 --service-variant 20193554 --date 2026-08-19 --time 10:00

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Booking funnel
- **`book`** — Book an appointment end to end — it previews the exact service, staffer, time, and price first, and only sends the real booking when you pass --confirm.

  _This is the one command that turns 'find me a haircut' into an actual appointment; the dry-run-by-default guard means an agent can plan safely and the human confirms._

  ```bash
  booksy-pp-cli book 297360 --service-variant 20193554 --date 2026-08-19 --time 10:00
  ```
- **`availability`** — List open time slots for a service at a business over a date range, grouped by day. No login required.

  _Lets an agent answer 'when can I get in?' without loading the web calendar — no token needed._

  ```bash
  booksy-pp-cli availability 297360 --service-variant 20193554 --from 2026-08-19 --to 2026-08-31
  ```
- **`services`** — Flatten a business profile into a clean table of bookable services with price, duration, and the service-variant id you pass to book.

  _Surfaces the exact --service-variant id an agent needs for availability and booking, with prices to compare._

  ```bash
  booksy-pp-cli services 297360 --query haircut
  ```
- **`cancel`** — Cancel one of your Booksy appointments by id — previews the appointment first and only cancels with --confirm.

  _Lets an agent undo a booking it made without opening the Booksy app._

  ```bash
  booksy-pp-cli cancel 746784544 --confirm
  ```

### Local intelligence
- **`earliest`** — Find the single earliest open slot for a service across a date window in one call.

  _Answers 'what's the soonest I can get a haircut here?' directly instead of returning a whole calendar._

  ```bash
  booksy-pp-cli earliest 297360 --service-variant 20193554 --within 14d
  ```
- **`compare`** — Compare several businesses side by side on rating, review count, and cheapest matching service price — from the local cache.

  _Turns 'which of these barbers is best value?' into one command instead of opening N tabs._

  ```bash
  booksy-pp-cli compare 297360 161624 --service haircut
  ```
- **`cheapest`** — Scan search results in a city and rank businesses by the cheapest service matching your query (e.g. haircut), with rating alongside price.

  _Answers 'where's the cheapest decent haircut near me?' — a query Booksy's own UI cannot express._

  ```bash
  booksy-pp-cli cheapest --location-id 47905 --service haircut --limit 10
  ```

## Recipes

### Cheapest haircut near a city

```bash
booksy-pp-cli cheapest --location-id 47905 --service haircut --limit 10
```

Ranks nearby barbershops by their cheapest haircut price with rating alongside.

### Soonest opening for a service

```bash
booksy-pp-cli earliest 297360 --service-variant 20193554 --within 14d
```

Returns the single earliest open slot in the next two weeks.

### Narrow a search payload for an agent

```bash
booksy-pp-cli businesses search --query barber --location-id 47905 --agent --select businesses.id,businesses.name,businesses.reviews_stars,businesses.reviews_count
```

Booksy business objects are large; --select trims to just the fields an agent needs to rank results.

### Safe booking preview then confirm

```bash
booksy-pp-cli book 297360 --service-variant 20193554 --date 2026-08-19 --time 10:00
```

Prints the exact service, staffer, time, and price; re-run with --confirm to place it.

## Usage

Run `booksy-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `BOOKSY_CONFIG_DIR`, `BOOKSY_DATA_DIR`, `BOOKSY_STATE_DIR`, or `BOOKSY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `BOOKSY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export BOOKSY_HOME=/srv/booksy
booksy-pp-cli doctor
```

Under `BOOKSY_HOME=/srv/booksy`, the four dirs resolve to `/srv/booksy/config`, `/srv/booksy/data`, `/srv/booksy/state`, and `/srv/booksy/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "booksy": {
      "command": "booksy-pp-mcp",
      "env": {
        "BOOKSY_HOME": "/srv/booksy"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `BOOKSY_DATA_DIR` overrides an explicit `--home` for that kind. Use `BOOKSY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `BOOKSY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `booksy-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### businesses

Search and inspect Booksy businesses (barbers, salons)

- **`booksy-pp-cli businesses get`** - Full business profile: services (with prices/variant ids), staff, hours, reviews summary.
- **`booksy-pp-cli businesses reviews`** - Customer reviews for a business.
- **`booksy-pp-cli businesses search`** - Search barbers/salons. Filter by query, city (location-id), and category; sort by score/distance/reviews.

### discover

Discovery helpers: query suggestions and location resolution

- **`booksy-pp-cli discover locations`** - Resolve a place/city name to Booksy location ids for search.
- **`booksy-pp-cli discover suggest`** - Query suggestions (treatments/categories) for free text.
- **`booksy-pp-cli discover treatments`** - Popular treatments to browse/seed a search.

### me

Your Booksy account (requires BOOKSY_ACCESS_TOKEN)

- **`booksy-pp-cli me home`** - Your Booksy home: active booking box, favorite/visited businesses, categories. Requires auth.
- **`booksy-pp-cli me profile`** - Your customer profile (name, email, phone). Requires auth.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`booksy-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`booksy-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`booksy-pp-cli learnings list`** - Inspect taught rows
- **`booksy-pp-cli learnings forget <query>`** - Undo a teach
- **`booksy-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`booksy-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`booksy-pp-cli teach-pattern`** - Install a query/resource template up front
- **`booksy-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `BOOKSY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `booksy-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
booksy-pp-cli businesses get mock-value

# JSON for scripting and agents
booksy-pp-cli businesses get mock-value --json
# Filter to specific fields
booksy-pp-cli businesses get mock-value --json --select id,name,slug

# Dry run — show the request without sending
booksy-pp-cli businesses get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
booksy-pp-cli businesses get mock-value --agent
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
booksy-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `booksy-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/booksy-pp-cli/config.toml`; `--home`, `BOOKSY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BOOKSY_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `booksy-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `booksy-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BOOKSY_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 with 'Wymagany jest token autoryzacji' (auth_header_token required)** — Set your session token: booksy-pp-cli auth set-token <x-access-token from booksy.com>.
- **Search returns nothing for a city name** — Resolve the city first with `discover locations --query <city>` and pass its id via --location-id.
- **book says 'refusing to book under a test harness'** — Expected — book never commits during automated verification; run it interactively with --confirm.
