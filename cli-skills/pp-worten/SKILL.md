---
name: pp-worten
description: "Printing Press CLI for Worten. Local OpenAPI seed extracted from the working housebuy Worten CLI."
author: "Alexandre Santos"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - worten-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/commerce/worten/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Worten — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `worten-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install worten --cli-only
   ```
2. Verify: `worten-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Local OpenAPI seed extracted from the working housebuy Worten CLI.
This does not claim to describe the full Worten platform.
It only captures the load-bearing endpoints currently used by the housebuy operator.

## Command Reference

**worten-api** — Manage worten api

- `worten-pp-cli worten-api get-offer-stock` — Fetch nearby store pickup stock for an offer.
- `worten-pp-cli worten-api get-product-details` — Fetch product details by Worten product identifier.
- `worten-pp-cli worten-api get-search-suggestions` — Fetch search suggestions for a text query.
- `worten-pp-cli worten-api get-technical-specifications` — Fetch technical specifications for a Worten product.
- `worten-pp-cli worten-api search-products` — Search Worten products by query and context.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
worten-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `worten-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  worten-pp-cli worten-api get-offer-stock --offer-id 550e8400-e29b-41d4-a716-446655440000 --search-query example-value --radius 42 --agent --select id,name,status
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
worten-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
worten-pp-cli feedback --stdin < notes.txt
worten-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/worten-pp-cli/feedback.jsonl`. They are never POSTed unless `WORTEN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WORTEN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
worten-pp-cli profile save briefing --json
worten-pp-cli --profile briefing worten-api get-offer-stock --offer-id 550e8400-e29b-41d4-a716-446655440000 --search-query example-value --radius 42
worten-pp-cli profile list --json
worten-pp-cli profile show briefing
worten-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `worten-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add worten-pp-mcp -- worten-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which worten-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   worten-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `worten-pp-cli <command> --help`.