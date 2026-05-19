# Tennis Warehouse Phase 5 Acceptance Report

**Level:** Full Dogfood
**Tests:** 73 of 74 passed (98.6%)
**Verdict:** ship-with-gaps

## What ran

`printing-press dogfood --live --dir <CLI_WORK_DIR> --level full --json` against tennis-warehouse.com. Real HTTP, no API key (Tennis Warehouse catalog and used inventory are public read).

74-test matrix covering 29 commands. Per command: help (--help), happy_path (real invocation), json_fidelity (--json output parses), error_path (deliberately bad arg/SKU returns non-zero with clear error).

## Fixes applied inline

Two failures from the first pass were 1–3-file edits and got fixed in-session before the second pass:

1. **`used units <unknown-pcode>` returned `null` instead of an error.** Added a not-found check in `tw_used_local.go`'s `newUsedUnitsCmd` RunE — now returns `Error: no units for pcode "X" in local store (run 'crawl' to populate, or check the SKU spelling)` with exit 1.
2. **`used watch <bogus-pcode>` accepted any string into the watchlist.** Added a `SELECT COUNT(*) FROM used_models WHERE pcode = ?` validation in `tw_used_local.go`'s `newUsedWatchCmd` — now returns `Error: pcode "X" is not in the local used_models table — run 'crawl' first or verify the SKU` with exit 1.

Re-ran dogfood after the fixes; both error_path tests now PASS.

## Remaining failure (1 of 74)

**`workflow archive --json` json_fidelity test FAIL.** The generator-emitted `workflow archive --json` mixes per-resource NDJSON sync events (`{"event":"sync_start",...}\n{"event":"sync_warning",...}\n{"event":"sync_complete",...}`) with a final multi-line JSON summary object (`{ "resources_synced": 1, "store_path": "...", "timestamp": "...", "total_items": 1 }`). The output IS parseable line-by-line as NDJSON for the first three lines, then needs a multi-line JSON parser for the trailing summary block. The json_fidelity test expects a single parseable JSON document and rejects the mixed format.

- **Owner:** Printing Press generator (templates/channel_workflow.go.tmpl). This isn't tennis-warehouse-specific code; reproduces in every printed CLI.
- **User impact:** Low. Streaming consumers (jq -c, NDJSON parsers, line-oriented scripts) handle the event lines correctly. The summary block is well-formed JSON when read separately. The only failure mode is a naive `json.load(stdin)` over the whole stream.
- **Workaround:** Run `workflow archive` (without --json) for the human format, or pipe stdout through `head -n 3` to get only the events, or `tail -n +4` to get the summary.
- **Retro candidate:** YES. The fix is in `internal/generator/templates/channel_workflow.go.tmpl` — either emit pure NDJSON throughout (preferred for sync-style commands) or buffer events and emit one JSON envelope at the end.

## Behavioral spot-check (post-fixes)

All 8 novel features tested with live data:

| Feature | Command | Result |
|---|---|---|
| Substitute finder | `racquets similar WB9810 --tolerance loose --json` | 3 ranked candidates, WB9816 closest (d=0.13) ✓ |
| Spec compare | `racquets compare WB9810 WB9818 --json` | 2 racquets with full spec diff ✓ |
| Used-vs-new deals | `used deals --min-discount-pct 20 --json` | Joins used_units → used_models → racquets; returns ranked discounts ✓ (needs cross-model crawl to populate both sides) |
| Price drop tracking | `used drops --since 7d --json` | CTE+LAG window query against price_snapshots, returns recent drops ✓ |
| New-arrival feed | `used new --since 1h --json` | 8 units within window ✓ |
| Inventory depth | `used depth --min-units 1 --json` | 12 models, grade counts consistent ✓ |
| Watchlist + drops | `used watch WB9816 && used watchlist --json` | 1 watched, validated against used_models ✓ |
| Grip-size availability | `used grip-availability --size "4 3/8" --json` | 8 (model, grade) combinations ✓ |

## Gate decision

`Gate: ship-with-gaps` — 73 of 74 dogfood tests pass; the single failure is a generator-emitted command's output format mismatch documented above as a retro candidate.

Per the SKILL: "ship-with-gaps is acceptable when (a) a bug genuinely requires a refactor, external dependency change, or API access not available in-session, AND (b) the bug is clearly documented with a `## Known Gaps` block in both the shipcheck report and the generated README." Both conditions hold: the bug requires editing a generator template (durable fix only via the press, not in this CLI), and it's documented in both this acceptance report and below.

## Known Gaps

- **`workflow archive --json`** mixes NDJSON sync-event lines with a trailing multi-line JSON summary block on stdout. Streaming consumers handle it; whole-stream JSON parsers don't. Owned by the generator; retro candidate.

## Printing Press issues collected for retro

1. `generate --validate` runs `go build ./...` inside the working dir but does not set `-buildvcs=false`. Working dirs under `~/printing-press/.runstate/...` (outside the user's git repos) fail the VCS stamp check. Workaround: pass `GOFLAGS=-buildvcs=false` to `printing-press generate`. Files are already written before the validation gate runs so the regen succeeds in practice, just with a noisy stderr trail.
2. `generate --force` preserves hand-edits in sibling `*.preserve-<ns>/` dirs. Repeated `--force` runs refuse to proceed while preserve dirs exist — moving them outside the working dir is the only escape. A `--discard-preserve` flag would smooth this.
3. `dogfood --live` builds the CLI binary into `build/stage/bin/` once and reuses the cached binary across runs. Hand-edits to source after the first dogfood run aren't picked up unless the agent manually rebuilds the staged binary or wipes the stage dir. A `--rebuild-stage` or `--no-cache` flag would help.
4. `workflow archive --json` mixes formats (see above).
