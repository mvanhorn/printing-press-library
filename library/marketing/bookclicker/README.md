# Bookclicker CLI

**Every Bookclicker workflow, plus a local mirror of the newsletter marketplace that makes launch planning a single query.**

Bookclicker is a marketplace where authors swap, sell and buy newsletter promo slots. Its web UI shows 25 newsletters at a time across many pages and makes you open one calendar per list. This CLI syncs the marketplace into local SQLite, so 'plan' can rank every candidate newsletter for a launch window by reach, price, or opens-per-dollar in one call, and keeps history so 'partner-roi', 'drift' and 'swap-balance' can answer questions the product structurally cannot.

## Install

The recommended path installs both the `bookclicker-pp-cli` binary and the `pp-bookclicker` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install bookclicker
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install bookclicker --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install bookclicker --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install bookclicker --agent claude-code
npx -y @mvanhorn/printing-press-library install bookclicker --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/bookclicker/cmd/bookclicker-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bookclicker-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install bookclicker --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bookclicker --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bookclicker --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install bookclicker --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bookclicker-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BOOKCLICKER_SESSION` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/bookclicker/cmd/bookclicker-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "bookclicker": {
      "command": "bookclicker-pp-mcp",
      "env": {
        "BOOKCLICKER_SESSION": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Bookclicker has no API keys or OAuth. Authentication is a Rails session cookie plus a CSRF token, exactly like a browser. Run 'auth login' to sign in and store the session in your local config, or import an existing browser session. Mutating commands automatically attach the X-CSRF-Token header. The stored session is a credential: it lives in your local config file and is never written to logs or output.

## Quick Start

```bash
# Check config, session state and local store before anything else.
bookclicker-pp-cli doctor --dry-run

# Mirror the marketplace, your lists, books and pen names into local SQLite.
bookclicker-pp-cli sync

# Full-text search the synced marketplace offline.
bookclicker-pp-cli search "romantic suspense"

# Rank every newsletter that can run this book in the window.
bookclicker-pp-cli plan --book 12345 --from 2026-09-01 --to 2026-09-30

# See which sent promos still need confirming.
bookclicker-pp-cli confirm-due

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Launch planning
- **`plan`** — Find every newsletter that can run your book in a date window, ranked by fit.

  _Reach for this instead of paging the marketplace: it answers 'who can promote this book, when, for how much' in one call._

  ```bash
  bookclicker-pp-cli plan --book 12345 --from 2026-09-01 --to 2026-09-30 --max-price 25 --agent
  ```
- **`search`** — Full-text search every synced newsletter by name, pen name, or genre in milliseconds.

  _Use for any 'which lists cover X' question rather than walking paginated API results._

  ```bash
  bookclicker-pp-cli search "romantic suspense" --agent
  ```
- **`plan`** — Rank candidate lists by estimated opens per dollar instead of raw subscriber count.

  _Biggest list is rarely the best value; this is the ranking the UI cannot express._

  ```bash
  bookclicker-pp-cli plan --from 2026-09-01 --to 2026-09-30 --rank value --limit 20 --agent
  ```

### Partner intelligence
- **`swap-balance`** — See which swap partners agreed to a swap and then cancelled or declined.

  _Use before rebooking a partner: it surfaces repeat cancellers that the product never flags._

  ```bash
  bookclicker-pp-cli swap-balance --flaky --agent
  ```
- **`partner-roi`** — Rank past promo partners by delivered reach against what they cost.

  _Use when deciding who to rebook; needs at least one prior sync of history._

  ```bash
  bookclicker-pp-cli partner-roi --since 180d --agent
  ```
- **`drift`** — Show newsletters whose open or click rate has decayed since earlier syncs.

  _Catches decaying lists before you spend a launch slot on them._

  ```bash
  bookclicker-pp-cli drift --min-drop 0.05 --agent
  ```

### Operations
- **`confirm-due`** — List every promo awaiting your confirmation, oldest first.

  _This is the product's recurring manual chore; run it before confirming promos._

  ```bash
  bookclicker-pp-cli confirm-due --agent
  ```
- **`launch health`** — Show which dates in a book's launch window still have no promo booked.

  _Answers 'is this launch actually covered' without reading a calendar by eye._

  ```bash
  bookclicker-pp-cli launch health --book 12345 --agent
  ```
- **`capacity`** — Show remaining Solo, Feature and Mention slots per newsletter per date.

  _Use before sending an offer to confirm the slot type is actually available._

  ```bash
  bookclicker-pp-cli capacity --list 12345 --from 2026-09-01 --to 2026-09-14 --agent
  ```
- **`stale`** — List pending offers that have gone unanswered the longest.

  _Surfaces offers worth cancelling and rebooking elsewhere._

  ```bash
  bookclicker-pp-cli stale --days 7 --agent
  ```

## Recipes

### Fill a launch window on a budget

```bash
bookclicker-pp-cli plan --book 12345 --from 2026-09-01 --to 2026-09-30 --max-price 25 --rank value --agent --select lists.name,lists.solo_price,lists.active_member_count,lists.open_rate
```

Ranks candidate newsletters by opens-per-dollar and narrows the payload to the four fields that drive the decision.

### Find partners who cancel swaps

```bash
bookclicker-pp-cli swap-balance --flaky --agent
```

Lists counterparties who agreed to a swap and then cancelled or declined it.

### Clear the confirmation backlog

```bash
bookclicker-pp-cli confirm-due --agent
```

Shows every promo awaiting confirmation so none silently ages out.

### Check a list before booking it

```bash
bookclicker-pp-cli capacity --list 12345 --from 2026-09-01 --to 2026-09-14
```

Shows remaining Solo, Feature and Mention slots per date against the platform caps.

### Spot decaying newsletters

```bash
bookclicker-pp-cli drift --min-drop 0.05
```

Flags lists whose open or click rate fell since an earlier sync, before you rebook them.

## Usage

Run `bookclicker-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `BOOKCLICKER_CONFIG_DIR`, `BOOKCLICKER_DATA_DIR`, `BOOKCLICKER_STATE_DIR`, or `BOOKCLICKER_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `BOOKCLICKER_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export BOOKCLICKER_HOME=/srv/bookclicker
bookclicker-pp-cli doctor
```

Under `BOOKCLICKER_HOME=/srv/bookclicker`, the four dirs resolve to `/srv/bookclicker/config`, `/srv/bookclicker/data`, `/srv/bookclicker/state`, and `/srv/bookclicker/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "bookclicker": {
      "command": "bookclicker-pp-mcp",
      "env": {
        "BOOKCLICKER_HOME": "/srv/bookclicker"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `BOOKCLICKER_DATA_DIR` overrides an explicit `--home` for that kind. Use `BOOKCLICKER_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `BOOKCLICKER_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `bookclicker-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account

Your Bookclicker account snapshot

- **`bookclicker-pp-cli account`** - Get the account snapshot: user settings, owned lists, books and pen names

### booking_calendars

Booking-side availability view

- **`bookclicker-pp-cli booking-calendars`** - Get booking availability for a newsletter and book

### books

Books you promote, grouped under pen names

- **`bookclicker-pp-cli books create`** - Add a book under a pen name
- **`bookclicker-pp-cli books delete`** - Delete a book
- **`bookclicker-pp-cli books get`** - Get one book by id
- **`bookclicker-pp-cli books list`** - List every book on your account
- **`bookclicker-pp-cli books update`** - Update a book

### calendars

Dated availability for a newsletter

- **`bookclicker-pp-cli calendars`** - Get dated availability for a newsletter over N months

### confirm_promos

Confirming that booked promotions actually went out

- **`bookclicker-pp-cli confirm-promos create`** - Confirm a promotion was sent, naming the campaign that carried it
- **`bookclicker-pp-cli confirm-promos options`** - List the newsletter campaigns that could satisfy a promotion

### conversations

Messages with counterparties

- **`bookclicker-pp-cli conversations`** - Start a conversation with another user

### external_reservations

Promotions booked outside Bookclicker

- **`bookclicker-pp-cli external-reservations create`** - Record a promotion booked off-platform
- **`bookclicker-pp-cli external-reservations update`** - Update an off-platform promotion record

### integrations

Newsletter provider integrations

- **`bookclicker-pp-cli integrations <id>`** - Get one newsletter provider integration and its health status. The provider API key is redacted.

### inventories

Per-date promotion slots on a newsletter

- **`bookclicker-pp-cli inventories get`** - Get the promotion slots offered on one date
- **`bookclicker-pp-cli inventories set`** - Set the promotion slots offered on one date

### lists

The newsletter marketplace

- **`bookclicker-pp-cli lists campaigns`** - Campaign history for a newsletter
- **`bookclicker-pp-cli lists search`** - Search marketplace newsletters available to promote a book

### my_lists

Newsletters you own and sell or swap spots on

- **`bookclicker-pp-cli my-lists`** - List your own newsletters with pricing and swap settings

### pen_names

Author identities that own books and newsletters

- **`bookclicker-pp-cli pen-names create`** - Create a pen name
- **`bookclicker-pp-cli pen-names delete`** - Delete a pen name
- **`bookclicker-pp-cli pen-names for-buyer`** - List pen names eligible to book promotions as a buyer
- **`bookclicker-pp-cli pen-names list`** - List your pen names with their books, requests and groups
- **`bookclicker-pp-cli pen-names update`** - Update a pen name

### reservations

Swap and paid promotion bookings

- **`bookclicker-pp-cli reservations accept`** - Accept an incoming promotion offer
- **`bookclicker-pp-cli reservations buyer-cancel`** - Cancel a promotion you booked
- **`bookclicker-pp-cli reservations buyer-cancel-all`** - Cancel every promotion you booked (bulk, destructive)
- **`bookclicker-pp-cli reservations buyer-refund`** - Process a buyer-side refund
- **`bookclicker-pp-cli reservations decline`** - Decline an incoming promotion offer
- **`bookclicker-pp-cli reservations dismiss`** - Dismiss a reservation notice from the feed
- **`bookclicker-pp-cli reservations refund-request`** - Request a refund for a paid promotion
- **`bookclicker-pp-cli reservations request-confirmation`** - Ask the seller to confirm a promotion was sent
- **`bookclicker-pp-cli reservations seller-cancel`** - Cancel a promotion booked on your newsletter
- **`bookclicker-pp-cli reservations seller-cancel-all`** - Cancel every promotion booked on your newsletters (bulk, destructive)
- **`bookclicker-pp-cli reservations seller-refund`** - Issue a refund as the newsletter owner


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`bookclicker-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`bookclicker-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`bookclicker-pp-cli learnings list`** - Inspect taught rows
- **`bookclicker-pp-cli learnings forget <query>`** - Undo a teach
- **`bookclicker-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`bookclicker-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`bookclicker-pp-cli teach-pattern`** - Install a query/resource template up front
- **`bookclicker-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `BOOKCLICKER_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `bookclicker-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bookclicker-pp-cli account

# JSON for scripting and agents
bookclicker-pp-cli account --json

# Filter to specific fields
bookclicker-pp-cli account --json --select id,name,status

# Dry run — show the request without sending
bookclicker-pp-cli account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bookclicker-pp-cli account --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
bookclicker-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `bookclicker-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/bookclicker-pp-cli/config.toml`; `--home`, `BOOKCLICKER_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BOOKCLICKER_SESSION` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `bookclicker-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `bookclicker-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BOOKCLICKER_SESSION`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Every command returns 401 or redirects to the login page** — Run 'bookclicker-pp-cli auth login' — the Rails session cookie has expired.
- **A mutating command fails with 422 Unprocessable Entity** — The CSRF token is stale; run 'bookclicker-pp-cli auth refresh' to re-scrape it.
- **search or plan returns nothing** — Run 'bookclicker-pp-cli sync' first; both read the local mirror, not the live API.
- **partner-roi or drift shows little or no data** — Run `bookclicker-pp-cli reservations pull --all` to mirror reservation history, then re-run.
- **A GET against an endpoint returns 404** — Several Bookclicker routes are POST-only and reject GET; use the matching subcommand rather than a raw path.
