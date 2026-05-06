---
name: pp-agent-capture
description: "macOS screen capture, window recording, GIF conversion, and agent evidence bundles from the terminal. Built on ScreenCaptureKit for window-level targeting ffmpeg cannot do. Use when the user wants a screenshot of a specific window or app, a screen recording, a GIF conversion, a before/after diff, an evidence bundle for a PR, OCR text from a window, a terminal VHS recording, a Remotion render, or wants to watch a UI for changes. Requires macOS Screen Recording permission on first run."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - agent-capture-pp-cli
    install:
      - kind: go
        bins: [agent-capture-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture/cmd/agent-capture-pp-cli
---

# Agent Capture - Printing Press CLI

Consolidates macOS screen capture, window recording, GIF conversion, frame stitching, and styled code screenshots into one agent-native CLI. Built on ScreenCaptureKit for window-level targeting that ffmpeg alone cannot do. Every command supports `--json` for machine parsing.

## When to Use This CLI

Reach for this when the user wants:

- screenshot a specific window or app (`screenshot`, `batch` for multiple)
- record video of a window, app, display, or region (`record`)
- convert a recording to an optimized GIF (`convert`)
- do a full capture + record + GIF pipeline in one command (`pipeline`)
- diff against a baseline screenshot (`diff`) for before/after evidence
- bundle screenshots + recording + GIF as evidence for a PR or bug report (`evidence`)
- find the right window by fuzzy-matching its title (`find`)
- stitch multiple screenshots into an animated GIF (`stitch`)
- extract text from a window using macOS Vision OCR (`ocr`)
- record a terminal session via VHS tape files (`vhs`)
- render Remotion compositions to video or stills (`remotion`)
- monitor a UI by periodic screenshots (`watch`)
- save and replay capture configs (`preset`)

Skip it on non-macOS hosts; the CLI uses ScreenCaptureKit (macOS only). On first run it will prompt for Screen Recording permission; the `permissions` command guides that flow.

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** -> show `agent-capture --help`
2. **Starts with `install`** -> CLI installation (no MCP server ships today)
3. **Anything else** -> Direct Use (map to the best command and run it)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+).
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture/cmd/agent-capture-pp-cli@latest
   ```
3. Verify: `agent-capture --version`.
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Grant macOS Screen Recording permission:
   ```bash
   agent-capture permissions
   ```
   The CLI opens System Settings -> Privacy & Security -> Screen & System Audio Recording and walks the grant. The first capture after granting permission succeeds; the process does not need a restart.
6. Verify: `agent-capture health` (machine-readable) or `agent-capture doctor`.

## Direct Use

1. Check installed: `which agent-capture`. If missing, offer CLI installation.
2. If permissions aren't granted, run `agent-capture permissions` first.
3. Use `agent-capture list` to see available capture targets (open windows, displays).
4. Use `agent-capture find <text>` to fuzzy-match a window title before capturing.
5. Execute with `--json` for structured output (agent-native default):
   ```bash
   agent-capture <command> [args] --json
   ```

## Notable Commands

| Command | What it does |
|---------|--------------|
| `screenshot` | Capture a window, app, display, or region |
| `record` | Record video of a window, app, display, or region |
| `pipeline` | Record + convert + optimize in one command |
| `convert` | Video -> optimized GIF (two-pass palette) |
| `diff` | Capture + diff against a baseline |
| `evidence` | Full bundle (screenshots + recording + GIF) for a PR |
| `batch` | Screenshot multiple apps in one invocation |
| `find` | Fuzzy search open window titles |
| `list` | List available capture targets |
| `ocr` | Extract text from a window using macOS Vision |
| `stitch` | Combine screenshots into an animated GIF |
| `vhs` | Run a VHS tape file for terminal recording |
| `remotion` | Render Remotion compositions |
| `watch` | Periodic capture for UI monitoring |
| `preset` | Save / load capture configs |
| `permissions` | Guide Screen Recording permission setup |
| `health` | Machine-readable CI / agent preflight |

Run any command with `--help` for full flag documentation.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields, with dotted-path support (see below)
- **Previewable** — `--dry-run` shows the request without sending
- **Cacheable** — GET responses cached for 5 minutes, bypass with `--no-cache`
- **Non-interactive** — never prompts, every input is a flag


### Filtering output

`--select` accepts dotted paths to descend into nested responses; arrays traverse element-wise:

```bash
<cli>-pp-cli <command> --agent --select id,name
<cli>-pp-cli <command> --agent --select items.id,items.owner.name
```

Use this to narrow huge payloads to the fields you actually need — critical for deeply nested API responses.


## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Target not found (no matching window or display) |
| 4 | Permissions missing (Screen Recording not granted) |
| 5 | Capture error (ScreenCaptureKit failure, ffmpeg failure) |
