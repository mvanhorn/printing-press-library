---
name: pp-vagaro
description: "Every Vagaro discovery feature, plus the marketplace-wide availability search, price comparison, and local database no Vagaro tool has. Trigger phrases: `find a massage near me`, `book a haircut at my barber`, `compare these salons on vagaro`, `is this spa price fair`, `rebook my usual appointment`, `use vagaro`, `run vagaro`."
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - vagaro-pp-cli
    install:
      - kind: go
        bins: [vagaro-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/health/vagaro/cmd/vagaro-pp-cli
---

# Vagaro — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `vagaro-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install vagaro --cli-only
   ```
2. Verify: `vagaro-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/health/vagaro/cmd/vagaro-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Vagaro's website is a per-business click-through funnel with no way to ask a question across the whole marketplace. This CLI syncs businesses, services, providers, reviews, and availability into a local SQLite store, so you can find any open slot matching your constraints (find), compare businesses head to head (compare), check whether a price is fair (price-check), and rebook your usual with the same provider (me rebook) — all with agent-native --json output.

## When to Use This CLI

Use this CLI when a user wants to discover salon/spa/barber/fitness businesses, compare them by price or rating, check availability across many businesses at once, or manage and rebook their own Vagaro appointments from the command line or an agent. It shines at cross-business questions the Vagaro website makes tedious.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for Vagaro Pro / business-owner operations (POS, payroll, staff management) — that is a different, paid API.
- Do not use it to manage payment methods or edit your Vagaro profile — those are intentionally out of scope.
- Do not use it to complete a prepaid/deposit booking — payment happens on Vagaro's site, not here.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`find`** — Find nearby businesses with a service open soonest, filtered by price, rating, and a date/time window.

  _Reach for this when a user wants any open slot matching constraints across many businesses, not a specific known place._

  ```bash
  vagaro-pp-cli find massage --max-price 120 --min-rating 4.5 --from thu --to sat --agent
  ```
- **`compare`** — Compare named businesses side by side: rating, review count, price range, matching-service price, and next-available.

  _Use when the user already has 2-3 businesses in mind and wants a decision table._

  ```bash
  vagaro-pp-cli compare centralbarber rudysbarbershop --agent
  ```
- **`price-check`** — Show the price spread (min/median/max) for a service across a metro and flag who's below median.

  _Use to judge whether a quoted price is fair or to find below-median providers._

  ```bash
  vagaro-pp-cli price-check haircut --city seattle --agent
  ```
- **`market`** — One-shot landscape of a metro: business count, rating distribution, and price ranges by category.

  _Use when someone is new to an area and wants the lay of the land before picking a regular spot._

  ```bash
  vagaro-pp-cli market seattle --agent
  ```
- **`menu-diff`** — Diff a business's service menu across synced snapshots to catch price changes and added/removed services.

  _Use to detect silent price hikes or menu changes at a business you follow._

  ```bash
  vagaro-pp-cli menu-diff centralbarber --agent
  ```

### Booking that remembers you
- **`me rebook`** — Re-run your usual: reads your past appointment (business + service + provider) and lists that provider's open slots in a date window so you can pick a future time.

  _Use when the business/service/provider are already known from history, such as another haircut with the same barber. The variable is the future slot, not provider discovery._

  ```bash
  vagaro-pp-cli me rebook --last --from thu --to sat --agent
  ```
- **`watch`** — Check one business/provider's next-available against a stored baseline and report if a slot opened up sooner.

  _Use when waiting on a booked-out provider to open a sooner slot._

  ```bash
  vagaro-pp-cli watch centralbarber --service haircut --before 2026-07-05 --agent
  ```
- **`business availability`** — Query one known business for available slots, scoped to a service, provider, and date range.

  _Use when the user already knows the business and needs a precise provider/service/window answer._

  ```bash
  vagaro-pp-cli business availability sample-shop --service haircut --provider alex --from 2026-07-20 --to 2026-07-31 --agent
  ```

## Command Reference

**business** — Look up a Vagaro business (salon/spa/barber/fitness) by its slug

- `vagaro-pp-cli business availability` — Get a business's next-available booking summary; supports `--service <name-or-id>`, `--provider <name-or-id>`, `--from <date>`, `--to <date>`, and `--weeks <n>`
- `vagaro-pp-cli business get` — Get a business profile (name, rating, address, categories)
- `vagaro-pp-cli business services` — List a business's services with prices and durations

**classes** — Browse upcoming livestream classes

- `vagaro-pp-cli classes` — List upcoming livestream classes

**listings** — Browse businesses by service and location (live JSON-LD listings)

- `vagaro-pp-cli listings <service> <location>` — List businesses for a service in a city (city--state slug)

**me** — Your own Vagaro account (requires auth login --chrome)

- `vagaro-pp-cli me` — List your appointments (upcoming or past)
- `vagaro-pp-cli me rebook --last --from <date> --to <date>` — Use your most recent past appointment's business/service/provider and list matching future openings

**booking-link / book** — Booking handoff only

- `vagaro-pp-cli booking-link <slug> --service <id> --provider <id> --at <YYYY-MM-DDTHH:MM>` — Verify a selected slot and print the booking handoff details. This does not submit the appointment.
- `book` is an alias for `booking-link`; with `--confirm` it prints the tightest Vagaro booking URL plus exact browser steps for the verified slot, but still does not complete checkout for you.
- Use the business slug from the Vagaro URL, for example `centralbarber` from `https://www.vagaro.com/centralbarber`.

**favorites** — Saved-business discovery

- `vagaro-pp-cli favorites` — Tries to read saved/favorite businesses from the authenticated account. Use this when the user forgot which business they saved; do not use it for routine repeat appointments where `me rebook` can infer the business/service/provider from history.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
vagaro-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find an open massage this weekend under budget

```bash
vagaro-pp-cli find massage --max-price 120 --min-rating 4.5 --from sat --to sun --agent
```

Fans out across nearby businesses and ranks those with an open slot in the window under your price and rating floor.

### Compare two barbers before committing

```bash
vagaro-pp-cli compare centralbarber rudysbarbershop --agent --select name,rating,priceRange,nextAvailable
```

Side-by-side decision table narrowed to the fields an agent needs, avoiding the verbose full payload.

### Rebook your usual haircut with the same provider

```bash
vagaro-pp-cli me rebook --last --from thu --to sat --agent
```

Reads your most recent past appointment, keeps the same business/service/provider, and lists that provider's open times inside the requested window. For a recurring haircut, this is the default path: use history to keep "Central Barber + Ronnel + haircut" fixed, then choose the future slot.

If the most recent appointment is not the one you want to repeat, first list your appointment history and pass the desired appointment id:

```bash
vagaro-pp-cli me --past --agent --select appointment_id,business_name,service_name,provider_name,date
vagaro-pp-cli me rebook <appointment-id> --from 2026-07-20 --to 2026-07-31 --agent
```

### Turn a selected rebook slot into a booking handoff

After `me rebook` returns slots, choose one `YYYY-MM-DDTHH:MM` value and verify it with `booking-link` / `book` before opening Vagaro:

```bash
vagaro-pp-cli booking-link centralbarber --service 34098477 --provider 43931725 --at 2026-07-24T10:00 --agent
vagaro-pp-cli booking-link centralbarber --service 34098477 --provider 43931725 --at 2026-07-24T10:00 --confirm --agent
```

`booking-link` verifies availability and prints what would be booked. `book --confirm` prints the tightest Vagaro URL plus the exact browser steps for the verified service/provider/time. Neither command submits the appointment or performs checkout; the final confirmation still happens on Vagaro's site.

### When to use favorites instead of rebook

Use `favorites` when the question is "which businesses did I save?" or when there is no relevant past appointment. For a routine repeat appointment, such as another haircut with the same barber at the same shop, skip favorites and use `me rebook`: appointment history already carries the business, service, and provider, so the only real choice is the future slot.

### Check if a haircut price is fair in your city

```bash
vagaro-pp-cli price-check haircut --city seattle --agent
```

Shows the metro price distribution and flags below-median providers.

## Auth Setup

Public discovery (search, business detail, services, reviews, classes) needs no auth. For your own appointment history and profile, run 'vagaro-pp-cli auth login --chrome' to import your logged-in Vagaro session from Chrome (a JWT plus session cookies). Booking commands are handoff-only: `booking-link` / `book` verify a selected slot and can print the closest Vagaro booking URL plus browser steps, but they do not submit the appointment.

Run `vagaro-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  vagaro-pp-cli business get mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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

- Use `--home <dir>` for one invocation, or set `VAGARO_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `VAGARO_CONFIG_DIR`, `VAGARO_DATA_DIR`, `VAGARO_STATE_DIR`, `VAGARO_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `VAGARO_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `vagaro-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "vagaro": {
        "command": "vagaro-pp-mcp",
        "env": {
          "VAGARO_HOME": "/srv/vagaro"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `VAGARO_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `VAGARO_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
vagaro-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
vagaro-pp-cli feedback --stdin < notes.txt
vagaro-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `VAGARO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `VAGARO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
vagaro-pp-cli profile save briefing --json
vagaro-pp-cli --profile briefing business get mock-value
vagaro-pp-cli profile list --json
vagaro-pp-cli profile show briefing
vagaro-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `vagaro-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/health/vagaro/cmd/vagaro-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add vagaro-pp-mcp -- vagaro-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which vagaro-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   vagaro-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `vagaro-pp-cli <command> --help`.
