# Trip-planning workflow

1. Resolve destination IDs with geos autocomplete. Preview trips create with explicit dates and privacy; remove --dry-run only to create the authorized trip.
2. Read plan outline; identify dated days and undated candidate sections. Read full block notes by stable ID.
3. Read saved plan suggestions or search places by --query, --lat and --lng. Use real place IDs in fill-day; note_md supports Markdown in batch additions.
4. Review saved route legs, including missing estimates, with --modes driving,walking. Supply --travel-mode only when the user selected that mode. --with-planning adds saved hours/status/visit ranges. Check per-mode totals for incomplete coverage.
5. Add reservations and budget entries with currency explicitly preserved. Do not mistake recorded reservations for bookings.
6. Preview semantic plan edit batches against one snapshot. Apply the authorized changes, then re-read blocks and route legs.

```bash
wanderlog-pp-cli places autocomplete --query 'Eiffel Tower' --lat 48.8584 --lng 2.2945 --agent
wanderlog-pp-cli plan route legs --target-key naertjcoixqrgrfc --day 1 --modes driving,walking --travel-mode walking --with-planning --agent
wanderlog-pp-cli plan suggestions --target-key naertjcoixqrgrfc --day 1 --agent
```

A schedule gap is feasible only for the selected mode and the available estimate. Missing or unknown estimates are not feasible zero-minute trips. Saved hours and route data can be stale; keep provenance visible. Check transport reservations, fixed booking times and overnight boundaries separately.

## Start a blank trip

Use destination autocomplete to obtain a real geo ID, then preview creation. The numeric ID below is illustrative; use the returned destination ID and the traveler's dates. Creation uses REST: removing `--dry-run` creates the trip immediately, without an `--apply` flag.

```bash
wanderlog-pp-cli geos autocomplete --help
wanderlog-pp-cli trips create --geo-ids '[123]' --title 'My trip' --privacy private --dry-run --agent
```

## Read and revise complete notes

Get stable block IDs from `plan outline`. Substitute the target key and block IDs below.

```bash
wanderlog-pp-cli plan block get --target-key YOUR_TRIP_KEY --block-id 123 --markdown --agent
wanderlog-pp-cli plan edit --target-key YOUR_TRIP_KEY --changes-file changes.json --dry-run --agent
```

Example `changes.json`:

```json
[
  {"block_id":123,"markdown":"**Bring tickets**\nAllow time for the entrance queue.","start":"09:00","duration_minutes":60},
  {"block_id":456,"name":"Lunch reservation","start":"12:30","end":"13:30"}
]
```

After reviewing the before/after fields, use `--apply` without `--dry-run` to submit the same file. The CLI validates all changes against one snapshot and submits one ShareDB operation. Duplicate IDs, unknown fields, invalid schedules and missing blocks fail before submission. An uncertain acknowledgement requires reading the plan before retrying.
