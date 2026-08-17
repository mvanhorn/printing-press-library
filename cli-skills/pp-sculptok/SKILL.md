---
name: pp-sculptok
description: "The first CLI and MCP for SculptOK: turn a local image into a depth map, printable STL, or 3D model in one command Trigger phrases: `make a depth map from this image`, `convert this photo to an STL`, `generate a 3D relief`, `how many SculptOK credits will this cost`, `check my SculptOK credits`, `use sculptok`, `run sculptok`."
author: "Vincent Colombo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - sculptok-pp-cli
    install:
      - kind: go
        bins: [sculptok-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/sculptok/cmd/sculptok-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/ai/sculptok/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# SculptOK — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `sculptok-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install sculptok --cli-only
   ```
2. Verify: `sculptok-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/sculptok/cmd/sculptok-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

SculptOK's web tool is async and credit-metered: you upload, submit a draw, wait, refresh, then download. This CLI collapses that into one command (generate depthmap/stl/threed/restore) that uploads a local image, previews the credit cost, polls to completion, and saves the results. Every draw, credit event, and upload is mirrored into a local SQLite database so you can search past jobs offline, track spend by kind with analytics, and reconcile credits against produced jobs.

## When to Use This CLI

Use this CLI when an agent or script needs to turn images into depth maps, reliefs, printable STLs, or 3D models via SculptOK without driving the web UI. It is ideal for batch asset pipelines, credit-aware automation, and answering questions about past jobs and credit spend from a local store.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to edit depth-map geometry by hand; SculptOK generation is AI-driven and the output is post-processed in CAD/CAM tools.
- Do not use this CLI for SculptOK web-app-only tools (AI image generator, pixel upscale, DXF/SVG) that are not part of the public api-open API.
- Do not use this CLI to buy credits or manage billing; credit purchase happens in the SculptOK web dashboard.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### One-command generation
- **`generate depthmap`** — Turn a local image into SculptOK depth-map candidates in one command: upload, submit, auto-poll to completion, and download the results.

  _Reach for this whenever a user wants a depth map or relief from an image without babysitting an async job._

  ```bash
  sculptok-pp-cli generate depthmap photo.jpg --style pro --dry-run
  ```
- **`generate stl`** — Convert a local image straight to a printable STL with full thickness, width, invert, and scale control, polled to completion and saved.

  _Use this to go from a photo or logo to a 3D-printable mesh for lithophanes and relief plaques._

  ```bash
  sculptok-pp-cli generate stl logo.png --width-mm 120 --max-thickness 5 --dry-run
  ```
- **`generate threed`** — Generate a 3D model from a local image at basic, standard, or high precision, polled and downloaded automatically.

  _Pick this when the goal is a full 3D model rather than a 2.5D relief._

  ```bash
  sculptok-pp-cli generate threed bust.jpg --hd-fix high --dry-run
  ```

### Credit awareness
- **`cost`** — Estimate exactly how many credits a draw or a whole batch would cost and compare it against your live balance before spending anything.

  _Run this before any paid generation to avoid surprise credit burn, especially for batches and 4k/pro draws._

  ```bash
  sculptok-pp-cli cost depthmap --style pro --draw-hd 4k --batch ./photos
  ```
- **`analytics`** — Aggregate your locally synced credit events by action type to see where credits actually went.

  _Use this to understand historical credit spend; pair it with cost for forward estimates._

  ```bash
  sculptok-pp-cli analytics --type credits --group-by actionType
  ```
- **`reconcile`** — Cross-check synced credit charges against produced draw jobs to surface credits spent with no matching result.

  _Reach for this to audit whether every credit charge produced a usable job._

  ```bash
  sculptok-pp-cli reconcile --db ./sculptok.db
  ```

### Local state that compounds
- **`search`** — Full-text search your local mirror of draws, credit events, and drawings by kind, status, parameters, remarks, or image URL without any API call.

  _Use this to find a past job and the settings that produced a good result._

  ```bash
  sculptok-pp-cli search --type jobs --limit 20 "stl"
  ```

## Command Reference

**credits** — Credit balance and change history (free reads)

- `sculptok-pp-cli credits balance` — Get the current credit balance (free)
- `sculptok-pp-cli credits history` — List credit-change history, newest first (free)

**draw** — Submit and track async draw jobs (each draw costs credits)

- `sculptok-pp-cli draw depthmap` — Submit a depth-map draw. Cost: 10 credits (style=pro 2k 15; style=pro + draw_hd=4k 30).
- `sculptok-pp-cli draw restore` — Submit background-removal and/or HD restoration. Cost: 2 credits. Async: returns promptId.
- `sculptok-pp-cli draw status` — Get the status of a submitted draw by promptId (free). status/currentStep/position and imgRecords on completion.
- `sculptok-pp-cli draw stl` — Submit an image-to-STL job. Cost: 3 credits. Async: returns promptId.
- `sculptok-pp-cli draw threed` — Submit a 3D draw. Cost: 10 credits. Async: returns promptId.

**drawings** — History of generated images/draws (free read)

- `sculptok-pp-cli drawings` — List your past generated images, newest first (free)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
sculptok-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Depth map from a photo, picking lean fields

```bash
sculptok-pp-cli generate depthmap portrait.jpg --style pro --agent --select promptId,status,imgRecords
```

Generates depth-map candidates and returns only the job id, status, and result URLs so an agent does not parse the full job envelope.

### Printable STL with explicit thickness

```bash
sculptok-pp-cli generate stl logo.png --width-mm 120 --min-thickness 1.6 --max-thickness 5 --invert
```

Produces a printable STL with documented thickness and width and inverted grayscale for a recessed relief.

### Clean then carve in one step

```bash
sculptok-pp-cli generate depthmap jewelry.jpg --restore-first --style pro
```

Runs background removal plus HD restoration first, then depth-maps the cleaned image, persisting both jobs.

### Estimate a batch before spending

```bash
sculptok-pp-cli cost stl --batch ./photos
```

Sums the credit cost of converting every image in a folder to STL and compares it to your live balance.

### Find the settings that worked

```bash
sculptok-pp-cli search --type jobs --limit 10 --agent "depthmap"
```

Searches the local job mirror for past depth-map runs so you can reuse the parameters that produced a good carve.

## Auth Setup

SculptOK uses a single API key sent in the apikey header. Create a key in the logged-in SculptOK dashboard (it is shown only once), then set SCULPTOK_API_KEY or run the auth command. Credits are shared with the web app; reads (credits, history, status) are free, while draws cost credits (depth-map 10, pro 2k 15, pro 4k 30; background/HD 2; 3D 10; STL 3).

Run `sculptok-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  sculptok-pp-cli drawings --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
sculptok-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
sculptok-pp-cli feedback --stdin < notes.txt
sculptok-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/sculptok-pp-cli/feedback.jsonl`. They are never POSTed unless `SCULPTOK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SCULPTOK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
sculptok-pp-cli profile save briefing --json
sculptok-pp-cli --profile briefing drawings
sculptok-pp-cli profile list --json
sculptok-pp-cli profile show briefing
sculptok-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `sculptok-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/sculptok/cmd/sculptok-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add sculptok-pp-mcp -- sculptok-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which sculptok-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   sculptok-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `sculptok-pp-cli <command> --help`.
