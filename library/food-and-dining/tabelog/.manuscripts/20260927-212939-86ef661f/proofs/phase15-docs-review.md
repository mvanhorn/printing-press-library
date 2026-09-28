# Phase 15 documentation correctness audit

**PASS — README/SKILL/AGENTS correctness verified.**

One factual documentation error was found and fixed; no open findings remain.

| File:line | Severity | Finding | Fix |
|---|---|---|---|
| `README.md:28` | error | “Supported yen thresholds appear in help” is false. Actual `find` help describes source-supported budget flags without listing numerical choices. | Replace that claim with “Unsupported yen thresholds are rejected with the accepted values rather than rounded,” or add the actual values to help. |

Historical finding above is resolved: current `find --help` independently shows all 16 accepted nonzero thresholds, zero-disables-bound wording, explicit lunch/dinner requirements and source-bracket meaning. The list derives from the same validation slice via `source.BudgetThresholds()`.

Ground truth: the phase14 actual built help catalog, current `internal/cli` and relevant source/path/output definitions, `research.json` planned/verified feature sets, the approved absorb manifest and implementation contract. Unchanged help and tests were not rerun. Review only: product files and receipts were not edited.

## Checks that pass

- Commands, subcommands, arguments and flags in README/SKILL and linked saved-list examples resolve to the actual visible command tree. No unsupported exit codes or auth failures are advertised.
- README Unique Features and SKILL Unique Capabilities exactly match the five `novel_features_built` commands. No planned-only feature appears directly or indirectly.
- There are no `<cli>`, `<CLI>`, `<command>`, `<resource>` or `example-value` placeholders in executable examples.
- Unpublished installation is truthful: Go 1.26.6 matches `go.mod`; README builds `./cmd/tabelog-pp-cli`, and SKILL installs that local command with accurate GOBIN/GOPATH/PATH alternatives and version verification. No public-library download is invented.
- No-auth wording matches manifest `auth_type: none`; no auth troubleshooting or unsupported auth commands are included.
- The CLI is accurately described as reading remote data while writing its own local cache, source snapshots, notes and list memberships. Docs do not imply remote create/update/delete, reservations or account changes.
- Cache/live/local behavior, dry-run claims, offline compare/audit, fetch-first bare IDs, list-order alternatives, source-unknown evidence and partial refresh preservation match the implementation and help.
- SKILL anti-triggers exclude booking, payments/accounts, current seat/open status, walking routes and bulk review/image harvesting.
- Canonical prose uses “Tabelog.” Feature names are concrete and map to approved working commands; no promotional boilerplate promises unsupported behavior.
- All ten local Markdown pointers resolve: README to saved-list docs/SKILL/E2E/AGENTS/LICENSE; SKILL to README/saved-list docs; AGENTS to source/saved-list/E2E docs. The normalized domain directory and customization directory also exist.
- Linked source, notebook and replay/measurement procedures match the source structure: bounded public HTTP, isolated loopback replay, declared test-only transport override, documented environment names and measurement script options. Optional measurement dependencies are outside the shipped Go runtime.

## Evidence for the finding

`proofs/phase14-actual-help-catalog.md`, section `find`, shows only “Source-supported minimum/maximum average-price threshold in JPY.” `internal/cli/tabelog_discovery.go:85` and `:86` supply those exact descriptions. Accepted choices are defined in `internal/source/discovery.go:38` and included in the unsupported-value error at `:46`.

The help catalog contains 17 local help outputs. Its reviewed binary SHA-256 is `47593f2acf789e75583e7693fc71ed2eecb8a27717f262e4d0107a640a3e8049`. This audit reuses that snapshot and current source definitions; it makes no new runtime or live-source correctness claim.

## Fix verification

Only the changed `find --help` was rerun. The actual rebuilt working binary exited 0; its printed threshold list exactly matches the validator’s nonzero choices. Evidence: [phase15-find-help-fixed.txt](phase15-find-help-fixed.txt) and [phase15-find-help-fixed.json](phase15-find-help-fixed.json). New binary SHA-256: `ed5f125b5d54123542a96933bb3c5db094f7cd6275e75f31c11aa6adc5ef816b`. Unchanged checks were reused. No source request, notebook operation, product edit or receipt was performed by the reviewer.
