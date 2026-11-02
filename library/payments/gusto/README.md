# Gusto CLI

**Prep a Gusto payroll from the terminal, including mileage reimbursements that Gusto's hosted connector cannot enter, and stop at review so a human submits.**

gusto-pp-cli drives Gusto's own web app with your signed-in session. It sets hours, adds reimbursements (payrolls mileage turns miles into a labeled line), calculates, and reads back gross, net and debit. payrolls verify proves the draft matches your timesheet before anyone clicks Submit.

Learn more at [Gusto](https://app.gusto.com).

Created by [@beetz12](https://github.com/beetz12) (david).

## Install

The recommended path installs both the `gusto-pp-cli` binary and the `pp-gusto` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install gusto
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install gusto --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install gusto --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install gusto --agent claude-code
npx -y @mvanhorn/printing-press-library install gusto --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.10 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/gusto/cmd/gusto-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gusto-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install gusto --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-gusto --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-gusto --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install gusto --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
gusto-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gusto-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/gusto/cmd/gusto-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "gusto": {
      "command": "gusto-pp-mcp"
    }
  }
}
```

</details>

### Remote MCP over HTTP

`gusto-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `GUSTO_MCP_HTTP_TOKEN` is set, and every request must send `Authorization: Bearer <token>`. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`.

```bash
export GUSTO_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
gusto-pp-mcp --transport http
```

Set `GUSTO_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Authentication

This CLI uses your Gusto web session, not an API key, and works on macOS only. Step 1: install press-auth (go install github.com/mvanhorn/cli-printing-press/v4/cmd/press-auth@latest) and run press-auth login app.gusto.com --login-url https://app.gusto.com/login --complete-selector 'a[href="/pay"]'; sign in in the window it opens and choose Always Allow on the Keychain prompt. Step 2: run gusto-pp-cli auth login --chrome, which imports that session (it can also fall back to pycookiecheat, cookies or cookie-scoop-cli). Check it with gusto-pp-cli doctor. Sessions expire; repeat both steps when requests land on the login page.

## Quick Start

```bash
# Checks config and auth wiring without touching Gusto.
gusto-pp-cli doctor --dry-run

# Shows the open payroll and its submit deadline.
gusto-pp-cli payrolls due

# Previews the mileage reimbursement before writing it.
gusto-pp-cli payrolls mileage --period-end 2026-11-01 --employee 'Ava Example' --miles 5.2 --rate 0.60 --dry-run

# Calculates the draft and prints gross, net and debit for the human who submits.
gusto-pp-cli payrolls review --period-end 2026-11-01

```

## Unique Features

### Payroll prep without the web UI
- **`payrolls mileage`** — Add or correct the mileage reimbursement on the open draft payroll from miles and a per-mile rate; one mileage line per employee.

  _Reach for this whenever a timesheet lists trip miles; Gusto's hosted connector cannot put a reimbursement on a draft._

  ```bash
  gusto-pp-cli payrolls mileage --period-end 2026-11-01 --employee 'Ava Example' --miles 5.2 --rate 0.60 --dry-run
  ```
- **`payrolls prep`** — Load a week's hours and mileage from a CSV onto the open draft, calculate it, and print the review totals, stopping before submit.

  _Use this for the weekly payroll ritual so a human only has to review and submit._

  ```bash
  gusto-pp-cli payrolls prep --period-end 2026-11-01 --from week.csv --rate 0.60 --dry-run
  ```

### Read-back you can trust
- **`payrolls verify`** — Check the draft payroll against your timesheet CSV (hours, each reimbursement line, unexplained lines, duplicates) and exit 1 on any mismatch.

  _Run it before telling a human the payroll is ready; a non-zero exit means do not hand it off._

  ```bash
  gusto-pp-cli payrolls verify --period-end 2026-11-01 --from week.csv --rate 0.60 --agent
  ```
- **`payrolls compare`** — Calculate the draft and show how its total debit and payable tax changed against the last processed payroll, alongside the draft's gross, reimbursements, taxes and net.

  _Use it to explain an unusual debit before the human submits._

  ```bash
  gusto-pp-cli payrolls compare --period-end 2026-11-01 --agent
  ```

## Recipes

### Mileage for one trip week

```bash
gusto-pp-cli payrolls mileage --period-end 2026-11-01 --employee 'Ava Example' --miles 7.8 --rate 0.60
```

Writes one reimbursement line, Mileage 7.8 mi x $0.60, onto the draft.

### Weekly prep from a CSV

```bash
gusto-pp-cli payrolls prep --period-end 2026-11-01 --from week.csv --rate 0.60
```

Sets hours and mileage for each employee, calculates, and prints the review totals.

### Gate the handoff

```bash
gusto-pp-cli payrolls verify --period-end 2026-11-01 --from week.csv --rate 0.60 --agent --select ok,mismatches
```

Returns only the pass flag and the mismatch list so an agent can decide whether to hand off.

### Read only the totals that matter

```bash
gusto-pp-cli payrolls review --period-end 2026-11-01 --agent --select review.gross,review.net_pay,review.total_debit
```

Calculates the draft and returns just gross, net and total debit for the human who submits.

## Usage

Run `gusto-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `GUSTO_CONFIG_DIR`, `GUSTO_DATA_DIR`, `GUSTO_STATE_DIR`, or `GUSTO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `GUSTO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export GUSTO_HOME=/srv/gusto
gusto-pp-cli doctor
```

Under `GUSTO_HOME=/srv/gusto`, the four dirs resolve to `/srv/gusto/config`, `/srv/gusto/data`, `/srv/gusto/state`, and `/srv/gusto/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:GUSTO_HOME = "C:\srv\gusto"
gusto-pp-cli doctor
```

Under `$env:GUSTO_HOME = "C:\srv\gusto"`, the four dirs resolve to `C:\srv\gusto\config`, `C:\srv\gusto\data`, `C:\srv\gusto\state`, and `C:\srv\gusto\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "gusto": {
      "command": "gusto-pp-mcp",
      "env": {
        "GUSTO_HOME": "/srv/gusto"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `GUSTO_DATA_DIR` overrides an explicit `--home` for that kind. Use `GUSTO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `GUSTO_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `gusto-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `gusto-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `gusto-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `GUSTO_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `GUSTO_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### session

Your Gusto web session

- **`gusto-pp-cli session`** - Show recent sign-in events for the signed-in Gusto user (confirms the session works)


### payrolls


- **`gusto-pp-cli payrolls mileage`** - Add or correct the mileage reimbursement on the open draft payroll from miles and a per-mile rate; one mileage line per employee.

- **`gusto-pp-cli payrolls prep`** - Load a week's hours and mileage from a CSV onto the open draft, calculate it, and print the review totals, stopping before submit.

- **`gusto-pp-cli payrolls verify`** - Check the draft payroll against your timesheet CSV (hours, each reimbursement line, unexplained lines, duplicates) and exit 1 on any mismatch.

- **`gusto-pp-cli payrolls compare`** - Calculate the draft and show how its total debit and payable tax changed against the last processed payroll, alongside the draft's gross, reimbursements, taxes and net.

- **`gusto-pp-cli payrolls due`** - Show the next draft payroll: pay period, submit-by deadline and check date

- **`gusto-pp-cli payrolls history`** - List processed payrolls with period, check date and total debit

- **`gusto-pp-cli payrolls show`** - Show a draft payroll's inputs per employee: hours, bonus and reimbursement lines

- **`gusto-pp-cli payrolls hours set`** - Set an employee's regular hours on the draft payroll

- **`gusto-pp-cli payrolls reimburse add`** - Add a one-time reimbursement line; a line with the same description is replaced, not duplicated

- **`gusto-pp-cli payrolls reimburse list`** - List reimbursement lines on a draft payroll

- **`gusto-pp-cli payrolls reimburse remove`** - Remove reimbursement lines with a matching description from a draft payroll

- **`gusto-pp-cli payrolls review`** - Calculate the draft payroll and show gross, reimbursements, taxes, net and total debit; never submits

- **`gusto-pp-cli payrolls net-pay-preview`** - Preview each employee's gross and net pay on a draft payroll from its saved inputs


### reimbursements


- **`gusto-pp-cli reimbursements suggestions`** - List the reimbursement descriptions Gusto suggests for this company




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`gusto-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`gusto-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`gusto-pp-cli learnings list`** - Inspect taught rows
- **`gusto-pp-cli learnings forget <query>`** - Undo a teach
- **`gusto-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`gusto-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`gusto-pp-cli teach-pattern`** - Install a query/resource template up front
- **`gusto-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `GUSTO_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `gusto-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
gusto-pp-cli session

# JSON for scripting and agents
gusto-pp-cli session --json
# Filter to specific fields by name
gusto-pp-cli session --json --select <field>[,<field>...]

# Dry run — show the request without sending
gusto-pp-cli session --dry-run

# Agent mode — JSON + compact + no prompts in one flag
gusto-pp-cli session --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Draft writes only** - writes only to draft payrolls (hours and one-time reimbursements); never submits or approves a payroll and cannot add bonuses. Preview any write with `--dry-run`
- **Live payroll data** - payroll commands always read Gusto live; sync and search cover only the `session` resource
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `1` `payrolls verify` found a mismatch, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
gusto-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `gusto-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/gusto-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\gusto-pp-cli\config.toml` on Windows; `--home`, `GUSTO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `GUSTO_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `GUSTO_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `gusto-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check `--period-end` (YYYY-MM-DD) or `--draft-id`
- Run `gusto-pp-cli payrolls due` to see the open draft

### API-specific
- **Requests land on the Gusto login page or return 401** — Run press-auth login app.gusto.com --login-url https://app.gusto.com/login --force, then gusto-pp-cli auth login --chrome.
- **HTTP 403 with a Cloudflare challenge** — Re-run gusto-pp-cli auth login --chrome; the CLI already uses Chrome-compatible transport.
- **No draft payroll for a period** — Run gusto-pp-cli payrolls due to see the open draft, then pass that --period-end (YYYY-MM-DD).
- **payrolls verify exits 1** — Read the mismatches list; run gusto-pp-cli payrolls prep --from week.csv --rate 0.60 to apply the timesheet, then verify again.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `GUSTO_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override. `--insecure` overrides `--ca-cert` and `--tls-server-name`.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://app.gusto.com/example-co-llc/payroll_admin/pay
- Capture coverage: 14 API entries from 14 total network entries
- Reachability: browser_http (85% confidence)
- Protocols: graphql (92% confidence)
- Auth signals: chrome_mcp_session

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**gusto-mcp-2026-complete**](https://github.com/BusyBee3333/gusto-mcp-2026-complete) — TypeScript
- [**gusto-mcp-http**](https://github.com/agenticledger/gusto-mcp-http) — TypeScript
- [**gusto-typescript-client**](https://github.com/Gusto/gusto-typescript-client) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
