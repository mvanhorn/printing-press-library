# Is It Agent Ready CLI

**The terminal scanner for AI-agent readiness: every check the web tool runs, plus copy-paste fixes, CI gating, scan history, and a local store the web UI has no answer for.**

isitagentready.com gives you a one-shot score in a browser tab with no memory. This CLI turns it into a repeatable loop: check any site, get the prioritized fixes with advice, gate it in CI, diff it over time, compare it to competitors, and batch-scan a whole portfolio, with every scan stored locally so history and open-advice tell you exactly what changed and what is still unfixed.

## Install

The recommended path installs both the `isitagentready-pp-cli` binary and the `pp-isitagentready` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install isitagentready
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install isitagentready --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install isitagentready --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install isitagentready --agent claude-code
npx -y @mvanhorn/printing-press-library install isitagentready --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/cmd/isitagentready-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/isitagentready-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install isitagentready --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-isitagentready --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-isitagentready --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install isitagentready --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/isitagentready-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/cmd/isitagentready-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "isitagentready": {
      "command": "isitagentready-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No API key or login required. The scan endpoint is public and read-only, so every command works out of the box.

## Quick Start

```bash
# Confirm the scanner API is reachable before your first scan.
isitagentready-pp-cli doctor --dry-run

# Scan a site: prints the readiness level and per-category summary, and stores the scan locally.
isitagentready-pp-cli check https://example.com

# The headline: prioritized, copy-paste fixes to reach the next readiness level.
isitagentready-pp-cli advice https://example.com

# Use in CI: exits non-zero if the site is below level 3.
isitagentready-pp-cli gate https://example.com --min-level 3

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Ship it without regressing
- **`gate`** — Fail a build when a site drops below a target readiness level or when a previously-passing check regresses.

  _Reach for this in CI when you want a deterministic exit code, not a brittle grep of the JSON._

  ```bash
  isitagentready-pp-cli gate https://isitagentready.com --min-level 3
  ```
- **`open-advice`** — List every still-failing check across all your scanned sites with its fix prompt, so you see exactly what is left to do.

  _Reach for this to answer 'what fixes are still open across all my sites' in one command._

  ```bash
  isitagentready-pp-cli open-advice --agent
  ```

### Track readiness over time
- **`history`** — Show a site's readiness level across every past scan and flag which checks flipped pass/fail between scans.

  _Reach for this to confirm a fix actually landed and to catch silent regressions._

  ```bash
  isitagentready-pp-cli history https://example.com --agent
  ```
- **`diff`** — Diff two scans of a site (default: the latest two) into a per-check regressed/fixed/unchanged table plus the level delta.

  _Reach for this to see precisely what changed between two points in time, not just the new score._

  ```bash
  isitagentready-pp-cli diff https://example.com
  ```

### Across many sites
- **`compare`** — Scan several sites and print a check-by-check matrix of which agent standards each one implements, plus each site's level.

  _Reach for this to see exactly which standards a competitor implemented that you have not._

  ```bash
  isitagentready-pp-cli compare https://example.com https://isitagentready.com
  ```
- **`batch`** — Scan a list of URLs from a file or stdin, persist each, and print a leaderboard ranked by level or failing-check count.

  _Reach for this to triage a whole web estate worst-first instead of scanning sites one browser tab at a time._

  ```bash
  isitagentready-pp-cli batch urls.txt --rank failing --csv
  ```

## Recipes


### Get the copy-paste fixes for a site

```bash
isitagentready-pp-cli advice https://example.com --copy
```

Prints every next-level fix prompt as one pasteable block, ready to drop into a coding agent.

### See what is still unfixed across all your sites

```bash
isitagentready-pp-cli open-advice --agent
```

Cross-site backlog of every check still failing, with its fix prompt, from your local scan history.

### Narrow a verbose scan to just the checks you care about

```bash
isitagentready-pp-cli report https://example.com --agent --only-failing --select checks.discovery.mcpServerCard.status,checks.discovery.mcpServerCard.message
```

A full scan is large and deeply nested; --select pulls just the dotted fields you want so an agent does not parse tens of KB.

### Gate a deploy on readiness

```bash
isitagentready-pp-cli gate https://example.com --min-level 3 --no-regress
```

Exits non-zero if the site is below level 3 or any previously-passing check regressed; safe for CI.

### Compare against a competitor

```bash
isitagentready-pp-cli compare https://example.com https://stripe.com
```

Prints a per-standard matrix of which agent-readiness checks each site implements.

## Usage

Run `isitagentready-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ISITAGENTREADY_CONFIG_DIR`, `ISITAGENTREADY_DATA_DIR`, `ISITAGENTREADY_STATE_DIR`, or `ISITAGENTREADY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ISITAGENTREADY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ISITAGENTREADY_HOME=/srv/isitagentready
isitagentready-pp-cli doctor
```

Under `ISITAGENTREADY_HOME=/srv/isitagentready`, the four dirs resolve to `/srv/isitagentready/config`, `/srv/isitagentready/data`, `/srv/isitagentready/state`, and `/srv/isitagentready/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "isitagentready": {
      "command": "isitagentready-pp-mcp",
      "env": {
        "ISITAGENTREADY_HOME": "/srv/isitagentready"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ISITAGENTREADY_DATA_DIR` overrides an explicit `--home` for that kind. Use `ISITAGENTREADY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ISITAGENTREADY_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `isitagentready-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### scan

Scan a website for AI-agent readiness

- **`isitagentready-pp-cli scan`** - Scan a URL; returns readiness level (0-5), per-check results across 5 categories, and prioritized fix advice for reaching the next level


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
isitagentready-pp-cli scan --url https://example

# JSON for scripting and agents
isitagentready-pp-cli scan --url https://example --json

# Filter to specific fields
isitagentready-pp-cli scan --url https://example --json --select id,name,status

# Dry run — show the request without sending
isitagentready-pp-cli scan --url https://example --dry-run

# Agent mode — JSON + compact + no prompts in one flag
isitagentready-pp-cli scan --url https://example --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
isitagentready-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `isitagentready-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/isitagentready-pp-cli/config.toml`; `--home`, `ISITAGENTREADY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Scan returns a siteError block instead of checks** — The TARGET site was unreachable (403/timeout), not the scanner. Confirm the URL loads in a browser; the scanner could not fetch it.
- **advice, history, or open-advice prints nothing** — Run isitagentready-pp-cli check <url> first; those commands read the local scan store.
- **A scan takes 5 to 8 seconds** — Normal: the scanner runs many live probes per scan. Use batch to scan many sites in one command.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**llms-txt-hub**](https://github.com/thedaviddias/llms-txt-hub) — TypeScript (860 stars)
- [**searchstack-aeo**](https://github.com/alexpospekhov/searchstack-aeo) — Python (86 stars)
- [**makeitagentready.com**](https://github.com/thejahid/makeitagentready.com) — TypeScript (1 stars)
- [**geoskills**](https://github.com/Cognitic-Labs/geoskills) — Markdown
- [**geo-optimizer-skill**](https://github.com/Auriti-Labs/geo-optimizer-skill) — Python
- [**isagentready-skills**](https://github.com/BartWaardenburg/isagentready-skills) — Markdown
- [**siteaudit-mcp**](https://github.com/vdalhambra/siteaudit-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
