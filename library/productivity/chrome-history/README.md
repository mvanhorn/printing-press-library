# Chrome History CLI

`chrome-history-pp-cli` — every insight from your browsing history, without it ever leaving your machine. The local-first data source that feeds your private AI, vault, and dashboards.

## Platform Support

**macOS only.** The Chrome history DB path (`~/Library/Application Support/Google/Chrome/Default/History`) and the Full-Disk-Access model are macOS-specific. Linux (`~/.config/google-chrome/Default/History`) and Windows (`%LOCALAPPDATA%\Google\Chrome\User Data\Default\History`) use different paths and permission models — **not yet supported.** Cross-platform support requires per-OS path resolution in the source adapter and is tracked as future work.

Install (build) in one line:

```bash
go build -o chrome-history-pp-cli ./cmd/chrome-history-pp-cli
```

## Quick Start

```bash
export XDG_CACHE_HOME=$PWD/.cache
./chrome-history-pp-cli sync
./chrome-history-pp-cli search "github mcp" --since 30d --limit 10
./chrome-history-pp-cli journeys --limit 20
./chrome-history-pp-cli report --since 7d
```

## Unique Features

- `journeys`: surfaces Chrome's own topic clusters.
- `timeline`: reconstructs session-by-session navigation.
- `rabbitholes`: flags drift from productive starts into distracting domains.
- `dwell`: derived engagement estimate when `visit_duration` is sparse.
- `profile`: compact behavioral browsing profile.
- `topic`: merges FTS and journeys context around one theme.

## Agent Usage

Start MCP stdio server:

```bash
./chrome-history-pp-cli mcp
```

All MCP tools are read-only. They mirror the CLI and return JSON by shelling out to the same binary.

JSON/select examples:

```bash
./chrome-history-pp-cli search "model context protocol" --json --limit 5
./chrome-history-pp-cli domains --json --select domain,visit_sum --limit 20
```

### Categorization: prefer agent inference over the static map

`domains` ships a small static domain→category map (Coding/AI/Social/Search/…) for coarse productivity buckets, and `journeys` exposes Chrome's own topic clusters. **For meaningful topic categorization, an agent reading the page titles/URLs yields far better results** than either: the static map leaves niche/domain-specific sites as "Other," and Chrome's clusters are noisy (they fixate on one-off sessions and miss dominant themes). Treat `domains`/`journeys` as *signals*; let the agent infer the actual topics/projects from the `--json` output. (Use case: clustering history into a personal knowledge vault — the agent maps each page to the user's real projects, which a static map cannot.)

## Health Check

```bash
./chrome-history-pp-cli doctor --json
```

`doctor` reports source/snapshot health, row/index counts, and schema-version drift warning if detected schema is older than supported or newer than tested. On macOS, if Chrome DB is blocked, grant your terminal Full Disk Access.

## Troubleshooting

- DB appears locked: this tool always `cp` snapshots first, so Chrome can stay open.
- `run sync first`: create/update snapshot with `./chrome-history-pp-cli sync`.
- macOS permissions: grant Full Disk Access to your terminal.
- schema-version drift: warning in `doctor` is non-fatal; runtime feature detection still guards command behavior.

## Cookbook

1. Recent coding visits only:

```bash
./chrome-history-pp-cli list --since 14d --transition typed --limit 30
```

2. Search-term recall by domain:

```bash
./chrome-history-pp-cli searches --since 30d --domain github.com --json --limit 25
```

3. Feed top domains into `jq`:

```bash
./chrome-history-pp-cli domains --json --select domain,visit_sum --limit 50 | jq '.[] | {domain: .domain, visits: .visit_sum}'
```

4. Vault/agent topic context export:

```bash
./chrome-history-pp-cli topic "fountain pens" --since 90d --json --limit 100
```

5. Weekly engagement estimate:

```bash
./chrome-history-pp-cli dwell --since 7d --gap 30m --json --limit 25
```

6. Downloads audit:

```bash
./chrome-history-pp-cli downloads --since 30d --json --limit 50
```

## Privacy

- Zero network behavior in normal CLI usage.
- Single local binary.
- Reads Chrome DB snapshot locally and never transmits browsing data.
