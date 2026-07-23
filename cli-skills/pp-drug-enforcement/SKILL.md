---
name: pp-drug-enforcement
description: "Search FDA drug recall enforcement records — keyless, offline-cacheable, agent-native. Reports FDA facts only; never says a drug is safe."
author: "laci141"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - drug-enforcement-pp-cli
    install:
      - kind: go
        bins: [drug-enforcement-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/health/drug-enforcement/cmd/drug-enforcement-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/health/drug-enforcement/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Drug Enforcement — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `drug-enforcement-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install drug-enforcement --cli-only
   ```
2. Verify: `drug-enforcement-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/health/drug-enforcement/cmd/drug-enforcement-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

A keyless CLI over the openFDA drug enforcement endpoint. The check, firm, recent, and reference commands wrap FDA recall search with pre-built queries; every result cites the recall number, prints the FDA class legend, and carries an enforcement-not-medical-advice disclaimer. A drug with no recall is reported as 'no recall records found', never as safe.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Recall lookups
- **`check`** — Find active FDA recalls that mention a drug, optionally filtered to the most serious class.

  _Reach for this when an agent needs the recall status of a specific medication by name._

  ```bash
  drug-enforcement-pp-cli check "ibuprofen" --class 1
  ```
- **`firm`** — List every recall attributed to a recalling firm or manufacturer.

  _Use this to audit a manufacturer's recall history._

  ```bash
  drug-enforcement-pp-cli firm "Teva"
  ```
- **`recent`** — List recalls initiated in the last N days, most recent first.

  _Use this for a periodic sweep of newly initiated drug recalls._

  ```bash
  drug-enforcement-pp-cli recent --days 30
  ```
- **`reference`** — Show full detail for a single recall number, with the FDA class legend.

  _Use this to expand one recall's full facts after a check/firm/recent lookup surfaces its number._

  ```bash
  drug-enforcement-pp-cli reference D-0183-2023
  ```

## Recipes

### Serious recalls for a drug

```bash
drug-enforcement-pp-cli check "valsartan" --class 1 --json --select results.recall_number,results.reason_for_recall
```

Class I recalls for a drug, narrowed to the recall number and reason.

### Manufacturer recall history

```bash
drug-enforcement-pp-cli firm "Teva" --json
```

Every recall attributed to a firm, as JSON.

### Rolling 7-day sweep

```bash
drug-enforcement-pp-cli recent --days 7
```

Recalls initiated in the last week, most recent first.

## Command Reference

**enforcement** — FDA drug recall enforcement records (openFDA /drug/enforcement.json)

- `drug-enforcement-pp-cli enforcement` — Search drug recall enforcement records with an openFDA Lucene search expression


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
drug-enforcement-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `drug-enforcement-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  drug-enforcement-pp-cli enforcement --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `DRUG_ENFORCEMENT_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `DRUG_ENFORCEMENT_CONFIG_DIR`, `DRUG_ENFORCEMENT_DATA_DIR`, `DRUG_ENFORCEMENT_STATE_DIR`, `DRUG_ENFORCEMENT_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `DRUG_ENFORCEMENT_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `drug-enforcement-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "drug-enforcement": {
        "command": "drug-enforcement-pp-mcp",
        "env": {
          "DRUG_ENFORCEMENT_HOME": "/srv/drug-enforcement"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `DRUG_ENFORCEMENT_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `DRUG_ENFORCEMENT_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
drug-enforcement-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
drug-enforcement-pp-cli feedback --stdin < notes.txt
drug-enforcement-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `DRUG_ENFORCEMENT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DRUG_ENFORCEMENT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
drug-enforcement-pp-cli profile save briefing --json
drug-enforcement-pp-cli --profile briefing enforcement
drug-enforcement-pp-cli profile list --json
drug-enforcement-pp-cli profile show briefing
drug-enforcement-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `drug-enforcement-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/health/drug-enforcement/cmd/drug-enforcement-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add drug-enforcement-pp-mcp -- drug-enforcement-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which drug-enforcement-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   drug-enforcement-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `drug-enforcement-pp-cli <command> --help`.
