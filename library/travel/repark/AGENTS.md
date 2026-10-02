# Repark CLI maintenance

The public parking commands live in `internal/cli/parking*.go`; source contract and domain parsing live in `internal/repark`. Treat native-browser/source evidence and source-derived fixtures as the contract. Parking commands read through HTTP every time; preserve explicit input anchors, bounded requests and independent matching/output caps.

Keep occupancy separate from published vehicle-limit checks and remaining-bay suitability. Preserve Japanese day types, source tariff/maximum wording, explicit repeating/one-time applicability, and calendar/overnight boundaries. Bay-scoped or multi-amount maximums remain unparsed numeric nulls until their applicability can be represented accurately. Quotes call only the source's harmless simulator for explicit bay/JST input; never recreate complete totals or invoke payments/reservations.

Unknown source facts remain null or explicit unknowns: exact remaining spaces, measurement time, unspecified bay variation, tax inclusion, source import/update timezone semantics and coverage. Retain fetch timestamps and source URLs. Use the generated JSON pipeline; project parking result payloads before adding their provenance envelope. Refuse `--data-source local` for live parking commands before IO.

Meaningful parser and source-contract tests are in `internal/repark/repark_test.go`; CLI projection/strategy tests are in `internal/cli/parking_projection_test.go`. `workflow_verify.yaml` exercises live discovery -> inspected lot -> source quote. Live tests and Go HTTP fixtures need approved networking/loopback access in this desktop sandbox. A sandbox DNS/listener refusal is not a source blocker.

Manual `sync` snapshots require an explicitly supplied, bounded source `range` parameter; these snapshots do not establish a complete parking inventory. Public GitHub publication remains a separate user authorization; use personal GitHub identity `zjsng` for authorized writes. Build-specific `.press*` state is ignored by Git.
