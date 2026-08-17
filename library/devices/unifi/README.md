# UniFi CLI

**Every UniFi Network API operation, plus drift detection, topology, and rule prediction no other UniFi tool has.**

unifi-pp-cli wraps the full local Network integration API (devices, clients, firewall, ACL, networks, VPN, switching) with a local SQLite mirror. That mirror is what lets it answer questions the live API can't: what changed since yesterday, what device just joined, and which firewall rule would match a given packet.

## Install

The recommended path installs both the `unifi-pp-cli` binary and the `pp-unifi` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install unifi
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install unifi --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install unifi --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install unifi --agent claude-code
npx -y @mvanhorn/printing-press-library install unifi --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/unifi/cmd/unifi-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/unifi-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install unifi --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-unifi --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-unifi --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install unifi --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/unifi-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `UNIFI_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/unifi/cmd/unifi-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "unifi": {
      "command": "unifi-pp-mcp",
      "env": {
        "UNIFI_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Generate a local API key from the gateway's own UI (Settings -> Control Plane -> Integrations -> Create API Key) and set UNIFI_API_KEY. The gateway's self-signed certificate is handled automatically for private/loopback/link-local hosts; no --insecure flag needed for the common case.

## Quick Start

```bash
# Confirm config and connectivity without making a live call.
unifi-pp-cli doctor --dry-run

# Find the site ID you'll use for every other command.
unifi-pp-cli sites

# Populate the local mirror so drift/topology/newcomer have a baseline.
unifi-pp-cli sync

# See the physical device tree.
unifi-pp-cli topology

# Check what changed in the last day.
unifi-pp-cli drift --since 24h

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`topology`** — See the physical device tree (gateway to switches to APs) built entirely from local mirror data, no live crawl needed.

  _Reach for this when an agent needs to understand physical network layout without walking every device endpoint individually._

  ```bash
  unifi-pp-cli topology --site default --json
  ```
- **`drift`** — Show what changed in site config (networks, firewall, wifi, DNS) since the last sync snapshot.

  _Use after a suspected config change to see exactly what moved, without manually diffing the controller UI._

  ```bash
  unifi-pp-cli drift --site default --since 24h --json
  ```
- **`newcomer`** — List devices and clients first seen since a given sync, for spotting new hardware joining the network.

  _Use for periodic security review of what joined the network recently._

  ```bash
  unifi-pp-cli newcomer --since 7d --json
  ```

### Agent-native plumbing
- **`port-audit`** — Review port utilization and PoE status across every switch on a site in one table.

  _Use before adding new PoE devices to check headroom, or to find unused ports across a stack._

  ```bash
  unifi-pp-cli port-audit --site default --json
  ```
- **`guest report`** — Summarize guest network usage: active vouchers and connected guest clients, from local data.

  _Use for a quick guest-network health check without cross-referencing three separate UI screens._

  ```bash
  unifi-pp-cli guest report --site default --json
  ```
- **`rule-predict`** — Predict which firewall policy would match a hypothetical packet before making a live change.

  _Use to check the effect of a proposed firewall change before applying it live._

  ```bash
  unifi-pp-cli rule-predict --src 10.0.3.0/24 --dst 10.0.0.1 --port 443 --json
  ```

## Recipes

### Find who just joined the network

```bash
unifi-pp-cli newcomer --since 7d --json --select id,name,mac
```

Narrow a potentially large newcomer list down to just the fields needed to identify each device.

### Audit switch port headroom before adding a PoE device

```bash
unifi-pp-cli port-audit --site default --json
```

Lists PoE status and free ports across every switch on the site.

### Check what a firewall change would match

```bash
unifi-pp-cli rule-predict --src 10.0.3.0/24 --dst 10.0.0.1 --port 443 --json
```

Simulates rule evaluation order against the synced ruleset before making a live change.

## Usage

Run `unifi-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `UNIFI_CONFIG_DIR`, `UNIFI_DATA_DIR`, `UNIFI_STATE_DIR`, or `UNIFI_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `UNIFI_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export UNIFI_HOME=/srv/unifi
unifi-pp-cli doctor
```

Under `UNIFI_HOME=/srv/unifi`, the four dirs resolve to `/srv/unifi/config`, `/srv/unifi/data`, `/srv/unifi/state`, and `/srv/unifi/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "unifi": {
      "command": "unifi-pp-mcp",
      "env": {
        "UNIFI_HOME": "/srv/unifi"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `UNIFI_DATA_DIR` overrides an explicit `--home` for that kind. Use `UNIFI_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `UNIFI_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `unifi-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### countries

Manage countries

- **`unifi-pp-cli countries`** - Returns ISO-standard country codes and names,
used for region-based configuration or regulatory compliance.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`code`|`STRING`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
</details>

### dpi

Manage dpi

- **`unifi-pp-cli dpi get-application-categories`** - Returns predefined Deep Packet Inspection (DPI) application categories used for traffic identification and filtering.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`INTEGER`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
</details>
- **`unifi-pp-cli dpi get-applications`** - Lists DPI-recognized applications grouped under categories. Useful for firewall or traffic analytics integration.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`INTEGER`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn` `like`|
</details>

### info

Manage info

- **`unifi-pp-cli info`** - Retrieve general information about the UniFi Network application.

### pending-devices

Manage pending devices

- **`unifi-pp-cli pending-devices`** - Retrieve a paginated list of devices pending adoption, including basic device information.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`macAddress`|`STRING`|`eq` `ne` `in` `notIn`|
|`ipAddress`|`STRING`|`eq` `ne` `in` `notIn`|
|`model`|`STRING`|`eq` `ne` `in` `notIn`|
|`state`|`STRING`|`eq` `ne` `in` `notIn`|
|`supported`|`BOOLEAN`|`eq` `ne`|
|`firmwareVersion`|`STRING`|`isNull` `isNotNull` `eq` `ne` `gt` `ge` `lt` `le` `like` `in` `notIn`|
|`firmwareUpdatable`|`BOOLEAN`|`eq` `ne`|
|`features`|`SET(STRING)`|`isEmpty` `contains` `containsAny` `containsAll` `containsExactly`|
</details>

### sites

Endpoints for listing and managing UniFi sites within a local Network application.
Site ID is required for most other API requests.

- **`unifi-pp-cli sites`** - Retrieve a paginated list of local sites managed by this Network application.
Site ID is required for other UniFi Network API calls.

<details>
<summary>Filterable properties (click to expand)</summary>

|Name|Type|Allowed functions|
|-|-|-|
|`id`|`UUID`|`eq` `ne` `in` `notIn`|
|`internalReference`|`STRING`|`eq` `ne` `in` `notIn`|
|`name`|`STRING`|`eq` `ne` `in` `notIn`|
</details>


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`unifi-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`unifi-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`unifi-pp-cli learnings list`** - Inspect taught rows
- **`unifi-pp-cli learnings forget <query>`** - Undo a teach
- **`unifi-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`unifi-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`unifi-pp-cli teach-pattern`** - Install a query/resource template up front
- **`unifi-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `UNIFI_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `unifi-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
unifi-pp-cli countries

# JSON for scripting and agents
unifi-pp-cli countries --json

# Filter to specific fields
unifi-pp-cli countries --json --select id,name,status

# Dry run — show the request without sending
unifi-pp-cli countries --dry-run

# Agent mode — JSON + compact + no prompts in one flag
unifi-pp-cli countries --agent
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
unifi-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `unifi-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/unifi-network-pp-cli/config.toml`; `--home`, `UNIFI_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `UNIFI_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `unifi-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `unifi-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $UNIFI_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **x509: certificate signed by unknown authority** — This should auto-resolve for private/loopback/link-local gateway hosts; if your gateway is reachable via a public hostname, set UNIFI_INSECURE_SKIP_VERIFY=1 explicitly.
- **401 Unauthorized** — Regenerate the API key under Settings -> Control Plane -> Integrations on the gateway and re-export UNIFI_API_KEY.
- **drift/newcomer/topology return empty** — Run 'unifi-pp-cli sync' first to populate the local mirror; these commands are local-mirror-only.

## Known Gaps

- **`topology` and `port-audit` are not local-mirror-only.** Every other
  novel command reads only the synced local database. These two also make
  live calls: neither device-to-device uplink chaining nor per-port
  link/PoE state appears in any list/sync response for this API — only a
  per-device detail fetch returns them. Both commands still read device IDs
  from the local mirror first, then fetch details live, one call per
  device.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**homelab-mcp**](https://github.com/bjeans/homelab-mcp) — Python (39 stars)
- [**go-unifi-mcp**](https://github.com/claytono/go-unifi-mcp) — Go (3 stars)
- [**unifi-mcp**](https://github.com/gordcurrie/unifi-mcp) — Go (2 stars)
- [**unifi-cli**](https://github.com/lucasilverentand/unifi-cli) — TypeScript
- [**mcp-unifi**](https://github.com/pete-builds/mcp-unifi) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
