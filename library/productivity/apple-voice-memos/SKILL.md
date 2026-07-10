---
name: pp-apple-voice-memos
description: Use apple-voice-memos-pp-cli to refresh, list, search, export, and extract embedded transcripts from Apple Voice Memos on macOS.
tags: [apple, voice-memos, macos, transcription, cli, printing-press]
---

# pp-apple-voice-memos

## Prerequisites: Install the CLI

This skill drives the `apple-voice-memos-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install apple-voice-memos --cli-only
   ```
2. Verify: `apple-voice-memos-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/apple-voice-memos/cmd/apple-voice-memos-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Use this skill when the user asks to work with Apple Voice Memos recordings on macOS. Verify local store access before handling private recording data:

```bash
apple-voice-memos-pp-cli doctor --agent
```

## Safe defaults

- Reads local macOS Voice Memos data only.
- Makes no application-level network requests.
- Opens Apple’s database read-only and query-only.
- Never modifies or deletes recordings.
- `recent` refreshes through `voicememod` by default. Use `--cached` only when stale local data is explicitly acceptable.
- `list` is cached by default. Use `list --fresh` when current iCloud state matters.
- The sync fallback launches Voice Memos hidden and cleans up only when a single new process retains the expected PID, executable, and process-start identity.
- `export` copies one selected recording media file to a destination directory with private file permissions.
- Prefer `--agent` for machine-readable output.

## Common commands

```bash
apple-voice-memos-pp-cli doctor --agent
apple-voice-memos-pp-cli sync --agent
apple-voice-memos-pp-cli recent --limit 10 --agent
apple-voice-memos-pp-cli recent --cached --limit 10 --agent
apple-voice-memos-pp-cli list --fresh --search "keyword" --agent
apple-voice-memos-pp-cli transcript <id> --agent
apple-voice-memos-pp-cli export <id> --out ~/Downloads --agent
```

## Operational guidance

- When the user says “latest” or “recent,” do not add `--cached`.
- If refresh fails, report the failure rather than silently presenting cached records as current.
- Do not expose titles, transcripts, UUIDs, or paths beyond what the user requested.
- Do not attach a database or recording to bug reports.
- `transcript` extracts Apple’s embedded ISO-BMFF `tsrp` transcript from recording media such as `.m4a` or `.qta`. If it is absent, report that honestly. Use another STT tool only when the user asks for transcription.
