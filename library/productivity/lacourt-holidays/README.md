# LA Superior Court Holidays CLI

**LA Superior Court judicial holidays plus court-day deadline math and calendar export, from the court's own published list.**

Lists the Superior Court of Los Angeles County's judicial holidays straight from the court's site, caches them offline, and answers the questions the page can't: is-court-day, add-court-days, deadline roll-forward, court-days-between, next-holiday. ics exports every published year as calendar files.

Learn more at [LA Superior Court Holidays](https://www.lacourt.ca.gov/apps/court-holidays).

Created by [@bbenjyesq](https://github.com/bbenjyesq) (Bob Benjy).

## Install

The recommended path installs both the `lacourt-holidays-pp-cli` binary and the `pp-lacourt-holidays` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install lacourt-holidays
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install lacourt-holidays --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install lacourt-holidays --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install lacourt-holidays --agent claude-code
npx -y @mvanhorn/printing-press-library install lacourt-holidays --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.10 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/lacourt-holidays/cmd/lacourt-holidays-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/lacourt-holidays-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install lacourt-holidays --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-lacourt-holidays --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-lacourt-holidays --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install lacourt-holidays --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/lacourt-holidays-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/lacourt-holidays/cmd/lacourt-holidays-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "lacourt-holidays": {
      "command": "lacourt-holidays-pp-mcp"
    }
  }
}
```

</details>

### Remote MCP over HTTP

`lacourt-holidays-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `LACOURT_HOLIDAYS_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `LACOURT_HOLIDAYS_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export LACOURT_HOLIDAYS_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
lacourt-holidays-pp-mcp --transport http
```

Set `LACOURT_HOLIDAYS_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Quick Start

```bash
# Confirm the CLI is installed and configured
lacourt-holidays-pp-cli doctor --dry-run

# See the next day the court is closed
lacourt-holidays-pp-cli next-holiday

# Count 16 court days from a date
lacourt-holidays-pp-cli add-court-days 2027-03-01 16

# Write one .ics per published year for your calendar
lacourt-holidays-pp-cli ics --out-dir ./lasc-holidays

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Court-day math
- **`is-court-day`** — Answer yes/no for whether a date is an LASC court day, with the reason.

  _Reach for this before setting a hearing or filing date._

  ```bash
  lacourt-holidays-pp-cli is-court-day 2027-11-26 --agent
  ```
- **`add-court-days`** — Count N court days forward or backward from a date, skipping weekends and LASC holidays.

  _Use for any court-day deadline instead of counting by hand._

  ```bash
  lacourt-holidays-pp-cli add-court-days 2027-03-01 16 --agent
  ```
- **`next-holiday`** — Show the next LASC court holiday and how many days away it is.

  _Quick answer to 'when is the court closed next?'._

  ```bash
  lacourt-holidays-pp-cli next-holiday --agent
  ```
- **`deadline`** — Add N calendar days and roll a weekend or holiday end date to the next court day.

  _Use for calendar-day periods; use add-court-days for court-day periods._

  ```bash
  lacourt-holidays-pp-cli deadline 2027-03-01 30 --agent
  ```
- **`court-days-between`** — Count court days between two dates and list the holidays in the span.

  _Check whether there is still time before a hearing._

  ```bash
  lacourt-holidays-pp-cli court-days-between 2027-03-01 2027-04-15 --agent
  ```

### Calendar export
- **`ics`** — Export LASC holidays as calendar files, for one year or every published year.

  _Import every upcoming court holiday onto your own calendar or device._

  ```bash
  lacourt-holidays-pp-cli ics --out-dir ./lasc-holidays
  ```

## Recipes

### Is the day after Thanksgiving a court day?

```bash
lacourt-holidays-pp-cli is-court-day 2027-11-26 --agent
```

Returns false with the holiday name as the reason.

### Count back 16 court days from a hearing

```bash
lacourt-holidays-pp-cli add-court-days 2027-05-20 16 --back
```

--back counts backward, skipping weekends and holidays.

### Just the dates and names

```bash
lacourt-holidays-pp-cli next-holiday --agent --select date,description
```

Narrow agent output to the fields you need.

### Every year as calendar files

```bash
lacourt-holidays-pp-cli ics --out-dir ./lasc-holidays
```

One .ics per published year to import on your device.

## Usage

Run `lacourt-holidays-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `LACOURT_HOLIDAYS_CONFIG_DIR`, `LACOURT_HOLIDAYS_DATA_DIR`, `LACOURT_HOLIDAYS_STATE_DIR`, or `LACOURT_HOLIDAYS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `LACOURT_HOLIDAYS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export LACOURT_HOLIDAYS_HOME=/srv/lacourt-holidays
lacourt-holidays-pp-cli doctor
```

Under `LACOURT_HOLIDAYS_HOME=/srv/lacourt-holidays`, the four dirs resolve to `/srv/lacourt-holidays/config`, `/srv/lacourt-holidays/data`, `/srv/lacourt-holidays/state`, and `/srv/lacourt-holidays/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:LACOURT_HOLIDAYS_HOME = "C:\srv\lacourt-holidays"
lacourt-holidays-pp-cli doctor
```

Under `$env:LACOURT_HOLIDAYS_HOME = "C:\srv\lacourt-holidays"`, the four dirs resolve to `C:\srv\lacourt-holidays\config`, `C:\srv\lacourt-holidays\data`, `C:\srv\lacourt-holidays\state`, and `C:\srv\lacourt-holidays\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "lacourt-holidays": {
      "command": "lacourt-holidays-pp-mcp",
      "env": {
        "LACOURT_HOLIDAYS_HOME": "/srv/lacourt-holidays"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `LACOURT_HOLIDAYS_DATA_DIR` overrides an explicit `--home` for that kind. Use `LACOURT_HOLIDAYS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `LACOURT_HOLIDAYS_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `lacourt-holidays-pp-cli doctor --fail-on warn` to check path warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. The learn journal lives under `learn/` in the state directory. `lacourt-holidays-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `lacourt-holidays-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `LACOURT_HOLIDAYS_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `LACOURT_HOLIDAYS_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### holidays

Manage holidays

- **`lacourt-holidays-pp-cli holidays`** - List court holidays for a year (published 2006 through about five years ahead; years before 2006 and beyond the published window return an empty list)


### add-court-days


- **`lacourt-holidays-pp-cli add-court-days`** - Count N court days forward or backward from a date, skipping weekends and LASC holidays.


### court-days-between


- **`lacourt-holidays-pp-cli court-days-between`** - Count court days between two dates and list the holidays in the span.


### deadline


- **`lacourt-holidays-pp-cli deadline`** - Add N calendar days and roll a weekend or holiday end date to the next court day.


### ics


- **`lacourt-holidays-pp-cli ics`** - Export LASC holidays as calendar files, for one year or every published year.


### is-court-day


- **`lacourt-holidays-pp-cli is-court-day`** - Answer yes/no for whether a date is an LASC court day, with the reason.


### next-holiday


- **`lacourt-holidays-pp-cli next-holiday`** - Show the next LASC court holiday and how many days away it is.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`lacourt-holidays-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`lacourt-holidays-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`lacourt-holidays-pp-cli learnings list`** - Inspect taught rows
- **`lacourt-holidays-pp-cli learnings forget <query>`** - Undo a teach
- **`lacourt-holidays-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`lacourt-holidays-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`lacourt-holidays-pp-cli teach-pattern`** - Install a query/resource template up front
- **`lacourt-holidays-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `LACOURT_HOLIDAYS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `lacourt-holidays-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
lacourt-holidays-pp-cli holidays --year 2027

# JSON for scripting and agents
lacourt-holidays-pp-cli holidays --year 2027 --json
# Filter to specific fields by name
lacourt-holidays-pp-cli holidays --year 2027 --json --select date,description

# Dry run — show the request without sending
lacourt-holidays-pp-cli holidays --year 2027 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
lacourt-holidays-pp-cli holidays --year 2027 --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - holiday lookups are cached locally (GET responses for five minutes) and also stored in the local SQLite store
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited or blocked, `10` config error.

## Health Check

```bash
lacourt-holidays-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `lacourt-holidays-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/lacourt-holidays-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\lacourt-holidays-pp-cli\config.toml` on Windows; `--home`, `LACOURT_HOLIDAYS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `LACOURT_HOLIDAYS_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `LACOURT_HOLIDAYS_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

## Troubleshooting

**Not found errors (exit code 3)**
- The date or year is outside what the court has published (2006 through about five years ahead). Check a year with `lacourt-holidays-pp-cli holidays --year 2027`.

### API-specific
- **Date is outside published years** — The court publishes 2006 through about five years ahead (2030 today); run lacourt-holidays-pp-cli holidays --year <year> to check a year
- **Results look stale** — Add --no-cache to bypass the local response cache and fetch the court's current list

## When not to use this CLI

- Other courts' holidays (federal courts, other counties)
- Looking up statutory notice periods; it only does the date arithmetic you give it
- As legal advice; confirm critical deadlines against the governing statute and the court

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `LACOURT_HOLIDAYS_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override. `--insecure` overrides `--ca-cert` and `--tls-server-name`.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
