---
name: pp-jsonplaceholder
description: "Printing Press CLI for Jsonplaceholder. Discovered API spec for jsonplaceholder-typicode"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["jsonplaceholder-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/other/jsonplaceholder-pp-cli/cmd/jsonplaceholder-pp-cli@latest","bins":["jsonplaceholder-pp-cli"],"label":"Install via go install"}]}}'
---

# Jsonplaceholder — Printing Press CLI

Discovered API spec for jsonplaceholder-typicode

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 6 API entries from 6 total network entries
- Protocols: rest_json (75% confidence)
- Candidate command ideas: get_posts — Derived from observed GET /posts/{id} traffic.; get_users — Derived from observed GET /users/{id} traffic.; list_albums — Derived from observed GET /albums traffic.; list_comments — Derived from observed GET /comments traffic.; list_posts — Derived from observed GET /posts traffic.; list_users — Derived from observed GET /users traffic.

## Command Reference

**albums** — Operations on albums

- `jsonplaceholder-pp-cli albums list_albums` — GET /albums

**comments** — Operations on comments

- `jsonplaceholder-pp-cli comments list_comments` — GET /comments

**posts** — Operations on posts

- `jsonplaceholder-pp-cli posts get_posts` — GET /posts/{id}
- `jsonplaceholder-pp-cli posts list_posts` — GET /posts

**users** — Operations on users

- `jsonplaceholder-pp-cli users get_users` — GET /users/{id}
- `jsonplaceholder-pp-cli users list_users` — GET /users


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
jsonplaceholder-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `jsonplaceholder-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  jsonplaceholder-pp-cli albums --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
jsonplaceholder-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
jsonplaceholder-pp-cli feedback --stdin < notes.txt
jsonplaceholder-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.jsonplaceholder-pp-cli/feedback.jsonl`. They are never POSTed unless `JSONPLACEHOLDER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `JSONPLACEHOLDER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
jsonplaceholder-pp-cli profile save briefing --json
jsonplaceholder-pp-cli --profile briefing albums
jsonplaceholder-pp-cli profile list --json
jsonplaceholder-pp-cli profile show briefing
jsonplaceholder-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `jsonplaceholder-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.25+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/jsonplaceholder-pp-cli/cmd/jsonplaceholder-pp-cli@latest
   ```
3. Verify: `jsonplaceholder-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/jsonplaceholder-pp-cli/cmd/jsonplaceholder-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add jsonplaceholder-pp-mcp -- jsonplaceholder-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which jsonplaceholder-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   jsonplaceholder-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `jsonplaceholder-pp-cli <command> --help`.
