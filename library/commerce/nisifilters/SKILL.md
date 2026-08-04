---
name: pp-nisifilters
description: "A read-only, offline-first mirror and search engine for the public NiSi Italia filter catalog and content. Trigger phrases: `search nisifilters`, `find a NiSi filter`, `list NiSi products`, `what's new on nisifilters`, `read the NiSi academy post`, `use nisifilters`, `run nisifilters`."
author: "chiotas"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - nisifilters-pp-cli
    install:
      - kind: go
        bins: [nisifilters-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/nisifilters/cmd/nisifilters-pp-cli
---

# NiSi Italia — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `nisifilters-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install nisifilters --cli-only
   ```
2. Verify: `nisifilters-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/nisifilters/cmd/nisifilters-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Sync every public product, category, attribute, post, page, and media item into a local SQLite database, then search and browse it offline with agent-clean output. Includes a filter-finder (filters) that surfaces available sizes/types, an enriched priced shop view (shop), a clean content reader (read), and a catalog digest (digest) — all without authentication.

## When to Use This CLI

Use this CLI to read, search, or mirror the public NiSi Italia store and content from the terminal or an agent: the filter catalog (ND, GND, polarizers, holders, kits), product categories and attributes, prices and stock, and Academy posts and pages. It is ideal for offline, scriptable, agent-clean catalog access without scraping the storefront.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to place orders, manage a cart, or check out — it only reads public product listings.
- Do not use it to create, edit, or delete content — it is strictly read-only.
- Do not use it for stores other than nisifilters.it; the base URL is fixed.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local joins that compound
- **`shop`** — List filters joined with their categories — name, price, stock (in / backorder / out) — sorted by price.

  _Use to scan the filter catalog with prices and categories in one view._

  ```bash
  nisifilters-pp-cli shop --sort price --json
  ```
- **`variants`** — List a variable product's WooCommerce variations — variation id, attributes, price, stock — invisible via wp-json/wp/v2.

  _Use to get the exact variation_id and stock for a size (e.g. 82mm vs 95mm adapter ring) before building cart links._

  ```bash
  nisifilters-pp-cli variants 278495 --json
  ```
- **`filters`** — Discover available filter sizes/types from product attributes and find matching products.

  _Reach for this to answer 'which sizes exist' or 'which products are 82mm' without clicking the storefront._

  ```bash
  nisifilters-pp-cli filters --json
  ```
- **`digest`** — Summarize the mirror: product count, price range, in-stock vs out, top categories, and content counts.

  _Use for a one-shot overview of the catalog and site without paging every collection._

  ```bash
  nisifilters-pp-cli digest --json
  ```

### Agent-native plumbing
- **`read`** — Print one post, page, or product as title plus plain-text content, stripping the WordPress envelope.

  _Use to read Academy guides without burning context on WordPress envelope fields._

  ```bash
  nisifilters-pp-cli read 289989
  ```
- **`since`** — List posts and pages modified within a time window, across those types at once.

  _Use to answer 'what is new on the site' without checking each collection separately._

  ```bash
  nisifilters-pp-cli since 30d --json
  ```
- **`image`** — Resolve the featured image of a post or page, or a product's inline images, to full-resolution URLs.

  _Use when you have a content item or product and need its actual image URL._

  ```bash
  nisifilters-pp-cli image 289989 --type posts --json
  ```

## Command Reference

**authors** — Public authors

- `nisifilters-pp-cli authors get` — Get a single author by numeric ID
- `nisifilters-pp-cli authors list` — List public authors

**categories** — Post categories

- `nisifilters-pp-cli categories get` — Get a single category by numeric ID
- `nisifilters-pp-cli categories list` — List post categories

**comments** — Public comments

- `nisifilters-pp-cli comments get` — Get a single comment by numeric ID
- `nisifilters-pp-cli comments list` — List approved public comments

**find** — Live global content search across the whole site (wp/v2 /search)

- `nisifilters-pp-cli find` — Search all public content live

**media** — Media library (images / attachments)

- `nisifilters-pp-cli media get` — Get a single media item by numeric ID (resolves source_url)
- `nisifilters-pp-cli media list` — List media library items

**pages** — Public site pages

- `nisifilters-pp-cli pages get` — Get a single page by numeric ID
- `nisifilters-pp-cli pages list` — List published pages

**posts** — Public blog / Academy posts

- `nisifilters-pp-cli posts get` — Get a single post by numeric ID
- `nisifilters-pp-cli posts list` — List published posts

**product_categories** — WooCommerce product categories — public Store API

- `nisifilters-pp-cli product-categories` — List product categories

**products** — WooCommerce shop products (NiSi filters, holders, kits) — public Store API

- `nisifilters-pp-cli products get` — Get a single product by numeric ID
- `nisifilters-pp-cli products list` — List shop products

**tags** — Post tags

- `nisifilters-pp-cli tags get` — Get a single tag by numeric ID
- `nisifilters-pp-cli tags list` — List post tags


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
nisifilters-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Mirror then search offline

```bash
nisifilters-pp-cli sync && nisifilters-pp-cli search "ND1000"
```

Sync once, then full-text search the catalog and content with no further network calls.

### Cheapest filters first

```bash
nisifilters-pp-cli shop --sort price --limit 10 --json
```

List the ten lowest-priced products joined with their categories and stock.

### Pick the right size variant before ordering

```bash
nisifilters-pp-cli variants 278495 --json
```

Variable products hide their sizes behind one parent id; this lists every variation with its own id, price, and stock — including backorder items that are still orderable.

### Narrow a verbose product to a few fields

```bash
nisifilters-pp-cli products list --per-page 5 --agent --select id,name,prices.price,categories.name
```

WooCommerce products are large; --select with dotted paths returns only the fields you need.

### Discover available filter sizes

```bash
nisifilters-pp-cli filters --json
```

Surface the product attributes (e.g. Dimensione) and their values from the local mirror.

### What changed this month

```bash
nisifilters-pp-cli since 30d
```

Show posts and pages modified in the last 30 days.

## Auth Setup

No authentication required.

Run `nisifilters-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  nisifilters-pp-cli authors list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
nisifilters-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
nisifilters-pp-cli feedback --stdin < notes.txt
nisifilters-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/nisifilters-pp-cli/feedback.jsonl`. They are never POSTed unless `NISIFILTERS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NISIFILTERS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
nisifilters-pp-cli profile save briefing --json
nisifilters-pp-cli --profile briefing authors list
nisifilters-pp-cli profile list --json
nisifilters-pp-cli profile show briefing
nisifilters-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `nisifilters-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/nisifilters/cmd/nisifilters-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add nisifilters-pp-mcp -- nisifilters-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which nisifilters-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   nisifilters-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `nisifilters-pp-cli <command> --help`.
