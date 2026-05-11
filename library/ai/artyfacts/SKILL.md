---
name: pp-artyfacts
description: "Printing Press CLI for Artyfacts. Artyfacts — persistent workspace for AI agent work products"
author: "Bernard Maltais"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - artyfacts-pp-cli
    install:
      - kind: go
        bins: [artyfacts-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/artyfacts/cmd/artyfacts-pp-cli
---

# Artyfacts — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `artyfacts-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install artyfacts --cli-only
   ```
2. Verify: `artyfacts-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/artyfacts/cmd/artyfacts-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Artyfacts — persistent workspace for AI agent work products

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**artifacts** — Manage AI-generated work product artifacts

- `artyfacts-pp-cli artifacts create` — Create a new artifact envelope. Add content using section tools after creation.
- `artyfacts-pp-cli artifacts get` — Get a specific artifact with all sections
- `artyfacts-pp-cli artifacts list` — List artifacts, optionally filtered by type, folder, or root-level
- `artyfacts-pp-cli artifacts update` — Update artifact metadata (title, summary, status, tags, visibility, retention)

**org** — Organization context and settings

- `artyfacts-pp-cli org` — Get organization details, agent conventions, and preferred workflows

**sections** — Manage sections within an artifact

- `artyfacts-pp-cli sections create` — Create a new section with content in one step
- `artyfacts-pp-cli sections delete` — Delete a section from an artifact
- `artyfacts-pp-cli sections get` — Get a specific section by artifact and section ID
- `artyfacts-pp-cli sections list` — List all sections of an artifact, ordered by position
- `artyfacts-pp-cli sections update` — Update section content or streaming state


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
artyfacts-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Store your access token:

```bash
artyfacts-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `ARTYFACTS_API_KEY` as an environment variable.

Run `artyfacts-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  artyfacts-pp-cli artifacts list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
artyfacts-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
artyfacts-pp-cli feedback --stdin < notes.txt
artyfacts-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.artyfacts-pp-cli/feedback.jsonl`. They are never POSTed unless `ARTYFACTS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ARTYFACTS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
artyfacts-pp-cli profile save briefing --json
artyfacts-pp-cli --profile briefing artifacts list
artyfacts-pp-cli profile list --json
artyfacts-pp-cli profile show briefing
artyfacts-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `artyfacts-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/artyfacts/cmd/artyfacts-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add artyfacts-pp-mcp -- artyfacts-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which artyfacts-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   artyfacts-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `artyfacts-pp-cli <command> --help`.
