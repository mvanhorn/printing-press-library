---
name: pp-adguard-home
description: "Printing Press CLI for Adguard Home. AdGuard Home REST-ish API. Our admin web interface is built on top of this REST-ish API."
author: "Eric Jung"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - adguard-home-pp-cli
    install:
      - kind: go
        bins: [adguard-home-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/monitoring/adguard-home/cmd/adguard-home-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/monitoring/adguard-home/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Adguard Home — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `adguard-home-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install adguard-home --cli-only
   ```
2. Verify: `adguard-home-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/adguard-home/cmd/adguard-home-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Command Reference

**access** — Manage access

- `adguard-home-pp-cli access list` — List (dis)allowed clients, blocked hosts, etc.
- `adguard-home-pp-cli access set` — Set (dis)allowed clients, blocked hosts, etc.

**adguard-home-profile** — Manage adguard home profile

- `adguard-home-pp-cli adguard-home-profile get` — Get
- `adguard-home-pp-cli adguard-home-profile update` — Updates current user info

**apple** — Manage apple

- `adguard-home-pp-cli apple mobile-config-do-h` — Get DNS over HTTPS .mobileconfig.
- `adguard-home-pp-cli apple mobile-config-do-t` — Get DNS over TLS .mobileconfig.

**blocked-services** — Blocked services controls

- `adguard-home-pp-cli blocked-services all` — Get available services to use for blocking
- `adguard-home-pp-cli blocked-services available-services` — Deprecated: Use `GET /blocked_services/all` instead.
- `adguard-home-pp-cli blocked-services list` — Deprecated: Use `GET /blocked_services/get` instead.
- `adguard-home-pp-cli blocked-services schedule` — Get blocked services
- `adguard-home-pp-cli blocked-services schedule-update` — Update blocked services
- `adguard-home-pp-cli blocked-services set` — Deprecated: Use `PUT /blocked_services/update` instead.

**cache-clear** — Manage cache clear

- `adguard-home-pp-cli cache-clear` — Clear DNS cache

**clients** — Clients list operations

- `adguard-home-pp-cli clients add` — Add a new client
- `adguard-home-pp-cli clients delete` — Remove a client
- `adguard-home-pp-cli clients find` — Get information about clients by their IP addresses or ClientIDs.
- `adguard-home-pp-cli clients search` — Retrieve information about clients by performing an exact match search using IP addresses, CIDRs, MAC addresses, or...
- `adguard-home-pp-cli clients status` — Get information about configured clients
- `adguard-home-pp-cli clients update` — Update client information

**dhcp** — Built-in DHCP server controls

- `adguard-home-pp-cli dhcp add-static-lease` — Adds a static lease
- `adguard-home-pp-cli dhcp check-active` — Searches for an active DHCP server on the network
- `adguard-home-pp-cli dhcp interfaces` — Gets the available interfaces
- `adguard-home-pp-cli dhcp remove-static-lease` — Removes a static lease
- `adguard-home-pp-cli dhcp reset` — Reset DHCP configuration
- `adguard-home-pp-cli dhcp reset-leases` — Reset DHCP leases
- `adguard-home-pp-cli dhcp set-config` — Updates the current DHCP server configuration
- `adguard-home-pp-cli dhcp status` — Gets the current DHCP settings and status
- `adguard-home-pp-cli dhcp update-static-lease` — Updates IP address, hostname of the static lease. IP version must be the same as previous.

**dns-config** — Manage dns config

- `adguard-home-pp-cli dns-config` — Set general DNS parameters

**dns-info** — Manage dns info

- `adguard-home-pp-cli dns-info` — Get general DNS parameters

**filtering** — Rule-based filtering

- `adguard-home-pp-cli filtering add-url` — Add filter URL or an absolute file path
- `adguard-home-pp-cli filtering check-host` — Check if host name is filtered
- `adguard-home-pp-cli filtering config` — Set filtering parameters
- `adguard-home-pp-cli filtering refresh` — Reload filtering rules from URLs. This might be needed if new URL was just added and you don't want to wait for...
- `adguard-home-pp-cli filtering remove-url` — Remove filter URL
- `adguard-home-pp-cli filtering set-rules` — Set user-defined filter rules
- `adguard-home-pp-cli filtering set-url` — Set URL parameters
- `adguard-home-pp-cli filtering status` — Get filtering parameters

**i18n** — Application localization

- `adguard-home-pp-cli i18n change-language` — Change current language. Argument must be an ISO 639-1 two-letter code.
- `adguard-home-pp-cli i18n current-language` — Get currently set language. Result is ISO 639-1 two-letter code. Empty result means default language.

**install** — First-time install configuration handlers

- `adguard-home-pp-cli install check-config` — Checks configuration
- `adguard-home-pp-cli install configure` — Applies the initial configuration.
- `adguard-home-pp-cli install get-addresses` — Gets the network interfaces information.

**login** — Manage login

- `adguard-home-pp-cli login` — Perform administrator log-in

**logout** — Manage logout

- `adguard-home-pp-cli logout` — Perform administrator log-out

**parental** — Blocking adult and explicit materials

- `adguard-home-pp-cli parental disable` — Disable parental filtering
- `adguard-home-pp-cli parental enable` — Enable parental filtering
- `adguard-home-pp-cli parental status` — Get parental filtering status

**protection** — Manage protection

- `adguard-home-pp-cli protection` — Set protection state and duration

**querylog** — Manage querylog

- `adguard-home-pp-cli querylog get-query-log-config` — Get query log parameters
- `adguard-home-pp-cli querylog put-query-log-config` — Set query log parameters
- `adguard-home-pp-cli querylog query-log` — Get DNS server query log.

**querylog-clear** — Manage querylog clear

- `adguard-home-pp-cli querylog-clear` — Clear query log

**querylog-config** — Manage querylog config

- `adguard-home-pp-cli querylog-config` — Deprecated: Use `PUT /querylog/config/update` instead.

**querylog-info** — Manage querylog info

- `adguard-home-pp-cli querylog-info` — Deprecated: Use `GET /querylog/config` instead. NOTE: If `interval` was configured by editing configuration file or...

**rewrite** — DNS rewrites

- `adguard-home-pp-cli rewrite add` — Add a new Rewrite rule
- `adguard-home-pp-cli rewrite delete` — Remove a Rewrite rule
- `adguard-home-pp-cli rewrite list` — Get list of Rewrite rules
- `adguard-home-pp-cli rewrite settings-get` — Get rewrite settings
- `adguard-home-pp-cli rewrite settings-update` — Update rewrite settings
- `adguard-home-pp-cli rewrite update` — Update a Rewrite rule

**safebrowsing** — Blocking malware/phishing sites

- `adguard-home-pp-cli safebrowsing disable` — Disable safebrowsing
- `adguard-home-pp-cli safebrowsing enable` — Enable safebrowsing
- `adguard-home-pp-cli safebrowsing status` — Get safebrowsing status

**safesearch** — Enforce family-friendly results in search engines

- `adguard-home-pp-cli safesearch disable` — Disable safesearch
- `adguard-home-pp-cli safesearch enable` — Enable safesearch
- `adguard-home-pp-cli safesearch settings` — Update safesearch settings
- `adguard-home-pp-cli safesearch status` — Get safesearch status

**stats** — AdGuard Home statistics

- `adguard-home-pp-cli stats get-config` — Get statistics parameters
- `adguard-home-pp-cli stats put-config` — Set statistics parameters
- `adguard-home-pp-cli stats stats` — Get DNS server statistics

**stats-config** — Manage stats config

- `adguard-home-pp-cli stats-config` — Deprecated: Use `PUT /stats/config/update` instead.

**stats-info** — Manage stats info

- `adguard-home-pp-cli stats-info` — Deprecated: Use `GET /stats/config` instead. NOTE: If `interval` was configured by editing configuration file or new...

**stats-reset** — Manage stats reset

- `adguard-home-pp-cli stats-reset` — Reset all statistics to zeroes

**status** — Manage status

- `adguard-home-pp-cli status` — Get DNS server current status and general settings

**test-upstream-dns** — Manage test upstream dns

- `adguard-home-pp-cli test-upstream-dns` — Test upstream configuration

**tls** — AdGuard Home HTTPS/DoH/DoQ/DoT settings

- `adguard-home-pp-cli tls configure` — Updates current TLS configuration
- `adguard-home-pp-cli tls status` — Returns TLS configuration and its status
- `adguard-home-pp-cli tls validate` — Checks if the current TLS configuration is valid

**update** — Manage update

- `adguard-home-pp-cli update` — Begin auto-upgrade procedure

**version-json** — Manage version json

- `adguard-home-pp-cli version-json` — Gets information about the latest available version of AdGuard


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
adguard-home-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `adguard-home-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export ADGUARD_HOME_USERNAME="<your-key>"
```

Or persist it in `~/.config/adguard-home-pp-cli/config.toml`.

Run `adguard-home-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  adguard-home-pp-cli access list --agent --select id,name,status
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
adguard-home-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
adguard-home-pp-cli feedback --stdin < notes.txt
adguard-home-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.adguard-home-pp-cli/feedback.jsonl`. They are never POSTed unless `ADGUARD_HOME_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ADGUARD_HOME_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
adguard-home-pp-cli profile save briefing --json
adguard-home-pp-cli --profile briefing access list
adguard-home-pp-cli profile list --json
adguard-home-pp-cli profile show briefing
adguard-home-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `adguard-home-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/monitoring/adguard-home/cmd/adguard-home-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add adguard-home-pp-mcp -- adguard-home-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which adguard-home-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   adguard-home-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `adguard-home-pp-cli <command> --help`.
