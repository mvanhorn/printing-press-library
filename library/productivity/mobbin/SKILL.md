---
name: pp-mobbin
description: "Every Mobbin screen, flow, and pattern — searchable offline, deckable in one command, with longitudinal drift no Mobbin tool ships. Trigger phrases: `show me paywall examples`, `build a design deck for <pattern>`, `what does <app> ship for onboarding`, `find empty states across web fintech`, `cross-platform parity for <pattern>`, `use mobbin`, `run mobbin`."
author: "Darin Kishore"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - mobbin-pp-cli
    install:
      - kind: go
        bins: [mobbin-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/mobbin/cmd/mobbin-pp-cli
---

# Mobbin — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `mobbin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install mobbin --cli-only
   ```
2. Verify: `mobbin-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/mobbin/cmd/mobbin-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Wraps Mobbin's curated library of shipped UI (web + mobile) with a local SQLite mirror, full-res batch downloads from the Bytescale CDN, and time-windowed audits across apps. Built for the Wednesday design crit and the quarterly onboarding audit. Uses your existing Chrome session via `auth login --chrome` — no extra API key, no paid MCP.

## When to Use This CLI

Use this CLI when an agent needs to compose design-research workflows that go beyond "search and show." Reach for it for: batch downloading screens to a folder for downstream tooling; building cross-app pattern decks; running time-windowed audits ("every web SaaS onboarding from the last quarter"); or watching a competitor app for flow drift. It is intentionally web-first but iOS works on every command. The official Mobbin MCP exposes one search tool; this CLI exposes the full Mobbin surface plus six commands no Mobbin tool ships.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Design crit workflows
- **`deck`** — Build a design-crit deck for a pattern across an industry: searches matching screens, downloads full-res images from the Bytescale CDN, packages a zip with a manifest CSV.

  _Replaces the 20-tab screenshot-and-rename loop designers do every crit week with one command and a reproducible artifact._

  ```bash
  mobbin-pp-cli deck "fintech paywalls" --platform web --limit 20 --export-zip ./deck.zip --agent
  ```
- **`bench`** — Cross-app leaderboard for any pattern from the local store: count, last seen, top apps that ship it.

  _Answers "who has shipped this pattern lately?" instantly without re-scrolling Mobbin's UI._

  ```bash
  mobbin-pp-cli bench --pattern paywall --industry fintech --platform web --agent
  ```
- **`grab`** — Batch-download matching screens at 1920px from Bytescale with deterministic filenames and a manifest.json side-car for downstream tooling.

  _Replaces hand-dragging 20 PNGs into Finder. The manifest.json is what Figma/Storyboard plugins consume._

  ```bash
  mobbin-pp-cli grab --pattern empty-state --platform web --industry fintech --out ./refs --rename '{app}_{pattern}_{idx}.png'
  ```
- **`cross`** — Fan out a pattern query across web AND iOS for one app set; join results on app slug; output a side-by-side parity manifest.

  _Web product designers checking whether the iOS reference still matches the desktop pattern. The user's explicit dual-platform ask._

  ```bash
  mobbin-pp-cli cross "paywall" --apps stripe,linear,figma --agent
  ```

### Longitudinal analysis
- **`audit`** — Time-windowed flow audit across an industry: app, flow name, step count, captured_at — with --since support for delta-since-last-quarter reports.

  _Quarterly onboarding/checkout/empty-state audits stop being a manual diff against last quarter's Notion doc._

  ```bash
  mobbin-pp-cli audit onboarding --platform web --industry b2b-saas --since 60d --agent --select app,flow,step_count,captured_at
  ```
- **`drift`** — Diff an app's flows + screens between local snapshots; surface what changed (added/removed/screen count).

  _Tracks competitor product evolution. "What did Stripe ship since last month?" was unanswerable before._

  ```bash
  mobbin-pp-cli drift stripe-web --since 30d --agent
  ```

## Command Reference

**apps** — Browse and search apps in Mobbin's library (web, iOS, Android).

- `mobbin-pp-cli apps discover` — Paginated discover-page apps. Tabs are latest / popular / animations.
- `mobbin-pp-cli apps list` — List every app for a platform. Returns a flat array of {id, appName, platform, ...}.
- `mobbin-pp-cli apps popular` — Popular apps grouped by category with preview screenshots.
- `mobbin-pp-cli apps search` — Authenticated app search with category filters.

**autocomplete** — Cross-entity autocomplete search across apps, screens, and flows.

- `mobbin-pp-cli autocomplete` — Fast autocomplete across apps, screens, and flows. Returns matching IDs grouped by relevance.

**collections** — Manage your saved Mobbin collections (decks of screens, flows, or apps). Read endpoints hit mobbin.com/api; write endpoints hit Supabase PostgREST directly (auth-token + apikey).

- `mobbin-pp-cli collections add-app` — Add an app to a collection.
- `mobbin-pp-cli collections add-flow` — Add a flow to a collection.
- `mobbin-pp-cli collections add-screen` — Add a screen to a collection.
- `mobbin-pp-cli collections contents` — Items inside a collection, paginated. Bucketed by --content-type and --platform-type.
- `mobbin-pp-cli collections create` — Create a new collection in the authenticated user's workspace.
- `mobbin-pp-cli collections delete` — Delete a collection. Uses PostgREST filter ?id=eq.<id>.
- `mobbin-pp-cli collections list` — All collections owned by the authenticated user.
- `mobbin-pp-cli collections remove-screen` — Remove a screen from a collection.

**filters** — Browse the filter taxonomy — every app category, screen pattern, UI element, and flow action with definitions and content counts.

- `mobbin-pp-cli filters` — Full filter taxonomy.

**flows** — Search and browse user-flow recordings (onboarding, checkout, settings).

- `mobbin-pp-cli flows` — Cross-app flow search. Filter by --flow-actions like 'creating-account' or 'subscribing'. Requires Mobbin Pro session.

**screens** — Search and inspect individual screens. Filter by pattern, element, OCR keywords, and app category.

- `mobbin-pp-cli screens` — Cross-app screen search. Use --screen-patterns 'paywall' or --screen-elements 'search-bar' to filter. Requires a Mobbin Pro session (`auth login --chrome`).

**sites** — Web sites in Mobbin's library (the web-app equivalent of `apps` for mobile).

- `mobbin-pp-cli sites` — Full searchable-sites list for the web experience.

**trending** — Trending entities updated daily by Mobbin's editorial team.

- `mobbin-pp-cli trending apps` — Trending apps for a platform — what's hot this week on Mobbin.
- `mobbin-pp-cli trending filter-tags` — Trending filter tags — patterns, elements, or categories users search for right now.
- `mobbin-pp-cli trending keywords` — Trending OCR keywords found inside screenshots — what text-in-screenshots users are searching.
- `mobbin-pp-cli trending sites` — Trending web sites (web-only surface).

**workspaces** — List your Mobbin workspaces. The default workspace is required to create collections.

- `mobbin-pp-cli workspaces` — All workspaces the authenticated user belongs to.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
mobbin-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Wednesday paywall deck

```bash
mobbin-pp-cli deck "fintech paywalls" --platform web --limit 20 --export-zip ./paywall-deck-$(date +%Y%m%d).zip
```

Date-stamped zip with full-res PNGs and a manifest CSV — drag straight into Figma for the design crit.

### Cross-platform pattern parity

```bash
mobbin-pp-cli cross "empty-state" --apps stripe,linear,figma --agent --select app,platform,screen_id
```

Web + iOS in one fan-out; the join on app slug is what makes parity comparisons cheap.

### Quarterly onboarding audit

```bash
mobbin-pp-cli audit onboarding --platform web --industry b2b-saas --since 90d --agent --select app,flow,step_count,captured_at
```

Returns only flows captured in the last 90 days, grouped by app — drop into a CSV diff to show your PM what's changed.

### Narrow a verbose screen response

```bash
mobbin-pp-cli screens --platform web --screen-patterns paywall --agent --select data.id,data.appName,data.imageUrl,data.screenPatterns
```

Screen-search payloads ship ~30 fields per row; `--select` with dotted paths keeps only the four agents actually need, cutting context burn ~85%.

### Bench a pattern

```bash
mobbin-pp-cli bench --pattern checkout --industry e-commerce --platform web --agent
```

Local-SQL aggregate: count of screens per app, latest captured_at. Answers "who's shipping checkout flows lately?" without re-scrolling Mobbin.

## Auth Setup

Mobbin has no public API key. The CLI imports your logged-in Chrome cookies via pycookiecheat, cookies, or cookie-scoop-cli (one is required). Run `mobbin-pp-cli auth login --chrome` and the CLI handles the split Supabase JWT cookies (`sb-ujasntkfphywizsdaapi-auth-token.0` / `.1`) and refreshes them automatically. Free-tier sessions work for trending / discover / popular / filters; full content search and per-app HTML scraping need a Mobbin Pro session.

Run `mobbin-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  mobbin-pp-cli apps list mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `MOBBIN_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `MOBBIN_CONFIG_DIR`, `MOBBIN_DATA_DIR`, `MOBBIN_STATE_DIR`, `MOBBIN_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `MOBBIN_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `mobbin-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "mobbin": {
        "command": "mobbin-pp-mcp",
        "env": {
          "MOBBIN_HOME": "/srv/mobbin"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `MOBBIN_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `MOBBIN_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
mobbin-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
mobbin-pp-cli feedback --stdin < notes.txt
mobbin-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `MOBBIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MOBBIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
mobbin-pp-cli profile save briefing --json
mobbin-pp-cli --profile briefing apps list mock-value
mobbin-pp-cli profile list --json
mobbin-pp-cli profile show briefing
mobbin-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `mobbin-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/mobbin/cmd/mobbin-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add mobbin-pp-mcp -- mobbin-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which mobbin-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   mobbin-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `mobbin-pp-cli <command> --help`.
