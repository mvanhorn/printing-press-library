---
name: pp-nap-camp
description: "Plan campervan campsite stays with source-backed pitch rules and dated acceptance evidence. Trigger phrases: `find campervan-entry campsites in Japan`, `check Nap Camp pitch restrictions`, `compare campsite pitches`, `read a campsite acceptance calendar`, `use nap-camp`, `run nap-camp`."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

# なっぷ (Nap Camp) CLI

Use the existing verified binary when installed. The library installer and direct Go install below apply after this CLI is merged into the public library; before merge, build this source checkout with Go 1.26.6 or newer and verify the resulting binary.
## Prerequisites: Install the CLI

This skill drives the `nap-camp-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install nap-camp --cli-only
   ```
2. Verify: `nap-camp-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/nap-camp/cmd/nap-camp-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Discover public campervan-entry campsites, inspect the specific pitch and compare a bounded shortlist. Planner commands distinguish source acceptance and starting prices from unknown vehicle clearance, vacancy and full totals.

## When to Use This CLI

Use for public Japanese Nap Camp campsite discovery, specific pitch restrictions, conservative campervan requirements checks, dated acceptance candidates and saved-observation comparison. Preserve Japanese names, canonical links, JST dates, source qualifiers and observation timestamps in itineraries.

## Anti-triggers

Use the canonical booking site for account, reservation or payment actions. Vehicle clearance certification, road routing, guaranteed vacancy and complete dated group/option quotes require other source evidence and human confirmation.

## Unique Capabilities


### Pitch evidence
- **`planner fit`** — Check one specific pitch for vehicle, group, pet and power requirements without certifying unreported dimensions.

  _Choose this for explicit contradictions and unknowns on a chosen pitch._

  ```bash
  nap-camp-pp-cli planner fit 11007 20005062 --people 2 --length-m 6 --agent
  ```
- **`planner compare`** — Compare a bounded shortlist of specific campsite/pitch pairs with the same requirements.

  _Choose this when comparing selected pitches across an itinerary._

  ```bash
  nap-camp-pp-cli planner compare 11007:20005062 11007:20005063 --people 2 --agent
  ```

### Dates and observations
- **`planner windows`** — Find candidate consecutive nights from dated source acceptance evidence.

  _Choose this for multi-night planning with explicit unknown inventory._

  ```bash
  nap-camp-pp-cli planner windows 11007 20005062 --month 2026-10 --nights 2 --agent
  ```
- **`planner snapshot`** — Capture a compact dated observation spanning campsite, pitch and optional calendar.

  _Choose this to save a reviewable itinerary observation._

  ```bash
  nap-camp-pp-cli planner snapshot 11007 20005062 --month 2026-10 --agent
  ```
- **`planner changes`** — Compare two saved observations of the same campsite and pitch.

  _Choose this to detect source changes without repeating HTTP requests._

  ```bash
  nap-camp-pp-cli planner changes before.json after.json --agent
  ```

## Command Reference

Choose `campsite discover` for one result page, `campsite inspect` for facility details and a bounded plan list, `pitch inspect` for one selected plan, `pitch calendar` for a requested-month status projection and `catalog` for source IDs. Choose the planner capability above for the intended compound workflow. Inspect the leaf command's `--help` for current flags; source is an advanced raw contract group.

## Recipes

### Compare specific pitches

```bash
nap-camp-pp-cli planner compare 11007:20005062 11007:20005063 --people 2 --agent
```

Each pair is checked independently; facility categories are not pitch permission.

### Consecutive candidate nights

```bash
nap-camp-pp-cli planner windows 11007 20005062 --month 2026-10 --nights 2 --limit 5 --agent
```

Only nights with source acceptance statuses 1/2 form candidates; vacancy is unknown.

### Narrow pitch facts

```bash
nap-camp-pp-cli planner snapshot 11007 20005062 --agent --select campsite.name,pitch.vehicle_entry,pitch.power
```

Return only the selected source facts; those fields do not establish vehicle dimensions or a full dated total.

### Read a dated source calendar

```bash
nap-camp-pp-cli pitch calendar 11007 20005062 --month 2026-10 --limit 7 --json
```

Filter the two-month upstream response to the requested month; zero nonacceptance prices stay unknown.

## Auth Setup

Public read-only source contracts need no API key or account. Booking is a canonical-site handoff.

Run `nap-camp-pp-cli doctor` to verify setup.

## Agent Mode

Use `--agent` for structured output, `--select` for known fields and `--dry-run` for a no-I/O request preview. Planning lists are bounded and empty collections are `[]`. Keep unknown fields and qualification text. A fit verdict `needs_confirmation` is unresolved evidence; `contradiction` identifies a source mismatch. Partial compare failures are emitted with a nonzero exit.

## Paths and state

`nap-camp-pp-cli agent-context --pretty` reports actual runtime config/data/cache locations. Fresh planning commands read the source; `planner snapshot --save-to` optionally creates a new full JSON observation and refuses existing paths. Omit month for `calendar: []`. `planner changes` accepts two such files with identical campsite/pitch IDs and schema version 1, ignores observation-time-only changes and separates unequal date coverage.

## Direct Use

Resolve the installed binary, inspect the matching leaf command's help, then run the bounded command with `--agent`. Use `--no-learn` or `NAP_CAMP_NO_LEARN=true` for deterministic work. Fresh commands reject `--data-source local`; changes rejects `--data-source live`. Existing source observations are not current inventory.
