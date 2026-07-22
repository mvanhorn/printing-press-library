# Vagaro CLI

**Every Vagaro discovery feature, plus the marketplace-wide availability search, price comparison, and local database no Vagaro tool has.**

Vagaro's website is a per-business click-through funnel with no way to ask a question across the whole marketplace. This CLI syncs businesses, services, providers, reviews, and availability into a local SQLite store, so you can find any open slot matching your constraints (find), compare businesses head to head (compare), check whether a price is fair (price-check), and rebook your usual with the same provider (me rebook) — all with agent-native --json output.

Learn more at [Vagaro](https://www.vagaro.com).

Created by [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `vagaro-pp-cli` binary and the `pp-vagaro` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install vagaro
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install vagaro --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install vagaro --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install vagaro --agent claude-code
npx -y @mvanhorn/printing-press-library install vagaro --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/health/vagaro/cmd/vagaro-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/vagaro-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install vagaro --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-vagaro --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-vagaro --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install vagaro --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
vagaro-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/vagaro-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/health/vagaro/cmd/vagaro-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "vagaro": {
      "command": "vagaro-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Public discovery (search, business detail, services, reviews, classes) needs no auth. For your own appointment history and profile, run 'vagaro-pp-cli auth login --chrome' to import your logged-in Vagaro session from Chrome (a JWT plus session cookies). Booking commands are handoff-only: `booking-link` / `book` verify a selected slot and can print the closest Vagaro booking URL plus browser steps, but they do not submit the appointment.

## Quick Start

```bash
# Health check — confirms the CLI is wired up before any network calls.
vagaro-pp-cli doctor --dry-run

# Look up a business by its vagaro.com/<slug> handle.
vagaro-pp-cli business get centralbarber

# See that business's service menu with prices as JSON.
vagaro-pp-cli business services centralbarber --json

# Browse upcoming livestream classes.
vagaro-pp-cli classes --page-size 10 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`find`** — Find nearby businesses with a service open soonest, filtered by price, rating, and a date/time window.

  _Reach for this when a user wants any open slot matching constraints across many businesses, not a specific known place._

  ```bash
  vagaro-pp-cli find massage --max-price 120 --min-rating 4.5 --from thu --to sat --agent
  ```
- **`compare`** — Compare named businesses side by side: rating, review count, price range, matching-service price, and next-available.

  _Use when the user already has 2-3 businesses in mind and wants a decision table._

  ```bash
  vagaro-pp-cli compare centralbarber rudysbarbershop --agent
  ```
- **`price-check`** — Show the price spread (min/median/max) for a service across a metro and flag who's below median.

  _Use to judge whether a quoted price is fair or to find below-median providers._

  ```bash
  vagaro-pp-cli price-check haircut --city seattle --agent
  ```
- **`market`** — One-shot landscape of a metro: business count, rating distribution, and price ranges by category.

  _Use when someone is new to an area and wants the lay of the land before picking a regular spot._

  ```bash
  vagaro-pp-cli market seattle --agent
  ```
- **`menu-diff`** — Diff a business's service menu across synced snapshots to catch price changes and added/removed services.

  _Use to detect silent price hikes or menu changes at a business you follow._

  ```bash
  vagaro-pp-cli menu-diff centralbarber --agent
  ```

### Booking that remembers you
- **`me rebook`** — Re-run your usual: reads your past appointment (business + service + provider) and lists that provider's open slots in a date window so you can pick a future time.

  _Use when the business/service/provider are already known from history, such as another haircut with the same barber. The variable is the future slot, not provider discovery._

  ```bash
  vagaro-pp-cli me rebook --last --from thu --to sat --agent
  ```
- **`watch`** — Check one business/provider's next-available against a stored baseline and report if a slot opened up sooner.

  _Use when waiting on a booked-out provider to open a sooner slot._

  ```bash
  vagaro-pp-cli watch centralbarber --service haircut --before 2026-07-05 --agent
  ```
- **`business availability`** — Query one known business for available slots, scoped to a service, provider, and date range.

  _Use when the user already knows the business and needs a precise provider/service/window answer._

  ```bash
  vagaro-pp-cli business availability sample-shop --service haircut --provider alex --from 2026-07-20 --to 2026-07-31 --agent
  ```

## Recipes

### Find an open massage this weekend under budget

```bash
vagaro-pp-cli find massage --max-price 120 --min-rating 4.5 --from sat --to sun --agent
```

Fans out across nearby businesses and ranks those with an open slot in the window under your price and rating floor.

### Compare two barbers before committing

```bash
vagaro-pp-cli compare centralbarber rudysbarbershop --agent --select name,rating,priceRange,nextAvailable
```

Side-by-side decision table narrowed to the fields an agent needs, avoiding the verbose full payload.

### Rebook your usual haircut with the same provider

```bash
vagaro-pp-cli me rebook --last --from thu --to sat --agent
```

Reads your most recent past appointment, keeps the same business/service/provider, and lists that provider's open times inside the requested window. For a recurring haircut, this is the default path: use history to keep "Central Barber + Ronnel + haircut" fixed, then choose the future slot.

If the most recent appointment is not the one you want to repeat, first list your appointment history and pass the desired appointment id:

```bash
vagaro-pp-cli me --past --agent --select appointment_id,business_name,service_name,provider_name,date
vagaro-pp-cli me rebook <appointment-id> --from 2026-07-20 --to 2026-07-31 --agent
```

### Turn a selected rebook slot into a booking handoff

After `me rebook` returns slots, choose one `YYYY-MM-DDTHH:MM` value and verify it with `booking-link` / `book` before opening Vagaro:

```bash
vagaro-pp-cli booking-link centralbarber --service 34098477 --provider 43931725 --at 2026-07-24T10:00 --agent
vagaro-pp-cli booking-link centralbarber --service 34098477 --provider 43931725 --at 2026-07-24T10:00 --confirm --agent
```

`booking-link` verifies availability and prints what would be booked. `book --confirm` prints the tightest Vagaro URL plus the exact browser steps for the verified service/provider/time. Neither command submits the appointment or performs checkout; the final confirmation still happens on Vagaro's site.

### When to use favorites instead of rebook

Use `favorites` when the question is "which businesses did I save?" or when there is no relevant past appointment. For a routine repeat appointment, such as another haircut with the same barber at the same shop, skip favorites and use `me rebook`: appointment history already carries the business, service, and provider, so the only real choice is the future slot.

### Check a known business for one provider's openings

```bash
vagaro-pp-cli business availability sample-shop --service haircut --provider alex --from 2026-07-20 --weeks 2 --agent
```

Resolves the service and provider by exact ID, exact name, or a unique name substring, then queries the requested weekly availability window. If a requested service or provider cannot be resolved uniquely, the command fails instead of falling back to the first service or any provider. Omit all flags to preserve the legacy behavior: first listed service, any provider, current week.

### Check if a haircut price is fair in your city

```bash
vagaro-pp-cli price-check haircut --city seattle --agent
```

Shows the metro price distribution and flags below-median providers.

## Usage

Run `vagaro-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `VAGARO_CONFIG_DIR`, `VAGARO_DATA_DIR`, `VAGARO_STATE_DIR`, or `VAGARO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `VAGARO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export VAGARO_HOME=/srv/vagaro
vagaro-pp-cli doctor
```

Under `VAGARO_HOME=/srv/vagaro`, the four dirs resolve to `/srv/vagaro/config`, `/srv/vagaro/data`, `/srv/vagaro/state`, and `/srv/vagaro/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "vagaro": {
      "command": "vagaro-pp-mcp",
      "env": {
        "VAGARO_HOME": "/srv/vagaro"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `VAGARO_DATA_DIR` overrides an explicit `--home` for that kind. Use `VAGARO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `VAGARO_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `vagaro-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### business

Look up a Vagaro business (salon/spa/barber/fitness) by its slug

- **`vagaro-pp-cli business availability`** - Get a business's next-available booking summary; supports `--service <name-or-id>`, `--provider <name-or-id>`, `--from <date>`, `--to <date>`, and `--weeks <n>`
- **`vagaro-pp-cli business get`** - Get a business profile (name, rating, address, categories)
- **`vagaro-pp-cli business services`** - List a business's services with prices and durations

### classes

Browse upcoming livestream classes

- **`vagaro-pp-cli classes`** - List upcoming livestream classes

### listings

Browse businesses by service and location (live JSON-LD listings)

- **`vagaro-pp-cli listings <service> <location>`** - List businesses for a service in a city (city--state slug)

### me

Your own Vagaro account (requires auth login --chrome)

- **`vagaro-pp-cli me`** - List your appointments (upcoming or past)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
vagaro-pp-cli business get mock-value

# JSON for scripting and agents
vagaro-pp-cli business get mock-value --json

# Filter to specific fields
vagaro-pp-cli business get mock-value --json --select id,name,status

# Dry run — show the request without sending
vagaro-pp-cli business get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
vagaro-pp-cli business get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
vagaro-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `vagaro-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/vagaro/config.toml`; `--home`, `VAGARO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `vagaro-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Search always returns businesses from the wrong city** — Vagaro geo-scopes by your IP; pass an explicit location slug like 'search listings barber seattle--washington' or use 'find --city <city>'.
- **me appointments / book returns unauthorized** — Run 'vagaro-pp-cli auth login --chrome' to import your Vagaro session; the token expires after ~30 days, so re-run it if it lapses.
- **book says the business requires prepayment** — That business takes payment up front; complete it at the printed book-now URL (payment is out of scope for the CLI).

## Known Gaps

- **`booking-link --confirm` does not place appointments programmatically** (Vagaro has no booking-submit API — the checkout is a JavaScript widget). Instead it verifies the slot is open and returns a one-click-away handoff: the tightest booking URL (`/{slug}/services`) plus numbered steps naming the exact service, provider, and time to select, so finishing in the browser takes as few clicks as possible. The code has a `placeBooking()` seam for wiring a real submit if the endpoint is ever captured.
- **`favorites` may be unavailable.** Vagaro serves saved businesses through a signed endpoint this CLI intentionally avoids; the command attempts an authenticated read and reports honestly if no open endpoint is reachable.
- **`me` commands need a session.** Run `vagaro-pp-cli auth login --chrome` to import your Vagaro login; `me appointments` and `me rebook` require it.
- **Location is IP-based.** Search scopes to your machine's metro; the `--city` argument is advisory (Vagaro geolocates by IP).
