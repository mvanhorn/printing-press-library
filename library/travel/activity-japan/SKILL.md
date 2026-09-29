---
name: pp-activity-japan
description: Inspect known Activity Japan plan IDs or URLs, compare dated prices and sessions, check language-sitemap coverage, and hand off to canonical booking pages. Use for an Activity Japan itinerary shortlist; destination search is access gated.
author: zjsng
license: Apache-2.0
allowed-tools: Read Bash
---

# Activity Japan plan inspection

Use the installed `activity-japan-pp-cli` binary. In a source checkout, build with `go build -o ./bin/activity-japan-pp-cli ./cmd/activity-japan-pp-cli`, add `./bin` to PATH, and verify `activity-japan-pp-cli --version`. The installer below applies to released library packages. Run leaf `--help` when a flag is unclear.

## Prerequisites: Install the CLI

This skill drives the `activity-japan-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install activity-japan --cli-only
   ```
2. Verify: `activity-japan-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/activity-japan/cmd/activity-japan-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Inspect Activity Japan plans and future dates in compact JSON. Compare a bounded shortlist against traveler constraints and carry source uncertainty through booking handoff.

## Workflow

1. Get a plan ID from the traveler or an Activity Japan `/publish/plan/<id>` URL. If they only give a destination or activity type, use the Activity Japan website to identify IDs; CLI search pages are currently WAF challenged.
2. Run `experience detail ID --lang en --agent`. Keep `name_original_ja`, `operator_id`, option IDs, meeting point, venue, restrictions, cancellation text, and `missing` separate. The website locale and `locale_support_flag` do not establish spoken guide languages.
3. For an itinerary date, run `experience sessions ID --date YYYY-MM-DD --agent`, then `experience price ID --date YYYY-MM-DD --adults N --agent`. Use `--option ID` to inspect one selected price option. A zero yen child or infant option is not an adult price.
4. If an exact per-person session needs a stock recheck, pass its current `session_id` to `experience check ID --date YYYY-MM-DD --session ID --adults N --agent`. Treat the result as an observation. Group-priced quantity checks are refused because the source count basis is unverified.
5. For 2–5 known IDs, run `experience compare ID ID --date YYYY-MM-DD --adults N --age A --max-jpy B --agent`. Use `--max-minutes N` only for total experience duration; mixed source prose remains unknown. `unknown` is not a match. A subtotal below a budget remains unknown when mandatory fees are unpriced.
6. Before handoff, run `inventory languages ID --agent` and `experience handoff ID --lang en --agent`. `experience brief` includes a verified canonical URL when the selected language is indexed; otherwise it marks the URL unresolved.

Done when the answer names the exact plan and option where relevant, source URL, Asia/Tokyo observation time, price unit and age band, dated availability state, and unresolved conditions. Never call an observation a reservation.

## Quick commands

```bash
activity-japan-pp-cli experience detail 62375 --lang en --agent
activity-japan-pp-cli experience sessions 62375 --date 2026-10-08 --limit 5 --agent
activity-japan-pp-cli experience price 62375 --date 2026-10-08 --adults 2 --option 176511 --agent
activity-japan-pp-cli experience dates 62375 --dates 2026-10-07,2026-10-08 --agent
activity-japan-pp-cli inventory languages 62375 --agent
```

Replace dates with future dates in Asia/Tokyo. stdout is one JSON object; diagnostics are on stderr. `--select` reduces fields. Session and price results paginate with `--page` and `--limit`; truncation and partial failures are explicit. Plan, price and session reads refresh every run. Language inventory has a 24-hour bounded cache; use `--refresh` for a new sitemap observation.

The public plan endpoints need no credential. The official partner API requires an NDA and commercial agreement. The CLI makes no booking or payment request.

## Do not use for

- Destination or category search without a known Activity Japan plan ID or URL; use the website to identify plans first.
- Confirming a reservation, payment, or instructor language; this CLI only reports source observations and explicit unknowns.
- Experiences on other marketplaces.

## Unique Capabilities

These commands assemble source observations for an Activity Japan itinerary.

### Itinerary fit
- **`experience dates`** — See observed session and option-price differences across up to seven itinerary dates.

  _Use when a traveler can move an activity between nearby dates._

  ```bash
  activity-japan-pp-cli experience dates 62375 --dates 2026-10-07,2026-10-08 --agent
  ```
- **`experience compare`** — Compare a few known plans against explicit age, party, time, price, and language constraints.

  _Use after collecting Activity Japan plan IDs or links._

  ```bash
  activity-japan-pp-cli experience compare 62375 2044 --date 2026-10-08 --adults 2 --agent
  ```

### Price clarity
- **`experience price`** — See source option IDs, selected-date amounts, participant basis, and excluded extras.

  _Use before comparing activity costs for a specific party._

  ```bash
  activity-japan-pp-cli experience price 62375 --date 2026-10-08 --adults 2 --option 176511 --agent
  ```

### Source coverage
- **`inventory languages`** — Check whether a plan ID is indexed in English, Japanese, or both.

  _Use before handing an English-language source link to a traveler._

  ```bash
  activity-japan-pp-cli inventory languages 62375 --agent
  ```

### Booking handoff
- **`experience brief`** — Get a compact booking-handoff packet with known conditions and unresolved questions.

  _Use when the traveler is ready to review the official booking page._

  ```bash
  activity-japan-pp-cli experience brief 62375 --date 2026-10-08 --adults 2 --lang en --agent
  ```

## Recipes

### Inspect a cultural workshop

```bash
activity-japan-pp-cli experience detail 62375 --lang en --agent
```

Read the original Japanese name, operator ID, age range, meeting point and conditions.

### Check a future session

```bash
activity-japan-pp-cli experience sessions 62375 --date 2026-10-08 --agent
```

Keep instant and request states distinct.

### Compare two plans

```bash
activity-japan-pp-cli experience compare 62375 2044 --date 2026-10-08 --adults 2 --agent
```

Show known matches, mismatches and unknown constraints.

## Auth Setup

Known-plan JSON endpoints and plan sitemaps are anonymous. Independent CLI search listings are WAF challenged. Booking remains on Activity Japan.

Run `activity-japan-pp-cli doctor` to verify setup.
