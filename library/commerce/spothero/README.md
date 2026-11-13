# SpotHero CLI

**Find, quote, book and cancel SpotHero parking from the terminal, with spend caps and hard gates.**

SpotHero only offers an API to business partners. This CLI uses the same consumer endpoints as spothero.com, signs in from your Chrome session, and adds what a web checkout lacks for agents: preflight checks, --max-total spend caps, and explicit gates on anything that charges or refunds.

## Install

**Today, build from source.** This CLI is not in the public library yet, so the `npx` and release options below work only after it is published. Its module path is the bare `spothero-pp-cli`, so `go install github.com/amansk/...` does not work.

```bash
git clone https://github.com/mvanhorn/printing-press-library
cd spothero-pp-cli
go build -trimpath -o ~/.local/bin/spothero-pp-cli ./cmd/spothero-pp-cli
```

Once published:

The recommended path installs both the `spothero-pp-cli` binary and the `pp-spothero` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install spothero
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install spothero --cli-only
```

For skill only - installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install spothero --skill-only
```

To constrain the skill install to one or more specific agents (repeatable - agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install spothero --agent claude-code
npx -y @mvanhorn/printing-press-library install spothero --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.11 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/spothero/cmd/spothero-pp-cli@latest
```

This installs the CLI only - no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/spothero-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Available once this CLI is published to the public library.

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install spothero --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-spothero --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-spothero --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Available once this CLI is published to the public library.
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install spothero --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

The bundle below is available once this CLI is published; until then, build `spothero-pp-mcp` from source.

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle - Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session - set it up first if you haven't:

```bash
spothero-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/spothero-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/spothero/cmd/spothero-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "spothero": {
      "command": "spothero-pp-mcp"
    }
  }
}
```

</details>

### Remote MCP over HTTP

`spothero-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `SPOTHERO_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `SPOTHERO_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export SPOTHERO_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
spothero-pp-mcp --transport http
```

Set `SPOTHERO_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Authentication

Sign in at spothero.com in Chrome, then run auth login --chrome. The CLI reads the AUTH_TOKEN session cookie directly from your Chrome profile, decrypting it with the Chrome Safe Storage key (macOS may ask once for Keychain access), and only saves it after the API accepts it. No token copying. The session lasts about 30 days. auth status re-checks against the API every time. You can also use --cookies-file or the SPOTHERO_COOKIES env var.

## Booking and cancelling

Booking and cancelling are real and hard gated. Everything else reads.

**Book.** `book preview` quotes without charging. `book place` with no gates shows that same preview and reports `"placed": false`. It charges only with all three of `--enable-live-booking --owner-approved --confirm "PLACE SPOTHERO BOOKING"`; a partial or misspelled gate is refused with exit 2. `--max-total` refuses when the fresh quote exceeds your cap. `--dry-run` prints the request with secrets masked and never posts.

**Cancel.** `cancel preview <id>` shows refund eligibility and issues a token that is single use, lasts 15 minutes, and fixes the reservation, reason and refund destination. Then `cancel <id> --yes --confirm <token>`. Naive `--starts`/`--ends` use the facility's own time zone. A reservation refundable only as credit needs `cancel preview <id> --refund-as-credit`.

**No retries.** If a booking or cancel gets a dropped connection or a server error, the outcome is unknown. The CLI does not retry, because that could charge twice. It reads reservations, reports `"placed": "unknown"` with what it found, and exits 5. Check before trying again.

**MCP.** The MCP server exposes search, reservations, `book preview` and refund eligibility, but not `book place`, `cancel`, or `cancel preview`, whose token is useless without `cancel`. An MCP `confirm` value is set by the model, which is not a human approval, so spending and refunding stay in the CLI where a person can see the command.

### What has been proven live

Checked 2026-11-07 against a real account, including one real booking that was then cancelled, with the owner approving the exact garage, window and price.

| Step | Status |
|---|---|
| `auth login --chrome`, `auth status`, `doctor` | Proven live |
| `find`, `best`, `book preview`, `preflight`, `commute` | Proven live |
| `reservations list`, `reservations get`, `next`, `refundable`, `rebook`, `sync`, `regulars` | Proven live |
| `account cards`, `account vehicles`, `facilities` | Proven live |
| `cancel preview` (issues only a local token) | Proven live |
| Refund eligibility read behind `cancel preview` | Proven live |
| `book place` checkout request | Proven live: placed, and the reservation read back as valid |
| `cancel` refund request | Proven live: reservation read back as cancelled. The refund reaching the card was not observed by the CLI |

To repeat that test, run this. It charges a real card for a cheap booking and then cancels it. Pick a garage and a window a few days out.

```bash
spothero-pp-cli preflight --facility-id <ID> --starts <START> --ends <END> --max-total 15 --agent
spothero-pp-cli book place --facility-id <ID> --starts <START> --ends <END> --max-total 15 \
  --enable-live-booking --owner-approved --confirm "PLACE SPOTHERO BOOKING" --json
spothero-pp-cli reservations list --agent --select rental_id,facility_id,starts,reservation_status
spothero-pp-cli cancel preview <RENTAL_ID> --json
spothero-pp-cli cancel <RENTAL_ID> --yes --confirm <CONFIRM_TOKEN> --json
spothero-pp-cli reservations get <RENTAL_ID> --agent --select rental_id,reservation_status
```

Expect `"placed": true`, the new reservation in the list, `"cancelled": true`, and a final `reservation_status` of `cancelled`.

### Commands

| Command | What it does |
|---|---|
| `find` | Live inventory near an address or point |
| `facilities` | One facility's details, with no quote |
| `book preview` | Quote without charging |
| `book place` | Charge a saved card, hard gated |
| `cancel preview` | Refund eligibility and a confirm token |
| `cancel` | Refund a reservation, token gated |
| `account cards` | Saved cards, last4 and ids only |
| `account vehicles` | Saved vehicles, last two plate characters only |

`search` is different from `find`: it searches only your synced local data.

## Quick Start

```bash
# Confirm the CLI runs
spothero-pp-cli doctor --dry-run

# Import and verify your Chrome session
spothero-pp-cli auth login --chrome

# Search parking for a window
spothero-pp-cli find --address "233 S Wacker Dr, Chicago, IL" --starts 2027-01-15T09:00 --ends 2027-01-15T12:00 --agent

# Quote one garage without charging
spothero-pp-cli book preview --facility-id 2175 --starts 2027-01-15T09:00 --ends 2027-01-15T12:00 --agent

# See your reservations
spothero-pp-cli reservations list --agent --select rental_id,stall_name,starts,reservation_status

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Decide faster
- **`best`**: Get one ranked parking pick plus alternatives for a place and time window.

  _Reach for this when asked to pick a garage rather than list them._

  ```bash
  spothero-pp-cli best --address "233 S Wacker Dr, Chicago, IL" --starts 2027-01-15T09:00 --ends 2027-01-15T12:00 --agent
  ```

### Book with guardrails
- **`preflight`**: Check session, vehicle, card, overlaps and spend cap before you try to book.

  _Run this before asking an owner to approve a booking._

  ```bash
  spothero-pp-cli preflight --facility-id 2175 --starts 2027-01-15T09:00 --ends 2027-01-15T12:00 --max-total 40 --agent
  ```
- **`rebook`**: Re-quote a past reservation's garage for a new window and compare with what you paid.

  _Use when the owner wants the same garage again._

  ```bash
  spothero-pp-cli rebook --starts 2027-01-15T09:00 --ends 2027-01-15T12:00 --agent
  ```
- **`commute`**: Quote a recurring series of parking windows with a total.

  _Use to budget a recurring commute._

  ```bash
  spothero-pp-cli commute --facility-id 2175 --days mon,wed --start 09:00 --end 17:00 --weeks 1 --agent
  ```

### Your reservations
- **`next`**: See your soonest upcoming reservation and when it starts.

  _Use for what parking do I have next._

  ```bash
  spothero-pp-cli next --agent
  ```
- **`refundable`**: See refund status across upcoming reservations at once.

  _Use before cancelling or when plans change._

  ```bash
  spothero-pp-cli refundable --days 30 --agent
  ```
- **`regulars`**: See which garages you book most, with last booked and last price.

  _Use to resolve my usual garage._

  ```bash
  spothero-pp-cli regulars --agent
  ```

## Recipes

### Pick a garage under a cap

```bash
spothero-pp-cli best --address "233 S Wacker Dr, Chicago, IL" --starts 2027-01-15T09:00 --ends 2027-01-15T12:00 --max-total 40 --agent
```

Returns the top pick and alternatives within the cap.

### Trim reservations to key fields

```bash
spothero-pp-cli reservations list --agent --select rental_id,stall_name,starts,reservation_status
```

Reservation rows are large; select only what you need.

### Check before booking

```bash
spothero-pp-cli preflight --facility-id 2175 --starts 2027-01-15T09:00 --ends 2027-01-15T12:00 --max-total 40 --agent
```

Read-only verdict on session, vehicle, card, overlap and cap.

### Refund status

```bash
spothero-pp-cli refundable --days 30 --agent
```

Eligibility across upcoming reservations, read only.

## Usage

Run `spothero-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SPOTHERO_CONFIG_DIR`, `SPOTHERO_DATA_DIR`, `SPOTHERO_STATE_DIR`, or `SPOTHERO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SPOTHERO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SPOTHERO_HOME=/srv/spothero
spothero-pp-cli doctor
```

Under `SPOTHERO_HOME=/srv/spothero`, the four dirs resolve to `/srv/spothero/config`, `/srv/spothero/data`, `/srv/spothero/state`, and `/srv/spothero/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:SPOTHERO_HOME = "C:\srv\spothero"
spothero-pp-cli doctor
```

Under `$env:SPOTHERO_HOME = "C:\srv\spothero"`, the four dirs resolve to `C:\srv\spothero\config`, `C:\srv\spothero\data`, `C:\srv\spothero\state`, and `C:\srv\spothero\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "spothero": {
      "command": "spothero-pp-mcp",
      "env": {
        "SPOTHERO_HOME": "/srv/spothero"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SPOTHERO_DATA_DIR` overrides an explicit `--home` for that kind. Use `SPOTHERO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SPOTHERO_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `spothero-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `spothero-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `spothero-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `SPOTHERO_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `SPOTHERO_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### airports

Airports SpotHero serves (public)

- **`spothero-pp-cli airports`** - List airports with IATA codes and time zones

### cities

Cities SpotHero serves (public)

- **`spothero-pp-cli cities`** - List SpotHero cities with slugs and coordinates

### reservations

Your SpotHero reservations (read only)

- **`spothero-pp-cli reservations eligibility`** - Show whether a reservation can be refunded and which cancellation reasons apply
- **`spothero-pp-cli reservations get`** - Get one reservation by rental id
- **`spothero-pp-cli reservations list`** - List your reservations, most recent first

### users

Your SpotHero account profile (read only)

- **`spothero-pp-cli users`** - Show the signed in account


### best


- **`spothero-pp-cli best`** - Get one ranked parking pick plus alternatives for a place and time window.


### commute


- **`spothero-pp-cli commute`** - Quote a recurring series of parking windows with a total.


### next


- **`spothero-pp-cli next`** - See your soonest upcoming reservation and when it starts.


### preflight


- **`spothero-pp-cli preflight`** - Check session, vehicle, card, overlaps and spend cap before you try to book.


### rebook


- **`spothero-pp-cli rebook`** - Re-quote a past reservation's garage for a new window and compare with what you paid.


### refundable


- **`spothero-pp-cli refundable`** - See refund status across upcoming reservations at once.


### regulars


- **`spothero-pp-cli regulars`** - See which garages you book most, with last booked and last price.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`spothero-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`spothero-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`spothero-pp-cli learnings list`** - Inspect taught rows
- **`spothero-pp-cli learnings forget <query>`** - Undo a teach
- **`spothero-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`spothero-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`spothero-pp-cli teach-pattern`** - Install a query/resource template up front
- **`spothero-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SPOTHERO_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `spothero-pp-cli` opens the database, older binaries refuse it with a version error - upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
spothero-pp-cli airports

# JSON for scripting and agents
spothero-pp-cli airports --json
# Filter to specific fields
spothero-pp-cli airports --json --select iata_code,name,city

# Dry run - show the request without sending
spothero-pp-cli airports --dry-run

# Agent mode - JSON + compact + no prompts in one flag
spothero-pp-cli airports --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Reads by default** - the only commands that change anything at SpotHero are `book place` and `cancel`, both hard gated. See Booking and cancelling
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
spothero-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `spothero-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/spothero-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\spothero-pp-cli\config.toml` on Windows; `--home`, `SPOTHERO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `SPOTHERO_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `SPOTHERO_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SPOTHERO_COOKIES` | per_call | No | A raw Cookie header holding AUTH_TOKEN, as an alternative to `auth login --chrome`. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `spothero-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `spothero-pp-cli doctor` to check credentials
- Verify the environment variable is set: `test -n "$SPOTHERO_COOKIES" && echo set` (never echo the value)
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **auth login says the session was rejected**: Sign in at https://spothero.com in Chrome, wait a minute for Chrome to write cookies to disk, then rerun spothero-pp-cli auth login --chrome
- **auth login --chrome cannot read the Keychain**: Allow the macOS prompt for Chrome Safe Storage, or use spothero-pp-cli auth login --cookies-file <path>
- **book place reports an ambiguous result**: Do not rerun it. Run spothero-pp-cli reservations list --agent and look for the new reservation first
