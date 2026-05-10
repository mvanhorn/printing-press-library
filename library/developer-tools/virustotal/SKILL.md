---
name: pp-virustotal
description: "Printing Press CLI for Virustotal. Malware analysis and threat intelligence platform"
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - virustotal-pp-cli
---

# Virustotal — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `virustotal-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install virustotal --cli-only
   ```
2. Verify: `virustotal-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Malware analysis and threat intelligence platform

## Command Reference

**domains** — Manage domains

- `virustotal-pp-cli domains <domain>` — Retrieve information about a domain

**files** — Manage files

- `virustotal-pp-cli files get` — Retrieve scan results for a file hash
- `virustotal-pp-cli files scan` — Submit a file for scanning

**ip-addresses** — Manage ip addresses

- `virustotal-pp-cli ip-addresses <ip>` — Retrieve information about an IP address

**urls** — Manage urls

- `virustotal-pp-cli urls get` — Retrieve scan results for a URL
- `virustotal-pp-cli urls scan` — Submit a URL for scanning

**virustotal-search** — Manage virustotal search

- `virustotal-pp-cli virustotal-search` — Search using VirusTotal query syntax


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
virustotal-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Set your API key via environment variable:

```bash
export VIRUSTOTAL_API_KEY="<your-key>"
```

Or persist it in `~/.config/virustotal-pp-cli/config.toml`.

Run `virustotal-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  virustotal-pp-cli domains mock-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
virustotal-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
virustotal-pp-cli feedback --stdin < notes.txt
virustotal-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.virustotal-pp-cli/feedback.jsonl`. They are never POSTed unless `VIRUSTOTAL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `VIRUSTOTAL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
virustotal-pp-cli profile save briefing --json
virustotal-pp-cli --profile briefing domains mock-value
virustotal-pp-cli profile list --json
virustotal-pp-cli profile show briefing
virustotal-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `virustotal-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add virustotal-pp-mcp -- virustotal-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which virustotal-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   virustotal-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `virustotal-pp-cli <command> --help`.
