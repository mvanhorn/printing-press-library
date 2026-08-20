# Research Brief — grants (Research Grants Finder)

**Date:** 2026-07-05 · **Run:** 20260705-183000 · **Author:** @laci141

## Goal

One keyless CLI answering the two questions researchers actually ask:

1. *What can I apply for right now?* → open federal opportunities.
2. *How much does this topic typically get?* → historically awarded grants as a benchmark.

## Upstream APIs (all public, keyless)

### Grants.gov Search2 — open opportunities

- `POST https://api.grants.gov/v1/api/search2` — JSON body: `keyword`, `oppStatuses` (`posted` = open), `rows`, `startRecordNum`, optional `agencies`.
- **Payload reality check (live-tested):** `agencies` must be a **plain string** agency code (`"HHS-NIH11"`). Sending a JSON array (`["HHS-NIH11"]`) is accepted with `errorcode 0` but silently matches nothing (`hitCount: 0`). Verified 2026-07-06 against the live API with both shapes.
- `POST https://api.grants.gov/v1/api/fetchOpportunity` — per-opportunity synopsis: `awardCeiling`, `awardFloor`, `estimatedFunding`, `applicantTypes`, `responseDate`. Money fields arrive as string *or* number; `awardCeiling` is frequently `0` on real opportunities, so display/filtering falls back to `estimatedFunding` (labelled as an estimate).
- Result dates are `MM/DD/YYYY`; titles contain HTML entities (`&ndash;`) needing unescaping.

### NIH RePORTER — awarded NIH projects

- `POST https://api.reporter.nih.gov/v2/projects/search` — criteria `advanced_text_search` over projects; supports fiscal-year filter; sorted by `award_amount` desc for benchmarking.
- Returns **awarded** projects (not open calls) — deliberately positioned as the "how much do they give for this" view.

### NSF Awards — awarded NSF grants

- `GET https://api.nsf.gov/services/v1/awards.json?keyword=...` — max 25 per page; `fundsObligatedAmt` as string.
- Keyword relevance is loose upstream full-text OR; documented as expected behavior, not a bug.

## Design decisions

- **Keyless, stdlib-only Go** (no cobra, no third-party deps, no `exec.Command`) — zero supply-chain surface, nothing to configure.
- **Flexible flag parsing:** Go's `flag.Parse` stops at the first positional; a re-parse loop lets flags appear anywhere (`search cancer --rows 5` works).
- **Client-side deadline filter** (`--closing-before`) over the fetched page, with an explicit stderr truncation warning when the page was full, because Search2 has no closing-date filter parameter.
- **HTTP layer:** 20s timeout, one retry on network error/5xx, 8 MiB response cap, error bodies excerpted into error messages (`%.200s`).

## Origin note

This CLI was hand-built (multi-agent orchestrated build, reviewed and live-verified) following the printed-CLI conventions, then brought to canonical published-library shape for PR mvanhorn/printing-press-library#1443. The proofs in `../proofs/` are from live verification runs against all three upstream APIs.
