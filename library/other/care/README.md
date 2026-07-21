# Care CLI

care.com CLI: search caregivers, review applicants/messages, analyze and reach out - over your own authenticated session

Learn more at [Care](https://www.care.com).

Created by [@beetz12](https://github.com/beetz12) (david).

## Install

The recommended path installs both the `care-pp-cli` binary and the `pp-care` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install care
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install care --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install care --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install care --agent claude-code
npx -y @mvanhorn/printing-press-library install care --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/care/cmd/care-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/care-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install care --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-care --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-care --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install care --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
care-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/care-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/care/cmd/care-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "care": {
      "command": "care-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Authenticate

This CLI uses your browser session for authentication. Log in to .care.com in Chrome, then:

```bash
care-pp-cli auth login --chrome
```

Or import an existing browser capture:

```bash
care-pp-cli auth login --cookies-file storage-state.json
```

`--cookies-file` accepts Playwright storage-state JSON or a raw `Cookie:` header text file. The Chrome path requires a cookie extraction tool. Install one:

```bash
pip install pycookiecheat          # Python (recommended)
brew install barnardb/cookies/cookies  # Homebrew
```

When your session expires, run `auth login --chrome` again.

### 3. Verify Setup

```bash
care-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
care-pp-cli calendars --operation-name example-resource
```

## Usage

Run `care-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CARE_CONFIG_DIR`, `CARE_DATA_DIR`, `CARE_STATE_DIR`, or `CARE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CARE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CARE_HOME=/srv/care
care-pp-cli doctor
```

Under `CARE_HOME=/srv/care`, the four dirs resolve to `/srv/care/config`, `/srv/care/data`, `/srv/care/state`, and `/srv/care/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "care": {
      "command": "care-pp-mcp",
      "env": {
        "CARE_HOME": "/srv/care"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CARE_DATA_DIR` overrides an explicit `--home` for that kind. Use `CARE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CARE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `care-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### availabilities

GraphQL BFF operations for availabilities

- **`care-pp-cli availabilities`** - Fetch availabilities events

### bookings

GraphQL BFF operations for bookings

- **`care-pp-cli bookings request-list-jobs-basic`** - Fetch bookings request list jobs basic
- **`care-pp-cli bookings request-list-jobs-extended`** - Fetch bookings request list jobs extended

### calendars

GraphQL BFF operations for calendars

- **`care-pp-cli calendars`** - Fetch calendars

### care_jobs

GraphQL BFF operations for jobs

- **`care-pp-cli care-jobs applicant-hired`** - Fetch jobs applicant hired
- **`care-pp-cli care-jobs application-interest`** - Fetch jobs application interest
- **`care-pp-cli care-jobs application-seeker-interest-counts`** - Fetch jobs application seeker interest counts
- **`care-pp-cli care-jobs applications`** - Fetch jobs applications
- **`care-pp-cli care-jobs by-seeker-u-u-i-d`** - Fetch jobs by seeker u u i d
- **`care-pp-cli care-jobs by-seeker-u-u-i-d-applicants`** - Fetch jobs by seeker u u i d applicants
- **`care-pp-cli care-jobs child-care-one-time-update`** - Fetch jobs child care one time update
- **`care-pp-cli care-jobs profile`** - Fetch jobs profile
- **`care-pp-cli care-jobs setup-c-c`** - Fetch jobs setup c c
- **`care-pp-cli care-jobs wages`** - Fetch jobs wages
- **`care-pp-cli care-jobs wages-2`** - Fetch jobs wages

### caregivers

GraphQL BFF operations for caregivers

- **`care-pp-cli caregivers booking-list-provider`** - Fetch caregivers booking list provider
- **`care-pp-cli caregivers get`** - Fetch caregivers
- **`care-pp-cli caregivers profile`** - Fetch caregivers profile

### conversations

GraphQL BFF operations for conversations

- **`care-pp-cli conversations`** - Fetch conversations most relevant care needs

### members

GraphQL BFF operations for members

- **`care-pp-cli members entitlements`** - Fetch members entitlements
- **`care-pp-cli members ids`** - Fetch members ids

### memberships

GraphQL BFF operations for memberships

- **`care-pp-cli memberships`** - Fetch memberships information

### messages

GraphQL BFF operations for messages

- **`care-pp-cli messages`** - Fetch messages thread

### notifications

GraphQL BFF operations for notifications

- **`care-pp-cli notifications counts`** - Fetch notifications counts
- **`care-pp-cli notifications counts-2`** - Fetch notifications counts

### providers

GraphQL BFF operations for providers

- **`care-pp-cli providers child-care`** - Fetch providers child care
- **`care-pp-cli providers child-care-2`** - Fetch providers child care
- **`care-pp-cli providers get`** - Fetch providers

### reviewees

GraphQL BFF operations for reviewees

- **`care-pp-cli reviewees`** - Fetch reviewees metrics

### reviews

GraphQL BFF operations for reviews

- **`care-pp-cli reviews by-reviewee`** - Fetch reviews by reviewee
- **`care-pp-cli reviews request-search`** - Fetch reviews request search

### seekers

GraphQL BFF operations for seekers

- **`care-pp-cli seekers get`** - Fetch seekers
- **`care-pp-cli seekers job-application`** - Fetch seekers job application
- **`care-pp-cli seekers job-summaries`** - Fetch seekers job summaries
- **`care-pp-cli seekers membership-information`** - Fetch seekers membership information

### zipcodes

GraphQL BFF operations for zipcodes

- **`care-pp-cli zipcodes`** - Fetch zipcodes summary


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`care-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`care-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`care-pp-cli learnings list`** - Inspect taught rows
- **`care-pp-cli learnings forget <query>`** - Undo a teach
- **`care-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`care-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`care-pp-cli teach-pattern`** - Install a query/resource template up front
- **`care-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CARE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `care-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
care-pp-cli calendars --operation-name example-resource

# JSON for scripting and agents
care-pp-cli calendars --operation-name example-resource --json

# Filter to specific fields
care-pp-cli calendars --operation-name example-resource --json --select id,name,status

# Dry run — show the request without sending
care-pp-cli calendars --operation-name example-resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
care-pp-cli calendars --operation-name example-resource --agent
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
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
care-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `care-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/care-pp-cli/config.toml`; `--home`, `CARE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `care-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `CARE_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
