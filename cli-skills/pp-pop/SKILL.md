---
name: pp-pop
description: "Printing Press CLI for Pop. POP API for European electronic invoicing workflows. This Printing Press blueprint intentionally exposes only the..."
author: "Mirco Babini"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pop-pp-cli
    install:
      - kind: go
        bins: [pop-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/pop/cmd/pop-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/payments/pop/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Pop — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `pop-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install pop --cli-only
   ```
2. Verify: `pop-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/pop/cmd/pop-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Command Reference

**create-pdf** — Create branded PDF invoices and optionally email them.

- `pop-pp-cli create-pdf` — Generate a branded PDF invoice. Depending on account setup and payload, POP may return a direct file, a URL, or a...

**create-ubl** — Create PEPPOL UBL invoices and optionally submit them to the network.

- `pop-pp-cli create-ubl` — Generate a PEPPOL BIS / UBL invoice and optionally submit it through POP's PEPPOL integration.

**create-xml** — Create Italian FatturaPA XML invoices and optionally submit them to SdI.

- `pop-pp-cli create-xml` — Generate an Italian FatturaPA XML document and optionally submit it to SdI. The `data` object must follow POP's...

**peppol** — Manage PEPPOL document workflows

- `pop-pp-cli peppol get-document` — Retrieve a PEPPOL document from POP by UUID and optional zone.

**sdi** — Manage SdI document workflows

- `pop-pp-cli sdi get-invoice-status` — Read POP's recorded SdI notifications for a submitted invoice UUID.
- `pop-pp-cli sdi get-sdi-document` — Retrieve a stored SdI document by UUID.
- `pop-pp-cli sdi preserve-sdi-document` — Archive an accepted SdI document in POP's long-term storage.
- `pop-pp-cli sdi verify-sdi-document` — Validate a Base64-encoded SdI XML document before submission.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pop-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `pop-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export POP_API_KEY="<your-key>"
```

Or persist it in `~/.config/pop-pp-cli/config.toml`.

Run `pop-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  pop-pp-cli create-pdf --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
pop-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pop-pp-cli feedback --stdin < notes.txt
pop-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.pop-pp-cli/feedback.jsonl`. They are never POSTed unless `POP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `POP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
pop-pp-cli profile save briefing --json
pop-pp-cli --profile briefing create-pdf
pop-pp-cli profile list --json
pop-pp-cli profile show briefing
pop-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `pop-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/pop/cmd/pop-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add pop-pp-mcp -- pop-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pop-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   pop-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `pop-pp-cli <command> --help`.
