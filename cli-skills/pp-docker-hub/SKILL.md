---
name: pp-docker-hub
description: "Printing Press CLI for Docker Hub. Docker Hub public API. Search container images, browse tags, check sizes, inspect Dockerfiles, and explore the..."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - docker-hub-pp-cli
    install:
      - kind: go
        bins: [docker-hub-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/docker-hub/cmd/docker-hub-pp-cli
---

# Docker Hub — Printing Press CLI

Docker Hub public API. Search container images, browse tags, check sizes,
inspect Dockerfiles, and explore the world's largest container registry.
No authentication required for public repositories (rate limited to ~18 req/min).

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**docker-hub-search** — Manage docker hub search

- `docker-hub-pp-cli docker-hub-search` — Full-text search across all Docker Hub repositories. Returns name, description, stars, and pull counts.

**repositories** — Repository metadata and details

- `docker-hub-pp-cli repositories <namespace> <repository>` — Full metadata for a Docker Hub repository including pull count, stars, description, and last update time.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
docker-hub-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `docker-hub-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  docker-hub-pp-cli docker-hub-search --agent --select id,name,status
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
docker-hub-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
docker-hub-pp-cli feedback --stdin < notes.txt
docker-hub-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.docker-hub-pp-cli/feedback.jsonl`. They are never POSTed unless `DOCKER_HUB_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DOCKER_HUB_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
docker-hub-pp-cli profile save briefing --json
docker-hub-pp-cli --profile briefing docker-hub-search
docker-hub-pp-cli profile list --json
docker-hub-pp-cli profile show briefing
docker-hub-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `docker-hub-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/docker-hub-pp-cli/cmd/docker-hub-pp-cli@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/other/docker-hub-pp-cli/cmd/docker-hub-pp-cli@main
   ```
3. Verify: `docker-hub-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/docker-hub-pp-cli/cmd/docker-hub-pp-mcp@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/other/docker-hub-pp-cli/cmd/docker-hub-pp-mcp@main
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add docker-hub-pp-mcp -- docker-hub-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which docker-hub-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   docker-hub-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `docker-hub-pp-cli <command> --help`.
