# Phase 17 maintainability review — round 1

Review path: direct reviewer subagent, maintainability/data-migrations focus. Read-only review; no source changes during this phase.

## Findings

- **[P2] Preserve SEC fact numeric values during decode.** `internal/cli/gov_edgar.go:99` unmarshals arbitrary SEC JSON into `map[string]any`, converting every numeric financial fact to float64. Integers above 2^53 and decimals that cannot round-trip through float64 can be silently altered before printing or selecting. The feature promises upstream financial fact preservation. Use a JSON decoder with `UseNumber`; adapt ticker CIK handling at line 129 to parse its `json.Number` explicitly instead of `%f`. Add a decoder test with adjacent integers 9007199254740992 and 9007199254740993 and verify plain `--json` fidelity. Generated projection paths should be checked separately for template numeric fidelity rather than silently patched as part of this custom-source fix.

- **[P2] Avoid copying the entire snapshot just to read Scope.** `internal/cli/gov_local.go:192-198` marshals every saved dataset and raw metadata payload, then unmarshals the whole snapshot into a second object graph solely to obtain `scope`. Every analysis command calls this helper, including the resource manifest path. Large bounded government snapshots can contain substantial raw metadata, so the header operation unnecessarily multiplies peak memory and CPU even when `--limit=1` is requested. The shared Snapshot type now exposes `Scope`; assign `v["scope"] = s.Scope` directly. This also removes ignored marshal errors and avoids an unnecessary number conversion in scope values.

## Checked invariants

- Immutable snapshot creation inserts header and per-snapshot record membership in one SQL transaction; duplicate IDs are rejected rather than overwritten.
- Snapshot records are keyed by snapshot/source/dataset; catalog sources are validated and SEC records are excluded.
- Read paths drain rows before follow-up queries; no nested pooled Upsert occurs inside the snapshot write transaction.
- Initial schema is additive `CREATE TABLE IF NOT EXISTS`; no destructive migration or generated store migration edits.
- Exact resource URL joins retain source references; snapshot absence is labeled `absent_from_snapshot`, not deletion.
- All five live command adapters resolve to real local factories; no TODO/scaffold body remains in overlap/changes/coverage/intake audit/intake lock after regeneration.
- Live custom catalog and EDGAR command call paths establish `boundCtx` before requests; local commands do not make HTTP requests.
- Native source annotations are local/live as appropriate. Lock writes a user-visible file and is not marked read-only; output creation uses exclusive create.
- Custom snapshot freshness no longer calls an existing snapshot unsynced merely because generic generated sync_state is absent.

## Validation

Fresh targeted govcache and CLI lifecycle tests passed after regeneration: immutable membership, cross-source exact URL overlap, snapshot field/membership diffs, canonical metadata fingerprints including >2^53 integers, intake selection validation, missing-field audit, scoped coverage, manifest output exclusivity, and dry-run no-IO.

Build and vet passed after the local slice; parent full suite passed before regeneration. Round-1 findings above require small custom-source fixes and a re-review; no scope reduction is necessary.
