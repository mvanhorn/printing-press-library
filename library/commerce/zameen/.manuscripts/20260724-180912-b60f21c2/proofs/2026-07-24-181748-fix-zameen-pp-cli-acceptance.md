# Zameen CLI — Phase 5 Acceptance Report

- **Level:** Full Dogfood (binary-owned live matrix, `dogfood --live --level full`)
- **Auth context:** none required (public read-only search) — key gate skipped legitimately.
- **Tests:** 86/86 passed. Gate: **PASS**.

## What the matrix exercised
Every leaf subcommand's help, happy-path, JSON-fidelity, and (where applicable) error-path against the live site: `find`, `listings`, `pull`, `get`, `open`, `watch`, `comps`, `deals`, `aging`, `agencies`, plus framework commands (`doctor`, `workflow status`, learn loop, profile, feedback, etc.). The live run populated the local store (25 listings) and offline store-backed commands (comps/deals/aging/agencies/workflow status) returned correct aggregates over it.

## Fixes applied during Phase 5
- `open <bad-id>` error-path: `open` builds a Zameen URL for any id, so it cannot distinguish a bogus id from a valid one — annotated `pp:no-error-path-probe: "true"` (the documented mechanism). Resolved the single initial failure (87→86 matrix, all pass).

## Printing Press issues (retro)
- Same as shipcheck: `sync_hint_test.go` emitted while `syncHintsEnabled=false`; `html_extract` can't parse `window.state`; generated "not in the official API docs" Highlights header re-synced by dogfood.

## PII
No org names, emails, or personal contact strings quoted in this report; results described generically. (Live listing data includes public agency/contact fields; none reproduced here.)

## Gate: PASS → proceed to polish + promote.
