---
name: pp-makerworld
description: "Every MakerWorld model, searchable offline — plus trend deltas, designer-watch Trigger phrases: `search makerworld for`, `find a 3d model of`, `what's trending on makerworld`, `models by this designer`, `download this makerworld model`, `use makerworld`, `run makerworld`."
author: "Vincent Colombo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - makerworld-pp-cli
    install:
      - kind: go
        bins: [makerworld-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/makerworld/cmd/makerworld-pp-cli
---

# MakerWorld — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `makerworld-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install makerworld --cli-only
   ```
2. Verify: `makerworld-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/makerworld/cmd/makerworld-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

A fast, agent-native CLI over MakerWorld's public catalog. Browse, search, and inspect 3D models from the terminal, mirror them into a local SQLite database for offline full-text search, then run queries the platform never exposes: what is newly rising (movers), which tracked designers shipped (designers deltas), and which models match a precise tag combination (tags). Reads need no account; an optional token unlocks 3MF downloads and your favorites.

## When to Use This CLI

Use this CLI when an agent or script needs MakerWorld catalog data as structured JSON: discovering printable models by quality or printer fit, tracking designers and trends over time, or resolving a model to its downloadable 3MF. It shines for repeated, queryable access where the web UI's per-visit browsing and opaque ranking fall short.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to slice models or generate G-code — it fetches catalog data and 3MF files, not slicer output.
- Do not use it to control or monitor a physical Bambu printer (MQTT/print jobs) — it is catalog-only.
- Do not use it to publish, edit, or moderate your own designs — it is read-and-download oriented.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`discover`** — Find models that are both highly rated and printable on your setup, ranked by a composite quality score from your local mirror; constrain with --printable, --no-ams, --material, or --max-weight.

  _Reach for this when an agent needs models filtered by real quality and printer fit, not just popularity._

  ```bash
  makerworld-pp-cli discover --sort quality --no-ams --limit 20 --agent
  ```
- **`designers deltas`** — See which tracked designers posted new models or climbed in rank since your last sync.

  _Use this to answer 'what changed for the creators I follow' without re-scanning every profile._

  ```bash
  makerworld-pp-cli designers deltas --limit 50 --agent
  ```
- **`movers`** — Rank models by the biggest jump in likes, downloads, or prints between your two most recent syncs.

  _Pick this over 'browse --nav Trending' when the question is what is rising right now, not what is popular overall._

  ```bash
  makerworld-pp-cli movers --metric downloads --limit 25 --agent
  ```

### Service-specific structure
- **`tags`** — List the most common tags across your synced catalog, or find models matching ALL of several tags at once.

  _Use this to narrow to models matching a precise combination of tags, not just one._

  ```bash
  makerworld-pp-cli tags toy fidget --limit 15 --agent
  ```

## Command Reference

**categories** — List MakerWorld navigation categories

- `makerworld-pp-cli categories` — List navigation categories and their keys

**designers** — Look up a designer's published models

- `makerworld-pp-cli designers` — List a designer's published designs by their numeric user ID

**designs** — Browse, search, and inspect MakerWorld designs (3D models)

- `makerworld-pp-cli designs get` — Get full detail for one design (instances, creator, tags, counts)
- `makerworld-pp-cli designs list` — List designs by navigation category (Trending, For You, or a category key)
- `makerworld-pp-cli designs ratings` — List comments and star ratings for a design
- `makerworld-pp-cli designs recommend` — List recommended-for-you designs
- `makerworld-pp-cli designs related` — List designs related to a given design
- `makerworld-pp-cli designs remixes` — List designs that are remixes of a given design
- `makerworld-pp-cli designs search` — Keyword-search the live MakerWorld catalog


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
makerworld-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Inspect a model's printer requirements (deeply nested — narrow with --select)

```bash
makerworld-pp-cli designs get 2865269 --agent --select title,downloadCount,instances.weight,instances.needAms,instances.materialCnt
```

Pull just the fields that decide whether a model fits your printer, instead of the full multi-KB design payload.

### Find newly-rising household models

```bash
makerworld-pp-cli movers --metric downloads --limit 15 --agent
```

Rank models by download growth between your two most recent syncs.

### Track what your favorite designers shipped

```bash
makerworld-pp-cli designers deltas --limit 30 --agent
```

Roll up new uploads and rank rises across all synced designers.

### Only models printable without AMS

```bash
makerworld-pp-cli discover keychain --printable --no-ams --limit 20 --agent
```

Constrain discovery to single-material models that print on a base machine (no AMS).

## Auth Setup

Browsing, search, and model details work with no credentials against Bambu Lab's public API. To download 3MF files or read your favorites, set MAKERWORLD_TOKEN to a Bambu Cloud token (the same JWT the Bambu Handy app uses).

Run `makerworld-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  makerworld-pp-cli categories --agent --select id,name,status
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

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
makerworld-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
makerworld-pp-cli feedback --stdin < notes.txt
makerworld-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/makerworld-pp-cli/feedback.jsonl`. They are never POSTed unless `MAKERWORLD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MAKERWORLD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
makerworld-pp-cli profile save briefing --json
makerworld-pp-cli --profile briefing categories
makerworld-pp-cli profile list --json
makerworld-pp-cli profile show briefing
makerworld-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `makerworld-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/makerworld/cmd/makerworld-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add makerworld-pp-mcp -- makerworld-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which makerworld-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   makerworld-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `makerworld-pp-cli <command> --help`.
