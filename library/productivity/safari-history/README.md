# Safari History CLI

`safari-history-pp-cli` — a local-first Safari history CLI. It reads `~/Library/Safari/History.db`, snapshots to `~/.cache/safari-history/`, builds an offline FTS index, and answers queries with zero network usage.

## Platform Support

**macOS only** — and macOS-exclusive by nature: Safari does not exist on Linux or Windows, so there is no equivalent history DB to read there. Requires macOS Full Disk Access to read `~/Library/Safari/History.db` (see Access & Permissions below).

## Build

```bash
go build -o safari-history-pp-cli ./cmd/safari-history-pp-cli
```

## Access & Permissions

- Safari DB path: `~/Library/Safari/History.db`.
- macOS **Full Disk Access** is required for your terminal (System Settings → Privacy & Security → Full Disk Access). Without it, `sync` exits with code 4 (`safari db not found`).
- Snapshot-first design: `sync` copies the live DB to a temp snapshot before indexing, so it never writes to Safari's own files.
- No API key, no auth, no network — everything runs against the local snapshot.

## Quick Start

```bash
./safari-history-pp-cli sync                                   # snapshot + build FTS
./safari-history-pp-cli doctor                                 # confirm db + snapshot health
./safari-history-pp-cli search "github mcp" --since 30d --limit 10
./safari-history-pp-cli report --since 7d                      # activity summary
```

## Output Formats

Every read command supports machine-readable output:

```bash
./safari-history-pp-cli domains --since 30d --json             # JSON (default for non-TTY)
./safari-history-pp-cli domains --since 30d --csv              # CSV
./safari-history-pp-cli domains --since 30d --select domain,visit_sum
./safari-history-pp-cli report --since 7d --compact            # compact high-gravity output
```

## Useful Commands

```bash
./safari-history-pp-cli list --since 14d --limit 30
./safari-history-pp-cli domains --since 30d --json --limit 20
./safari-history-pp-cli visited github.com
./safari-history-pp-cli timeline --since 7d
./safari-history-pp-cli profile --since 30d --json
./safari-history-pp-cli topic "fountain pens" --since 90d
./safari-history-pp-cli sql "SELECT url, title FROM urls ORDER BY visit_count DESC LIMIT 10"
```

## iCloud Tabs

`icloud-tabs` lists synced iCloud tabs — the open tabs from your **other** Apple devices (iPhone, iPad, …) — read directly from Safari's `CloudTabs.db`. This is a separate datastore from `History.db`, so `icloud-tabs` does **not** require `sync`.

```bash
./safari-history-pp-cli icloud-tabs --json                    # all synced tabs (no silent cap)
./safari-history-pp-cli icloud-tabs --summary                 # deterministic per-device tab counts
./safari-history-pp-cli icloud-tabs --device-name iPhone      # filter to one device (substring match)
./safari-history-pp-cli icloud-tabs --pinned                  # only pinned tabs
./safari-history-pp-cli icloud-tabs --refresh --wait 5        # activate Safari + wait, then read freshest tabs
```

- **Freshness — Safari must be open for the latest tabs.** `CloudTabs.db` only updates **while Safari is running**, so a pure read can show Safari's last-synced tab set when Safari is closed. Without `--refresh`, the CLI detects a closed Safari and prints a stderr warning that tabs may be stale; `--refresh` **opens Safari for you** (via `osascript`) and waits `--wait` seconds (default 5) for iCloud to sync before reading.
- **No silent cap:** all tabs are returned by default; the root `--limit` is only applied when you pass it explicitly.
- **Exit code:** `4` if `CloudTabs.db` is absent (iCloud Tabs not enabled on your other devices, or this terminal lacks Full Disk Access).
- The MCP tool exposes the read view (`--summary`, `--device-name`, `--pinned`, `--limit`); `--refresh` is intentionally **not** exposed over MCP because it brings Safari to the foreground.

## Accumulating Archive

By default `sync` mirrors only what Safari currently retains — and Safari prunes old history over time. The
**opt-in accumulating archive** keeps a durable `archive.db` that grows across syncs, so your history outlives
Safari's pruning. It's additive and off until you enable it; plain `sync` is unchanged.

```bash
./safari-history-pp-cli archive enable                  # seed the archive from the current snapshot + turn it on
./safari-history-pp-cli sync --accumulate               # sync, then append new visits into the archive (deduped)
./safari-history-pp-cli archive status                  # enabled? baseline date, url/visit counts, size
./safari-history-pp-cli archive disable                 # stop accumulating (keeps the file)
./safari-history-pp-cli archive clobber --force         # rebuild the archive from the current snapshot (guarded)
./safari-history-pp-cli archive reset --force [--purge] # disable + move (or --purge delete) archive.db
./safari-history-pp-cli archive vacuum                  # compact archive.db
```

- **Active store (no flags to remember):** when archive mode is on, the history-faithful commands — `list`,
  `search`, `domains`, `report`, `heatmap`, `timeline`, `sql` — automatically read the archive; the
  richer commands (`dwell`, `graph`, `journeys`, `profile`, …) keep reading the current snapshot.
- **Semantics:** in archive mode, `visit_count` reflects visits the archive has accumulated, not Safari's live
  per-page count; `visited` referrer chains read the live snapshot because archive rowid remapping cannot preserve
  redirect lineage.
- **`clobber` and `reset` are guarded:** without `--force` each prints what it *would* destroy and touches nothing.
- **MCP:** `archive_status` (read-only) + `archive_enable`/`archive_disable` are exposed, and `sync` gains an
  `accumulate` option. The destructive `clobber`/`reset`/`vacuum` are CLI-only (human-gated).

## Capability Notes

`searches`, `downloads`, and `journeys` are intentionally unavailable for Safari because `History.db` does not store those datasets — the commands exist for cross-browser parity and report `not available` rather than erroring. `graph` and `rabbitholes` depend on referrer/transition data that Safari records sparsely, so results may be thin.

## Agent Usage

- All read commands carry the `mcp:read-only` annotation, so MCP hosts can call them without a write-permission prompt.
- Typed exit codes: `0` success, `2` usage error, `3` no snapshot yet (run `sync`), `4` Safari DB not found (grant Full Disk Access).
- `agent-context` prints the machine-readable command tree (schema, commands, flags) for agents and tooling.

### Categorization: prefer agent inference over the static map

`domains` ships a small static domain→category map for coarse productivity buckets. **For meaningful topic categorization, an agent reading the page titles/URLs from `--json` output yields far better results** — the static map leaves niche/domain-specific sites as "Other." (Safari has no `journeys` clusters, so agent inference is the only path to real topics here.) Treat `domains` as a coarse signal; let the agent infer the actual topics/projects.

## MCP Server

```bash
./safari-history-pp-cli mcp
```

All tools are read-only and shell out to the same binary with `--json`. This includes `icloud-tabs`
(synced tabs from your other Apple devices); its `--refresh` flag — the only side effect, since it
activates Safari — is intentionally **not** exposed over MCP, so the MCP surface stays purely read-only.

## Troubleshooting

| Symptom | Exit code | Fix |
| --- | --- | --- |
| `safari db not found` | 4 | Grant your terminal Full Disk Access, then re-run `sync`. |
| `icloud-tabs` reports iCloud Tabs not found | 4 | Enable iCloud Tabs on your other devices (iPhone: Settings → Apps → Safari → iCloud Tabs) and grant this terminal Full Disk Access. |
| `icloud-tabs` returns stale tabs | 0 | The CLI warns when Safari is closed and CloudTabs may be showing the last-synced set; run with `--refresh` to open Safari and read current tabs. |
| `run sync first` | 3 | Run `./safari-history-pp-cli sync` to build the snapshot. |
| `searches`/`downloads`/`journeys` say "not available" | 0 | Expected — Safari's `History.db` does not store these datasets. |
| Empty results for a recent window | 0 | Widen `--since`, or re-run `sync` to refresh the snapshot. |
