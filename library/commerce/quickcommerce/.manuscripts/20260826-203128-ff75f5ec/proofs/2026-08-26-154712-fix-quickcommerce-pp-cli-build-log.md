Manifest transcendence rows: 8 planned, 8 built. Phase 3 completion checks pending.

## Implementation
- Generated seven documented endpoint surfaces with API-key auth, canonical public flags, and dynamic pincode header support.
- Implemented all 8 approved transcendence commands using an append-only SQLite observation table.
- Added local history, field diffs, ETA ranking, credit planning, mirror ingestion, stale checks, coverage, and unit-price normalization.
- Added table-driven tests for location/unit parsing and custom store schema/insert behavior.

## Planned coverage
- 8/8 transcendence rows implemented; final command-resolution and dogfood checks pending.

## Intentionally deferred
- Background watch, notifications, checkout, basket optimization, and LLM recommendation were killed at the absorb gate because the API is read-only or lacks the required contract.

## Generator notes
- Generation initially hit permission-sensitive store tests under umask 0077; rerunning with umask 022 passed all generated gates.
- Generator emitted a warning that the novel delivery parent overlaps generated delivery; the hand-coded fastest leaf is attached to that parent.
