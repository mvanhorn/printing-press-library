# Phase 17 correctness review — round 2

**PASS — no new or remaining actionable product correctness finding in the reviewed fixes.**

Reviewed the frozen A/B/C implementation against the approved absorb manifest, implementation contract, round-1 findings, archived before files and focused red/green proofs. This round was read-only. No broad tests, origin requests or receipts were run; the builder owns integrated verification. A/B changes were authored by other agents; C's regressions are also independently reviewed by the security/maintainability personas.

- **PASS C1 — source identity and route:** `internal/source/parser.go:352` independently validates canonical/schema routes before parsing succeeds. Conflicting venue/route declarations reject; fragments alone cannot prove identity. Array/graph Restaurant forms, ratingCount, relative canonical references and the genuine relocation pair remain accepted by the recorded fixture cases.

- **PASS C1 — replacement boundary:** `internal/source/client.go:172` writes raw detail cache only after validated parsing; `internal/cli/tabelog_discovery.go:147` replaces snapshots only after success. Executable mismatch cases retain raw cache, saved snapshots and notes; an unfetched wrong response creates no normalized identity. Refresh still retains failed records and reports partial failure.

- **PASS C2 — effective sort:** `internal/source/discovery.go:443` requires exactly one active highest-rated tab, rejects conflicting pagination sort, and reports genuine empty pages without sorting controls as `not_applicable`. `internal/source/parser.go:312` applies this on every fetched page before caching. Page-2 drift preserves the prior ranked cache and normalized snapshots.

- **PASS C3 — factual output:** `internal/cli/tabelog_helpers.go:110` preserves facilities in default summaries. Existing full-record projection and provenance inheritance remain unchanged. Executable public-fixture output retains “Credit card accepted” and “Non smoking”; the approved 1,300-token default target accommodates the measured 1,247-token fact-complete output.

- **PASS M2 — budget deltas:** `internal/cli/tabelog_lists_refresh.go:41` compares raw text and numeric bounds alongside evidence state, excluding retrieval-source-only differences. Complete previous/current values remain available for real changes, and complete refreshed snapshots retain source provenance. The fixture refresh preserves notes and suppresses only the unchanged unknown lunch-budget delta.

- **PASS checkpoint/context and retention:** `internal/cli/tabelog_discovery.go:73` and `:147` preserve required snapshot-write errors while removing unused contextless sync checkpoints. Recorded SQL traps and caller-deadline cases verify the intended boundaries. `internal/notebook/notebook.go:44` adds an index without altering protection/eviction transactions; recorded protected and unsaved totals stay identical.

- **PASS S1/T1 regression boundary:** `internal/mcp/intents.go:68` and `:104` put recipe flags before `--` and literal values after, retaining required-input guards. `internal/cli/helpers.go:2040` returns a total-miss error before warnings; successful partial warnings and the explicit dry-run-sentinel downgrade remain. Actual stdio/machine tests and fresh generation pass.

Evidence reused: `phase17-cluster-a-after-oracle.jsonl` (21 passing records, zero failures/skips), `phase17-cluster-a-after-executable.jsonl` (18 passing records, zero failures/skips), `cluster-b-green.log`, `cluster-b-generator-green.log`, `cluster-b-retention-before.json`, `cluster-b-retention-after.json`, `phase17-cluster-c-focused-tests.log`, `phase17-cluster-c-valid-stdio-tests.log`, and `phase17-cluster-c-generated-tests.log`. The before/after code and assertions were inspected; passing test counts were not treated as coverage of unexamined behavior.

The round-1 generated Cobra pre-RunE/unknown-flag machine-error limitation remains documented as the explicitly excluded template retro candidate. It is unchanged and is not a new product finding or a claim of universal machine-error coverage.
