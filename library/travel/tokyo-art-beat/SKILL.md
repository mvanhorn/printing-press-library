---
name: tokyo-art-beat
author: "zjsng"
description: Discover Japan exhibitions, galleries and museums through Tokyo Art Beat; inspect a source edition, compare visits, resolve areas/categories or hand off official links. Use for Tokyo Art Beat exhibition searches, artist/venue discovery and trip-date overlap.
---

# Tokyo Art Beat

## Prerequisites: Install the CLI

This skill drives the `tokyo-art-beat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install tokyo-art-beat --cli-only
   ```
2. Verify: `tokyo-art-beat-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/tokyo-art-beat/cmd/tokyo-art-beat-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Setup

Build with `go build -o tokyo-art-beat-pp-cli ./cmd/tokyo-art-beat-pp-cli`. Public discovery needs no credentials. Use `tokyo-art-beat-pp-cli agent-context` for the current command tree and each leaf's help for flags.

## Workflow

1. Resolve an area/category/type with `catalogs areas`, `catalogs categories` or `catalogs types`. Keep stable IDs and Japanese names. Ambiguous names return candidates.
2. Run `events search --area Roppongi --from 2026-10-01 --to 2026-10-07 --limit 5 --agent`. Date windows require years. `--relation starts` or `--relation ends` narrows the date boundary. Default overlap compares spans.
3. Inspect selected IDs with `events detail ID --on 2026-10-05 --agent`. Source URLs are accepted. Venue defaults and event overrides remain separate.
4. Use `venues search --query Mori --limit 5 --agent`, then `venues detail ID` or `venues events ID --status all` for schedules including archives. `compare ID1 ID2 --agent` inspects 2..4 exhibition editions.
5. `nearby --lat 35.6605 --lon 139.7292 --area Roppongi --candidates 100 --limit 5 --agent` ranks locally by straight-line venue distance, within fetched candidates. The source ignores coordinate operators. Empty bounded output is not a nationwide absence claim.
6. Hand off source-provided official links. Empty ticket links and unknown inventory mean no booking availability has been established.

## Output

Compact JSON contains meta, results and errors. Retain meta.partial, meta.truncated, meta.pagination and meta.freshness when interpreting results. Empty successful lists are []. Missing source values are null; EN/JA omissions are explicit. `--fields id,name,starts,ends` projects result fields while retaining metadata; `--select results.id,meta.stats` projects the envelope. `--offset` continues at the returned next_offset.

## Freshness and limits

A one-hour response cache supports `--fresh` and exact-query `--offline`. Choose `--cache-dir PATH` when needed. Offline can return stale cached data with a flag; a cache miss is an error. Search defaults to 10 results, maximum 50. Artist matching is local over bounded full-text candidates; use `--max-scan-pages` to widen coverage. Sorting uses one source key, with ID tiebreaks only inside a page. Network work is sequential and bounded. Inspect lazy detail only for selected cards.

## Interpretation

An inclusive exhibition date span is not an actual open-day calendar. `--on` gives conservative assessments: exceptional/holiday notes, hidden schedules, missing boundaries and uncertain end dates yield unknown. No-listed-weekly-closure does not confirm opening. Preserve original admission category/member conditions in bilingual fee text, exceptional closure notes, source year and venue identity. Last-admission text comes from source notes; no time is inferred. Publication status is not a live opening status.

Public MuPon indicators do not establish a usable discount. Membership redemption and member-only features are excluded. No ticket inventory, availability, purchase, reservation or account changes are provided. All source data comes from Tokyo Art Beat's undocumented published website feed; unavailable structured capabilities remain null/empty.

## Errors

Structured errors go to stdout; diagnostics to stderr. Exit 2 means invalid input, 3 denied access, 4 no source identity, 5 network/schema/budget and 7 throttling. Partial joins and comparisons keep successful records, list failures and set meta.partial. Correct the named input or retry after the reported source failure; never substitute unrelated exhibitions.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`events search`** — Filter inclusive date spans; closures require detail.

  _Filter inclusive date spans; closures require detail._

  ```bash
  tokyo-art-beat-pp-cli events search --from 2026-10-01 --to 2026-10-07 --limit 2 --agent
  ```
- **`events search`** — Filter exhibition starts or ends within trip dates.

  _Filter exhibition starts or ends within trip dates._

  ```bash
  tokyo-art-beat-pp-cli events search --from 2026-10-01 --to 2026-10-07 --relation starts --limit 2 --agent
  ```
- **`events detail`** — Assess a full-year date, preserving schedule uncertainty.

  _Assess a full-year date, preserving schedule uncertainty._

  ```bash
  tokyo-art-beat-pp-cli events detail 2ccd6619-6e5e-410c-851a-56bf5d4662dc --on 2027-05-03 --agent
  ```
- **`nearby`** — Rank bounded candidates by straight-line public venue distance.

  _Rank bounded candidates by straight-line public venue distance._

  ```bash
  tokyo-art-beat-pp-cli nearby --lat 35.6605 --lon 139.7292 --area Roppongi --candidates 100 --limit 2 --agent
  ```
- **`events search`** — Project fields and expose cache/source freshness.

  _Project fields and expose cache/source freshness._

  ```bash
  tokyo-art-beat-pp-cli events search --from 2026-10-01 --to 2026-10-07 --fields id,name,starts,ends --limit 2 --agent
  ```

## Recipes

### Trip exhibition search

```bash
tokyo-art-beat-pp-cli events search --area Roppongi --from 2026-10-01 --to 2026-10-07 --limit 2 --agent
```

Filter source date spans, preserving bilingual identities.
