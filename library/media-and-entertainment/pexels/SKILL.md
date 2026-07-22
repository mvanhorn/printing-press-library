---
name: pp-pexels
description: "Every Pexels photo, video, and collection endpoint — plus a local store that adds quota forecasting, dedup-aware bulk download, and one-shot attribution exports no other Pexels tool has. Trigger phrases: `search pexels for photos`, `download stock video b-roll`, `get a stock photo of`, `find royalty-free images`, `export photo attribution credits`, `use pexels`, `run pexels`."
author: "Vincent Colombo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pexels-pp-cli
    install:
      - kind: go
        bins: [pexels-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/cmd/pexels-pp-cli
---

# Pexels — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `pexels-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install pexels --cli-only
   ```
2. Verify: `pexels-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/cmd/pexels-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

A single binary for the full Pexels API: search and curated photos, video search and popular feeds, and collections including your own. It mirrors results into a local SQLite store so you can re-search offline, dedup downloads across sessions, forecast your rate budget, pick the best-fit resolution for a target, and export license-compliant attribution in one command.

## When to Use This CLI

Use this CLI when an agent or script needs stock photos or video b-roll from Pexels with deterministic JSON output, license-compliant attribution, and rate-budget awareness. It is the right choice for repeated or bulk media harvesting, re-finding previously pulled media offline, and picking exact resolutions for known media ids.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to upload, like, or modify content on Pexels — the API is strictly read-only.
- Do not use it as a wallpaper app or to replicate Pexels' core browsing experience; Pexels' guidelines forbid that and gate such clients.
- Do not use it for non-Pexels stock sources (Unsplash, Pixabay); it only speaks the Pexels API.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Rate-limit intelligence
- **`quota forecast`** — Check before a bulk pull whether it fits your remaining hourly/monthly Pexels quota, with a reset ETA.

  _Reach for this before any large download to avoid burning the 200/hour budget mid-batch._

  ```bash
  pexels-pp-cli quota forecast --resources photos,videos --max-pages 10 --agent
  ```

### Resolution & download intelligence
- **`resolve`** — Pick the smallest photo size or video file that meets a target dimension without upscaling.

  _Use when you know the media id and need exactly one URL sized for a target, instead of parsing eight src variants._

  ```bash
  pexels-pp-cli resolve 2014422 --target-width 1280 --target-height 720 --agent
  ```
- **`download`** — Bulk-download a query, skipping media already in your ledger and checkpointing each page so a hit quota stops you gracefully.

  _Use for repeated or large harvests that must not re-download files or exceed the rate budget._

  ```bash
  pexels-pp-cli download "mountain lake" --type photo --limit 30 --max-pages 3 --size large --sidecar
  ```

### License compliance
- **`attribution export`** — Emit a SOURCES.md and per-file .meta.json sidecars crediting every photographer in your local download ledger.

  _Reach for this to satisfy Pexels' attribution guideline in one shot rather than hand-building credit lists._

  ```bash
  pexels-pp-cli attribution export --resources photos,videos --csv
  ```

### Local store that compounds
- **`search`** — Full-text search media you already synced, with stable ordering and no API call.

  _Reach for this to re-find media you pulled earlier instead of spending quota on a fresh live search._

  ```bash
  pexels-pp-cli search "sunset" --type photos --limit 10 --agent --select id,photographer,alt
  ```
- **`analytics`** — Group your synced photos by photographer to review credit balance and licensing coverage.

  _Use during a licensing or credit review to see which photographers dominate your pulled set._

  ```bash
  pexels-pp-cli analytics --type photos --group-by photographer --limit 20
  ```

## Command Reference

**collections** — Browse featured collections, your own collections, and collection media

- `pexels-pp-cli collections featured` — List Pexels featured collections
- `pexels-pp-cli collections list` — List the authenticated user's own collections (requires PEXELS_API_KEY)
- `pexels-pp-cli collections media` — List the media (photos and/or videos) inside a collection

**photos** — Search, curate, and fetch Pexels photos

- `pexels-pp-cli photos curated` — Browse the Pexels editor-curated photo feed (use --random for a random page)
- `pexels-pp-cli photos get` — Get a single photo by its numeric id
- `pexels-pp-cli photos search` — Search photos by keyword with orientation/size/color/locale filters

**videos** — Search, browse popular, and fetch Pexels videos

- `pexels-pp-cli videos get` — Get a single video by its numeric id
- `pexels-pp-cli videos popular` — Browse popular videos with optional dimension/duration filters
- `pexels-pp-cli videos search` — Search videos by keyword with orientation/size/locale filters


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pexels-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Agent-native photo search, narrowed

```bash
pexels-pp-cli photos search --query "foggy forest" --orientation portrait --per-page 8 --agent --select photos.id,photos.photographer,photos.src.large2x,photos.alt
```

Returns only the four fields an agent needs from a deeply-nested response, instead of all eight src sizes per photo.

### Forecast then harvest

```bash
pexels-pp-cli quota forecast --resources photos --max-pages 5 && pexels-pp-cli download "city skyline night" --type photo --limit 50 --max-pages 5 --size large --sidecar
```

Check the rate budget, then run a dedup-aware bulk pull that writes attribution sidecars.

### Best-fit resolution for a known id

```bash
pexels-pp-cli resolve 2014422 --target-width 1920 --target-height 1080 --agent
```

Picks the smallest src/video_file that covers 1080p without upscaling.

### Browse your own collections

```bash
pexels-pp-cli collections list --per-page 20 --json
```

Lists the authenticated user's collections — a surface only two ecosystem tools expose.

### Re-find synced media offline

```bash
pexels-pp-cli sync --resources photos --max-pages 3 && pexels-pp-cli search "sunset" --type photos --limit 10
```

Mirror a few pages locally, then full-text search them without spending more quota.

## Auth Setup

Pexels uses a free API key sent as a raw `Authorization: <key>` header — there is NO `Bearer` prefix (the single most common 401 in the ecosystem). Set `PEXELS_API_KEY` or run `pexels-pp-cli auth login`. Every request also sends a User-Agent, since Pexels' edge returns 403 to header-less clients.

Run `pexels-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  pexels-pp-cli collections list --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `PEXELS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `PEXELS_CONFIG_DIR`, `PEXELS_DATA_DIR`, `PEXELS_STATE_DIR`, `PEXELS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `PEXELS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `pexels-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "pexels": {
        "command": "pexels-pp-mcp",
        "env": {
          "PEXELS_HOME": "/srv/pexels"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `PEXELS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `PEXELS_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
pexels-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pexels-pp-cli feedback --stdin < notes.txt
pexels-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `PEXELS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PEXELS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
pexels-pp-cli profile save briefing --json
pexels-pp-cli --profile briefing collections list
pexels-pp-cli profile list --json
pexels-pp-cli profile show briefing
pexels-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `pexels-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/cmd/pexels-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add pexels-pp-mcp -- pexels-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pexels-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   pexels-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `pexels-pp-cli <command> --help`.
