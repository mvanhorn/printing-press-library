# Exa CLI Build Log

Manifest transcendence rows: 4 planned, 4 built. Phase 3 will not pass until all 4 ship.

## Built

1. **spend** — `internal/cli/spend.go` + cost journal hook `internal/client/client.go` (`RecordCostJournal`, patched + recorded in `.printing-press-patches/cost-journal-client-hook.json`). Every live Exa response carrying `costDollars.total` appends one JSONL line to `<data-dir>/cost.jsonl`; `spend` aggregates by day/resource. Verified live: answer call journaled ($0.005), `spend --json` reports it.
2. **monitor diff** — `internal/cli/monitor_diff.go`. Diffs two synced monitor runs (two newest by default, or `--from`/`--to`), reports added/removed/unchanged URLs. Storage-ID parent-suffix handling added.
3. **entity report** — `internal/cli/entity_report.go`. Scans synced webset items + monitor runs for entity mentions, builds first-seen/last-seen/mention-count timeline; `--type company|person`, `--since` window support.
4. **webset new** — `internal/cli/webset_new.go`. Lists items added to a webset within the sync window (`--since`, default 7d); scoped by `websets_id`.

## Supporting changes

- `parseHumanDuration` helper (Go's ParseDuration lacks day units) in `internal/cli/entity_report.go`.
- Tests: `internal/cli/spend_test.go` (journal recording, verify-env skip, aggregation, resource filter, empty journal, path mapping), `internal/cli/novel_local_test.go` (monitor diff, explicit runs, entity report hit/no-hit, webset new scoping/window, data-source rejection).
- All novel commands: `pp:data-source local` with `--data-source live` rejection, verify-friendly RunE (help fallback → dry-run → usageErr), `--json` via `printJSONFiltered`, sync hints, missing-mirror guard.

## Verification

- `go build ./...` PASS
- `go test ./... -count=1` — 13 packages ok
- Live smoke: websearch/answer/contents/teams reachable; cost journal + spend verified end-to-end.
- Spec fixes: `/v0/teams/me` missing per-path server override (added `https://api.exa.ai/websets`); `/search` resource renamed `websearch` via `x-pp-resource` (collided with reserved framework template).

Built count: 4/4.
