# GL.iNet CLI

**The first maintained CLI for GL.iNet travel routers — named config profiles, travel macros, and version-aware diagnostics no other GL.iNet tool ships.**

Save a known-good 'home' config, adjust it at a hotel or abroad, then revert in one command with snapshot save/apply/diff. Get online at a new venue and fix the 'my foreign network is invisible' regulatory-domain problem instantly. Every command probes your exact model and firmware first, so it adapts instead of assuming — and it all runs locally over the router's own API, no GoodCloud.

## Install

The recommended path installs both the `gl-inet-pp-cli` binary and the `pp-gl-inet` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install gl-inet
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install gl-inet --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install gl-inet --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install gl-inet --agent claude-code
npx -y @mvanhorn/printing-press-library install gl-inet --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gl-inet-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install gl-inet --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-gl-inet --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-gl-inet --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install gl-inet --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gl-inet-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "gl-inet": {
      "command": "gl-inet-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Authenticates against the router's local admin password via GL's challenge/crypt/sha256 handshake (read from the live challenge, so it works across firmware versions). Config snapshots and UCI access use SSH to the router. No cloud account needed.

## Quick Start

```bash
# confirm reachability, model, firmware, and what this device supports
gl-inet-pp-cli doctor

# bank your current known-good config as a named profile
gl-inet-pp-cli snapshot save home

# see the current configuration at a glance
gl-inet-pp-cli config summary

# diagnose 'router up but no internet' and get the fix
gl-inet-pp-cli troubleshoot

# after a venue changes things, see exactly what differs from home
gl-inet-pp-cli snapshot diff home

# revert cleanly back to your standard config
gl-inet-pp-cli snapshot apply home

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Travel config profiles
- **`snapshot save`** — Capture your whole router config as a named, reusable profile.

  _Reach for this when the user wants to bank a known-good config before changing anything on the road._

  ```bash
  gl-inet-pp-cli snapshot save home --agent
  ```
- **`snapshot apply`** — Restore a saved profile, with a safety check that it matches this device.

  _Use to revert cleanly to a standard config without risking a mismatched-device restore._

  ```bash
  gl-inet-pp-cli snapshot apply home --agent
  ```
- **`snapshot diff`** — See exactly which settings a venue changed vs your standard config.

  _Use to answer 'what did this hotel network change?' before reverting._

  ```bash
  gl-inet-pp-cli snapshot diff home --agent
  ```

### Travel macros
- **`wifi region diagnose`** — Find out why a foreign network is invisible and fix the regulatory domain in one command.

  _Use when repeater mode can't see a network abroad (e.g. EU 2.4GHz ch12/13 under a US regdomain)._

  ```bash
  gl-inet-pp-cli wifi region diagnose --agent
  ```
- **`venue connect`** — One command to get online at a new venue: scan, region-check, join, and prep for captive portals.

  _Use on arrival at a hotel/cafe to skip the multi-step LuCI dance._

  ```bash
  gl-inet-pp-cli venue connect "Hotel WiFi" --agent
  ```
- **`vpn toggle`** — Start a VPN tunnel, arm the kill-switch, and confirm your public IP actually changed.

  _Use to trust that the VPN is really protecting traffic, not just 'connected'._

  ```bash
  gl-inet-pp-cli vpn toggle mullvad --agent
  ```
- **`vpn verify`** — Verify the VPN is up and your connection isn't leaking: egress country, DNS leaks, and STUN/UDP (WebRTC-style) leaks.

  _Use to confirm a VPN is actually protecting all traffic (no IP/DNS/WebRTC leak) before trusting it on an untrusted network._

  ```bash
  gl-inet-pp-cli vpn verify --expect-country US --agent
  ```
- **`wan mode`** — Switch the router's WAN source and verify it reconnected.

  _Use when changing how the router gets upstream internet at a new venue._

  ```bash
  gl-inet-pp-cli wan mode repeater --agent
  ```

### Diagnostics
- **`troubleshoot`** — Diagnose why the router has no internet and get the exact fix command.

  _Use first whenever the router is up but there's no internet — most often it's not connected in repeater mode._

  ```bash
  gl-inet-pp-cli troubleshoot --agent
  ```
- **`uplink`** — Diagnose a weak/slow venue WiFi uplink and get ranked ways to improve it.

  _Use when you have internet but it's bad and you want to know whether to move, switch bands, or tether._

  ```bash
  gl-inet-pp-cli uplink --agent
  ```
- **`doctor`** — Report this router's model, firmware, OpenWrt/LuCI versions, reachable surfaces, and per-feature availability.

  _Use to confirm what a given firmware supports before relying on a feature._

  ```bash
  gl-inet-pp-cli doctor --agent
  ```

### Power-user + config
- **`config summary`** — A structured, per-subsystem report of the router's current configuration.

  _Use to quickly understand or capture the current state of a router._

  ```bash
  gl-inet-pp-cli config summary --agent
  ```
- **`config find`** — Find any config option anywhere across the whole router config tree.

  _Use to locate where a setting lives when config is spread across dozens of modules._

  ```bash
  gl-inet-pp-cli config find country --agent
  ```
- **`rpc call`** — Call any GL RPC module/function or any UCI option directly.

  _Use as the power-user fallback for anything not covered by a typed command._

  ```bash
  gl-inet-pp-cli rpc call netmode get_mode --agent
  ```

## Recipes


### Bank home, travel, revert

```bash
gl-inet-pp-cli snapshot save home && gl-inet-pp-cli snapshot diff home && gl-inet-pp-cli snapshot apply home
```

Capture a known-good profile, see what a venue changed, then revert cleanly.

### Get online abroad

```bash
gl-inet-pp-cli venue connect "Hotel WiFi"
```

Scan, region-check, join, and prep for the captive portal in one command.

### Fix an invisible foreign network

```bash
gl-inet-pp-cli wifi region diagnose --fix
```

Detects a network hidden by your regulatory domain and switches the region to unhide it.

### Why is there no internet?

```bash
gl-inet-pp-cli troubleshoot --fix
```

Walks the connectivity decision tree and applies safe remedies (most often: not joined in repeater mode).

### Narrow a verbose status payload for an agent

```bash
gl-inet-pp-cli system status --agent --select clients.online,wan.proto,wan.ip,memory.free
```

Use --select with dotted paths to return only the fields an agent needs from a large status response.

## Usage

Run `gl-inet-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `GL_INET_CONFIG_DIR`, `GL_INET_DATA_DIR`, `GL_INET_STATE_DIR`, or `GL_INET_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `GL_INET_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export GL_INET_HOME=/srv/gl-inet
gl-inet-pp-cli doctor
```

Under `GL_INET_HOME=/srv/gl-inet`, the four dirs resolve to `/srv/gl-inet/config`, `/srv/gl-inet/data`, `/srv/gl-inet/state`, and `/srv/gl-inet/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "gl-inet": {
      "command": "gl-inet-pp-mcp",
      "env": {
        "GL_INET_HOME": "/srv/gl-inet"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `GL_INET_DATA_DIR` overrides an explicit `--home` for that kind. Use `GL_INET_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `GL_INET_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `gl-inet-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### clients

Connected client devices

- **`gl-inet-pp-cli clients`** - List connected clients (ip, mac, name, tx/rx, online)

### snapshots

Named config profiles captured from the router

- **`gl-inet-pp-cli snapshots`** - Saved config snapshots (local store)

### system

Router system info and live status

- **`gl-inet-pp-cli system get`** - Model, firmware, OpenWrt version, hardware/software capability maps
- **`gl-inet-pp-cli system status`** - Live CPU, memory, clients, services, WAN/internet state

### vpn

VPN tunnels (WireGuard / OpenVPN clients)

- **`gl-inet-pp-cli vpn`** - Configured VPN client tunnels and their status

### wifi

WiFi radios and SSIDs

- **`gl-inet-pp-cli wifi`** - Radio states, channels, bands


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
gl-inet-pp-cli clients

# JSON for scripting and agents
gl-inet-pp-cli clients --json

# Filter to specific fields
gl-inet-pp-cli clients --json --select id,name,status

# Dry run — show the request without sending
gl-inet-pp-cli clients --dry-run

# Agent mode — JSON + compact + no prompts in one flag
gl-inet-pp-cli clients --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
gl-inet-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `gl-inet-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/gl-inet/config.toml`; `--home`, `GL_INET_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Login fails / 'access denied' from the router** — The admin password is the router's web-UI password (RPC user is 'root'). Set GL_INET_PASSWORD or pass --password.
- **Repeater can't see a network abroad** — Run 'gl-inet-pp-cli wifi region diagnose' — the network is likely on a channel your current regulatory domain forbids; it will name the country to switch to.
- **snapshot apply refuses with a model mismatch** — Snapshots are model+firmware stamped; restore on the same model, or pass --force to override (settings may not map cleanly).
- **Config/snapshot commands fail to connect** — Those use SSH (port 22). Ensure SSH is enabled on the router LAN and your key or password works.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**python-glinet**](https://github.com/tomtana/python-glinet) — Python (31 stars)
- [**glinet_api-hass**](https://github.com/spusuf/glinet_api-hass) — Python (4 stars)
- [**glinet**](https://github.com/johnhalbert/glinet) — JavaScript (3 stars)
- [**gli4py**](https://github.com/HarvsG/gli4py) — Python (2 stars)
- [**GL-iNet_utils**](https://github.com/GLiNet-Community-Scripts/GL-iNet_utils) — Shell (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
