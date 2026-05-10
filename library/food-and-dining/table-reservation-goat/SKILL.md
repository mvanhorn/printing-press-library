---
name: pp-table-reservation-goat
description: "One reservation CLI for OpenTable and Tock — search both networks at once, watch for cancellations, book, and... Trigger phrases: `book a table`, `find me a reservation`, `watch for a cancellation`, `tasting menu availability`, `earliest reservation across these restaurants`, `use table-reservation-goat`, `run table-reservation-goat`."
author: "Pejman Pour-Moezzi"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - table-reservation-goat-pp-cli
---

# Table Reservation GOAT — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `table-reservation-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install table-reservation-goat --cli-only
   ```
2. Verify: `table-reservation-goat-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

OpenTable and Tock split the US fine-dining world between them and share zero data. This CLI unifies them: `goat` searches both at once, `watch` polls both for cancellations, `earliest` composes availability across both, and `drift` surfaces what changed at a venue since your last look. Auth is one `auth login --chrome` import — your real Chrome cookies for both sites, no partner keys.

## When to Use This CLI

Use this CLI any time a user or agent needs to search, compare, watch, or book across OpenTable and Tock together — and especially for multi-venue questions ('soonest table at any of these'), cancellation hunting, or tracking changes at a specific venue. For single-network simple lookups, the official site UI is faster.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-network ground truth
- **`goat`** — One query across OpenTable and Tock simultaneously, ranked by relevance, earliest availability, and price band.

  _When a user asks an agent to find a table, this is the single command that searches both reservation networks and returns structured ranked results — agents do not need to know which network covers which restaurant._

  ```bash
  table-reservation-goat-pp-cli goat 'tasting menu chicago' --party 2 --when 'this weekend' --agent --select results.name,results.network,results.earliest_slot,results.price_band
  ```
- **`earliest`** — Across a list of restaurants from either network, return the earliest open slot per venue within a time horizon.

  _When a user gives an agent a shortlist of venues and wants the soonest opportunity, this is the right shape — one structured response with one row per venue across both networks._

  ```bash
  table-reservation-goat-pp-cli earliest 'alinea,le-bernardin,smyth,atomix' --party 4 --within 21d --agent --select earliest.venue,earliest.network,earliest.slot_at,earliest.attributes
  ```

### Local state that compounds
- **`watch`** — Persistent local watcher that polls both networks for openings on your target venues and party size, with notifications and optional auto-book.

  _Resy's Notify covers Resy only; tockstalk covers Tock only; restaurant-mcp's snipe covers Resy+OT only. None covers both networks; none persists state. Use this when an agent or user needs a hot reservation that isn't currently available._

  ```bash
  table-reservation-goat-pp-cli watch add 'le-bernardin' --party 2 --window 'Fri 7-9pm' --notify slack
  ```
- **`drift`** — Show what changed at a specific venue since the last sync — new experiences, slot price moves, hours changes.

  _Hot-target deep-watch: when an agent or user is hunting one venue, drift surfaces every meaningful change since the last look._

  ```bash
  table-reservation-goat-pp-cli drift alinea --since '2026-04-01' --agent
  ```

## Command Reference

**availability** — Check open reservation slots across OpenTable and Tock

- `table-reservation-goat-pp-cli availability check` — Check open slots for a restaurant on a specific date and party size
- `table-reservation-goat-pp-cli availability multi-day` — Multi-day availability for a single restaurant — Mon-Sun matrix

**restaurants** — Search and inspect restaurants across OpenTable and Tock

- `table-reservation-goat-pp-cli restaurants get` — Get a restaurant's full detail — hours, address, cuisine, price band, photos, accolades
- `table-reservation-goat-pp-cli restaurants list` — List restaurants across OpenTable and Tock; filter by location, cuisine, price band, accolades, and party size

**watch** — Persistent local cancellation watcher across both networks

- `table-reservation-goat-pp-cli watch add` — Register a watch for a venue, party size, and time window
- `table-reservation-goat-pp-cli watch list` — List active watches
- `table-reservation-goat-pp-cli watch cancel` — Cancel a watch by id
- `table-reservation-goat-pp-cli watch tick` — Run one polling tick across all active watches (for cron / agents)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
table-reservation-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Geographic Lookups (Agent Playbook)

The reservation networks index restaurants by metro. `--metro <slug>` is the
fastest way to constrain a search. Two things to know before composing a query:

**1. Discover the available metros first.** The CLI hydrates the live Tock
metro list (~248 metros worldwide) and merges it with a US-focused static
fallback. To see everything available:

```bash
table-reservation-goat-pp-cli goat --list-metros --agent
```

Returns `{metros: [{slug, name, lat, lng}], city_hints: {...}, total: N}`. The
`city_hints` field maps secondary cities (Bellevue, Oakland, Cambridge,
Brooklyn, etc.) onto the parent metro they're indexed under — useful when a
user asks about a city that isn't a standalone metro.

**2. Use the city-hint when the user names a secondary city.** Example flow
for "find me a Bellevue WA reservation":

```bash
# Bellevue isn't its own metro on either network — it's lumped into Seattle.
table-reservation-goat-pp-cli goat 'steakhouse' --metro seattle --metro-radius-km 20 --party 6 --agent
```

`--metro-radius-km 20` (vs the 50km default) constrains the Autocomplete-result
filter to Bellevue-area venues only, dropping Seattle proper. The CLI will
return per-row `metro_centroid_distance_km` so you can verify each result is
actually in the requested geo.

If `--metro <slug>` is rejected as unknown, the error message names the parent
metro and suggests the right radius:

```
unknown metro "bellevue" — neither OpenTable nor Tock breaks this out as its
own metro. Bellevue is lumped under metro "seattle" (centroid 47.6062,
-122.3321). Try `--metro seattle --metro-radius-km 20` to constrain results
to Bellevue-area venues, or pass `--latitude 47.6062 --longitude -122.3321`
directly with a tight `--metro-radius-km`.
```

**3. Slug suffixes work too.** If you compose a venue slug with a city suffix
(`joey-bellevue`, `13-coins-bellevue`), the CLI peels the suffix as a metro
hint and anchors the Autocomplete search at the metro centroid. Wrong-city
matches (the issue #406 "Joey's Bold Flavors" / Tampa class of failures) are
geo-filtered out before they reach the response.

**4. When you have a known numeric ID, use it.** `restaurants list` returns
OpenTable numeric IDs (`id: "3688"` for Daniel's Broiler - Bellevue). Pass
those directly to `availability check` / `availability multi-day` / `earliest`
to bypass the slug resolver entirely:

```bash
table-reservation-goat-pp-cli availability check 3688 --party 6 --date 2026-12-25 --agent
```

Numeric IDs route through a separate code path that doesn't touch the
Autocomplete-based resolver, so they're the most reliable input shape when
the agent already has the ID in hand.

## Error Recovery for Agents

The CLI surfaces a typed `error_kind` field on availability rows so agents
can branch on the recovery strategy without parsing free-text `reason`
strings. Three cases the agent should handle distinctly:

### `error_kind: "session_blocked"`

The entire OpenTable session is shadow-banned (Akamai sees the cookies as
expired/invalid). **All** OT operations will fail until cookies are refreshed.

**Recovery:** ask the user to run `auth login --chrome` (interactive). The
CLI's in-memory cooldown will clear once the new cookies pass through
Bootstrap. The disk-persisted cooldown auto-expires (5min → 60min exponential
backoff per consecutive 403).

### `error_kind: "operation_blocked"`

A specific GraphQL opname (typically `RestaurantsAvailability` or
`Autocomplete`) is on a WAF blocklist. **Sibling operations on the same
session still work.**

**Recovery paths, in order of preference:**

1. **Pivot to a numeric OpenTable ID.** `restaurants list` returns ids like
   `3688`. Passing `availability check 3688` bypasses the Autocomplete-based
   resolver entirely, so an `Autocomplete`-specific WAF rule doesn't apply:

   ```bash
   table-reservation-goat-pp-cli availability check 3688 --party 6 --agent
   ```

2. **Use the chromedp escape hatch.** When Chrome is running with remote-
   debugging enabled, the CLI routes blocked requests through the real
   browser's TLS stack:

   ```bash
   # Launch Chrome with debugging once per session
   open -a "Google Chrome" --args --remote-debugging-port=9222
   # Or point at a custom port via env var
   export TABLE_RESERVATION_GOAT_OT_CHROME_DEBUG_URL=http://localhost:9222
   ```

   The CLI auto-falls-back to chromedp on `BotDetectionError`s in the
   availability path.

3. **Surface the venue URL to the user.** Every row carries `url` (e.g.,
   `https://www.opentable.com/restaurant/profile/3688`). When both code
   paths are blocked, the agent should hand the user the URL so they can
   click through to OpenTable directly.

### No `error_kind` field (Tock errors, non-WAF errors)

Tock doesn't have a Kind discriminator yet — its errors arrive as plain
text in `reason`. The reason strings name the upstream condition
(`venue not found`, `calendar empty`, etc.); agents should surface them
to the user without retry.

## Recipes


### Headline omakase search across both networks (agent-shaped)

```bash
table-reservation-goat-pp-cli goat 'omakase manhattan' --party 2 --when 'this fri 7-9pm' --agent --select results.name,results.network,results.earliest_slot,results.price_band,results.attributes
```

Single command, ranked merged output with the deeply-nested fields agents actually need — narrows a multi-KB response to five columns.

### Watch one Tock-only and one OT-only venue at the same party size

```bash
table-reservation-goat-pp-cli watch add 'alinea' --party 2 --window 'sat 7-9pm' --notify local && table-reservation-goat-pp-cli watch add 'le-bernardin' --party 2 --window 'sat 7-9pm' --notify local
```

Two watches, one local store, one polling daemon — the printer handles both networks via per-source adaptive limiters.

### Soonest table among my shortlist

```bash
table-reservation-goat-pp-cli earliest 'narisawa,sushi-saito,den,florilege' --party 2 --within 14d --agent --select earliest.venue,earliest.network,earliest.slot_at
```

One row per venue with the soonest slot, sortable by slot time. Agents pipe into a planner without re-querying.

### Watched venue: what changed in the last week

```bash
table-reservation-goat-pp-cli drift alinea --since 7d --agent
```

Snapshot diff at a single venue — new experiences, slot price moves, hours changes — exactly what hot-target hunters need.

### Headline search, then check live availability for the top hit

```bash
table-reservation-goat-pp-cli goat 'le bernardin' --party 2 --json | jq -r '.results[0] | (.network + ":" + .slug)' | xargs -I{} table-reservation-goat-pp-cli availability check {} --party 2 --date "$(date +%Y-%m-%d)"
```

Compose the cross-network search with a follow-up live availability check — `goat` returns the best matched venue, `availability check` then queries OpenTable or Tock directly for open slots on that venue and date.

## Auth Setup

No authentication required.

Run `table-reservation-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  table-reservation-goat-pp-cli restaurants list --agent --select id,name,neighborhood,price_band
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
table-reservation-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
table-reservation-goat-pp-cli feedback --stdin < notes.txt
table-reservation-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.table-reservation-goat-pp-cli/feedback.jsonl`. They are never POSTed unless `TABLE_RESERVATION_GOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TABLE_RESERVATION_GOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
table-reservation-goat-pp-cli profile save briefing --json
table-reservation-goat-pp-cli --profile briefing restaurants list
table-reservation-goat-pp-cli profile list --json
table-reservation-goat-pp-cli profile show briefing
table-reservation-goat-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `table-reservation-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add table-reservation-goat-pp-mcp -- table-reservation-goat-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which table-reservation-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   table-reservation-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `table-reservation-goat-pp-cli <command> --help`.
