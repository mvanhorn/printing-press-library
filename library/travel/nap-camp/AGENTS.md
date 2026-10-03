# Nap Camp CLI Agent Guide

Use current binary help and `agent-context --pretty` for command/config truth. This is an unofficial public read-only campsite planner; source-side bookings, accounts and payments are outside its scope.

## Domain Invariants

- Campervan-entry facility categories are not proof of particular pitch permission or vehicle clearance. Preserve specific plan memo, capacity, power, pet, season and group-wide restrictions.
- Pitch area cannot establish vehicle length/width/height fit. Unknown dimensions stay unknown.
- Calendar statuses preserve the source legend; source acceptance is not vacancy. Evaluate stay nights excluding checkout. Starting prices and sums are not full dated group/option quotes; zero prices are unknown.
- Keep Japanese names, stable IDs, canonical URLs, JST dates, original observation timestamps and explicit requested coverage. Empty arrays are `[]`.

## Changes and Verification

Keep domain behavior in preserved hand-authored `internal/napcamp` and command extension files. Every novel command declares its data-source strategy and matching annotation. Validate inputs inside RunE after its dry-run guard. Fresh source commands reject local mode; local changes rejects live mode.

Run meaningful contract/normalization tests and the smallest affected command checks after edits. Full shipcheck, real live dogfood, independent review and representative performance are the release gates. Test category/pitch mismatches, empty data, unknown dates/prices, checkout semantics, file preservation and unchanged/changed observation coverage.

Save source observations only to explicitly supplied new files; preserve existing user files. The snapshot MCP tool is classified as write-capable because --save-to can create a new observation file; the other domain tools are read-only. No browser/session remains part of ordinary command transport. Respect source restrictions and fail explicitly on rate limits, non-JSON responses and shape drift.

Record customizations in `.printing-press-patches/` so regeneration retains the source and docs contract. Do not edit unrelated installed tools, global config or custom skills. Preserve the development release ledger and runtime version declarations; the public library post-merge workflow owns release stamping.
