# Novel-Features Brainstorm — fec-pp-cli (run 20260821-194418)

Question: what does this CLI give an agent that raw curl against api.open.fec.gov does not?

## Candidates considered

### 1. Money-window default flow (KEPT)
Raw problem: `schedule_a` without committee + two-year-period filters times out upstream (504 "Query
timed out", observed repeatedly during dogfood). The CLI's documented flow pairs `--committee-id` with
`--two-year-transaction-period` and defaults sort to `contribution_receipt_date`, so the most common
journalist/researcher query ("who gave money to X recently") works on the first try.
Command: `schedules list --committee-id C00703975 --two-year-transaction-period 2024`

### 2. Donor-first search fan-out (KEPT)
Every workflow starts by resolving a name to FEC IDs. `names list --q <name>` returns candidate AND
committee matches in one call; from there schedules/totals/filings hang off the ID.
Command: `names list --q bush`

### 3. Realtime efile window (KEPT)
Nightly processed data vs realtime electronic filings is a real provenance distinction. The `-6`
efile variant takes date windows (`--min-date/--max-date`) and shows contributions as they post.
Command: `schedules list-schedulea-6 --min-date 01/01/2024 --max-date 12/31/2024`

### 4. Bulk downloader wrapper (REJECTED)
Duplicates fec.gov's bulk-data site; no command-level value over the API surface.

## Error-path findings worth documenting in the PR
- `elections list-search --office` accepts only house|senate|president (422 otherwise).
- `legal get` needs a real document number; unknown docs 404 cleanly.
- Unfiltered schedule_a: CLI retries 3x then exits non-zero with a clear message (upstream behavior).
