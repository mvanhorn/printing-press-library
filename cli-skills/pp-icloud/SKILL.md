---
name: icloud-pp-cli
description: "Query your iCloud data from the command line — Photos library storage analysis, largest-file finder, and delete via AppleScript. macOS only. No network calls or iCloud API token required."
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
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/icloud/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# iCloud — CLI Skill

## Prerequisites: Install the CLI

This skill drives the `icloud-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install icloud --cli-only
   ```
2. Verify: `icloud-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/icloud/cmd/icloud-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Pre-flight Check

Always run `doctor` first to confirm your setup:

```bash
icloud-pp-cli doctor
```

Verifies: macOS, Photos.app installed, library path found, database schema valid, asset count queryable.

If your Photos library is in a non-default location:

```bash
icloud-pp-cli doctor --library "/Volumes/External/Photos Library.photoslibrary/database/Photos.sqlite"
```

## Command Reference

**photos** — Query and manage your Photos library.

- `icloud-pp-cli photos stats` — Quick summary: total items and total library size.
- `icloud-pp-cli photos storage` — Storage breakdown by media type (photo/video) and by year.
- `icloud-pp-cli photos top` — Top heaviest files across all media types.
- `icloud-pp-cli photos videos` — List your largest videos sorted by file size.
- `icloud-pp-cli photos delete <uuid...>` — Move items to Recently Deleted in Photos.app (requires `--confirm`).
- `icloud-pp-cli photos download [uuid...] --output <dir>` — Export originals from iCloud to a local folder. Photos.app downloads from iCloud automatically if Optimize Mac Storage is enabled.
- `icloud-pp-cli photos download --sensitive --confirm --output <dir>` — Export items Apple's on-device ML has flagged as containing nudity (`--confirm` required).

**doctor** — Run pre-flight checks before using any other command.

## Agent Mode

Add `--agent` to any command. Expands to `--json --compact --no-color`.

```bash
icloud-pp-cli photos top --agent
icloud-pp-cli photos storage --agent | jq '.by_year'
icloud-pp-cli photos stats --agent
```

Output is always JSON on stdout with no color. Pipe-friendly — commands also auto-detect pipes and switch to JSON without `--agent`.

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
