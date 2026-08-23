# ShareDB And JSON0 Reference

Low-level ShareDB apply, local undo/redo journal, and raw JSON0. Prefer named commands. Format notes with `plan block edit-text --markdown`, rename with `plan block rename`, span lodging with `plan reservation add --kind lodging --span-nights`.

## Apply Model

Fine-grained edits fetch the latest TripPlans snapshot over Wanderlog's ShareDB WebSocket, build JSON0 ops, and submit them at the current version. Default is dry-run; `--apply` mutates.

## Local Journal

Successful ShareDB applies append `~/.config/wanderlog-pp-cli/edit-journal.json` (target key, command, forward/inverse ops, paths, version, status).

```bash
wanderlog-pp-cli plan history --target-key KEY --agent
wanderlog-pp-cli plan undo --target-key KEY --record-id RECORD --apply --agent
wanderlog-pp-cli plan redo --target-key KEY --record-id RECORD --apply --agent
```

## When To Use `plan raw op`

Only when no named command covers the field. Always dry-run first. Re-fetch with `plan outline` (or `trips get KEY --full --no-cache` if you must see a blob) before computing paths. `od` must match the current value exactly. Array indices shift on `li`/`ld`; a batch of in-place `od`/`oi` replaces from one fetch stays valid, a batch with inserts/deletes does not — re-fetch between them.

Object replace / list insert / list delete:

```json
[{"p":["itinerary","budget","amount"],"od":{"amount":0,"currencyCode":"SGD"},"oi":{"amount":500,"currencyCode":"SGD"}}]
[{"p":["itinerary","sections",3,"blocks",0],"li":{"id":123,"type":"note"}}]
[{"p":["itinerary","sections",3,"blocks",0],"ld":{"id":123,"type":"note"}}]
```

If a future Delta still carries `header`, drop it; dry-run should report `stripped: ["header"]`. Bold + `list:bullet` persist; headers do not.
