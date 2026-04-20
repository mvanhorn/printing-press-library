---
name: pp-agent-capture
description: "Printing Press CLI for agent-capture. Record, screenshot, and convert macOS windows and screens for AI agent evidence Capabilities include: batch, convert, diff, evidence, find, health, list, ocr, permissions, pipeline, preset, record, remotion, screenshot, stitch, vhs, watch. Trigger phrases: 'install agent-capture', 'use agent-capture', 'run agent-capture', 'agent-capture commands', 'setup agent-capture'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["agent-capture"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture/cmd/agent-capture@latest","bins":["agent-capture"],"label":"Install via go install"}]}}'
---

# agent-capture — Printing Press CLI

Record, screenshot, and convert macOS windows and screens for AI agent evidence

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `agent-capture --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture/cmd/agent-capture@latest
   ```

   If `@latest` installs a stale build (the Go module proxy cache can lag the repo by hours after a fresh merge), install from main directly:
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/developer-tools/agent-capture/cmd/agent-capture@main
   ```
3. Verify: `agent-capture --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## Direct Use

1. Check if installed: `which agent-capture`
   If not found, offer to install (see CLI Installation above).
2. Discover commands: `agent-capture --help`
   Key commands:
   - `batch` — Screenshot multiple apps in one command
   - `convert` — Convert video to optimized GIF with two-pass palette generation
   - `diff` — Capture and diff against a baseline screenshot to highlight changes
   - `evidence` — Capture a complete evidence bundle: screenshots + recording + GIF
   - `find` — Fuzzy search window titles to find the right capture target
   - `health` — Machine-readable health check for CI and agent preflight
   - `list` — List available capture targets (windows, displays)
   - `ocr` — Extract text from a window or image using macOS Vision framework
   - `permissions` — Check and guide Screen Recording permission setup
   - `pipeline` — Record, convert, and optimize in one command (no intermediate files)
   - `preset` — Save and load capture configuration presets
   - `record` — Record video of a window, app, display, or region
   - `remotion` — Render Remotion compositions as video or still frames
   - `screenshot` — Capture a screenshot of a window, app, display, or region
   - `stitch` — Stitch multiple screenshots into an animated GIF
   - `vhs` — Run a VHS tape file and produce a terminal recording GIF
   - `watch` — Take screenshots at regular intervals for monitoring UI changes
3. Match the user query to the best command. Drill into subcommand help if needed: `agent-capture <command> --help`
4. Execute with the `--agent` flag:
   ```bash
   agent-capture <command> [subcommand] [args] --agent
   ```
5. The `--agent` flag sets `--json --compact --no-input --no-color --yes` for structured, token-efficient output.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
