---
name: pp-gamma
description: "Printing Press CLI for Gamma. Generate presentations, documents, websites, and social posts programmatically."
author: "Anchal Sharma"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - gamma-pp-cli
---

# Gamma — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `gamma-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install gamma --cli-only
   ```
2. Verify: `gamma-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Generate presentations, documents, websites, and social posts programmatically. Auth: X-API-KEY header (not Authorization Bearer). Env var: GAMMA_API_KEY. All generation is async: POST to create, then poll GET /generations/{id} until completed or failed.

## Command Reference

**folders** — Manage folders

- `gamma-pp-cli folders` — List folders the authenticated user can access. Use returned id values in folderIds in generation requests.

**gammas** — Manage gammas

- `gamma-pp-cli gammas <gammaId>` — Delete a Gamma permanently. Requires workspace admin role. Auto-archives first if needed.

**generations** — Create and poll AI generations

- `gamma-pp-cli generations create` — Start an asynchronous generation from text. Returns generationId immediately. Poll GET /v1.
- `gamma-pp-cli generations create-from-template` — Adapt an existing Gamma by providing the template file ID and instructions. Template structure preserved by default.
- `gamma-pp-cli generations get-status` — Poll this endpoint until status is completed or failed. On completion returns gammaId (g_...

**themes** — Manage themes

- `gamma-pp-cli themes` — List themes available in the authenticated workspace. Use returned id as themeId in generation requests.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
gamma-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `gamma-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export GAMMA_API_KEY="<your-key>"
```

Or persist it in `~/.config/gamma-public-pp-cli/config.toml`.

Run `gamma-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  gamma-pp-cli folders --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
gamma-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
gamma-pp-cli feedback --stdin < notes.txt
gamma-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/gamma-pp-cli/feedback.jsonl`. They are never POSTed unless `GAMMA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GAMMA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
gamma-pp-cli profile save briefing --json
gamma-pp-cli --profile briefing folders
gamma-pp-cli profile list --json
gamma-pp-cli profile show briefing
gamma-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `gamma-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add gamma-pp-mcp -- gamma-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which gamma-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   gamma-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `gamma-pp-cli <command> --help`.
