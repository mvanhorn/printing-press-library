---
name: pp-namecheap
description: "Printing Press CLI for Namecheap. Curated OpenAPI description for Namecheap's XML API. The real API uses a single endpoint (`/xml.response`) with a..."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - namecheap-pp-cli
---

# Namecheap — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `namecheap-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install namecheap --cli-only
   ```
2. Verify: `namecheap-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/namecheap/cmd/namecheap-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Curated OpenAPI description for Namecheap's XML API. The real API uses a single
endpoint (`/xml.response`) with a `Command` query parameter plus Namecheap's
query-string authentication parameters (`ApiUser`, `ApiKey`, `UserName`, `ClientIp`).
Generation uses command-shaped pseudo paths that are normalized back to `/xml.response`
by the Namecheap printed CLI patch layer.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**dns** — Manage dns

- `namecheap-pp-cli dns get-email-forwarding` — Runs `namecheap.domains.dns.getEmailForwarding`.
- `namecheap-pp-cli dns get-hosts` — Runs `namecheap.domains.dns.getHosts`.
- `namecheap-pp-cli dns get-list` — Get DNS nameserver type and nameservers.
- `namecheap-pp-cli dns set-custom` — Runs `namecheap.domains.dns.setCustom`.
- `namecheap-pp-cli dns set-default` — Switch a domain to Namecheap default DNS.
- `namecheap-pp-cli dns set-hosts` — Runs `namecheap.domains.dns.setHosts`; HostName1/RecordType1/Address1/TTL1 style parameters can be passed via...

**domains** — Manage domains

- `namecheap-pp-cli domains check` — Check domain availability for one or more domains.
- `namecheap-pp-cli domains create` — Runs `namecheap.domains.create`. This is a mutating paid operation; use dry-run unless intentionally registering.
- `namecheap-pp-cli domains get-contacts` — Runs `namecheap.domains.getContacts`.
- `namecheap-pp-cli domains get-info` — Runs `namecheap.domains.getInfo` for a domain.
- `namecheap-pp-cli domains get-list` — Runs `namecheap.domains.getList` with paging and optional filters.
- `namecheap-pp-cli domains get-registrar-lock` — Runs `namecheap.domains.getRegistrarLock`.
- `namecheap-pp-cli domains get-tld-list` — Runs `namecheap.domains.getTldList`.
- `namecheap-pp-cli domains renew` — Runs `namecheap.domains.renew`. Mutating paid operation.
- `namecheap-pp-cli domains set-registrar-lock` — Runs `namecheap.domains.setRegistrarLock`.

**ssl** — Manage ssl

- `namecheap-pp-cli ssl get-info` — Get SSL certificate information.
- `namecheap-pp-cli ssl get-list` — Runs `namecheap.ssl.getList`.
- `namecheap-pp-cli ssl parse-csr` — Parse a certificate signing request.

**users** — Manage users

- `namecheap-pp-cli users address-get-info` — Runs `namecheap.users.address.getInfo`.
- `namecheap-pp-cli users address-get-list` — Runs `namecheap.users.address.getList`.
- `namecheap-pp-cli users get-balances` — Runs `namecheap.users.getBalances`.
- `namecheap-pp-cli users get-pricing` — Runs `namecheap.users.getPricing`.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
namecheap-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `namecheap-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export NAMECHEAP_API_KEY="<your-key>"
```

Or persist it in `~/.config/namecheap-pp-cli/config.toml`.

Run `namecheap-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  namecheap-pp-cli dns get-email-forwarding --agent --select id,name,status
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

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
namecheap-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
namecheap-pp-cli feedback --stdin < notes.txt
namecheap-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.namecheap-pp-cli/feedback.jsonl`. They are never POSTed unless `NAMECHEAP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NAMECHEAP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
namecheap-pp-cli profile save briefing --json
namecheap-pp-cli --profile briefing dns get-email-forwarding
namecheap-pp-cli profile list --json
namecheap-pp-cli profile show briefing
namecheap-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `namecheap-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add namecheap-pp-mcp -- namecheap-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which namecheap-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   namecheap-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `namecheap-pp-cli <command> --help`.
