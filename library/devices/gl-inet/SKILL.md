---
name: pp-gl-inet
description: "The first maintained CLI for GL.iNet travel routers — named config profiles, travel macros, and version-aware diagnostics no other GL.iNet tool ships. Trigger phrases: `save my router config`, `revert to my home config`, `what did this network change`, `get online at this hotel`, `fix repeater region`, `why is my router offline`, `use gl-inet`, `run gl-inet`."
author: "Paul Bockewitz"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - gl-inet-pp-cli
---

# GL.iNet — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `gl-inet-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install gl-inet --cli-only
   ```
2. Verify: `gl-inet-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Save a known-good 'home' config, adjust it at a hotel or abroad, then revert in one command with snapshot save/apply/diff. Get online at a new venue and fix the 'my foreign network is invisible' regulatory-domain problem instantly. Every command probes your exact model and firmware first, so it adapts instead of assuming — and it all runs locally over the router's own API, no GoodCloud.

## When to Use This CLI

Use gl-inet for managing a GL.iNet travel router from the terminal or an agent: capturing and reverting config profiles while traveling, getting online at new venues, fixing regulatory-domain repeater problems, diagnosing 'no internet' and weak-uplink situations, and version/model-aware inspection of what a given firmware supports. It is local-only (the router's own API + SSH), with no GoodCloud dependency.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**clients** — Connected client devices

- `gl-inet-pp-cli clients` — List connected clients (ip, mac, name, tx/rx, online)

**snapshots** — Named config profiles captured from the router

- `gl-inet-pp-cli snapshots` — Saved config snapshots (local store)

**system** — Router system info and live status

- `gl-inet-pp-cli system get` — Model, firmware, OpenWrt version, hardware/software capability maps
- `gl-inet-pp-cli system status` — Live CPU, memory, clients, services, WAN/internet state

**vpn** — VPN tunnels (WireGuard / OpenVPN clients)

- `gl-inet-pp-cli vpn` — Configured VPN client tunnels and their status

**wifi** — WiFi radios and SSIDs

- `gl-inet-pp-cli wifi` — Radio states, channels, bands


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
gl-inet-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

Authenticates against the router's local admin password via GL's challenge/crypt/sha256 handshake (read from the live challenge, so it works across firmware versions). Config snapshots and UCI access use SSH to the router. No cloud account needed.

Run `gl-inet-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  gl-inet-pp-cli clients --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `GL_INET_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `GL_INET_CONFIG_DIR`, `GL_INET_DATA_DIR`, `GL_INET_STATE_DIR`, `GL_INET_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `GL_INET_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `gl-inet-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `GL_INET_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `GL_INET_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
gl-inet-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
gl-inet-pp-cli feedback --stdin < notes.txt
gl-inet-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `GL_INET_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GL_INET_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
gl-inet-pp-cli profile save briefing --json
gl-inet-pp-cli --profile briefing clients
gl-inet-pp-cli profile list --json
gl-inet-pp-cli profile show briefing
gl-inet-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `gl-inet-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add gl-inet-pp-mcp -- gl-inet-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which gl-inet-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   gl-inet-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `gl-inet-pp-cli <command> --help`.
