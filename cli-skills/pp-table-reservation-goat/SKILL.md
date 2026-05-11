---
name: pp-table-reservation-goat
description: "One reservation CLI for OpenTable, Tock, and Resy — search each network at once, watch for cancellations, book, and... Trigger phrases: `book a table`, `find me a reservation`, `watch for a cancellation`, `tasting menu availability`, `earliest reservation across these restaurants`, `use table-reservation-goat`, `run table-reservation-goat`."
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

OpenTable, Tock, and Resy split the US fine-dining world between them and share zero data. This CLI unifies them: `goat` searches all three at once, `watch` polls each network for cancellations, `earliest` composes availability across all three, and `drift` surfaces what changed at a venue since your last look.

**Authentication** is two commands, each run once:

- `auth login --chrome` imports OpenTable + Tock session cookies from your local Chrome profile.
- `auth login --resy --email <you@example.com>` exchanges Resy email + password for a long-lived API token; the password is consumed and never persisted.

**Network-prefixed addressing.** Resy venues are addressed by numeric id (`resy:1387`); OpenTable by numeric id or URL slug (`opentable:le-bernardin`); Tock by domain-name slug (`tock:alinea`).

## When to Use This CLI

Use this CLI any time a user or agent needs to search, compare, watch, or book across OpenTable, Tock, and Resy together — and especially for multi-venue questions ('soonest table at any of these'), cancellation hunting, or tracking changes at a specific venue. For single-network simple lookups, the official site UI is faster.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-network ground truth
- **`goat`** — One query across OpenTable, Tock, and Resy simultaneously, ranked by relevance, earliest availability, and price band.

  _When a user asks an agent to find a table, this is the single command that searches both reservation networks and returns structured ranked results — agents do not need to know which network covers which restaurant._

  ```bash
  table-reservation-goat-pp-cli goat 'tasting menu chicago' --party 2 --when 'this weekend' --agent --select results.name,results.network,results.earliest_slot,results.price_band
  ```
- **`earliest`** — Across a list of restaurants from either network, return the earliest open slot per venue within a time horizon.

  _When a user gives an agent a shortlist of venues and wants the soonest opportunity, this is the right shape — one structured response with one row per venue across all three networks._

  ```bash
  table-reservation-goat-pp-cli earliest 'alinea,le-bernardin,smyth,atomix' --party 4 --within 21d --agent --select earliest.venue,earliest.network,earliest.slot_at,earliest.attributes
  ```

### Local state that compounds
- **`watch`** — Persistent local watcher that polls each network for openings on your target venues and party size, with notifications and optional auto-book.

  _Resy's Notify covers Resy only; tockstalk covers Tock only; restaurant-mcp's snipe covers Resy+OT only. None covers each network; none persists state. Use this when an agent or user needs a hot reservation that isn't currently available._

  ```bash
  table-reservation-goat-pp-cli watch add 'le-bernardin' --party 2 --window 'Fri 7-9pm' --notify slack
  ```
- **`drift`** — Show what changed at a specific venue since the last sync — new experiences, slot price moves, hours changes.

  _Hot-target deep-watch: when an agent or user is hunting one venue, drift surfaces every meaningful change since the last look._

  ```bash
  table-reservation-goat-pp-cli drift alinea --since '2026-04-01' --agent
  ```

## Command Reference

**availability** — Check open reservation slots across OpenTable, Tock, and Resy

- `table-reservation-goat-pp-cli availability check` — Check open slots for a restaurant on a specific date and party size
- `table-reservation-goat-pp-cli availability multi-day` — Multi-day availability for a single restaurant — Mon-Sun matrix

**restaurants** — Search and inspect restaurants across OpenTable, Tock, and Resy

- `table-reservation-goat-pp-cli restaurants get` — Get a restaurant's full detail — hours, address, cuisine, price band, photos, accolades
- `table-reservation-goat-pp-cli restaurants list` — List restaurants across OpenTable, Tock, and Resy; filter by location, cuisine, price band, accolades, and party size

**watch** — Persistent local cancellation watcher across all three networks

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

## Location Handling (Agent Playbook)

Every read command (`restaurants list`, `availability check`, `availability multi-day`, `earliest`, `goat`, `watch`) accepts a free-form `--location` flag that parses bare city, city+state, metro qualifier, or coordinates.

**Accepted `--location` shapes:**

```bash
--location bellevue              # bare city (ambiguous — see below)
--location 'bellevue, wa'        # city + state (unambiguous)
--location 'seattle metro'       # metro qualifier
--location '47.6101,-122.2015'   # coordinates (lat,lng)
```

The resolver returns one of three response shapes, classified by the categorical `tier` field:

- **`tier: "high"`** (one match, or specific input): response includes `location_resolved` field with the canonical name, centroid, reason, and any alternates considered. Results are filtered to that region.
- **`tier: "medium"`** (multiple candidates but one dominates): response includes both `location_resolved` and `location_warning`. The warning lists the alternates so the agent can sanity-check against conversation context.
- **`tier: "low"`** (genuinely ambiguous, e.g., bare "bellevue" matches WA/NE/KY): the command refuses to return results. Instead it emits a typed `needs_clarification` envelope with ranked candidates, each carrying `state`, `context_hints`, `tock_business_count`, and `score_if_picked`. The agent disambiguates and re-runs.

Note: `location_resolved.score` is the popularity prior (a mechanical [0,1] number derived from population + provider coverage). Do not branch on this number — Bellevue WA at city+state specificity is HIGH-certain but its absolute score is modest (~0.42), and Seattle at HIGH tier reads ~0.6. The categorical `tier` field is what agents branch on; `score` is informational.

### Three agent rules (load-bearing contract)

**1. Always check `location_resolved.tier` in successful responses.**
The `tier` string is the agent-facing categorical classification — branch on it, not on the numeric `score`.
- `tier == "high"` — the pick is reliable; proceed.
- `tier == "medium"` — alternates exist and the response includes a `location_warning` listing them. Sanity-check the pick against conversation context (e.g., did you pick Portland OR but the user clearly meant Maine?). Surface the pick to the user.
- `tier == "low"` — you'll receive a `needs_clarification` envelope instead of results; rule 2 applies.

**2. On `needs_clarification: true`, do NOT retry blindly.**
First, look back in the conversation for geographic clues (state mentions, nearby cities, prior locations, time-zone hints). If you find any, re-run with that location. If you don't, use the `agent_guidance.fallback_clarification` text (or your own phrasing) to ask the user. Concrete shape:

```json
{
  "needs_clarification": true,
  "error_kind": "location_ambiguous",
  "what_was_asked": "bellevue",
  "candidates": [
    {"name": "Bellevue, WA", "state": "WA",
     "context_hints": ["Seattle metro", "Eastside"],
     "tock_business_count": 28, "score_if_picked": 0.78,
     "centroid": [47.6101, -122.2015]},
    {"name": "Bellevue, NE", "state": "NE",
     "context_hints": ["Omaha metro"],
     "tock_business_count": 0, "score_if_picked": 0.18,
     "centroid": [41.1370, -95.9145]},
    {"name": "Bellevue, KY", "state": "KY",
     "context_hints": ["Cincinnati metro"],
     "tock_business_count": 0, "score_if_picked": 0.04,
     "centroid": [39.1067, -84.4744]}
  ],
  "agent_guidance": {
    "preferred_recovery": "Check conversation context for geographic clues. If the user mentioned a state or nearby city, re-run with that.",
    "rerun_pattern": "<command> --location '<chosen-name>'"
  }
}
```

**3. Never silently accept a MEDIUM-tier resolution.**
When `location_warning` is present on a successful response (`tier == "medium"`, or a `tier == "low"` forced pick from a batch caller), surface the pick to the user in your reply (e.g., "I'm searching in Bellevue, WA — let me know if you meant a different one"). The warning is the CLI's signal that *you* should hand the user a hand-off point. Do NOT reach for `--batch-accept-ambiguous` to silence the warning — that flag is for batch jobs only; in interactive use it defeats the disambiguation contract entirely.

### `--batch-accept-ambiguous` is a batch-only escape hatch

Every read command exposes `--batch-accept-ambiguous` (default false). When true, a LOW-tier resolution returns a forced pick (top candidate by popularity prior) with `location_warning` flagging the bypass, rather than the `needs_clarification` envelope. **Interactive agents must never use this flag — it defeats the disambiguation contract entirely.** The verbose `batch-` prefix is intentional: it exists exclusively for batch jobs, scheduled runs, and test fixtures where any-pick-is-fine semantics are correct. If you're answering a user in real time and you reach for this flag, stop — re-read rule 2 and disambiguate from conversation context or ask the user.

### `--metro` is a deprecated alias

`--metro <slug>` continues to work for back-compat, but the implicit `--batch-accept-ambiguous` is **canonical-only** — it is set automatically only when the value resolves to a single, unambiguous metro via the registry (slug lookup, alias chain, or a single `LookupByName` hit). Three cases:

- **Canonical slug** (`seattle`, `nyc`, `chicago`, `sf`, `san-francisco`, etc.) — single registry hit. The resolver silent-picks with the legacy result-shape preserved (no envelope). This is the back-compat path.
- **Ambiguous value** (e.g., `--metro bellevue` matches WA/NE/KY by display name) — `--batch-accept-ambiguous` is **not** implied. The resolver returns the same `needs_clarification` envelope `--location bellevue` would. **Legacy callers must handle the envelope path** — treat the response exactly like a `--location` envelope and disambiguate (Codex P1-D fix; silently picking the wrong city is worse than asking).
- **Unknown slug** — returns a `location_unknown` envelope, same as `--location <unknown>`.

A one-line stderr deprecation warning (`warning: --metro is deprecated; use --location <city>.`) fires once-per-process on first use regardless of the canonical-vs-ambiguous outcome. New code should use `--location`.

### Slug suffixes still work in `earliest` and `watch`

When you compose a venue slug with a city suffix (`joey-bellevue`, `13-coins-bellevue`) and don't pass `--location`, the CLI detects the city hint, anchors the Autocomplete search at the inferred metro's centroid, and tags the resulting `location_resolved.source` as `extracted_from_query` (signaling soft-demote post-filter). Explicit `--location` always wins over slug-suffix inference.

### `location resolve` is a primitive

When you need to verify a location is well-formed before running a search, use:

```bash
table-reservation-goat-pp-cli location resolve 'bellevue, wa' --agent
```

Emits the typed `GeoContext` JSON (HIGH/MEDIUM) or the disambiguation envelope (LOW). Useful for up-front verification before fanning out reads.

### Numeric IDs bypass location resolution

When you have an OpenTable numeric ID (from `restaurants list --json`), pass it directly to `availability check` / `availability multi-day` / `earliest`. The numeric-ID short-circuit skips the slug resolver entirely:

```bash
table-reservation-goat-pp-cli availability check 3688 --party 6 --date 2026-12-25 --agent
```

If you also pass `--location` with a numeric ID and the venue is outside the stated radius, the response will include a `location_warning` (not a hard-reject) — the numeric ID is treated as authoritative; the warning is informational.

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


### Headline omakase search across all three networks (agent-shaped)

```bash
table-reservation-goat-pp-cli goat 'omakase manhattan' --party 2 --when 'this fri 7-9pm' --agent --select results.name,results.network,results.earliest_slot,results.price_band,results.attributes
```

Single command, ranked merged output with the deeply-nested fields agents actually need — narrows a multi-KB response to five columns.

### Watch one Tock-only and one OT-only venue at the same party size

```bash
table-reservation-goat-pp-cli watch add 'alinea' --party 2 --window 'sat 7-9pm' --notify local && table-reservation-goat-pp-cli watch add 'le-bernardin' --party 2 --window 'sat 7-9pm' --notify local
```

Two watches, one local store, one polling daemon — the printer handles each network via per-source adaptive limiters.

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
