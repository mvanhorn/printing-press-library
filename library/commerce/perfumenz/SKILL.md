---
name: pp-perfumenz
description: "The full authentic Perfume NZ catalog, with powerful note-based search and offline intelligence no website offers. Trigger phrases: `use perfumenz`, `search perfumes by notes`, `perfume nz catalog`, `find fragrances with vanilla and oud`, `best value perfume nz`, `similar to rayhaan wolf`."
author: "Jan Medina"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - perfumenz-pp-cli
    install:
      - kind: go
        bins: [perfumenz-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/perfumenz/cmd/perfumenz-pp-cli
---

# Perfume NZ — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `perfumenz-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install perfumenz --cli-only
   ```
2. Verify: `perfumenz-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/perfumenz/cmd/perfumenz-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Perfume NZ (perfumenz.co.nz) is New Zealand's largest wholesale distributor of genuine designer and niche fragrances. This CLI syncs their public catalog into a local SQLite store with parsed Top/Heart/Base notes, then gives you filters, FTS, and novel features (note overlap, similarity, price-per-ml, discovery sets) that the Shopify frontend simply cannot provide.

## When to Use This CLI

Use this CLI when you want to treat the Perfume NZ catalog as a queryable database: complex note combinations, value comparisons, similarity, building discovery sets, or scripting/agents that need structured fragrance data offline.

## Anti-triggers

Do not use this CLI for:
- buying testers or grey-market bottles
- real-time cart/checkout (the CLI is read + local cache only)
- looking for official brand APIs or wholesale portals (this is the public retail face)

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Note intelligence

- **`search --notes`** — Find perfumes matching specific Top/Heart/Base notes (with exclude).

  _Website faceted search is weak on note combinations; this is the main reason power users will install the CLI._

  ```bash
  perfumenz search --notes "vanilla,oud" --exclude "patchouli" --max-price 120 --json
  ```
- **`similar`** — List perfumes with overlapping note profiles to a given one.

  _Agents and users want 'more like this but cheaper or fresher' — the site has no equivalent._

  ```bash
  perfumenz similar "wolf-by-rayhaan-100ml-edp" --limit 8 --json
  ```

### Value & stock intelligence

- **`value`** — Rank current stock by price per ml (or per 100ml) with filters.

  _Real NZ buyers care about value; this turns the catalog into a shopping optimizer._

  ```bash
  perfumenz value --notes "citrus,woody" --gender unisex --sort ppm --json
  ```

### Catalog intelligence

- **`stats notes`** — Show the most common notes across in-stock items (overall or filtered).

  _See what accords are actually available right now from an authentic NZ source._

  ```bash
  perfumenz stats notes --gender mens --limit 15 --json
  ```

### Agent & shopping workflows

- **`recommend`** — Build a small set of perfumes that together cover requested notes under a budget.

  _The killer workflow: 'give me a 5-perfume discovery box that hits these notes without breaking the bank'._

  ```bash
  perfumenz recommend --notes "vanilla,cedar,ginger" --budget 350 --count 5 --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**collections** — Manage collections

- `perfumenz-pp-cli collections` — List collections

**products** — Manage products

- `perfumenz-pp-cli products <handle>` — Get single product by handle

**products-json** — Manage products json

- `perfumenz-pp-cli products-json` — List all perfumes from public feed


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
perfumenz-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Fresh summer unisex under $100 with citrus top

```bash
perfumenz search --notes "lemon,grapefruit,mint" --gender unisex --max-price 100 --json
```

Combines note filters + gender + price in one structured query that the website search does not support at this precision.

### Best value woody scents right now

```bash
perfumenz value --notes "sandalwood,cedar,woody" --sort ppm --limit 10
```

Computes price-per-ml on the synced catalog and ranks — pure website cannot do this dynamically.

### Agent building a discovery box

```bash
perfumenz recommend --notes "vanilla,oud,spicy" --budget 400 --count 5 --json
```

The coverage + budget recommendation the enthusiast has always wanted; uses the local parsed notes + prices.

## Auth Setup

No authentication required.

Run `perfumenz-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  perfumenz-pp-cli collections --agent --select id,name,status
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
perfumenz-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
perfumenz-pp-cli feedback --stdin < notes.txt
perfumenz-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/perfumenz-pp-cli/feedback.jsonl`. They are never POSTed unless `PERFUMENZ_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PERFUMENZ_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
perfumenz-pp-cli profile save briefing --json
perfumenz-pp-cli --profile briefing collections
perfumenz-pp-cli profile list --json
perfumenz-pp-cli profile show briefing
perfumenz-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `perfumenz-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/perfumenz/cmd/perfumenz-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add perfumenz-pp-mcp -- perfumenz-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which perfumenz-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   perfumenz-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `perfumenz-pp-cli <command> --help`.
