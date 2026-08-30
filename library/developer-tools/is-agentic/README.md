# Is Agentic CLI

**Turn Is Agentic reports into durable, scriptable readiness evidence.**

Retrieve the official report with the same read-only contract as the upstream CLI, then keep local history, diffs, fleet comparisons, policy gates, issue lifecycles, and exportable evidence beside it. The API remains the source of truth; the local layer makes repeated engineering work compound.

Learn more at [Is Agentic](https://is-agentic.com/contact).

Created by [@SomSamantray](https://github.com/SomSamantray) (Som Samantray).

## Install

The recommended path installs both the `is-agentic-pp-cli` binary and the `pp-is-agentic` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install is-agentic
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install is-agentic --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install is-agentic --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install is-agentic --agent claude-code
npx -y @mvanhorn/printing-press-library install is-agentic --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/is-agentic/cmd/is-agentic-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/is-agentic-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install is-agentic --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-is-agentic --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-is-agentic --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install is-agentic --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/is-agentic-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/is-agentic/cmd/is-agentic-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "is-agentic": {
      "command": "is-agentic-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify the generated CLI and public-service configuration without network access.
is-agentic-pp-cli doctor --dry-run

# Preview the canonical report request safely.
is-agentic-pp-cli report get-is-agentic-v1 --url https://is-agentic.com --dry-run

# Fetch the latest completed report as structured JSON.
is-agentic-pp-cli report get-is-agentic-v1 --url https://is-agentic.com --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local evidence that compounds
- **`history`** — Keep a local, timestamped ledger of readiness reports and their provenance.

  _Choose this when you need an auditable local timeline instead of one ephemeral API response._

  ```bash
  is-agentic-pp-cli history --json --agent
  ```
- **`diff`** — See which readiness scores and findings changed between two retained audits.

  _Choose this after a deployment to verify whether agent-readiness regressions were introduced or fixed._

  ```bash
  is-agentic-pp-cli diff --target https://is-agentic.com --json --agent
  ```
- **`issues`** — Track when readiness findings first appeared, last appeared, were fixed, or regressed.

  _Choose this to turn recurring findings into a remediation queue with honest history._

  ```bash
  is-agentic-pp-cli issues --target https://is-agentic.com --json --agent
  ```

### Release and fleet controls
- **`check`** — Fail CI with an explicit, machine-readable readiness policy decision.

  _Choose this when a release pipeline must enforce readiness instead of merely displaying it._

  ```bash
  is-agentic-pp-cli check --target https://is-agentic.com --min-score 80 --json --agent
  ```
- **`portfolio`** — Compare a fleet of public sites in one sortable score and issue matrix.

  _Choose this for platform or agency work spanning multiple public targets._

  ```bash
  is-agentic-pp-cli portfolio --targets https://is-agentic.com,https://example.com --json --agent
  ```
- **`portfolio`** — Refresh many targets with bounded concurrency, deduplication, and Retry-After-aware pacing.

  _Choose this for repeatable fleet refreshes that must respect the public 120-per-minute quota._

  ```bash
  is-agentic-pp-cli portfolio --file sites.txt --max-requests 50 --json --agent
  ```

### Portable audit artifacts
- **`evidence`** — Package a report and its provenance into a portable evidence artifact.

  _Choose this when an audit must travel with a review, ticket, or release artifact._

  ```bash
  is-agentic-pp-cli evidence --target https://is-agentic.com --json
  ```

## Recipes

### Fetch one report

```bash
is-agentic-pp-cli report get-is-agentic-v1 --url https://is-agentic.com --json --agent
```

Return the current report in a compact agent-friendly shape.

### Compare a small fleet

```bash
is-agentic-pp-cli portfolio --targets https://is-agentic.com,https://example.com --json --agent --select target,score,score_label
```

Compare targets while selecting only the fields needed for a decision.

### Gate a release

```bash
is-agentic-pp-cli check --target https://is-agentic.com --min-score 80 --json --agent
```

Enforce a local minimum score with a structured decision.

## Usage

Run `is-agentic-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `IS_AGENTIC_CONFIG_DIR`, `IS_AGENTIC_DATA_DIR`, `IS_AGENTIC_STATE_DIR`, or `IS_AGENTIC_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `IS_AGENTIC_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export IS_AGENTIC_HOME=/srv/is-agentic
is-agentic-pp-cli doctor
```

Under `IS_AGENTIC_HOME=/srv/is-agentic`, the four dirs resolve to `/srv/is-agentic/config`, `/srv/is-agentic/data`, `/srv/is-agentic/state`, and `/srv/is-agentic/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "is-agentic": {
      "command": "is-agentic-pp-mcp",
      "env": {
        "IS_AGENTIC_HOME": "/srv/is-agentic"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `IS_AGENTIC_DATA_DIR` overrides an explicit `--home` for that kind. Use `IS_AGENTIC_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `IS_AGENTIC_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `is-agentic-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### report

Read completed public Is Agentic reports.

- **`is-agentic-pp-cli report get-is-agentic-legacy`** - Deprecated compatibility alias for GET /api/v1/report. Responses include the RFC 9745 Deprecation header and a successor-version link.
- **`is-agentic-pp-cli report get-is-agentic-v1`** - Returns one latest completed stored report for a public HTTP or HTTPS URL without launching a scan. This is a single-resource read operation: it has no request body, pagination, idempotency key, or asynchronous job.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`is-agentic-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`is-agentic-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`is-agentic-pp-cli learnings list`** - Inspect taught rows
- **`is-agentic-pp-cli learnings forget <query>`** - Undo a teach
- **`is-agentic-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`is-agentic-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`is-agentic-pp-cli teach-pattern`** - Install a query/resource template up front
- **`is-agentic-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `IS_AGENTIC_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `is-agentic-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
is-agentic-pp-cli report get-is-agentic-legacy --url https://example.com

# JSON for scripting and agents
is-agentic-pp-cli report get-is-agentic-legacy --url https://example.com --json
# Filter to specific fields
is-agentic-pp-cli report get-is-agentic-legacy --url https://example.com --json --select details,id,name

# Dry run — show the request without sending
is-agentic-pp-cli report get-is-agentic-legacy --url https://example.com --dry-run

# Agent mode — JSON + compact + no prompts in one flag
is-agentic-pp-cli report get-is-agentic-legacy --url https://example.com --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Local evidence** - report and portfolio commands populate a local SQLite history for history/diff/issues; this API has no bulk sync endpoint
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
is-agentic-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `is-agentic-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/is-agentic-public-pp-cli/config.toml`; `--home`, `IS_AGENTIC_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **report_not_found** — Open the canonical report URL printed in the error, start the scan there, then retry the CLI.
- **rate_limit_exceeded** — Honor Retry-After and lower portfolio concurrency or max requests.
- **temporary unavailability** — Retry without launching a new scan; the service may be refreshing stored evidence.
