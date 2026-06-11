---
name: pp-agent-desktop
description: "Printing Press bridge for the Rust agent-desktop desktop automation CLI. Use when an agent needs to discover, install, verify, or delegate to agent-desktop from the Printing Press catalog."
author: "Lahfir"
license: "Apache-2.0"
argument-hint: "<command> [args] | install"
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

## Prerequisites: Install the CLI

This skill drives the `agent-desktop-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install agent-desktop --cli-only
   ```
2. Verify: `agent-desktop-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/agent-desktop/cmd/agent-desktop-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

This Printing Press CLI is a bridge. It does not implement desktop automation itself; it installs or delegates to the real Rust `agent-desktop` package from `https://github.com/lahfir/agent-desktop`.

## When to Use This CLI

Use this skill when an agent needs Printing Press discovery for `agent-desktop`, needs to install the real binary from its remote package, needs to verify whether the binary is already available, or needs a stable pass-through command from the Printing Press catalog.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

- **`install`** - Installs the real `agent-desktop` package from the public npm/GitHub release channel, with explicit version pinning or latest resolution.
- **`doctor`** - Reports whether the real `agent-desktop` binary is on PATH without mutating the host.
- **`run`** - Delegates arguments to the real `agent-desktop` binary and preserves its exit code for agent retry logic.
- **`info`** - Explains that this catalog entry is a bridge to the existing Rust CLI, npm package, and GitHub release assets.

## Command Reference

- `install` installs the real remote package. Use `agent-desktop-pp-cli install --version latest` for the current npm release or `agent-desktop-pp-cli install --dry-run` to inspect the command first.
- `doctor` checks local availability. Use `agent-desktop-pp-cli doctor --json` when structured output is easier to parse.
- `info` prints distribution context for this bridge.
- `run` delegates to the real binary and preserves its exit code. The arguments after `run` belong to the real `agent-desktop` CLI.

## Safe Workflow

1. Verify this wrapper is installed with `agent-desktop-pp-cli --version`.
2. Check the real binary with `agent-desktop-pp-cli doctor`.
3. If the real binary is missing, install it with `agent-desktop-pp-cli install --version latest`.
4. Re-run `agent-desktop-pp-cli doctor`.
5. Delegate real commands through `run` or call `agent-desktop` directly once it is on PATH.

Do not use this wrapper to infer desktop UI state. Use the real `agent-desktop` binary for snapshots, clicks, typing, screenshots, and window operations.
