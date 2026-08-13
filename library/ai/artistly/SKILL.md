---
name: pp-artistly
description: "The first CLI for Artistly — scriptable AI image generation, batch runs Trigger phrases: `generate an image with artistly`, `batch generate artistly images`, `search my artistly designs`, `download my artistly art`, `re-run that artistly generation`, `use artistly`, `run artistly`."
author: "Vincent Colombo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - artistly-pp-cli
    install:
      - kind: go
        bins: [artistly-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/artistly/cmd/artistly-pp-cli
---

# Artistly — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `artistly-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install artistly --cli-only
   ```
2. Verify: `artistly-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/artistly/cmd/artistly-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Artistly (app.artistly.ai) is a login-only web app with no public API. This CLI authenticates with your browser session and turns the one-prompt-at-a-time web flow into something an agent or a Makefile can drive: batch-generate from a prompt file, block until images render and download them, search your whole generation history offline, and replay saved style presets — all while respecting the undocumented ~400/day generation cap.

## When to Use This CLI

Use this CLI when you need to drive Artistly image generation from a script or agent: batch-generating many prompt variations, blocking until images render and saving them to disk, searching or re-running past generations, or bulk-exporting finished assets into a deliverable folder. It is the right choice whenever the manual one-prompt web flow is too slow or not scriptable.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for outpaint or inpaint — not implemented in v1 (upscale and background removal ARE supported via `edit upscale` / `edit bg-remove`).
- Do not use this CLI for billing, subscription, or account-management changes — use the Artistly website.
- Do not use this CLI to bypass Artistly's generation limits — it respects the daily cap rather than evading it.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Scriptable generation
- **`batch`** — Generate many images from a file of prompts in one quota-aware run.

  _Reach for this when the user has more than one prompt or a spreadsheet of variations; it is the headline workflow no manual web session can match._

  ```bash
  artistly-pp-cli batch prompts.txt --style watercolor --aspect-ratio 1:1 --wait --download ./out
  ```
- **`generate`** — Submit a generation, block until it renders, and save the images to disk.

  _Use this to make image generation a single blocking step in a script or agent flow instead of a manual click-and-babysit loop._

  ```bash
  artistly-pp-cli generate "a watercolor fox in a misty forest" --wait --download ./out --json
  ```
- **`export`** — Download all images from designs matching a query into a directory tree, with templated filenames — no generation.

  _Use this to assemble a deliverable folder of already-generated assets without re-generating or clicking through downloads._

  ```bash
  artistly-pp-cli export --query "coloring book" --to ./client-assets --name-template '{prompt}-{id}'
  ```
- **`quota`** — Report remaining daily generations and whether a planned batch fits under the cap before you start.

  _Reach for this before a large batch so a quota wall does not silently truncate a run mid-way._

  ```bash
  artistly-pp-cli quota --for prompts.txt --quantity 4 --json
  ```

### Local archive that compounds
- **`search`** — Full-text search your own generation history by prompt, style, folder, or date — entirely offline.

  _Reach for this to find an old design's exact prompt/seed to reuse, instead of scrolling the web gallery._

  ```bash
  artistly-pp-cli search "pirate ship" --status private --json
  ```
- **`redo`** — Resubmit a past design's exact settings (prompt, style, dimensions, seed) with optional overrides.

  _Use this when a user liked a previous result and wants variations or a re-run with one tweak._

  ```bash
  artistly-pp-cli redo 57628105 --seed random --quantity 4 --wait --download ./out
  ```
- **`preset`** — Save a named bundle of generation settings (style, dimensions, aspect ratio, negative prompt, quality) and replay it.

  _Reach for this when a user reuses one 'house style' across many prompts or campaigns._

  ```bash
  artistly-pp-cli preset save house-style --style watercolor --aspect-ratio 1:1 --quality highQuality
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 4 API entries from 91 total network entries
- Protocols: websocket (95% confidence), rest_json (75% confidence)
- Candidate command ideas: create_auth — Derived from observed POST /broadcasting/auth traffic.; list_ai_design_agents — Derived from observed GET /ai/ai-design-agents traffic.; list_fetch_personal_designs — Derived from observed GET /fetch-personal-designs traffic.; list_personal_designs — Derived from observed GET /personal-designs traffic.

## Command Reference

**designs** — Browse and sync your Artistly designs (generations)

- `artistly-pp-cli designs by-folder` — List designs grouped by folder
- `artistly-pp-cli designs list` — List your personal designs (generations)

**checkpoints** — Browse Artistly's checkpoint (model) catalog

- `artistly-pp-cli checkpoints list` — List available checkpoints/models (filter with `--match`)

  Use this to resolve a model name (e.g. "comic") to the integer id that `generate`, `batch`, `redo`, and `preset` expect for `--checkpoint-id`:

  ```bash
  artistly-pp-cli checkpoints list --match comic --json
  ```


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
artistly-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Batch a spreadsheet of prompts and download finals

```bash
artistly-pp-cli batch prompts.csv --preset house-style --wait --download ./finals
```

Reads each row as a prompt (with optional per-row overrides), applies a saved preset, waits for each to render, and saves the images — the Sunday-afternoon merch workflow in one command.

### Find and re-run a past design

```bash
artistly-pp-cli search "axolotl astronaut" --agent --select designs.uuid,designs.positive_prompt,designs.checkpoint && artistly-pp-cli redo <uuid> --quantity 4
```

Search the local mirror for the old design, then resubmit its exact settings for more variations. The --agent --select pair narrows the verbose design records to just the fields you need.

### Bulk-export a client's assets without re-generating

```bash
artistly-pp-cli export --query "logo concept" --to ./client --name-template '{prompt}-{id}'
```

Downloads every finished image whose prompt matches, into a folder with readable filenames — zero quota cost.

### Enhance a prompt before generating

```bash
artistly-pp-cli prompt enhance "a fox in a forest"
```

Expand a short prompt into a rich, detailed one (no quota cost), then feed it to generate. Use prompt extract <design-id|image-url> for the reverse (image-to-prompt).

### Upscale or remove the background of a design

```bash
artistly-pp-cli edit upscale 57628105 --wait --download ./out
```

Run editor image-to-image tools on an existing design (by id/uuid), an image URL, or a local file. Use 'edit bg-remove' for background removal. Both auto-authenticate via your app session.

## Auth Setup

Artistly has no API keys. Authentication is your logged-in browser session: run `artistly-pp-cli auth login --chrome` to import the `artistly_session` cookie and CSRF token from Chrome (be logged in to app.artistly.ai first). Generation and other write commands send the CSRF token and Inertia headers automatically.

Run `artistly-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  artistly-pp-cli designs list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Mutating** — `generate`, `batch`, and `redo` create images (consuming generation quota); `designs delete`, `designs move`, and `folders` change remote state. Destructive commands (`designs delete`, `folders remove`) require `--yes`. Read commands (`designs list`, `search`, `export`, `quota`, `styles`) do not mutate.

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
artistly-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
artistly-pp-cli feedback --stdin < notes.txt
artistly-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/artistly-pp-cli/feedback.jsonl`. They are never POSTed unless `ARTISTLY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ARTISTLY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
artistly-pp-cli profile save briefing --json
artistly-pp-cli --profile briefing designs list
artistly-pp-cli profile list --json
artistly-pp-cli profile show briefing
artistly-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `artistly-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/artistly/cmd/artistly-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add artistly-pp-mcp -- artistly-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which artistly-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   artistly-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `artistly-pp-cli <command> --help`.
