---
name: pp-safari-history
description: Query local Safari browsing history with zero network access: search, visit checks, domain/activity reports, timelines, graphs, dwell estimates, and profile summaries. Trigger phrases: "what have I been browsing", "search my safari history", "what was that page I saw", "what sites do I visit most", "my browsing time report", "my browsing profile", "what was I researching last week", "use safari-history", "run safari-history-pp-cli".
---

# pp-safari-history

## Prerequisites: Install the CLI

This skill drives the `safari-history-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install safari-history --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `safari-history-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/safari-history/cmd/safari-history-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to use

Historical Safari browsing activity from `~/Library/Safari/History.db`: recall pages, check if you visited a site, rank domains, and generate time/profile reports.

## Anti-triggers

- **Non-macOS — macOS only.** Safari does not exist on Linux/Windows, so there is no history DB to read there.
- Live/open tabs are not in `History.db`.
- For Chrome history, use `chrome-history-pp-cli`.
- `searches`, `downloads`, and `journeys` are not available for Safari because Safari does not store those datasets in `History.db`.

## Categorization (for agents)

The `domains` static category map is coarse, and Safari has no `journeys` clusters — so for real topic categorization, read the `--json` titles/URLs and infer topics yourself. Agent inference is the only path to vault-quality topics here.

## Setup

```bash
safari-history-pp-cli sync
safari-history-pp-cli doctor
```

If Safari DB access fails, grant terminal Full Disk Access (System Settings -> Privacy & Security -> Full Disk Access).

## Core commands

- Find: `search <query>`, `visited <url|domain>`, `list`, `topic <name>`
- Aggregate: `domains`, `report`, `heatmap`, `profile`, `dwell`
- Reconstruct: `timeline <date>`, `rabbitholes`, `graph`
- Ops: `sync`, `doctor`, `sql "<SELECT...>"`, `mcp`

## Agent notes

- Prefer `--json` and `--select` for compact outputs.
- Run `sync` before analysis or when results are stale.
- Local-first, read-only, zero-network behavior by default.
