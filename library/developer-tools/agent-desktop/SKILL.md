---
name: agent-desktop
description: >
  Desktop automation through the real Rust agent-desktop CLI, published in
  Printing Press through a small bridge. Use when an agent needs to observe,
  inspect, or control native desktop applications with accessibility trees:
  snapshots, refs, clicks, typing, scrolling, windows, notifications,
  screenshots, clipboard, waits, and bundled version-matched skill docs.
author: "Lahfir"
license: "Apache-2.0"
argument-hint: "<agent-desktop command> [args]"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - agent-desktop-pp-cli
    install:
      - kind: go
        bins: [agent-desktop-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/agent-desktop/cmd/agent-desktop-pp-cli
---

# Agent Desktop - Printing Press CLI

`agent-desktop` is a native desktop automation CLI for AI agents. It is not an
agent and does not call an LLM. It exposes the operating system accessibility
tree as structured JSON, assigns snapshot-scoped refs such as `@e1`, and lets
the calling agent perform semantic actions against those refs.

This Printing Press package installs `agent-desktop-pp-cli`, a catalog bridge.
The bridge does not reimplement automation. It installs or delegates to the real
remote `agent-desktop` package from `https://github.com/lahfir/agent-desktop`
and the npm package `agent-desktop`.

## Prerequisites: Install the CLI

This skill drives the `agent-desktop-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install agent-desktop --cli-only
   ```
2. Verify: `agent-desktop-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/agent-desktop/cmd/agent-desktop-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Install the Real agent-desktop CLI

After the bridge is installed, install and verify the real remote desktop
automation CLI:

```bash
agent-desktop-pp-cli install --version latest
agent-desktop-pp-cli doctor
```

The bridge defaults to `agent-desktop@latest`, so future upstream releases are
pulled from the remote package channel instead of being copied into Printing
Press. Use `agent-desktop-pp-cli install --version <version>` when a task needs
a pinned version. Use `agent-desktop-pp-cli install --dry-run` before mutating a
host when you only need to inspect the install command.

## Load Version-Matched Docs

The real binary ships its own agent skills inside the executable. Prefer those
docs before nontrivial desktop work because they match the installed
`agent-desktop` version exactly:

```bash
agent-desktop skills
agent-desktop skills get desktop --full
agent-desktop skills get desktop workflows
agent-desktop skills get desktop commands-observation
agent-desktop skills get desktop commands-interaction
agent-desktop skills get desktop commands-system
agent-desktop skills get desktop macos
agent-desktop skills get ffi --full
```

After `doctor` confirms the real binary is installed, call `agent-desktop`
directly for commands that need their own flags.

## Core Loop

Every useful automation follows the same loop:

```bash
agent-desktop permissions
agent-desktop snapshot --app "App Name" -i
agent-desktop click @e5 --snapshot <snapshot_id>
agent-desktop wait --text "Done" --app "App Name" --timeout 5000
agent-desktop snapshot --app "App Name" -i
```

Rules for agents:

- Snapshot before acting. Do not guess UI state.
- Keep the returned `snapshot_id` and pass it to every ref-consuming command.
- Treat refs as ephemeral. Re-snapshot after UI-changing actions.
- Prefer refs over coordinates. Use `click @e5`, `type @e2`, `check @e4`, or
  `scroll @e1` before `mouse-click --xy`.
- Use `wait --predicate actionable`, `wait --text`, `wait --window`, or
  `wait --notification` instead of blind sleeps.
- On `STALE_REF` or `AMBIGUOUS_TARGET`, take a fresh snapshot and choose a new
  ref. Do not retry the same stale ref.
- Use `--session <id>` when concurrent agents may share latest-snapshot state.
- Use `--trace /tmp/agent-desktop.jsonl` when diagnosing stale refs,
  ambiguity, policy denial, or actionability failures.
- Use `--headed` only when physical input is explicitly required. Ref actions
  are headless and accessibility-first by default.

## JSON Contract

All real `agent-desktop` commands write a JSON envelope on stdout.

Success:

```json
{
  "version": "2.0",
  "ok": true,
  "command": "snapshot",
  "data": {
    "snapshot_id": "s8f3k2p9",
    "ref_count": 14
  }
}
```

Error:

```json
{
  "version": "2.0",
  "ok": false,
  "command": "click",
  "error": {
    "code": "STALE_REF",
    "message": "Element could not be resolved from the requested snapshot",
    "suggestion": "Run snapshot to refresh, then retry with an updated ref"
  }
}
```

Exit codes are `0` for success, `1` for structured runtime errors, and `2` for
argument or parse errors. The bridge `run` command preserves the real binary's
exit code.

## Command Surface

The real CLI exposes 54 commands:

| Category | Commands |
| --- | --- |
| Observation | `snapshot`, `screenshot`, `find`, `get`, `is`, `list-surfaces` |
| Interaction | `click`, `double-click`, `triple-click`, `right-click`, `type`, `set-value`, `clear`, `focus`, `select`, `toggle`, `check`, `uncheck`, `expand`, `collapse` |
| Scroll | `scroll`, `scroll-to` |
| Keyboard | `press`, `key-down`, `key-up` |
| Mouse | `hover`, `drag`, `mouse-move`, `mouse-click`, `mouse-down`, `mouse-up` |
| App/window | `launch`, `close-app`, `list-windows`, `list-apps`, `focus-window`, `resize-window`, `move-window`, `minimize`, `maximize`, `restore` |
| Notifications | `list-notifications`, `dismiss-notification`, `dismiss-all-notifications`, `notification-action` |
| Clipboard | `clipboard-get`, `clipboard-set`, `clipboard-clear` |
| Wait | `wait` |
| System | `status`, `permissions`, `version`, `skills` |
| Batch | `batch` |

Common examples:

```bash
agent-desktop snapshot --app Finder -i --compact
agent-desktop snapshot --skeleton --app Slack -i --compact
agent-desktop snapshot --root @e3 --snapshot <snapshot_id> -i --compact
agent-desktop find --app TextEdit --role button --name Save
agent-desktop get @e3 --snapshot <snapshot_id> --property value
agent-desktop is @e7 --snapshot <snapshot_id> --property checked
agent-desktop click @e5 --snapshot <snapshot_id>
agent-desktop type @e2 --snapshot <snapshot_id> "hello@example.com"
agent-desktop check @e6 --snapshot <snapshot_id>
agent-desktop scroll @e1 --snapshot <snapshot_id> --direction down --amount 3
agent-desktop press cmd+s --app TextEdit
agent-desktop list-windows --app Finder
agent-desktop focus-window --app Finder
agent-desktop clipboard-get
agent-desktop wait --element @e5 --snapshot <snapshot_id> --predicate actionable --timeout 5000
agent-desktop batch '[{"command":"status","args":{}}]' --stop-on-error
```

## Printing Press Bridge Commands

Use these only for catalog, install, diagnosis, and delegation:

```bash
agent-desktop-pp-cli info
agent-desktop-pp-cli install --version latest
agent-desktop-pp-cli install --version latest --dry-run
agent-desktop-pp-cli install --manager bun
agent-desktop-pp-cli doctor
agent-desktop-pp-cli doctor --json
agent-desktop-pp-cli run version
agent-desktop-pp-cli run status
```

After `doctor` confirms the real binary is on `PATH`, agents may call
`agent-desktop` directly. Keep using `agent-desktop-pp-cli run` when the caller
needs a stable Printing Press entry point.

## Platform Notes

macOS is the Phase 1 fully implemented adapter. The launching app, such as
Terminal, iTerm, Codex, or another agent runtime, needs Accessibility permission
for UI automation. Screenshots need Screen Recording permission. Some automation
surfaces may also require Automation permission.

Check and request permissions with:

```bash
agent-desktop permissions
agent-desktop permissions --request
```

If permissions are denied, stop and surface the returned `error.suggestion`.
Do not work around TCC failures with coordinate clicking.
