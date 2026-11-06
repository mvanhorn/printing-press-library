# lacourt-holidays-pp-cli Absorb Manifest

Ecosystem search: no LASC-specific CLI/SDK/MCP. Adjacent: codearranger/claude-legal `ca-deadlines` Claude skill (statewide Govt Code 6700 holiday list hardcoded, court/calendar-day modes). Many SEO-spam "LASC calculator" pages, no code.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List holidays for a year | lacourt.ca.gov court-holidays page | (generated endpoint) holidays get-holidays | --json/--select/--csv, decodes the double-encoded JSON |
| 2 | Local cache + search | none | (behavior in lacourt-holidays-pp-cli sync) framework sync/search | offline, CI-friendly |
| 3 | Court-day vs calendar-day counting (CCP 12/12a) | codearranger/claude-legal ca-deadlines | lacourt-holidays-pp-cli add-court-days | uses the court's own published LASC list instead of a hardcoded statewide list |

## Transcendence
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Court-day check | is-court-day | hand-code | weekend rule + synced LASC holidays; page can't answer yes/no (9/10) | none |
| 2 | Add court days | add-court-days | hand-code | date arithmetic over local holiday table, +/- N (9/10) | Use this command to count N court days forward or backward. Do NOT use it for calendar-day periods that roll forward to the next court day; use 'deadline' instead. |
| 3 | Next holiday | next-holiday | hand-code | cross-year lookup relative to today (8/10) | none |
| 4 | Calendar-day deadline with roll-forward | deadline | hand-code | CCP 12a roll-forward needs holiday awareness (7/10) | Use this command for calendar-day periods whose last day rolls to the next court day. Do NOT use it to count court days; use 'add-court-days' instead. |
| 5 | Court days between | court-days-between | hand-code | counts weekdays minus holidays across years (7/10) | Use this command to count court days between two known dates. Do NOT use it to find the date N court days away; use 'add-court-days' instead. |
| 6 | ICS export | ics | hand-code | page has no export (7/10) | none |

Guard: date-math commands error when a date falls in an unpublished year (only 2026-2030 published).
Stubs: none.

## User additions (Phase Gate 1.5)
| # | Feature | Command | Buildability | Why | Long Description |
|---|---------|---------|--------------|-----|------------------|
| 7 | All-years ICS export | ics (no year = every published year; --out-dir writes one <year>.ics per year) | hand-code | user workflow: import every available LASC holiday onto their own device (score 9/10, user-requested) | Use this command to export holidays as calendar files. With no year it covers every year the court has published. |
