# Retro candidates — generator defects found during the rightbrain run

These are Printing Press machine defects, not printed-CLI defects. Per the
template-shape escape hatch they were NOT patched in the printed CLI, because
patching here would hide the bug from the next CLI the machine prints.

## 1. Data race in the generated learn subsystem (severity: high)

`internal/store/learnings.go` mutates two package-level values —
`querySynonyms` (a map) and `querySynonymRules` (a slice) — from
`RegisterQuerySynonyms` with no mutex, while normalization reads them
concurrently.

Confirmed with the race detector against the generated test the machine itself
emits:

```
go test ./internal/cli/ -run TestPlaybookInit_ConcurrentSafe -count=5 -race
--- FAIL: TestPlaybookInit_ConcurrentSafe (0.83s)
    testing.go:1712: race detected during execution of test
```

This matters beyond the test: "concurrent map read and map write" is a fatal,
unrecoverable Go runtime error, and every printed CLI ships an MCP server that
services concurrent tool calls. The plain (non-race) suite passes, and shipcheck
does not run `-race`, so the machine's own gates do not catch it.

Affects every printed CLI that has the learn loop enabled, which is now the
default.

## 2. Composed resource-name collision produces uncompilable output

A top-level resource `task_run` and a sub-resource `task/{task_id}/run` both
normalize to `newProjectTaskRunCmd`, so the generated package fails to compile
with a redeclaration error. The same class affects `task_share` vs
`task/{id}/share`, and `task_agent_share` vs `task-agent/{id}/share`.

Worked around with `x-pp-resource` on the affected operations.

## 3. `x-pp-resource` is not threaded into the sync profiler

After the command layer honored the `x-pp-resource` override, `internal/cli/sync.go`
still emitted a duplicate `case "project_task_run"` in `syncResourceSinceParam`,
because the syncable-resource profiler derives its key from the path rather than
the override. Still uncompilable.

## 4. `x-pp-syncable: false` on an operation had no effect

Setting it on the two redundant run-list paths left all three
`dependentResourceDef` entries in place. The collision was only resolvable by
deleting the operations from the spec, which is a blunter instrument than the
extension is meant to provide.

## 5. `generate --force` reverted implemented novel commands and deleted a hand-authored file

The scaffold header promises "generate --force preserves implemented bodies".
On a re-run against the same output directory it instead reverted all seven
implemented novel commands (`gate`, `rollout`, `approvals`, `agent-trace`,
`drift`, `changelog`, `eval-flake`) plus their test files back to TODO
scaffolds, and **deleted** the markerless hand-authored file
`internal/cli/rightbrain_scope.go` outright.

Three independent subagents observed the same reversion at the same timestamp
and restored their own files from scratch copies. Recovery here was only
possible because a full backup had been taken immediately beforehand.

This is the most damaging of the five: it silently destroys hand-written Phase 3
work, and the skill actively encourages a regeneration pass for naming cleanup.

## 6. Published OpenAPI spec ships an unresolvable production server (upstream, not machine)

Not a Printing Press defect, but worth surfacing: Rightbrain's own published spec
declares `servers[0]` as `https://api.rightbrain.ai/api/v1`, a host with no DNS
record. Any generator consuming that spec verbatim emits a CLI that cannot
connect. The machine has no guard that probes a spec's declared server before
generating, and adding one would have caught this automatically.

---

## Added 2026-08-01, found while building a live failure-mode demo

## 7. Sync collapses two distinct collections onto one resource name and silently drops records (severity: high)

`/task/{id}/eval/set` and `/task/{id}/eval/run` are two different collections.
The generator's syncable-resource profiler maps BOTH onto the single resource
name `project_task_eval`, so they share one table and one `(resource_type, id)`
key space.

Measured against a live project holding one eval set and two eval runs, a clean
`sync` into an empty database persisted **exactly one of the three records**, and
reported:

```
{"event":"sync_summary","total_records":48,"resources":35,"success":35,"warned":0,"errored":0}
```

Success, zero warnings, zero errors, two-thirds of the data gone.

This is the same root cause as defects 2 and 3 above (composed resource-name
collision), but where those produced an uncompilable package — loud and
immediate — this one produces a silent data-correctness failure in the shipped
binary. Any command reasoning over the mirror gets a confident wrong answer.

The generated `dependentResourceDef` entries make it visible:

```
{Name: "project_task_eval", ..., PathTemplate: ".../task/{task_id}/eval/set"}
{Name: "project_task_eval", ..., PathTemplate: ".../task/{task_id}/eval/run"}
```

Two suggested machine fixes:
- Derive the sync resource name from the full path suffix rather than the parent
  resource, so `eval/set` and `eval/run` cannot collide.
- Fail generation (or warn loudly) when two dependent resource defs share a
  `Name` but have different `PathTemplate`s — that combination is never correct.

Worked around in the printed CLI by making `eval-flake` treat the API as
authoritative and report when the mirror under-counts. The workaround does not
help any other consumer of the mirror.

## 8. List and detail representations differ, and only the list shape is mirrored (severity: medium)

Rightbrain's eval-run list endpoint returns aggregate counters; the per-case
`results[]` array exists only on the detail endpoint. Sync mirrors the list
shape, so every field that exists solely on detail records is absent from the
local store with no indication it was ever there.

This is not Rightbrain-specific — it is the common REST list/detail split. The
generator has no notion of "this resource needs detail hydration to be useful
offline", so a novel command written against the documented schema compiles,
passes tests seeded from the schema, and returns empty results forever against
real data.

Worth considering: a spec-level `x-pp-sync-detail: true` that makes the walker
fetch each record's detail representation, or at minimum a generation-time
warning when a mirrored list schema is a strict subset of its detail schema.

## 9. Generated spec advertises an endpoint that 404s

`GET /org/{org_id}/project/{project_id}/task/{task_id}/revision` is declared in
the published OpenAPI and generated as a command, but returns HTTP 404 against
the live API. Task revisions are only reachable embedded in the task object.
Upstream spec defect rather than a machine defect, but it is the second
confirmed case (after the NXDOMAIN production server) of this spec advertising
something that does not exist — worth a generation-time reachability probe on
declared endpoints.

---

## Added 2026-08-01, found by the first full live-dogfood matrix (583 tests)

Running `dogfood --live --level full` against a disposable project — 583 tests,
578 passed — surfaced three machine-level defects that no read-only run reaches.

## 10. The framework's own `sync --json` violates the framework's own JSON contract (severity: medium)

`dogfood`'s `json_fidelity` check requires `--json` output to parse as a single
JSON document. The generated `sync` command emits newline-delimited JSON — 30
valid objects, one per progress event — which is the right shape for a streaming
command but fails that check:

```
json_fidelity  sync  invalid JSON
```

Both sides are generator-owned: `internal/cli/sync.go` is DO-NOT-EDIT, and the
checker is the generator's. The result is that **no printed CLI with a syncable
resource can pass `--level full`**, which in turn means `lock promote`'s phase5
gate can never be satisfied by a full run. That is a hard ceiling on the machine's
own publish path, not a property of any individual CLI.

Fix options: exempt streaming commands from single-document parsing, teach the
checker to accept NDJSON, or have `sync --json` buffer into one document and keep
NDJSON for the default human path.

## 11. OAuth callback endpoints are generated as user-invocable commands (severity: low)

`/integration/callback` and `/task_mcp_server/callback` are redirect targets for
an OAuth provider. They are generated as top-level commands (`integration`,
`task-mcp-server`) that a human can never successfully invoke — they exist to
receive a provider-issued `code`/`state`, so any synthesized call returns exit 5:

```
happy_path     integration       exit 5
json_fidelity  integration       exit 5
happy_path     task-mcp-server   exit 5
json_fidelity  task-mcp-server   exit 5
```

Four of the five remaining failures come from these two endpoints. A path whose
only caller is an OAuth redirect should be excluded from the command surface, the
way `/internal/*` operations are, or at minimum marked so the live matrix does not
probe it.

## 12. `dogfood --write-acceptance` emits an unusable marker when the CLI directory has no manifest

Run against a directory without `.printing-press.json`, `--write-acceptance`
writes a marker with no `api_name` and no `run_id`. `lock promote` then rejects it:

```
phase5 gate failed: phase5 marker missing api_name (manifest identifies the CLI);
                    phase5 marker missing run_id (manifest identifies the run)
```

The marker is written successfully and looks valid — `status: pass`, correct
counts — so the failure only appears one step later at promote, with no hint that
the cause is a missing manifest in the directory that was tested. Either fail fast
when the manifest is absent, or name the missing file in the promote error.
