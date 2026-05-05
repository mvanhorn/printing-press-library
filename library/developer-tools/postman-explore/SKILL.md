---
name: pp-postman-explore
description: "The CLI for the public API directory at postman.com/explore — search, rank, and watch community-contributed Postman Collections, agent-native and offline. Trigger phrases: `find a postman collection for`, `what postman collection should i fork for`, `is there a postman collection for`, `browse postman api network`, `compare postman publishers`, `what changed on the postman network`, `use postman-explore`, `run postman-explore`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - postman-explore-pp-cli
    install:
      - kind: go
        bins: [postman-explore-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/postman-explore/cmd/postman-explore-pp-cli
---

# Postman Explore — Printing Press CLI

Postman's API Network is the world's largest public API directory, but discovery is web-only and unscriptable. This CLI mirrors the public surface into a local SQLite store with FTS, then layers commands the website cannot offer: canonical-collection lookup, cross-publisher comparison, trend ranking by custom metrics, and drift detection across snapshots. No authentication required; designed for AI agents and power users who need the network as data, not as a webpage.

## When to Use This CLI

Pick this CLI when you need to programmatically discover, compare, or watch community-contributed Postman Collections / workspaces / APIs / flows. Ideal for AI agents asked to find a canonical Postman Collection for a vendor, for API curators comparing publishers in a category, and for monitoring scripts that watch network changes. Skip this CLI for managing your own private Postman workspace — that's the authenticated Postman product API, which is a different surface entirely.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Discovery that compounds locally
- **`canonical`** — One command finds the best community Postman Collection for a vendor, ranked by publisher verification, fork count, and recency.

  _When the user asks an agent for a vendor's Postman Collection, this returns the best canonical choice in one call instead of forcing the agent to dedupe a search result list._

  ```bash
  postman-explore-pp-cli canonical stripe --json
  ```
- **`top`** — Rank entities by any metric (weekForks, monthViewCount, etc.) with category and entity-type narrowing.

  _When agents need to recommend the most-forked collections THIS week, not the all-time leaders, this is the only path that works._

  ```bash
  postman-explore-pp-cli top --metric weekForkCount --type collection --category payments --limit 10
  ```
- **`similar`** — Given an entity numeric id, return collections with overlapping name, summary, and category — ranked by FTS relevance score against the seed entity's text.

  _When an integrator finds one collection but wants to compare against alternatives, this returns the rivals in one call rather than forcing a fresh search._

  ```bash
  postman-explore-pp-cli similar 10289 --limit 5 --json
  ```

### Comparative analysis
- **`publishers top`** — Aggregate fork counts across every entity per publisher within a category; rank teams by total community gravity.

  _API curators picking between vendors need cross-publisher comparison. Agents recommending an integration partner need this to break ties._

  ```bash
  postman-explore-pp-cli publishers top --category developer-productivity --limit 5 --json
  ```
- **`category landscape`** — For a category slug, return: total entity counts per type, top 5 publishers by aggregate fork count, and top 5 entities by viewCount. One command, one structured payload.

  _API curators evaluating a vertical (payments, AI, devops) want one snapshot of who dominates and what's popular; this is exactly that snapshot._

  ```bash
  postman-explore-pp-cli category landscape payments --json
  ```

### Time-windowed signals
- **`drift`** — Compare two synced snapshots and report new entities, removed entities, and entities whose updatedAt advanced.

  _Agents tracking when a vendor publishes a new collection or updates an existing one rely on this; there is no other way to know._

  ```bash
  postman-explore-pp-cli drift --since 7d --type collection --json
  ```
- **`velocity`** — Rank collections by acceleration ratio: (weekForkCount × 4) / monthForkCount. Surfaces collections breaking out before they top the popular list.

  _Catching a breakout collection before it tops the popular sort is high-value for API curators tracking emerging vendors._

  ```bash
  postman-explore-pp-cli velocity --type collection --top 10
  ```

### Quality signals
- **`browse`** — When passed --verified-only, the browse command filters to entities owned by publishers with the verified-team flag set.

  _When you want only the official-vendor collections (not community forks), this is the cleanest filter._

  ```bash
  postman-explore-pp-cli browse collection --verified-only --category payments --json --limit 5
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 17 API entries from 24 total network entries
- Protocols: rest_json (60% confidence), rpc_envelope (95% confidence)
- Auth signals: none
- Generation hints: client_pattern:proxy-envelope, uses_browser_http_transport, no_auth_required, skip_clearance_cookie
- Emitted command surface: browse; networkentity get-network-entity; networkentity get-network-entity-counts; category list-categories; category get; team get-workspaces; team get; search-all

## Command Reference

**category** — Manage category

- `postman-explore-pp-cli category get` — Returns full details for a category by its URL slug (e.g., `artificial-intelligence`, `developer-productivity`,...
- `postman-explore-pp-cli category list-categories` — Returns all categories used to organize the API network (e.g., AI, E-commerce, Communication, DevOps). Categories...

**networkentity** — Manage networkentity

- `postman-explore-pp-cli networkentity get-network-entity` — Returns full entity record by internal numeric id (the `id` field from `listNetworkEntities`, NOT the `entityId` UUID).
- `postman-explore-pp-cli networkentity get-network-entity-counts` — Returns aggregate counts across the entire public API network: collections, workspaces, APIs, flows, notebooks,...
- `postman-explore-pp-cli networkentity list-network-entities` — List public collections, workspaces, APIs, or flows. Supports category filtering, pagination, and minimum-fork...

**search-all** — Manage search all

- `postman-explore-pp-cli search-all` — Search collections, workspaces, requests, flows, and teams by free-text query. With no `queryIndices` set, the...

**team** — Publisher teams on the API network

- `postman-explore-pp-cli team get` — Returns small profile object (id, name, description, publicHandle, profileURL, createdAt, updatedAt) for a team by...
- `postman-explore-pp-cli team get-workspaces` — Returns the array of public workspaces owned by a team identified by its `publicHandle` (e.g., `stripedev`,...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
postman-explore-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find Stripe's official collection

```bash
postman-explore-pp-cli canonical stripe --json --select id,name,publisher.publicHandle,metrics.forkCount
```

The canonical command picks the verified-publisher, highest-fork-count match and narrows the JSON to the four fields agents actually need.

### Compare top payments publishers

```bash
postman-explore-pp-cli publishers top --category payments --limit 5 --json
```

Aggregate fork counts across each publisher's full portfolio in the payments category.

### What changed on the network this week

```bash
postman-explore-pp-cli sync && postman-explore-pp-cli drift --since 7d --type collection --json
```

Sync, then diff against the previous snapshot to find newly added or recently updated collections.

### Browse top developer-productivity collections, narrow to verified publishers

```bash
postman-explore-pp-cli browse collection --category 4 --verified-only --limit 10 --json --select name,publisher.publicHandle,metrics.viewCount,redirectURL
```

Use --select with dotted paths to avoid pulling the full ~5 KB metric array per record — keeps output small for agent context.

### Find accelerating collections in payments

```bash
postman-explore-pp-cli velocity --type collection --category payments --top 10
```

Velocity ranks by week-vs-month fork rate, surfacing collections that are breaking out before they top the all-time popular list.

### Find collections similar to a seed

```bash
postman-explore-pp-cli similar 10289 --limit 5 --json
```

More-like-this query against the local FTS index — useful for discovering rival or complementary community collections.

### Get a category-wide snapshot

```bash
postman-explore-pp-cli category landscape payments --json
```

Combines per-type counts, top publishers, and top entities for a category in one structured payload.

## Auth Setup

No authentication required.

Run `postman-explore-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  postman-explore-pp-cli category get mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
postman-explore-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
postman-explore-pp-cli feedback --stdin < notes.txt
postman-explore-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.postman-explore-pp-cli/feedback.jsonl`. They are never POSTed unless `POSTMAN_EXPLORE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `POSTMAN_EXPLORE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
postman-explore-pp-cli profile save briefing --json
postman-explore-pp-cli --profile briefing category get mock-value
postman-explore-pp-cli profile list --json
postman-explore-pp-cli profile show briefing
postman-explore-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `postman-explore-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.25+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/postman-explore/cmd/postman-explore-pp-cli@latest
   ```
3. Verify: `postman-explore-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/postman-explore/cmd/postman-explore-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add postman-explore-pp-mcp -- postman-explore-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which postman-explore-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   postman-explore-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `postman-explore-pp-cli <command> --help`.
