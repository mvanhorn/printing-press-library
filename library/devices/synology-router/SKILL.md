---
name: pp-synology-router
description: "Printing Press CLI for Synology Router. REST-style API for managing Synology routers via SRM (Synology Router Manager). All operations use form-encoded POST..."
author: "Eric Jung"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - synology-router-pp-cli
---

# Synology Router — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `synology-router-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install synology-router --cli-only
   ```
2. Verify: `synology-router-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

REST-style API for managing Synology routers via SRM (Synology Router Manager).
All operations use form-encoded POST requests to /webapi/entry.cgi with
api, method, and version parameters. Authentication uses SYNO.API.Auth to
obtain a session ID (sid) stored as a Cookie header id=<sid>.
The router model is RT2600ac. Router IP defaults to 192.168.1.254 port 8001 (HTTPS).

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**access-control** — Access control groups and safe access (SYNO.SafeAccess)

- `synology-router-pp-cli access-control` — Returns access control / safe access groups with their devices and time spent online. Wire call: POST...

**devices** — Connected device management — IP, MAC, hostname, online status (SYNO.Core.Network.NSM.Device)

- `synology-router-pp-cli devices` — Lists all devices known to the router with their IP, MAC, hostname, and online status. Wire call: POST...

**dns** — DDNS records and external IP (SYNO.Core.DDNS)

- `synology-router-pp-cli dns ddns-records-list` — Returns all configured DDNS records and their update status. Wire call: POST /webapi/entry.cgi with...
- `synology-router-pp-cli dns external-ip-get` — Returns the current external (WAN) IP address. Wire call: POST /webapi/entry.cgi with api=SYNO.Core.DDNS.ExtIP,...

**firewall** — Firewall policy route rules (SYNO.Core.Network.Router.PolicyRoute)

- `synology-router-pp-cli firewall rules-list` — Returns all IPv4 policy routing / firewall rules. Wire call: POST /webapi/entry.cgi with...
- `synology-router-pp-cli firewall rules-set` — Replaces all policy routing rules. Requires the complete rules array. Wire call: POST /webapi/entry.cgi with...

**mesh** — Mesh network nodes and system info (SYNO.Mesh)

- `synology-router-pp-cli mesh nodes-list` — Returns all mesh nodes with status, current rate, and connected device counts. Wire call: POST /webapi/entry.cgi...
- `synology-router-pp-cli mesh system-info-get` — Returns SRM system information including firmware version, model, and uptime. Wire call: POST /webapi/entry.cgi with...

**qos** — Quality of Service rules (SYNO.Core.NGFW.QoS.Rules)

- `synology-router-pp-cli qos` — Returns Quality of Service rules per device including bandwidth guarantees and maximums. Wire call: POST...

**session** — Manage session

- `synology-router-pp-cli session auth-login` — Authenticates with the router using admin credentials and returns a session ID (sid). Wire call: POST...
- `synology-router-pp-cli session auth-logout` — Destroys the current session. Wire call: POST /webapi/auth.cgi with api=SYNO.API.Auth, method=Logout.

**smartwan** — Smart WAN / multi-WAN configuration (SYNO.Core.Network.SmartWAN)

- `synology-router-pp-cli smartwan smart-wan-config-get` — Returns the Smart WAN (multi-WAN) configuration including failover/load-balancing mode. Wire call: POST...
- `synology-router-pp-cli smartwan smart-wan-config-set` — Updates the Smart WAN (multi-WAN) configuration. Wire call: POST /webapi/entry.cgi with...
- `synology-router-pp-cli smartwan smart-wan-gateway-list` — Returns all Smart WAN gateway configurations with their active/standby status. Wire call: POST /webapi/entry.cgi...

**system** — System information and utilization (SYNO.Core.System)

- `synology-router-pp-cli system` — Returns the list of all available SRM APIs with their supported versions. Wire call: GET /webapi/query.cgi with...

**traffic** — Network traffic and bandwidth statistics (SYNO.Core.NGFW.Traffic)

- `synology-router-pp-cli traffic` — Returns network traffic statistics per device for a given time interval. Wire call: POST /webapi/entry.cgi with...

**utilization** — Manage utilization

- `synology-router-pp-cli utilization` — Returns CPU, memory, and network interface utilization statistics. Wire call: POST /webapi/entry.cgi with...

**wan** — WAN connection status (SYNO.Core.Network.Router.ConnectionStatus)

- `synology-router-pp-cli wan` — Returns the current WAN connection status including IPv4/IPv6 status, IP address, gateway, DNS servers, and...

**wifi** — Wi-Fi network and client management (SYNO.Wifi.Network.Setting)

- `synology-router-pp-cli wifi settings-get` — Returns all Wi-Fi network profiles including SSIDs, security settings, and radio configuration. Wire call: POST...
- `synology-router-pp-cli wifi settings-set` — Updates Wi-Fi network profiles. Requires the complete profiles array. Wire call: POST /webapi/entry.cgi with...

**wol** — Wake-on-LAN management (SYNO.Core.Network.WOL)

- `synology-router-pp-cli wol device-add` — Registers a device MAC address for Wake-on-LAN. Wire call: POST /webapi/entry.cgi with api=SYNO.Core.Network.WOL,...
- `synology-router-pp-cli wol devices-list` — Returns all devices configured for Wake-on-LAN. Wire call: POST /webapi/entry.cgi with api=SYNO.Core.Network.WOL,...
- `synology-router-pp-cli wol wake` — Sends a Wake-on-LAN magic packet to the specified device. Wire call: POST /webapi/entry.cgi with...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
synology-router-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `synology-router-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export SYNOLOGY_ROUTER_COOKIE_AUTH="<your-key>"
```

Or persist it in `~/.config/synology-router-manager-pp-cli/config.toml`.

Run `synology-router-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  synology-router-pp-cli devices --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
synology-router-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
synology-router-pp-cli feedback --stdin < notes.txt
synology-router-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.synology-router-pp-cli/feedback.jsonl`. They are never POSTed unless `SYNOLOGY_ROUTER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SYNOLOGY_ROUTER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
synology-router-pp-cli profile save briefing --json
synology-router-pp-cli --profile briefing devices
synology-router-pp-cli profile list --json
synology-router-pp-cli profile show briefing
synology-router-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `synology-router-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add synology-router-pp-mcp -- synology-router-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which synology-router-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   synology-router-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `synology-router-pp-cli <command> --help`.
