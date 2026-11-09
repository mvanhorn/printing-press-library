# Board snapshot (board #116 for app #111): `deals list --board-snapshot --pipeline <id> --board-columns ...` (2026-10-16)

Contract: the app's public OpenAPI at main 5729bc7b4 (live), frozen in the Cerebro session-tools folder
(`frozen-contracts/5729bc7b4/openapi.yaml`). The delta against the previous frozen contract (f2d638db6) is GET /deals only:
two query parameters, the 503 response, a new oneOf branch on the 200 and the new `DealBoardSnapshot` component schema.

Spec (written identically into `spec.yaml` and `upstream/openapi.yaml`; the drift gate is operation-level and stays green):
- Taken VERBATIM from the frozen OpenAPI: the `board_columns` and `board_snapshot` query parameters, the whole `"503"` response
  (board_snapshot_busy / gate_busy, Retry-After, `{ error, code, retryAfter }`) and the `DealBoardSnapshot` schema. The 200
  description gains the frozen sentence about board_snapshot.
- Round 2 (Codex r1 P2s, 2026-10-16): the 200 now is a two-branch `oneOf`: `PaginatedResponse` plus the frozen fourth branch
  (`{ data: DealBoardSnapshot }`), so generated clients can type `data.columns`. NOT taken: the frozen health branches (the
  curated GET /deals never carried the PipelineHealth* schemas). A trial print of this spec still rewires `defaultSyncResources()`,
  the archive workflow resources and every sync hint (`sync --resources ...` became bare `sync`, as round 1 found), so only the
  snapshot-related generated output was applied by hand: `deals_list.go` (`--per-page` help) and the `tools-manifest.json`
  deals_list per_page description. `sync.go`, `sync_hint.go`, `internal/mcp/tools.go`, `doctor.go`, `search.go`, `analytics.go`,
  `channel_workflow.go`, `data_source.go` and SKILL.md were NOT taken from that print.
- `per_page` on GET /deals is declared endpoint-specifically (inline, maximum 1000, per the frozen contract) instead of the shared
  `PerPageParam` (maximum 200), which stays for every other operation; help and manifest say max 1000.
- The rest of the frozen GET /deals (about 30 more query parameters: pipeline_name, search, health_*, rows_only, summary, view,
  ...) was not imported: the repo's curated block predates them and they are not this lane's delta.

Generation: `printing-press` 4.32.1 on the old and the new spec.yaml into scratch dirs (gen-old, gen-new-b); the trees differ in
exactly `internal/cli/deals_list.go` (two flags + two param map entries), `internal/mcp/code_orch.go` (two QueryParams bindings on
deals.list), `internal/types/types.go` (`DealBoardSnapshot`) and `tools-manifest.json` (deals_list: two params, "plus 7 more").
Applied by hand in the 4.28.0 print's style. Not applied: `.printing-press.json` (as in earlier lanes).

## Hand-written, a reprint must keep these

- `internal/cli/deals_board_snapshot.go`: with `--board-snapshot` (or `--board-columns`) the command leaves the generated
  paginated path. Refused before any request (usage error, exit 2): `--board-columns` without `--board-snapshot`, `--page` (even
  1: a snapshot is always page one), `--stage`, `--all`, `--data-source local` (a snapshot is a live read), a missing or blank `--pipeline`, a missing `--board-columns`,
  `--per-page` outside 1-1000 (it is the default column limit) and a `--board-columns` value the server would refuse.
  `--rows-only` / `--summary` / `--pipeline-name` are not flags of this print (cobra's unknown flag). The read is
  `GetWithHeadersNoCache` (a snapshot never comes from the 5-minute response cache), is never paged and is never written into the
  local store (the object answer is not a deal row). The answer's shape is checked (`{ columns: [...], meta: { board_snapshot:
  true } }`, else exit 5) and the provenance line counts the deals across the columns (never "0 results").
- `internal/cliutil/board_snapshot_guards.go`: `ParseBoardColumns` (1-50 entries, stage UUIDs, limit digits only 1-1000, default
  per_page, no stage twice, at most 5000 rows counting the default) and `CheckBoardSnapshotParams` (the executor's map form:
  board_snapshot a JSON boolean; with true no page / stage aliases / rows_only / summary / system_occupancy / health_*, a pipeline
  or pipeline alias, a string board_columns; board_columns without board_snapshot=true refused).
- `internal/mcp/code_orch.go` `handleCodeOrchExecute`: `case "deals.list"` applies `CheckBoardSnapshotParams` before any client is
  built (the plain list is untouched).
- `deals_list.go`: `Long` (the mode, its refusals, both 503 codes); the two flags' help rewritten (the generator cut both at the
  first sentence and lost the `<stageId>[:<limit>]` format).
- `tools-manifest.json` deals_list description: one paragraph on the snapshot, the executor's refusals and both 503 codes.
- No command was renamed (operationId listDeals is unchanged), so no alias was needed.
- 503s: the generated client retries a 5xx with backoff (1, 2, 4 s) and then surfaces the body (code and retryAfter) through
  classifyAPIError (exit 5); its Retry-After header is not honoured for 5xx. That is the print's existing behaviour, left as is.
- Tests: `internal/cli/deals_board_snapshot_test.go` (query sent, per-page default, answer printed, every refusal sends nothing,
  shape check, --data-source local refused, cache bypass with a cache-on control, provenance count, both 503 codes, help and manifest),
  `internal/cliutil/board_snapshot_guards_test.go`, `internal/mcp/board_snapshot_guards_test.go`. Mutants: the cache-bypass test
  fails with `GetWithHeaders`; the executor test fails without the `deals.list` case.
- Round 2: `CheckBoardSnapshotParams` requires at least one pipeline alias to be a non-blank string ("", "  ", numbers, booleans,
  null, objects do not count); a blank alias beside a real one is fine. Tests: `TestCheckBoardSnapshotParams`, the executor test,
  `TestDealsListSpecDeclaresPerPage1000AndTheSnapshotBranch`. Mutants: dropping the non-blank check fails the guard and executor
  tests; reverting per_page to 200 in spec.yaml or in the flag help fails the spec test.
- Round 3 (Codex parity r2 P2): the two 200 branches are now mutually exclusive in `spec.yaml` and `upstream/openapi.yaml`. The list
  branch mirrors the frozen list arm (`allOf`: `PaginatedResponse` plus `data` requiring `data.data` and `data.meta`) and also forbids
  `data.columns` (`not: required: [columns]`); the snapshot branch is `DealBoardSnapshot` (requires `columns`, `meta`, and
  `meta.board_snapshot` = true). A snapshot answer therefore validates against exactly one branch. No generated output changed (no
  reprint needed: the print only reads the branch for typing). Test: `TestDealsListOKBranchesAreMutuallyExclusive`
  (`internal/cli/deals_spec_branches_test.go`) validates one real snapshot answer and one real paginated answer against every branch of
  both spec files and asserts exactly one match each (and zero for a half-and-half envelope). It needs `gopkg.in/yaml.v3` and
  `github.com/google/jsonschema-go`, now direct requires in go.mod (both were already in the module cache and go.sum; jsonschema-go
  was an indirect dependency). Mutants: the round 2 spec (permissive `PaginatedResponse` branch), and the new branch without
  `required: [data, meta]` and `not`, both fail it.

## Reprint 637 (2026-10-30)

The full public GET /deals adds about 30 query parameters (stage aliases, health_*, rows_only, summary, search, ...), now generated as
flags. `runDealsBoardSnapshot` forwards only --pipeline, --status, --assigned-to, --sort, --order and --per-page, so it now refuses
EVERY other local flag with --board-snapshot (before: only --page, --stage, --all) instead of silently dropping it; Long says so.
--status, --sort and --order keep the curated enums and defaults (createdAt / desc), which the public document no longer declares.
`deals_spec_branches_test.go` resolves only the components a branch reaches (the full document has lookahead patterns Go cannot
compile), and its half-and-half check now asserts "never two branches" (the public list branch does not forbid extra keys).
