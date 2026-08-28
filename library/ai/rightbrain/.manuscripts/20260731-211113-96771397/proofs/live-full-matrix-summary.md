# Full live dogfood matrix — results

Run against a disposable Rightbrain project created for the purpose, with writes
enabled. This is the coverage a read-only sweep cannot reach: the read-only run
exercised 13 tests, the full matrix exercises ~577.

Raw per-test dumps are deliberately not committed — they embed live request URLs
containing the workspace's org and project identifiers.

## Progression

| Run | Matrix | Passed | Failed | Change |
|---|---|---|---|---|
| 1 — baseline | 584 | 577 | **7** | first full run ever performed |
| 2 — after two CLI fixes | 583 | 578 | **5** | `eval-flake`, `rollout` |
| 3 — after removing OAuth callbacks | 577 | 576 | **1** | two dead commands pruned |
| 4 — after the sync json-fidelity fix | 577 | **577** | **0** | full matrix green |

## What each failure was

**Fixed — `eval-flake` exited 0 on an unknown task id.** An unknown id is
genuinely indistinguishable from a real task with no eval history; both are "no
eval runs". Rather than invent a not-found error the command has no way to
justify, it now carries `pp:no-error-path-probe` and returns an empty report
with a note.

**Fixed — `rollout` exited 0 on an unknown task id.** It degraded a failed
weights fetch to a warning, which is right while there is still observed traffic
to report, and wrong when there is none. A task whose revisions cannot be read
AND which has no local runs is a wrong id; it now exits 3 (not found).

**Fixed — four failures on two OAuth callback endpoints.**
`/integration/callback` and `/task_mcp_server/callback` are redirect targets an
OAuth provider calls. A person supplying their own `code`/`state` can never
succeed, so both returned exit 5 on every probe. They are removed from the
command surface.

**Fixed — `sync --json` emitted newline-delimited JSON.** An unfiltered sync
wrote ~25 valid JSON objects to stdout, one per progress event, where the
generator's own `json_fidelity` check requires a single document. The streaming
was right; the stream was wrong. Progress events now go to stderr whenever
output is machine-readable, leaving exactly the terminal `sync_summary` on
stdout — no information lost, and stdout parses strictly as one document.
`internal/cli/sync.go` is DO-NOT-EDIT generated code, so the change is recorded
in `.printing-press-patches/rightbrain-sync-json-single-document.json` and filed
upstream as finding 10 in retro-candidates.md, since it affects every printed
CLI with a syncable resource.

## Publish gate

Run 4 is green, so the phase5 gate is satisfied by a full run rather than a quick
one. The press wrote `proofs/phase5-acceptance.json` from it:

    {"status":"pass","level":"full","matrix_size":577,
     "tests_passed":577,"tests_skipped":965}

Raw per-test dumps remain uncommitted: they embed live request URLs carrying the
workspace's org and project identifiers. The acceptance marker above carries the
counts without the identifiers.
