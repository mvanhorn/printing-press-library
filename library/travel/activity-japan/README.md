# Activity Japan CLI

**Source-backed Japan experiences with explicit plan, price, language and availability evidence.**

Inspect Activity Japan plans and future dates in compact JSON. Compare a bounded shortlist against traveler constraints and carry source uncertainty through booking handoff.

## Build locally

Requires the Go version in `go.mod` (currently 1.26.6 or newer).

```bash
go build -o ./bin/activity-japan-pp-cli ./cmd/activity-japan-pp-cli
export PATH="$PWD/bin:$PATH"
activity-japan-pp-cli --help
```

The CLI has no account setup. It reads public Activity Japan plan JSON from `gd.activityjapan.com` and English/Japanese plan sitemaps. It does not submit reservations or payments. Build this source checkout locally. The Printing Press installer can provide the binary after the library release.

## Use

Replace example dates with future dates in Asia/Tokyo. Add `--agent` for one JSON object on stdout; errors go to stderr. `--select` can reduce fields. IDs can come from Activity Japan plan URLs.

```bash
activity-japan-pp-cli experience detail 62375 --lang en --agent
activity-japan-pp-cli experience sessions 62375 --date 2026-10-08 --limit 5 --agent
activity-japan-pp-cli experience price 62375 --date 2026-10-08 --adults 2 --option 176511 --agent
activity-japan-pp-cli experience check 62375 --date 2026-10-08 --session 189637 --adults 2 --agent
activity-japan-pp-cli experience dates 62375 --dates 2026-10-07,2026-10-08 --agent
activity-japan-pp-cli experience compare 62375 2044 --date 2026-10-08 --adults 2 --age 30 --max-jpy 7000 --max-minutes 90 --agent
activity-japan-pp-cli experience brief 62375 --date 2026-10-08 --adults 2 --agent
activity-japan-pp-cli inventory languages 62375 --agent
activity-japan-pp-cli experience handoff 62375 --lang en --agent
```

Use `experience sessions` first to obtain a current session ID for `experience check`. A check is a dated stock observation, never a confirmed booking. Group-priced stock quantity is left unverified and the check command refuses it. `experience price` shows each option's source unit amount and age class. An adult subtotal is derived only when the original Japanese option label establishes adult or generic participant applicability, the unit basis and option bounds fit the party, and the dated unit price is known. Fees and extras remain unknown. Website language does not prove instructor language.

## Commands and limits

| Command | Result | Bound |
| --- | --- | --- |
| `experience detail ID` | Plan, Japanese name, operator ID, conditions, option IDs | One exact plan |
| `experience sessions ID --date DATE` | Dated session IDs and states | 10 per page by default; `--limit` up to 50, `--page` up to 100 |
| `experience price ID --date DATE` | Plan headline, undated option units and dated quotes | 10 per page by default; `--option` selects one ID |
| `experience check ID --date DATE --session ID --adults N` | Read-only stock recheck | One per-person course, 1–50 units |
| `experience dates ID --dates D1,D2` | Compact date and price comparison | Up to 7 dates |
| `experience compare ID ID ...` | `match`, `mismatch`, or `unknown` per constraint | 2–5 plans |
| `experience brief ID --date DATE --adults N` | Compact handoff packet and unresolved facts | One plan and date |
| `inventory languages ID` | English/Japanese sitemap presence | Two plan sitemaps |
| `experience handoff ID --lang en|ja` | Indexed canonical plan URL | No booking mutation |

Every source request has a 2 MiB body cap. Novel commands refresh plan, price, and session observations on every run; they do not use a response cache. Sitemap inventory uses two fixed cache files, a 24-hour TTL, and a 2 MiB cap per file. Pass `--refresh` to `inventory languages` or `experience handoff` to refetch, or root `--no-cache` to bypass inventory cache. A command deadline is at most 25 seconds and individual sitemap requests at most 10 seconds.

## Coverage and interpretation

- Destination/category search pages expose useful filters, but Activity Japan's website challenges independent CLI requests. `experience search` is therefore **not available** in this CLI. Find plan IDs on the [English site](https://en.activityjapan.com/) or [Japanese site](https://activityjapan.com/), then use the CLI for exact details and comparisons. The official partner API requires a commercial agreement and NDA.
- English and Japanese plan sitemaps differ. Sitemap presence proves index coverage, not bookability or a spoken guide language.
- `experience detail` reports a constructed `source_url` and leaves `canonical_url` null until sitemap membership is checked. `experience brief`, `experience compare`, and `experience handoff` verify the selected language's index before labeling a URL canonical; any failed check is explicit partial coverage.
- Session state `instant_confirmable` means the site currently permits an instant booking path; `reservation_request` still requires operator acceptance. `closed`, `not_accepted`, `sold_out_for_party`, and `unknown` remain separate. None is a reservation.
- The plan headline is derived from `plan_data` base minus discount, while each option has its own undated unit amount; both can differ from a dated quote. The CLI uses selected-date source integers and never multiplies them by the API's `taxIn` factor. Unknown mandatory fees, optional extras, taxes, participant age fit, and venue pickup stay explicit.
- The source sometimes uses zero as a party maximum sentinel. The CLI reports that raw source value while treating the traveler maximum as unknown. An ambiguously named passenger-count field is preserved but not used as a minimum party size.
- Source prose may be machine translated or inconsistent. The CLI preserves it as source text rather than converting uncertain duration or cancellation wording into stronger claims.

Use `activity-japan-pp-cli <group> <command> --help` for current flags. Source endpoint mirrors under `source-plan` expose raw website JSON for investigation. The raw `source-plan stock-check` result does not validate party or age eligibility; use `experience check` for a party-aware stock observation.

## Authentication

Known-plan JSON endpoints and plan sitemaps are anonymous. Independent CLI search listings are WAF challenged. Booking remains on Activity Japan.

## Quick Start

```bash
# Inspect a known plan, Japanese name and source conditions.
activity-japan-pp-cli experience detail 62375 --lang en --agent

# Observe future sessions without making a reservation.
activity-japan-pp-cli experience sessions 62375 --date 2026-10-08 --limit 5 --agent

# Read one dated option quote and age-aware subtotal.
activity-japan-pp-cli experience price 62375 --date 2026-10-08 --adults 2 --option 176511 --agent

```

## Unique Features

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
