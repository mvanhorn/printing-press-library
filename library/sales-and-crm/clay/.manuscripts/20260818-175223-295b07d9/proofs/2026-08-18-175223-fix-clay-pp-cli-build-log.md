# Clay CLI Build Log

Manifest transcendence rows: 11 planned, 11 built. Phase 3 will not pass until all 11 ship.

## Built

### Priority 0 - foundation
- Generated data layer, sync, search, SQL, learn loop, MCP surface (30 endpoints, 8 resources).
- Composed auth: `claysession` cookie for `/v3`, `clay-api-key` header for `/public/v0`.

### Priority 1 - absorbed (27 rows)
All generated endpoint commands resolve. Full column CRUD is present:
`columns create` / `update` / `delete`, plus `tables create` / `update` and
`workbooks create`, which no other Clay tool offers.

### Priority 2 - transcendence (11 of 11)
| Command | File | Notes |
|---|---|---|
| blueprint export | internal/cli/blueprint_export.go | Serializes column graph; rewrites `{{f_id}}` to `{{Name}}` for portability |
| blueprint apply | internal/cli/blueprint_apply.go | Topological column ordering; remaps names back to new ids; per-column failure accounting |
| columns graph | internal/cli/columns_graph.go | dependsOn / usedBy DAG |
| columns doctor | internal/cli/columns_doctor.go | Dangling refs + orphan columns; exit 3 on errors |
| columns set-formula | internal/cli/columns_set_formula.go | Read resolves ids to names; write resolves names to ids |
| columns link | internal/cli/columns_link.go | Cross-table lookup via `lookup-row-in-other-table` |
| tables diff | internal/cli/tables_diff.go | Type / formula / action drift; exit 3 when different |
| workbooks graph | internal/cli/workbooks_graph.go | Clay node+edge topology with credit estimate rollup |
| enrichments compare | internal/cli/enrichments_compare.go | Catalog ranked, marked against connected accounts |
| errors | internal/cli/errors.go | Per-column run-status failure counts; exit 3 when failures exist |
| watch | internal/cli/watch.go | Polls until settled; curtails to 1 poll under dogfood |

Shared helpers in `internal/cli/clay_helpers.go`; unit tests in `clay_helpers_test.go`
cover formula ref parsing, id/name round-tripping, dependency ordering, and cycle detection.

## Deviations
- Blueprints serialize as **JSON**, not YAML. No YAML dependency ships in the generated
  module and `typeSettings` is deeply nested; TOML round-trips it badly. Narrative wording
  corrected to match.
- `formulas` has a single endpoint, so the generator collapsed the command to
  `clay-pp-cli formulas <workspaceId>` rather than `formulas generate`. Manifest corrected
  to the shipped path.
- Step 1.5c.5 novel-feature brainstorming ran inline rather than in a subagent, because
  this session is configured not to spawn agents unless the user asks. Output shape is
  unchanged. Disclosed to the user at Phase Gate 1.5.

## Known gaps
- **Row listing is unresolved.** `POST /records` turned out to be row creation/hydration
  and `bulk-fetch-records` requires explicit `recordIds`. Column and schema reads are
  unaffected. Row VALUE reads need known record ids or the Public API `/tables/query`
  (Enterprise sync).
- Workspace id is a positional on generated endpoint commands. Novel commands accept
  `--workspace` or `CLAY_WORKSPACE_ID`. A config-backed default for generated commands
  was not added.

## Environment note
Host Go is 1.26.4 while the generated module targets 1.26.6, and the user's persistent
`GOSUMDB=off` blocks toolchain download. All Go gates were run with scoped
`GOSUMDB=sum.golang.org GOPROXY=https://proxy.golang.org,direct`, which does not modify
the user's Go configuration.
