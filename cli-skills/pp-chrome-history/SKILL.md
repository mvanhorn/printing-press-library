---
name: pp-chrome-history
description: "Query your local Chrome browsing history — search, topic clusters (Chrome's own Journeys), time/productivity reports, session timelines, downloads, and a behavioral profile — all read-only and on-device. Trigger phrases: `what have I been browsing`, `search my chrome history`, `what was that page I saw`, `my browsing journeys / topics`, `what did I download in chrome`, `what sites do I visit most`, `my browsing time report`, `my browsing profile`, `what was I researching last week`, `use chrome-history`, `run chrome-history-pp-cli`."
license: "Apache-2.0"
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/chrome-history/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# pp-chrome-history

## Prerequisites: Install the CLI

This skill drives the `chrome-history-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install chrome-history --cli-only
   ```
2. Verify: `chrome-history-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/cmd/chrome-history-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

`chrome-history-pp-cli` reads your local Chrome history SQLite database, snapshots it to `~/.cache/chrome-history/`, builds an offline full-text index, and answers questions about your browsing. **Read-only, zero network — nothing leaves the machine.** Every query command supports `--json` and `--select`, and the same surface is exposed as MCP tools via `chrome-history-pp-cli mcp` — the query tools are read-only; only `sync` and `archive_enable`/`archive_disable` mutate local state (on-device, never the network). An optional accumulating **archive** (off by default) can retain history that Chrome later prunes or the user clears — see [Archive](#archive-keep-history-that-chrome-would-delete) below.

## When to use

Historical Chrome activity from the History database: "find that page I saw," "what have I been researching," "what topics has Chrome grouped my browsing into," "how much time on X," "what did I download," "have I visited this site before."

## When NOT to use (anti-triggers)

- **Non-macOS** — **macOS only.** The Chrome DB path and Full-Disk-Access model are macOS-specific; Linux/Windows are not yet supported.
- **Live/open tabs** — these are NOT in the History SQLite DB (Chrome keeps them in binary session files). This tool is history, not current tabs.
- **Safari history** — a separate CLI (Safari stores history differently; `chrome-history-pp-cli` only reads Chrome).
- **Bookmarks, passwords, autofill, cookies** — out of scope; this reads the History DB only.

## Categorization (for agents)

The `domains` static category map is coarse; `journeys` exposes Chrome's own (noisy) clusters. For real topic categorization, read the `--json` titles/URLs and infer topics yourself — agent inference beats both the static map and Chrome's clusters (especially for clustering history into a personal vault).

## Setup

Ensure `chrome-history-pp-cli` is on `PATH` (e.g. `go install` it, or symlink the built binary into `~/go/bin`). Then snapshot Chrome's history once (re-run anytime to refresh — Chrome locks its DB, so the tool copies it safely even while Chrome is open):

```bash
chrome-history-pp-cli sync      # build/refresh the local snapshot
chrome-history-pp-cli doctor    # health check; warns if Chrome's DB schema drifts from the tested version
```

If `doctor` reports the Chrome DB is unreadable, grant your terminal Full Disk Access (System Settings → Privacy & Security → Full Disk Access).

You do **not** need to `sync` before answering a question — query commands read the snapshot that already exists. Only `sync` to pull in newer browsing (or the first time, if no snapshot exists yet).

## Archive: keep history that Chrome would delete

By default the snapshot mirrors **Chrome's current** history — when Chrome prunes old visits or the user clears history, those visits leave the snapshot on the next `sync` too. The optional **archive** keeps an accumulating, durable copy so history outlives Chrome's pruning. It is **opt-in and off by default** — a normal "what did I browse" question needs only the snapshot, so don't enable it reflexively.

```bash
chrome-history-pp-cli archive enable     # turn on the durable archive (seeds from the current snapshot)
chrome-history-pp-cli sync --accumulate  # refresh: append new visits into the archive (dedup on url+visit_time)
chrome-history-pp-cli archive status --json
```

- **Once enabled, reads answer from the archive automatically** (it is a superset of the snapshot) — you don't change how you query; `search`/`list`/`domains`/`report`/`heatmap`/`timeline`/`sql` simply see the fuller history. Rich Chrome-only views (`journeys`/`downloads`/`searches`/`dwell`/`graph`/`profile`/`visited`) still read the current snapshot.
- **Keep it fresh** with `sync --accumulate` (appends, never drops).
- **Manage it:** `archive disable` (stop accumulating, keep the file) · `archive clobber` (reset the archive to a fresh current-snapshot baseline) · `archive reset --force [--purge]` (turn mode off and move, or with `--purge` delete, `archive.db`) · `archive vacuum` (compact).

## Key commands

- **Find:** `search <query>` (FTS), `visited <url|domain>`, `topic <name>` (FTS + Journeys merged), `list`, `searches` (your past search terms)
- **Aggregate:** `domains`, `report` (time + productivity buckets), `heatmap`, `profile` (behavioral summary), `dwell` (estimated time-on-site)
- **Reconstruct:** `journeys` (Chrome's own topic clusters), `timeline <date>` (sessionized), `rabbitholes` (distraction drift), `graph` (navigation graph)
- **Data:** `downloads`, `sql "<SELECT…>"` (read-only), `sync`, `doctor`, `version`, `mcp`
- **Archive (durable history, opt-in):** `archive enable|status|disable|clobber|reset|vacuum`, `sync --accumulate`

## Recipes

```bash
# Find a page you half-remember
chrome-history-pp-cli search "github actions cache" --since 30d --limit 20

# What topics has Chrome grouped my browsing into? (its own Journeys clusters)
chrome-history-pp-cli journeys --limit 25

# Everything I browsed about a topic (FTS + clusters merged) — good for feeding a vault/agent
chrome-history-pp-cli topic "model context protocol" --since 90d --json

# Where did my time go this week, and what was productive vs distracting?
chrome-history-pp-cli report --since 7d

# A behavioral snapshot: peak hours, busiest weekday, top domains/searches
chrome-history-pp-cli profile

# Agent-friendly: narrow deeply nested output to just the fields you need
chrome-history-pp-cli journeys --json --select label,page_count --limit 10

# Keep history that Chrome would later prune/clear (opt-in, one-time enable + periodic refresh)
chrome-history-pp-cli archive enable
chrome-history-pp-cli sync --accumulate     # run periodically; reads then see the fuller archive automatically
```

## Agent notes

- Prefer `--json` for parsing and `--select a,b` (dotted paths) to keep responses small.
- Query commands answer from the **existing** local snapshot — you do **not** need to `sync` before reading. Only `sync` to pull in newer browsing (a query on a truly empty store will tell you with `run sync first`). With archive mode on, `sync`/`sync --accumulate` also writes the durable archive, so don't sync just to read.
- `searches`/`downloads`/`journeys` are Chrome-specific; a future Safari CLI reports them as "not available" rather than faking data.
- All data stays local on-device; no browsing data leaves your machine.
