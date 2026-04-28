---
name: pp-twitter
description: "Printing Press CLI for Twitter. Twitter OpenAPI(Swagger) specification"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["twitter-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/twitter/cmd/twitter-pp-cli@latest","bins":["twitter-pp-cli"],"label":"Install via go install"}]}}'
---

# Twitter — Printing Press CLI

Twitter OpenAPI(Swagger) specification

## Command Reference

**1-1** — Manage 1 1

- `twitter-pp-cli 1-1 get-friends-following-list` — get friends following list
- `twitter-pp-cli 1-1 get-search-typeahead` — get search typeahead
- `twitter-pp-cli 1-1 post-create-friendships` — post create friendships
- `twitter-pp-cli 1-1 post-destroy-friendships` — post destroy friendships

**2** — Manage 2

- `twitter-pp-cli 2` — get search adaptive

**graphql** — Manage graphql


**other** — other

- `twitter-pp-cli other` — This is not an actual endpoint


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
twitter-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Set your API key via environment variable:

```bash
export TWITTER_ACCEPT="<your-key>"
```

Or persist it in `~/.config/twitter-pp-cli/config.toml`.

Run `twitter-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  twitter-pp-cli 1-1 get-friends-following-list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag

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
twitter-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
twitter-pp-cli feedback --stdin < notes.txt
twitter-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.twitter-pp-cli/feedback.jsonl`. They are never POSTed unless `TWITTER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TWITTER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
twitter-pp-cli profile save briefing --json
twitter-pp-cli --profile briefing 1-1 get-friends-following-list
twitter-pp-cli profile list --json
twitter-pp-cli profile show briefing
twitter-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `twitter-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/twitter/cmd/twitter-pp-cli@latest
   ```
3. Verify: `twitter-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/twitter/cmd/twitter-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add twitter-pp-mcp -- twitter-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which twitter-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   twitter-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `twitter-pp-cli <command> --help`.
