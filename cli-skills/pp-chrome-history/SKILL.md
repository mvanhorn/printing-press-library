---
name: pp-chrome-history
description: "Query your local Chrome browsing history — search, topic clusters (Chrome's own Journeys), time/productivity reports, session timelines, downloads, and a behavioral profile — all read-only and on-device. Trigger phrases: `what have I been browsing`, `search my chrome history`, `what was that page I saw`, `my browsing journeys / topics`, `what did I download in chrome`, `what sites do I visit most`, `my browsing time report`, `my browsing profile`, `what was I researching last week`, `use chrome-history`, `run chrome-history-pp-cli`."
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/chrome-history/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# pp-chrome-history

## Prerequisites: Install the CLI

This skill drives the `chrome-history-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install chrome-history --cli-only
   ```
2. Verify: `chrome-history-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

`chrome-history-pp-cli` reads your local Chrome history SQLite database, snapshots it to `~/.cache/chrome-history/`, builds an offline full-text index, and answers questions about your browsing. **Read-only, zero network — nothing leaves the machine.** Every command supports `--json` and `--select`, and the same surface is exposed as MCP tools (all read-only) via `chrome-history-pp-cli mcp`.

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

## Key commands

- **Find:** `search <query>` (FTS), `visited <url|domain>`, `topic <name>` (FTS + Journeys merged), `list`, `searches` (your past search terms)
- **Aggregate:** `domains`, `report` (time + productivity buckets), `heatmap`, `profile` (behavioral summary), `dwell` (estimated time-on-site)
- **Reconstruct:** `journeys` (Chrome's own topic clusters), `timeline <date>` (sessionized), `rabbitholes` (distraction drift), `graph` (navigation graph)
- **Data:** `downloads`, `sql "<SELECT…>"` (read-only), `sync`, `doctor`, `version`, `mcp`

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
```

## Agent notes

- Prefer `--json` for parsing and `--select a,b` (dotted paths) to keep responses small.
- All commands are read-only and operate on the local snapshot — run `sync` first (or when data feels stale).
- `searches`/`downloads`/`journeys` are Chrome-specific; a future Safari CLI reports them as "not available" rather than faking data.
- All data stays local on-device; no browsing data leaves your machine.
