Manifest transcendence rows: 2 planned, 0 built. Phase 3 will not pass until all 2 ship.

## Scope

Approved hand-code commands: `calls task run`, `calls search-task`.

## Progress
- 0/2 implemented at phase start.
- 1/2 implemented: `calls task run` issues one task call, waits for terminal state, returns the real final record, and stores it locally; refusal guard blocks verification/dogfood harnesses.
- 2/2 implemented: `calls search-task` uses the local FTS resource store and returns structured matching call records.
- Command-path collision resolved: generated `calls` create endpoint remains intact; the task runner is nested as `calls task run`.
- Narrative examples updated to actual command paths and verified with strict full-example validation.
