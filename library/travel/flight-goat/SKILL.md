---
name: pp-flight-goat
description: "Printing Press CLI for Flight Goat. AeroAPI is a simple, query-based API that gives software developers access to a variety of FlightAware's flight data."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - flight-goat-pp-cli
    install:
      - kind: go
        bins: [flight-goat-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/flight-goat/cmd/flight-goat-pp-cli
---

# Flight Goat — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `flight-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install flight-goat --cli-only
   ```
2. Verify: `flight-goat-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/flight-goat/cmd/flight-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

# Introduction
AeroAPI is a simple, query-based API that gives software developers access
to a variety of FlightAware's flight data. Users can obtain current or
historical data. AeroAPI is a RESTful API delivering accurate and
actionable aviation data. With the introduction of Foresight™, customers
have access to the data that powers over half of the predictive airline
ETAs in the US.

## Categories
AeroAPI is divided into several categories to make things easier to
discover.
- Flights: Summary information, planned routes, positions and more
- Foresight: Flight positions enhanced with FlightAware Foresight™
- Airports: Airport information and FIDS style resources
- Operators: Operator information and fleet activity resources
- Alerts: Configure flight alerts and delivery destinations
- History: Historical flight access for various endpoints
- Miscellaneous: Flight disruption, future schedule information, and aircraft owner information

## Development Tools
AeroAPI is defined using the OpenAPI Spec 3.0, which means it can be easily
imported into tools like Postman. To get started try importing the API
specification using
[Postman's instructions](https://learning.postman.com/docs/integrations/available-integrations/working-with-openAPI/).
Once imported as a collection only the "Value" field under the collection's
Authorization tab needs to be populated and saved before making calls.

The AeroAPI OpenAPI specification is located at:\
https://flightaware.com/commercial/aeroapi/resources/aeroapi-openapi.yml

Our [open source AeroApps project](/aeroapi/portal/resources)
provides a small collection of services and sample applications to help
you get started.

The Flight Information Display System (FIDS) AeroApp is an example of a
multi-tier application using multiple languages and Docker containers.
It demonstrates connectivity, data caching, flight presentation, and leveraging flight maps.

The Alerts AeroApp demonstrates the use of AeroAPI to set, edit, and
receive alerts in a sample application with a Dockerized Python backend
and a React frontend.

Our AeroAPI push notification [testing interface](/commercial/aeroapi/send.rvt)
provides a quick and easy way to test the delivery of customized alerts via AeroAPI push.

## Command Reference

**aircraft** — Manage aircraft

- `flight-goat-pp-cli aircraft <type>` — Returns information about an aircraft type, given an ICAO aircraft type designator string.

**airports** — Manage airports

- `flight-goat-pp-cli airports get` — Returns information about an airport given an ICAO or LID airport code such as KLAX, KIAH, O07, etc.
- `flight-goat-pp-cli airports get-all` — Returns the ICAO identifiers of all known airports.
- `flight-goat-pp-cli airports get-delays-for-all` — Returns a list of airports with delays.
- `flight-goat-pp-cli airports get-nearby` — Returns a list of airports located within a given distance from the given location.

**alerts** — AeroAPI alerting can be used to configure and receive real-time alerts on key flight
events. With customizable alerting offered by our alert endpoints, AeroAPI empowers
users to selectively pick various types of events/filters to alert on. By doing so,
you can receive specially tailored alerts delivered to you for events such as flight plan
filed, flight departure (out and off), flight arrival (on and in), and more!

To get started with alerting, the **PUT /alerts/endpoint** endpoint must first be used
to set up the account-wide default URL that alerts will be delivered to. This step must
be done before any alerts can be configured and will serve as the fallback URL that all
alerts will be sent to for the account if a specific delivery URL is not designated on a
particular alert. If this is not performed before configuring alerts, then you will
receive a 400 error with an error message reminding you of this step when trying to interact
with the **POST /alerts** endpoint. Once a URL is set via the **PUT /alerts/endpoint** endpoint,
then alerts can be configured using the **POST /alerts** endpoint. The **GET /alerts** endpoint
can also be used to retrieve all currently configured alerts associated with your AeroAPI key.
The **GET /alerts** endpoint will allow you to easily retrieve the id of any specific alerts of
interest configured for the account which can let you use the **GET** **PUT** and **DELETE**
**/alerts/{id}** endpoints to retrieve, update, and delete specific alerts.

When configuring an individual alert, the *target_url* field can be set to a URL that’s
different than the account-wide target endpoint set via the **PUT /alerts/endpoint**. If
the *target_url* field is set on an alert, then that specific alert will be delivered to
the specified *target_url* rather than the default account-wide one. If this field is not
configured for the alert, then the alert will be delivered to the default account-wide endpoint.
By setting this field, one can easily target different alerts to be received by different endpoints
which can be useful for configuring per-application alerts or sending alerts to an alternate
development environment without having to adjust a production alert configuration.

For each alert configured, one-to-many ‘events’ can be set for alert delivery. While most
events will result in one alert delivery, both the *arrival* and the *departure* events can
result in multiple alerts delivered (referred to as bundled). The *departure* event bundles the
departure (actual OFF the ground) alert, along with the flight plan filed alert and up to 5
per-departure changes which can include alerts for significant departure delays of over
30 minutes, gate changes, and airport delays. FlightAware Global customers will
also receive *Power on* and *Ready to taxi* alerts as part of the departure bundle. The *arrival* event
bundles the arrival (actual ON the ground) alert, along with up to 5 en-route changes (including delays
of over 30 minutes and excluding diversions) identified. FlightAware Global customers will also receive
*taxi stop* times as part of the *arrival* bundle. Setting a bundled type and unbundled type for an
On/Off will only result in a single alert in the case where events may overlap.

If there is a need to change the alert configurations, updating an alert using the **PUT /alerts/{id}**
endpoint and a unique alert identifier (id) is preferred rather than creating an additional alert.
By doing so, you can avoid duplicate alerts being delivered which could create unnecessary noise
if they are not of interest anymore.

If at any point there is a need to delete an alert, the **DELETE alerts/{id}** endpoint can be
leveraged to delete an alert so that it won’t be delivered anymore. As a reminder, specific alert
IDs can be retrieved from the **GET /alerts** endpoint.

- `flight-goat-pp-cli alerts create` — Create a new AeroAPI flight alert.
- `flight-goat-pp-cli alerts delete` — Deletes specific alert with given ID
- `flight-goat-pp-cli alerts delete-endpoint` — Remove the default account-wide URL that will be POSTed to for alerts that are not configured with a specific URL.
- `flight-goat-pp-cli alerts get` — Returns the configuration data for an alert with the specified ID.
- `flight-goat-pp-cli alerts get-all` — Returns all configured alerts for the FlightAware account (this includes alerts configured through other means by the
- `flight-goat-pp-cli alerts get-endpoint` — Returns URL that will be POSTed to for alerts that are delivered via AeroAPI.
- `flight-goat-pp-cli alerts set-endpoint` — Updates the default URL that will be POSTed to for alerts that are delivered via AeroAPI.
- `flight-goat-pp-cli alerts update` — Modifies the configuration for an alert with the specified ID.

**disruption-counts** — Manage disruption counts

- `flight-goat-pp-cli disruption-counts get` — Returns flight cancellation/delay counts in the specified time period for a particular airline or airport.
- `flight-goat-pp-cli disruption-counts get-all` — Returns overall flight cancellation/delay counts in the specified time period for either all airlines or all airports.

**flights** — Manage flights

- `flight-goat-pp-cli flights get` — Returns the flight info status summary for a registration, ident, or fa_flight_id.
- `flight-goat-pp-cli flights get-by-advanced-search` — Returns currently or recently airborne flights based on geospatial search parameters.
- `flight-goat-pp-cli flights get-by-position-search` — Returns flight positions based on geospatial search parameters.
- `flight-goat-pp-cli flights get-by-search` — Search for airborne flights by matching against various parameters including geospatial data.
- `flight-goat-pp-cli flights get-count-by-search` — Full search query documentation is available at the /flights/search endpoint.

**foresight** — Foresight endpoints provide access to FlightAware's Foresight predictive models and
predictions for key events. Our advanced machine learning (ML) models identify key
influencing factors for a flight to forecast future events in real-time, providing
unprecedented insight to improve operational efficiencies and facilitate better
decision-making in the air and on the ground. To learn more about the power of Foresight,
visit https://www.flightaware.com/commercial/foresight/

These endpoints each mirror a non-Foresight equivalent endpoint of similar functionality,
with the addition of all the ML 'predicted' values included in the Foresight response. The
respective non-Foresight endpoint response includes a flag, 'foresight_predictions_available',
which can optionally be used as a trigger to obtain and leverage Foresight predictions on an
as-needed basis and manage cost. Foresight is only available to Premium tier customers.
Contact integrationsales@flightaware.com for more information, pricing details, and to have
your account enabled for Foresight.

- `flight-goat-pp-cli foresight get-flight-position-with` — Get flight's current position, including Foresight data
- `flight-goat-pp-cli foresight get-flight-with` — Returns the flight info status summary for a registration, ident, or fa_flight_id
- `flight-goat-pp-cli foresight get-flights-by-advanced-search-with` — Returns currently or recently airborne flights based on geospatial search parameters.

**history** — Manage history

- `flight-goat-pp-cli history get-aircraft-last-flight` — Returns flight info status summary for an aircraft's last known flight given its registration.
- `flight-goat-pp-cli history get-flight` — Returns historical flight info status summary for a registration, ident, or fa_flight_id.
- `flight-goat-pp-cli history get-flight-map` — Returns a historical flight's track as a base64-encoded image.
- `flight-goat-pp-cli history get-flight-route` — Returns information about a historical flight's filed route including coordinates, names
- `flight-goat-pp-cli history get-flight-track` — Returns the track for a historical flight as an array of positions.

**operators** — Manage operators

- `flight-goat-pp-cli operators get` — Returns information for an operator such as their name, ICAO/IATA codes, headquarter location, etc.
- `flight-goat-pp-cli operators get-all` — Returns list of operator references (ICAO/IATA codes and URLs to access more information).

**schedules** — Manage schedules

- `flight-goat-pp-cli schedules` — Returns scheduled flights that have been published by airlines.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
flight-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `flight-goat-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export FLIGHT_GOAT_API_KEY="<your-key>"
```

To persist credentials, use `flight-goat-pp-cli auth set-token <token>`. Stored secrets live in `credentials.toml` under the data dir, not in `config.toml`.

Run `flight-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  flight-goat-pp-cli airports get mock-value --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `FLIGHT_GOAT_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `FLIGHT_GOAT_CONFIG_DIR`, `FLIGHT_GOAT_DATA_DIR`, `FLIGHT_GOAT_STATE_DIR`, `FLIGHT_GOAT_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `FLIGHT_GOAT_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `flight-goat-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "flight-goat": {
        "command": "flight-goat-pp-mcp",
        "env": {
          "FLIGHT_GOAT_HOME": "/srv/flight-goat"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `FLIGHT_GOAT_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `FLIGHT_GOAT_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
flight-goat-pp-cli recall "<user's question>" --agent
```

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "flight-goat-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `flight-goat-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `flight-goat-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `flight-goat-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
flight-goat-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
flight-goat-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
flight-goat-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
flight-goat-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`flight-goat-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `FLIGHT_GOAT_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
flight-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
flight-goat-pp-cli feedback --stdin < notes.txt
flight-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `FLIGHT_GOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FLIGHT_GOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
flight-goat-pp-cli profile save briefing --json
flight-goat-pp-cli --profile briefing airports get mock-value
flight-goat-pp-cli profile list --json
flight-goat-pp-cli profile show briefing
flight-goat-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `flight-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/flight-goat/cmd/flight-goat-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add flight-goat-pp-mcp -- flight-goat-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which flight-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   flight-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `flight-goat-pp-cli <command> --help`.
