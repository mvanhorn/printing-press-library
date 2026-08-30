---
name: kuma-operations
description: Use when investigating or safely modifying Uptime Kuma v2 monitors.
---

# Uptime Kuma operations

## Prerequisites: Install the CLI

This skill drives the `kuma-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install kuma --cli-only
   ```
2. Verify: `kuma-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/kuma/cmd/kuma-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.


Use `kuma-pp-cli` for authenticated Kuma v2 inspection. Credentials come from `UPTIME_KUMA_URL`, `UPTIME_KUMA_USERNAME`, and `UPTIME_KUMA_PASSWORD`. `UPTIME_KUMA_URL` must be the server origin (for example `https://kuma.example.com`), not a dashboard page URL.

## Discovering the command surface

Run `kuma-pp-cli agent-context --pretty` to get machine-readable JSON describing every command, flag, and auth variable. Prefer this over parsing `--help`. Commands annotated `mcp:read-only` are safe to run unattended; `set-retries` is annotated `mcp:destructive`.

## Investigation

1. Run `kuma-pp-cli health`.
2. Run `kuma-pp-cli monitors --json` and identify the exact monitor ID.
3. Run `kuma-pp-cli incident-context --monitor <id> --json`.
4. Use `kuma-pp-cli heartbeats --monitor-id <id> --hours 3 --json` for the timeline.

## Mutations

`set-retries` is dry-run by default. Review the proposed complete monitor update, then repeat the exact command with `--yes` only when explicitly authorized. Do not invent monitor IDs or alter notification settings as part of an unrelated retry change.
