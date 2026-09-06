# Wanderlog trip-planning implementation

Status: publication validation passes. The amendment preserves release metadata. Private fixture configuration and raw runner logs remain local.

## Implemented

| Planning task | Result |
|---|---|
| Create a blank trip | Existing `trips create` is discoverable; destination IDs, dates and privacy are validated. Preview and immediate REST write semantics are explicit. |
| Read a full note | New `plan block get --block-id` returns complete readable text, optional Markdown, links, checklist items, reservation details and saved place metadata. |
| Understand travel | New `plan route legs` joins saved directed route estimates to consecutive stops, including driving/walking time and distance. Missing coverage is explicit; totals are per mode and marked incomplete. An explicitly chosen mode enables schedule-gap checks. |
| Choose places | New `plan suggestions` returns bounded saved suggestions, deduplicated and excluding planned places. Autocomplete accepts ordinary query/location flags. |
| Edit many stops | New `plan edit --changes-file` accepts stable IDs and named note/name/schedule fields, validates one snapshot and previews before one ShareDB transaction. |
| Keep times consistent | Start/end/duration edits reconcile related fields, including midnight crossings, and reject contradictory triples. |
| Inspect plausibly | Opening hours and duration ranges no longer masquerade as intended visit times. Temporary and permanent closure flags are recognized. |
| Trust success reports | HTTP 200 application errors fail; invalid previews fail; uncertain acknowledgements cannot trigger blind repeat writes. ShareDB errors preserve scrubbed cause and stage. |
| Trust output and budgets | Numeric/wildcard selectors, no-match errors and post-projection limits prevent misleading or huge reads. Primary attachments survive compact mode. Mixed currencies remain separate; malformed costs make totals incomplete. |
| Learn the CLI | Improved discovery, common safety flags in short help, four new MCP entries and a substantially shorter skill with a planning reference. |


## Verification

All Go tests pass. Live acceptance: 260 passed, zero failed; 178 skipped/unverified. All 12 publication checks pass, including source-matched live acceptance, build, vet, govulncheck and skill consistency.

The original blockers are resolved using real records in a dedicated private synthetic itinerary and calendar-relative lodging dates. The fixture setup successfully exercised blank-trip creation and seven record additions. The subsequent acceptance matrix uses real API reads and dry-run mutation previews. Existing travel plans were not changed. The fixture remains available for repeated validation.

A further section-read bug was fixed: `plan sections --agent` now retains the primary section list. Boolean fixture annotations were removed because the runner turns `--flag=true` into a stray positional value; the runner's standard dry-run injection remains in force and strict command argument validation is preserved.

Read-only checks also verify complete block notes, saved suggestions and directed travel estimates. Synthetic protocol tests cover uncertain acknowledgements, schedule reconciliation, batch validation and raw API resource decoding. Skipped tests remain unverified; passing previews do not prove every live mutation.

## Limits kept explicit

Travel estimates, opening hours and recommendations come from saved API resources with unknown freshness. The CLI does not infer the UI's selected mode or invent missing route values. Markdown rendering is readable rather than a lossless round trip; raw text is opt-in. Overnight travel feasibility remains unknown where a clock alone is insufficient. Budget summaries do not invent exchange-rate conversions.


## Publication

The complete patch and PR draft are saved locally. No branch has been pushed and no PR opened. Runtime release metadata is unchanged; the publishing workflow owns version stamping.
