---
name: pp-mercadolivre
description: "Search Mercado Livre, compare prices and technical specs across listings, and build cotações — offline-persisted and agent-native, for procurement automation. Trigger phrases: `cotação no mercado livre`, `comparar preços mercado livre`, `cheapest mercado livre meeting spec`, `compare specs mercado livre`, `preço mercado livre`, `use mercadolivre`, `run mercadolivre`."
author: "wandreis"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - mercadolivre-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/commerce/mercadolivre/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Mercado Livre — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `mercadolivre-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install mercadolivre --cli-only
   ```
2. Verify: `mercadolivre-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

A buyer-side CLI for procurement/suprimentos: it extracts stable JSON-LD from Mercado Livre search and product pages (clearing the captcha wall with an imported Chrome session), persists prices and spec attributes to a local SQLite store, and adds comparison commands no scraper offers — an aligned spec matrix (compare), cheapest-meeting-spec (cheapest), cross-seller price dispersion (dispersion), and cotação bundles with decision-time provenance.

## When to Use This CLI

Use this CLI when an agent or analyst needs to turn Mercado Livre listings into a procurement decision: searching by term, comparing prices and technical specs across candidate products, finding the cheapest option meeting a spec floor, or assembling a cotação with a price-at-decision audit trail. It is the right tool for suprimentos automation where the official API is gated and manual browsing does not scale.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Procurement comparison
- **`compare`** — Compare N products side-by-side as a normalized attribute matrix with a price row, so 'which meets 220V and >=700W' is one command instead of six browser tabs.

  _Reach for this when an agent must justify a procurement choice across candidate listings with differing spec labels._

  ```bash
  mercadolivre-pp-cli compare MLB51764304 MLB40287816 --diff --agent
  ```
- **`cheapest`** — Find the lowest-priced listing that satisfies attribute floors like voltage and power, evaluated against locally synced spec data.

  _Reach for this to answer 'cheapest that meets spec' without eyeballing mismatched seller attribute sets._

  ```bash
  mercadolivre-pp-cli cheapest --query furadeira --spec "voltagem=220V" --spec "potencia>=700W" --agent
  ```
- **`cotacao`** — Assemble a purchase-request-shaped quotation across chosen products with price, seller, key specs, and captured_at provenance.

  _Reach for this to emit the final cotação document instead of a raw table dump._

  ```bash
  mercadolivre-pp-cli cotacao MLB51764304 MLB40287816 --format md
  ```

### Price intelligence
- **`dispersion`** — Report min/max/median/stddev of prices across the multiple sellers a single Mercado Livre catalog product aggregates.

  _Reach for this to gauge whether a quoted price is competitive against the full seller spread for the same catalog item._

  ```bash
  mercadolivre-pp-cli dispersion MLB51764304 --agent
  ```
- **`price-history`** — Show how a product's price changed across repeated local snapshots, with an optional --since window.

  _Reach for this to catch supplier price creep on recurring consumables before it hits a PO._

  ```bash
  mercadolivre-pp-cli price-history MLB51764304 --since 30d --agent
  ```
- **`stale`** — List products whose newest price snapshot is older than a threshold, so a cotacao is never built on cold prices.

  _Reach for this before finalizing a quote to confirm every price is fresh enough to trust._

  ```bash
  mercadolivre-pp-cli stale --older-than 7d --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 3 API entries from 3 total network entries
- Generation hints: browser_clearance_required

## Command Reference

**autosuggest** — Query autosuggest / expansion (no auth)

- `mercadolivre-pp-cli autosuggest` — Suggested queries for a search term

**categories** — Mercado Livre category metadata and attribute schema (no auth)

- `mercadolivre-pp-cli categories attributes` — Get the attribute schema for a category (which specs exist, value types) to normalize comparison columns
- `mercadolivre-pp-cli categories get` — Get category metadata by id

**discovery** — Map a free-text query to a Mercado Livre category/domain (no auth)

- `mercadolivre-pp-cli discovery` — Resolve a free-text query to domain_id/category_id

**listings** — Search Mercado Livre listings (JSON-LD extraction from the search page)

- `mercadolivre-pp-cli listings <query>` — Search listings by term; extracts the search page JSON-LD @graph (name, brand, price BRL, availability, product URL

**products** — Fetch a catalog product page (JSON-LD Product + technical spec table)

- `mercadolivre-pp-cli products <catalog_id>` — Fetch a catalog product's price, rating, shipping and full technical attribute table


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
mercadolivre-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Discover and persist candidates

```bash
mercadolivre-pp-cli listings "furadeira de impacto" --limit 30
```

Search, extract JSON-LD, and auto-store listings + a price snapshot for offline comparison.

### Cheapest meeting a spec floor

```bash
mercadolivre-pp-cli cheapest --query "furadeira de impacto" --spec "voltagem=220V" --spec "potencia>=700W" --agent
```

Filter synced listings by attribute predicates, then sort by price for the agent.

### Spec matrix, narrowed fields

```bash
mercadolivre-pp-cli compare MLB51764304 MLB40287816 --agent --select products.name,products.price,attributes.voltagem,attributes.potencia
```

Emit only the columns the decision needs from a deeply nested comparison payload.

### Check price spread before quoting

```bash
mercadolivre-pp-cli dispersion MLB51764304
```

See min/max/median across sellers for the catalog product.

### Build the cotacao

```bash
mercadolivre-pp-cli cotacao MLB51764304 MLB40287816 --format md
```

Assemble the purchase-request document with prices, specs, and captured_at provenance.

## Auth Setup

Mercado Livre serves search and product pages behind a captcha wall to plain HTTP clients. Run 'auth login --chrome' to import your logged-in Chrome cookies; the CLI replays requests with a Chrome fingerprint (no resident browser). The no-auth helper endpoints (category attributes, domain discovery, autosuggest) work without any login.

Run `mercadolivre-pp-cli doctor` to verify setup.

## Troubleshooting

- **Requests still hit /captcha/wall even after `auth login --chrome`** — The wall also scores IP reputation. Run the CLI from a host whose outbound traffic exits through a residential IP (for example, an HTTP/SOCKS proxy or VPN whose exit is on a residential connection). Combined with the imported Chrome cookies and the Chrome-fingerprint transport, residential egress clears the wall. Data-center IPs are challenged regardless of cookies.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  mercadolivre-pp-cli categories get mock-value --agent --select id,name,status
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `MERCADOLIVRE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `MERCADOLIVRE_CONFIG_DIR`, `MERCADOLIVRE_DATA_DIR`, `MERCADOLIVRE_STATE_DIR`, `MERCADOLIVRE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `MERCADOLIVRE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `mercadolivre-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "mercadolivre": {
        "command": "mercadolivre-pp-mcp",
        "env": {
          "MERCADOLIVRE_HOME": "/srv/mercadolivre"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `MERCADOLIVRE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `MERCADOLIVRE_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
mercadolivre-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
mercadolivre-pp-cli feedback --stdin < notes.txt
mercadolivre-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `MERCADOLIVRE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MERCADOLIVRE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
mercadolivre-pp-cli profile save briefing --json
mercadolivre-pp-cli --profile briefing categories get mock-value
mercadolivre-pp-cli profile list --json
mercadolivre-pp-cli profile show briefing
mercadolivre-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `mercadolivre-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add mercadolivre-pp-mcp -- mercadolivre-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which mercadolivre-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   mercadolivre-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `mercadolivre-pp-cli <command> --help`.
