---
name: pp-substack-notes
description: "Printing Press CLI for Substack Notes. Unofficial Substack Notes endpoints observed from the authenticated web app."
author: "Peter Yang"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - substack-notes-pp-cli
---

# Substack Notes — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `substack-notes-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install substack-notes --cli-only
   ```
2. Verify: `substack-notes-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/substack-notes/cmd/substack-notes-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Unofficial Substack Notes endpoints observed from the authenticated web app.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**notes** — Post, schedule, and manage Substack Notes

- `substack-notes-pp-cli notes post --text "Short note body"` — Publish a note immediately
- `substack-notes-pp-cli notes post --text "Image note" --image ./image.png` — Publish a note with an image attachment
- `substack-notes-pp-cli notes schedule --text "Tomorrow's note" --at "2026-07-15 09:00"` — Schedule a note
- `substack-notes-pp-cli notes draft --text "Idea to finish later"` — Save an unscheduled note draft
- `substack-notes-pp-cli notes draft --file note.txt --image ./image.jpg` — Save an image-backed draft
- `substack-notes-pp-cli notes recent --limit 5 --json` — Read recent published notes
- `substack-notes-pp-cli notes list --limit 20` — List note drafts and scheduled notes
- `substack-notes-pp-cli notes delete <draft-id>` — Delete a note draft or scheduled note

**comment** — Manage comment

- `substack-notes-pp-cli comment delete-draft` — Delete a note draft
- `substack-notes-pp-cli comment publish-note` — Publish a note immediately
- `substack-notes-pp-cli comment save-draft` — Save or schedule a note draft

**feed** — Manage feed

- `substack-notes-pp-cli feed` — List note drafts and scheduled notes


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
substack-notes-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run guided browser-session setup first:

```bash
substack-notes-pp-cli auth login --browser chrome
```

Supported browsers: `chrome`, `brave`, and `arc` on macOS Chromium-family profiles. This uses the user's own signed-in Substack session and saves the cookie header locally with restricted file permissions.

If browser discovery cannot read the profile or keychain, run `substack-notes-pp-cli auth setup` for the advanced manual fallback:

```bash
export SUBSTACK_NOTES_COOKIE_AUTH="<cookie-header>"
```

Run `substack-notes-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  substack-notes-pp-cli notes recent --limit 5 --agent --select id,body,canonical_url
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
substack-notes-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
substack-notes-pp-cli feedback --stdin < notes.txt
substack-notes-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/substack-notes-pp-cli/feedback.jsonl`. They are never POSTed unless `SUBSTACK_NOTES_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SUBSTACK_NOTES_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
substack-notes-pp-cli profile save briefing --json
substack-notes-pp-cli --profile briefing notes recent --limit 5 --json
substack-notes-pp-cli profile list --json
substack-notes-pp-cli profile show briefing
substack-notes-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `substack-notes-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add substack-notes-pp-mcp -- substack-notes-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which substack-notes-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
   - For "post this with image", prefer `notes post --text ... --image <path>` over raw `comment publish-note`.
   - For credential checks, prefer read-only `notes recent --limit 5 --json` before any write.
3. Execute with the `--agent` flag:
   ```bash
   substack-notes-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `substack-notes-pp-cli <command> --help`.
