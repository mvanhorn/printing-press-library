---
name: pp-safari-history
description: Query local Safari browsing history with zero network access: search, visit checks, domain/activity reports, timelines, graphs, dwell estimates, and profile summaries. Trigger phrases: "what have I been browsing", "search my safari history", "what was that page I saw", "what sites do I visit most", "my browsing time report", "my browsing profile", "what was I researching last week", "use safari-history", "run safari-history-pp-cli".
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/safari-history/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# pp-safari-history

## Prerequisites: Install the CLI

This skill drives the `safari-history-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install safari-history --cli-only
   ```
2. Verify: `safari-history-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

`safari-history-pp-cli` reads Safari's local `History.db`, snapshots it to `~/.cache/safari-history/`, builds an offline full-text index, and answers questions about your browsing. **Read-only, zero network — nothing leaves the machine.** Requires macOS Full Disk Access to read `~/Library/Safari/History.db`. Every command supports `--json` and `--select`, and the same surface is exposed as MCP tools (all read-only) via `safari-history-pp-cli mcp`.

## When to use

Historical Safari browsing activity from `~/Library/Safari/History.db`: recall pages, check if you visited a site, rank domains, and generate time/profile reports.

## Anti-triggers

- **Non-macOS — macOS only.** Safari does not exist on Linux/Windows, so there is no history DB to read there.
- Live/open tabs on *this* Mac are not in `History.db`. (Synced **iCloud tabs** open on your *other* Apple devices ARE available — see `icloud-tabs` below.)
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
- Synced tabs: `icloud-tabs` (open tabs from your other Apple devices)
- Ops: `sync`, `doctor`, `sql "<SELECT...>"`, `mcp`

## iCloud tabs (synced open tabs from other devices)

`icloud-tabs` reads synced iCloud tabs — the open tabs from your *other* Apple devices — directly from Safari's `CloudTabs.db` (separate from `History.db`; does **not** require `sync`).

- `icloud-tabs` — one row per tab (device_name, device_type, title, url, last_viewed_time, is_pinned, is_showing_reader). Returns ALL tabs by default (no silent cap).
- `icloud-tabs --summary` — per-device tab counts; use this for a **deterministic** "N tabs across M devices" total instead of estimating.
- Filters: `--device-name <substring>`, `--pinned`.
- `icloud-tabs --refresh [--wait <secs>]` — **opens Safari** and waits (default 5s) before reading so iCloud syncs the freshest tabs. **`CloudTabs.db` only updates while Safari is running, so use `--refresh` when you need current data**. Default (no `--refresh`) is a pure read with no app side effect; when Safari is closed, the CLI warns on stderr that CloudTabs may be showing the last-synced tab set.
- Exit 4 if `CloudTabs.db` is absent (iCloud Tabs not enabled, or Full Disk Access missing).

## Accumulating archive (history that outlives Safari's pruning)

Safari prunes old history; the **opt-in archive** keeps a durable `archive.db` that accumulates across syncs.
Off by default and additive — plain `sync` is unchanged.

- `archive enable` — seed the archive from the current snapshot and turn it on (sticky).
- `sync --accumulate` — sync, then append new visits into the archive (deduped). Use this instead of plain `sync` once the archive is on, to keep it growing.
- `archive status` — enabled? baseline date, distinct-url + visit counts, size. **Check this to know whether reads come from the archive vs the snapshot.**
- When archive mode is on, the history-faithful commands (`list`/`search`/`domains`/`report`/`heatmap`/`timeline`/`sql`) automatically read the **archive**; the richer commands (`dwell`/`graph`/`journeys`/`profile`/…) read the current snapshot. No flags needed.
- In archive mode, `visit_count` reflects visits the archive has accumulated, not Safari's live per-page count; `visited` referrer chains read the live snapshot because archive rowid remapping cannot preserve redirect lineage.
- `archive disable` (keep file) · `archive clobber --force` (rebuild from snapshot — **guarded**: without `--force` it only prints the destroy plan) · `archive reset --force [--purge]` (guarded — without `--force` it only prints the destroy plan) · `archive vacuum`.
- MCP: `archive_status`/`archive_enable`/`archive_disable` + `sync(accumulate=true)`. Destructive ops are CLI-only.

## Agent notes

- Prefer `--json` and `--select` for compact outputs.
- Do **not** reflexively run `sync` before reads. `search`/`sql`/`domains`/`list`/`report` query the cached snapshot/archive directly; run `sync` only to refresh known-stale results. If `sync` or `doctor` says the live Safari source is missing/inaccessible, that means "can't refresh", not "no data" — query the cached store directly.
- If the accumulating archive is enabled (`archive status`), use `sync --accumulate` only when you need to refresh and keep growing it.
- **`icloud-tabs` is the exception — do NOT `sync` for it.** It reads Safari's `CloudTabs.db` (synced tabs from the user's *other* Apple devices), a separate datastore from `History.db`, so it never needs `sync`. Use `--summary` for a deterministic per-device tab count, and `--refresh` when the user needs the freshest tabs or sees the closed-Safari stale-data warning (it activates Safari — a side effect, CLI-only, not exposed over MCP).
- Local-first, read-only, zero-network behavior by default.
