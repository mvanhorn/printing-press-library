---
name: pp-cisa-kev
description: "Printing Press CLI for Cisa Kev. Public JSON feed for the CISA Known Exploited Vulnerabilities catalog."
author: "Solomon Neas"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - cisa-kev-pp-cli
    install:
      - kind: go
        bins: [cisa-kev-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/cisa-kev/cmd/cisa-kev-pp-cli
---

# Cisa Kev — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `cisa-kev-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install cisa-kev --cli-only
   ```
2. Verify: `cisa-kev-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/cisa-kev/cmd/cisa-kev-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Public JSON feed for the CISA Known Exploited Vulnerabilities catalog. The catalog lists vulnerabilities known to be exploited in the wild and includes vendor, product, required-action, due-date, ransomware-use, CWE, and notes metadata.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**sites** — Manage sites

- `cisa-kev-pp-cli sites` — Returns the current CISA Known Exploited Vulnerabilities catalog

**vulns** — Search and triage CISA KEV vulnerabilities

- `cisa-kev-pp-cli vulns list` — Flatten the feed into vulnerability rows with filters for vendor, product, CWE, ransomware use, added date, and due date
- `cisa-kev-pp-cli vulns search <query>` — Full-text search across CVE IDs, vendors, products, names, descriptions, required actions, notes, and CWEs
- `cisa-kev-pp-cli vulns get <cve-id>` — Show one KEV entry by CVE ID
- `cisa-kev-pp-cli vulns due` — Triage due-soon or overdue KEV entries

Useful examples:

```bash
cisa-kev-pp-cli vulns list --vendor Ivanti --limit 10 --agent
cisa-kev-pp-cli vulns search "PeopleSoft" --agent
cisa-kev-pp-cli vulns get CVE-2026-10520 --agent
cisa-kev-pp-cli vulns due --due-before 2026-06-30 --ransomware-only --agent
```


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
cisa-kev-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `cisa-kev-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  cisa-kev-pp-cli sites --agent --select id,name,status
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
cisa-kev-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
cisa-kev-pp-cli feedback --stdin < notes.txt
cisa-kev-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/cisa-kev-pp-cli/feedback.jsonl`. They are never POSTed unless `CISA_KEV_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CISA_KEV_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
cisa-kev-pp-cli profile save briefing --json
cisa-kev-pp-cli --profile briefing sites
cisa-kev-pp-cli profile list --json
cisa-kev-pp-cli profile show briefing
cisa-kev-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `cisa-kev-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/cisa-kev/cmd/cisa-kev-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add cisa-kev-pp-mcp -- cisa-kev-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which cisa-kev-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   cisa-kev-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `cisa-kev-pp-cli <command> --help`.
