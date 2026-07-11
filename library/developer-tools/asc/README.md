# App Store Connect CLI

**A read-only App Store Connect cockpit that shows every app you ship at once — review state, builds, downloads, and ratings in one glance.**

Every existing App Store Connect tool is single-app and CI-shaped. asc is the opposite: a fleet-wide read cockpit for a developer shipping many apps. cockpit rolls up review state and action-needed flags across all apps, pipeline ages in-flight submissions, traction ranks downloads and rating trend, and blockers lists everything that can't ship right now.

## Install

The recommended path installs both the `asc-pp-cli` binary and the `pp-asc` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install asc
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install asc --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install asc --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install asc --agent claude-code
npx -y @mvanhorn/printing-press-library install asc --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/asc-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install asc --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-asc --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-asc --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install asc --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/asc-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `APP_STORE_CONNECT_ITC_BEARER_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "asc": {
      "command": "asc-pp-mcp",
      "env": {
        "APP_STORE_CONNECT_ITC_BEARER_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Uses an App Store Connect API key (JWT ES256). Create a key under Users and Access > Integrations, download the .p8, and set ASC_KEY_ID, ASC_ISSUER_ID, and ASC_PRIVATE_KEY_PATH. Keys are scoped to one team; run one team per invocation.

## Quick Start

```bash
# confirm the API key is valid and the API is reachable
asc doctor

# the one-glance board across every app you ship
asc cockpit

# list all apps in the team as structured JSON
asc apps list --json

# what is in review right now and how long it has waited
asc pipeline

# newest written reviews across the fleet
asc reviews recent --limit 20

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-app rollups
- **`cockpit`** — One-glance board across all your apps: review state, latest build + processing state, and an action-needed flag.

  _Reach for this to answer 'what across all my apps needs attention right now' in one call instead of N per-app lookups._

  ```bash
  asc cockpit --agent
  ```
- **`pipeline`** — Every version currently in the review pipeline across all apps, ranked by how long it has been waiting.

  _Use this to catch a submission that has been stuck in WAITING_FOR_REVIEW or IN_REVIEW too long._

  ```bash
  asc pipeline --agent
  ```
- **`traction`** — Ranks your apps by recent rating and review volume and flags apps whose rating is trending down; adds download counts when ASC_VENDOR_NUMBER is set.

  _Use this to decide which app is actually gaining traction and which one's rating is slipping._

  ```bash
  asc traction --window 30 --agent
  ```
- **`reviews recent`** — Newest written customer reviews across all your apps in one merged stream.

  _Use this for a daily feedback read across the whole portfolio without querying each app._

  ```bash
  asc reviews recent --limit 20 --agent
  ```
- **`blockers`** — Correlates failed/invalid builds, pending or absent submissions, and metadata rejections into one fleet blocker list.

  _Use this to find every app that cannot ship right now and why, in one call._

  ```bash
  asc blockers --agent
  ```

## Recipes

### Morning fleet check

```bash
asc cockpit --agent
```

One structured board of every app's review state, latest build, rating, and action-needed flag.

### What's stuck in review

```bash
asc pipeline --agent --select app,version,state,age_days
```

In-flight submissions ranked by age, narrowed to the fields that matter.

### Feedback pulse

```bash
asc reviews recent --limit 25 --json
```

Newest written reviews across all apps as JSON for triage.

## Usage

Run `asc-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ASC_CONFIG_DIR`, `ASC_DATA_DIR`, `ASC_STATE_DIR`, or `ASC_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ASC_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ASC_HOME=/srv/asc
asc-pp-cli doctor
```

Under `ASC_HOME=/srv/asc`, the four dirs resolve to `/srv/asc/config`, `/srv/asc/data`, `/srv/asc/state`, and `/srv/asc/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "asc": {
      "command": "asc-pp-mcp",
      "env": {
        "ASC_HOME": "/srv/asc"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ASC_DATA_DIR` overrides an explicit `--home` for that kind. Use `ASC_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ASC_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `asc-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### app-store-review-details

Manage app store review details

- **`asc-pp-cli app-store-review-details <id>`** - Get instance

### app-store-versions

Manage app store versions

- **`asc-pp-cli app-store-versions <id>`** - Get instance

### apps

Manage apps

- **`asc-pp-cli apps get-collection`** - Get collection
- **`asc-pp-cli apps get-instance`** - Get instance

### beta-groups

Manage beta groups

- **`asc-pp-cli beta-groups get-collection`** - Get collection
- **`asc-pp-cli beta-groups get-instance`** - Get instance

### beta-testers

Manage beta testers

- **`asc-pp-cli beta-testers get-collection`** - Get collection
- **`asc-pp-cli beta-testers get-instance`** - Get instance

### builds

Manage builds

- **`asc-pp-cli builds get-collection`** - Get collection
- **`asc-pp-cli builds get-instance`** - Get instance

### customer-reviews

Manage customer reviews

- **`asc-pp-cli customer-reviews <id>`** - Get instance

### sales-reports

Manage sales reports

- **`asc-pp-cli sales-reports`** - Get collection


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`asc-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`asc-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`asc-pp-cli learnings list`** - Inspect taught rows
- **`asc-pp-cli learnings forget <query>`** - Undo a teach
- **`asc-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`asc-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`asc-pp-cli teach-pattern`** - Install a query/resource template up front
- **`asc-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ASC_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `asc-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
asc-pp-cli app-store-review-details mock-value

# JSON for scripting and agents
asc-pp-cli app-store-review-details mock-value --json

# Filter to specific fields
asc-pp-cli app-store-review-details mock-value --json --select id,name,status

# Dry run — show the request without sending
asc-pp-cli app-store-review-details mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
asc-pp-cli app-store-review-details mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
asc-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `asc-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/app-store-connect-pp-cli/config.toml`; `--home`, `ASC_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `APP_STORE_CONNECT_ITC_BEARER_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `asc-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `asc-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $APP_STORE_CONNECT_ITC_BEARER_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized** — Token expired (max life 20 min) or wrong issuer/key id — re-check ASC_KEY_ID / ASC_ISSUER_ID / ASC_PRIVATE_KEY_PATH.
- **asc traction shows no downloads** — Set ASC_VENDOR_NUMBER (found in App Store Connect > Payments and Financial Reports) — the sales report requires a vendor number.
- **429 Too Many Requests** — App Store Connect limits ~3600/hour; asc backs off automatically, but narrow --window or add --limit on large fleets.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**fastlane**](https://github.com/fastlane/fastlane) — Ruby (39000 stars)
- [**appstoreconnect-cli**](https://github.com/ittybittyapps/appstoreconnect-cli) — Swift (400 stars)
- [**node-app-store-connect-api**](https://github.com/dfabulich/node-app-store-connect-api) — JavaScript (80 stars)
- [**asc-mcp**](https://github.com/pofky/asc-mcp) — TypeScript (30 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
