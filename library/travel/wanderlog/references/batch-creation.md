# Batch itinerary creation

Use `plan block add-batch --blocks-file blocks.json --target-key KEY --agent` to preview a mixed batch across existing days. The file is a strict JSON array (1–500 entries). Each entry chooses exactly one positive `day` or `section_id`; entries append in input order within that section. Resolve real place IDs with `places autocomplete` first.

```json
[
  {"ref":"arrival-note","type":"note","day":1,"markdown":"**Arrival**: allow 45 minutes for luggage."},
  {"ref":"museum","type":"place","day":1,"place_id":"RESOLVED_PLACE_ID","start":"10:00","duration_minutes":90,"markdown":"Collect reserved tickets before entry."},
  {"ref":"departure-list","type":"checklist","day":2,"title":"Before departure","items":["Bring tickets","Check out"]}
]
```

`ref` is an optional caller label for matching output entries; it must be unique in the batch. Use either `text` or `markdown`. Place blocks support a display `name`; checklists support `title` and string `items`. Schedules accept `start`, `end`, `duration_minutes` and `timezone`, with the same consistency validation as named schedule edits. Unknown fields, null values, duplicate fields and contradictory inputs are errors.

Preview the complete proposal, then repeat with `--apply` to create the blocks. `--dry-run` also validates the actual file and target. The batch resolves place details read-only and validates all destinations before one ShareDB transaction. Explicitly closed places block creation by default; a deliberate `--closed-place-policy warn` preserves the warning while permitting creation.

The apply receipt provides stable block IDs for later reads/edits. Preview IDs are provisional; use the IDs returned by the successful apply. If acknowledgement is uncertain, read the affected days before retrying; blindly repeating creation can duplicate blocks. `ref` is a response correlation label, not a server idempotency key.

First orient with `plan overview`; expand affected days with `plan days --days 1,2`. After applying, verify the affected days and overview to catch changes that affect another day. This command creates itinerary records, not paid bookings.
