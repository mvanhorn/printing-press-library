---
name: pp-creativefabrica
description: "Search Creative Fabrica's 20-million-plus fonts, graphics, crafts Trigger phrases: `search creative fabrica for svg`, `find POD fonts on creative fabrica`, `creative fabrica free graphics`, `profile a creative fabrica designer`, `what's new on creative fabrica`, `use creativefabrica`, `run creativefabrica`."
author: "Vincent Colombo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - creativefabrica-pp-cli
    install:
      - kind: go
        bins: [creativefabrica-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/cmd/creativefabrica-pp-cli
---

# Creative Fabrica — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `creativefabrica-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install creativefabrica --cli-only
   ```
2. Verify: `creativefabrica-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/cmd/creativefabrica-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Creative Fabrica has no official API; its site search is powered by a public Algolia index, and this CLI queries that same index directly over plain HTTPS (no browser, no Cloudflare at runtime). It adds the filters crafters and print-on-demand sellers actually need: file format (svg/dxf/png/eps), commercial-license (POD), subscription-free, and real discount depth — plus designer profiling and a "what's new since last run" tracker. All agent-native with `--json`, `--csv`, and `--select`.

## When to Use This CLI

Use this CLI when an agent or script needs to search, filter, or profile the Creative Fabrica design-asset catalog — finding fonts, graphics, SVG/DXF cut files, embroidery files, or POD-licensable designs by keyword, type, category, format, price, or designer. It is ideal for print-on-demand sourcing, Cricut/Silhouette project hunting, and tracking specific designers' new releases over time via the local mirror.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to download, purchase, or access files in a user's personal Creative Fabrica library or favorites — v1 is the anonymous public catalog only.
- Do not use this CLI to upload or sell products as a designer.
- Do not use this CLI for Creative Fabrica's AI Studio generation tools.
- Do not use this CLI to redeem the curated 24-hour daily-gifts — it surfaces the permanent free-item catalog, not the daily timed freebies.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Filters the web UI doesn't have
- **`find`** — Narrow craft results to a specific cut/print file format (SVG, DXF, PNG, EPS, PES) that Creative Fabrica has no facet for.

  _Reach for this when a crafter needs a machine-ready format the web UI can't filter on._

  ```bash
  creativefabrica-pp-cli find "mandala" --format svg,dxf --agent
  ```
- **`find`** — Keep only assets usable outside an active subscription.

  _Use this when you need standalone-licensable assets, distinct from free or POD._

  ```bash
  creativefabrica-pp-cli find "valentine svg" --no-subscription --agent
  ```

### Local synthesis
- **`deals`** — Rank on-sale items by their actual regular-to-sale price drop, not just whether they are flagged on sale.

  _Use this to surface the deepest genuine discounts instead of discount-theater._

  ```bash
  creativefabrica-pp-cli deals "font bundle" --agent
  ```
- **`designer-stats`** — Profile a designer's catalog: product-type mix, price band, free count, POD count, and newest release.

  _Pick this when you need a one-shot read on a creator instead of scrolling their page._

  ```bash
  creativefabrica-pp-cli designer-stats "DigiArt" --agent
  ```
- **`designer-compare`** — Compare two designers head-to-head: catalog size, type mix, price band, free/POD share, and popularity.

  _Use this for competitor research when sourcing or scouting creators._

  ```bash
  creativefabrica-pp-cli designer-compare "DigiArt" "CraftLab" --agent
  ```

### Local state that compounds
- **`new-since`** — Show only catalog items added since your last sync for a tracked query or designer.

  _Reach for this to track fresh drops without re-scanning the whole catalog._

  ```bash
  creativefabrica-pp-cli new-since "watercolor flowers" --agent
  ```
- **`tags`** — Show the top tags and categories that co-occur with a query, to refine the next search.

  _Use this to discover better refinement terms before a deep search._

  ```bash
  creativefabrica-pp-cli tags "christmas" --agent
  ```

## Command Reference

**products** — Search and browse the Creative Fabrica catalog (Algolia)

- `creativefabrica-pp-cli products` — Low-level Algolia multi-query passthrough (prefer the top-level search/browse/free/pod commands)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
creativefabrica-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Commercial-license cut files

```bash
creativefabrica-pp-cli find "floral monogram" --format svg --pod --limit 25 --csv
```

POD-safe SVG candidates exported as CSV for a sourcing sheet.

### Newest free fonts

```bash
creativefabrica-pp-cli free --type Fonts --sort newest --limit 30 --agent
```

The latest genuinely-free fonts in machine-readable form.

### Deepest real discounts

```bash
creativefabrica-pp-cli deals "bundle" --agent --select name,price,regular_price,discount_pct
```

On-sale items ranked by actual discount depth, narrowed to the price fields.

### Track a designer's drops

```bash
creativefabrica-pp-cli new-since --designer 2880714 --agent
```

Only the items this designer added since your last sync.

### Refine a broad search

```bash
creativefabrica-pp-cli tags "christmas" --select tag,count --agent
```

Top co-occurring tags to sharpen a vague query before a deep search.

## Auth Setup

No authentication required.

Run `creativefabrica-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  creativefabrica-pp-cli products --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
creativefabrica-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
creativefabrica-pp-cli feedback --stdin < notes.txt
creativefabrica-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/creativefabrica-pp-cli/feedback.jsonl`. They are never POSTed unless `CREATIVEFABRICA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CREATIVEFABRICA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
creativefabrica-pp-cli profile save briefing --json
creativefabrica-pp-cli --profile briefing products
creativefabrica-pp-cli profile list --json
creativefabrica-pp-cli profile show briefing
creativefabrica-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `creativefabrica-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/cmd/creativefabrica-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add creativefabrica-pp-mcp -- creativefabrica-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which creativefabrica-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   creativefabrica-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `creativefabrica-pp-cli <command> --help`.
