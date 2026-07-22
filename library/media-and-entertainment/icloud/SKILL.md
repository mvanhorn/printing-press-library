---
name: icloud-pp-cli
description: "Query your Apple iCloud data from the command line — Photos library storage analysis, iMessage history search and export, largest-file finder, and delete via AppleScript. macOS only. No network calls or iCloud API token required."
author: "Matias Sanchez Moises"
license: "Apache-2.0"
argument-hint: "<command> [args] | install"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - icloud-pp-cli
    install:
      - kind: go
        bins: [icloud-pp-cli]
        module: github.com/matysanchez/icloudcli/cmd/icloud-pp-cli
---

# Apple iCloud — CLI Skill

## Prerequisites: Install the CLI

This skill drives the `icloud-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install icloud --cli-only
   ```
2. Verify: `icloud-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/icloud/cmd/icloud-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Pre-flight Check

Always run `doctor` first to confirm your setup:

```bash
icloud-pp-cli doctor
```

Verifies: macOS, Photos.app installed, library path found, database schema valid, asset count queryable, chat.db readable (Full Disk Access).

If your Photos library is in a non-default location:

```bash
icloud-pp-cli doctor --library "/Volumes/External/Photos Library.photoslibrary/database/Photos.sqlite"
```

## Messages: Full Disk Access required

`messages` subcommands read `~/Library/Messages/chat.db` directly, which on
macOS requires Full Disk Access (FDA) for the terminal app invoking the binary.
If a messages command fails with "Full Disk Access not granted", open System
Settings > Privacy & Security > Full Disk Access, add your terminal, quit and
reopen the terminal, and rerun. `doctor` reports FDA state automatically.

## Command Reference

**photos** — Query and manage your Photos library.

- `icloud-pp-cli photos stats` — Quick summary: total items and total library size.
- `icloud-pp-cli photos storage` — Storage breakdown by media type (photo/video) and by year.
- `icloud-pp-cli photos top` — Top heaviest files across all media types.
- `icloud-pp-cli photos videos` — List your largest videos sorted by file size.
- `icloud-pp-cli photos delete <uuid...>` — Move items to Recently Deleted in Photos.app (requires `--confirm`).
- `icloud-pp-cli photos download [uuid...] --output <dir>` — Export originals from iCloud to a local folder. Photos.app downloads from iCloud automatically if Optimize Mac Storage is enabled.
- `icloud-pp-cli photos download --sensitive --confirm --output <dir>` — Export items Apple's on-device ML has flagged as containing nudity (`--confirm` required).

**messages** — Read your iMessage history from `~/Library/Messages/chat.db`. Requires Full Disk Access.

- `icloud-pp-cli messages list-chats` — Chats ordered by most-recent activity. Flags: `--limit`, `--since`, `--include-empty`.
- `icloud-pp-cli messages search <query>` — Full-text search across message bodies (decoded from `attributedBody` when `text` is NULL). Flags: `--chat`, `--handle`, `--from-me`, `--from-others`, `--since`, `--until`, `--limit`.
- `icloud-pp-cli messages stats` — Total messages / chats / handles, by-year breakdown, top N handles. Flags: `--top-handles`, `--include-tapbacks`.
- `icloud-pp-cli messages export --chat <guid|all>` — Export a chat (or every chat) to JSON with attachment paths. Flags: `--out`, `--since`, `--until`, `--include-tapbacks`.

**doctor** — Run pre-flight checks before using any other command.

## Agent Mode

Add `--agent` to any command. Expands to `--json --compact --no-color`.

```bash
icloud-pp-cli photos top --agent
icloud-pp-cli photos storage --agent | jq '.by_year'
icloud-pp-cli photos stats --agent

icloud-pp-cli messages list-chats --limit 10 --agent | jq '[.[] | {chat: .display_name // .chat_identifier, last: .last_message_date}]'
icloud-pp-cli messages search "lunch" --limit 5 --agent
icloud-pp-cli messages stats --agent
icloud-pp-cli messages export --chat <guid> --out /tmp/chat.json
```

Output is always JSON on stdout with no color. Pipe-friendly — commands also auto-detect pipes and switch to JSON without `--agent`. Messages JSON includes a `text_source` field on every row (`decoded`, `text_column`, or `unrecoverable`) so agents can detect coverage gaps from the typedstream decoder.

## Common Workflows

### Find what's eating storage

```bash
# Overview
icloud-pp-cli photos stats

# Breakdown by year and type
icloud-pp-cli photos storage

# Top 25 heaviest files
icloud-pp-cli photos top

# Top 10 largest videos
icloud-pp-cli photos top --limit 10 --type video
```

### Identify and delete large files

```bash
# Get UUIDs of top 5 largest videos
icloud-pp-cli photos top --type video --limit 5 --json | jq -r '.[].uuid'

# Dry run — see what would be deleted
icloud-pp-cli photos delete <uuid>

# Actually move to Recently Deleted
icloud-pp-cli photos delete <uuid> --confirm

# Pipe directly
icloud-pp-cli photos top --type video --limit 5 --json \
  | jq -r '.[].uuid' \
  | xargs icloud-pp-cli photos delete --confirm
```

### Filter by year

```bash
# Videos from 2022
icloud-pp-cli photos videos --year 2022 --json

# Videos from January 2022
icloud-pp-cli photos videos --year 2022 --month 1 --json
```

### Search a chat history for a phrase

```bash
# Recent messages mentioning "lunch"
icloud-pp-cli messages search "lunch" --since 2026-01-01

# Only messages I sent, last 6 months
icloud-pp-cli messages search "thanks" --from-me --since 2025-11-22

# Find a message in a specific chat
icloud-pp-cli messages search "happy birthday" --chat +15551234567
```

### Export a chat for downstream analysis

```bash
# One chat to stdout
icloud-pp-cli messages export --chat +15551234567

# One chat to a file
icloud-pp-cli messages export --chat +15551234567 --out /tmp/family.json

# Every chat in one document (large)
icloud-pp-cli messages export --chat all --out /tmp/all-messages.json
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error |
| 10 | Config error (wrong OS, library not found) |

## Direct Use

1. Check installation: `which icloud-pp-cli`
   If missing, see Prerequisites above.
2. Run doctor: `icloud-pp-cli doctor`
3. Execute with `--agent` for JSON output:
   ```bash
   icloud-pp-cli <command> [flags] --agent
   ```
