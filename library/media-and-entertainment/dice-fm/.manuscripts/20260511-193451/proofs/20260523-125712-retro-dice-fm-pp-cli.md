# Printing Press Retro: DICE FM (Partners GraphQL API)

## Session Stats
- API: dice-fm (DICE Partners GraphQL API)
- Spec source: docs-derived internal YAML (SpectaQL schema at partners-endpoint.dice.fm/graphql/docs)
- Scorecard: 81/100 (Grade A, post-polish; 77 pre-polish)
- Verify pass rate: 100%
- Fix loops: 1 (validate-narrative: sync --dry-run)
- Manual code edits: substantial — entire GraphQL data layer hand-authored (query layer, per-resource commands, sync rewrite, 7 transcendence commands)
- Features built from scratch: 7 transcendence commands + hand-authored GraphQL client/query/sync layer

## Context
First real GraphQL CLI printed (catalog/library had zero GraphQL entries before this).
The machine ships a dedicated GraphQL code path (`graphql_client.go.tmpl`,
`graphql_queries.go.tmpl`, `graphql_sync.go.tmpl`, `internal/graphql` SDL parser),
so these findings are about correctness of an existing shipped path, provable by
template inspection — not statistical cross-CLI claims. Evidence is n=1 (dice-fm) in
the catalog/library; the common Relay shape (`viewer { conn { edges { node } } }`)
is used by well-known GraphQL APIs not yet in the catalog (GitHub v4, Shopify Admin).

## Findings

### 1. Generated GraphQL transport rides the verify-gated POST → `sync` short-circuits under verify (Bug)
- **What happened:** The generated GraphQL `sync` command silently "syncs 0" under `PRINTING_PRESS_VERIFY=1`.
- **Scorer correct?** N/A (not a score-penalty finding; a behavior bug).
- **Root cause:** `internal/generator/templates/graphql_client.go.tmpl:170` — `Client.Query` (and `Mutate`, `PaginatedQuery`) call `c.Post(...)`, which routes through the verify-gated `do()`. Under `PRINTING_PRESS_VERIFY=1` (without `LIVE_HTTP=1`) mutating verbs short-circuit to the `__pp_verify_synthetic__` noop envelope. `graphql_sync.go.tmpl:307` calls `c.Query(ctx, def.Query, variables)`, so generated GraphQL sync receives the synthetic envelope, parses zero nodes, and reports success with 0 records. The generated per-endpoint REST-style commands already avoid this by using `PostQueryWithParams` (→ `doRead`, ungated); the GraphQL transport methods do not.
- **Cross-API check:** Deterministic for every GraphQL CLI — the defect is in the shared GraphQL transport template, provable by inspection.
- **Frequency:** every GraphQL CLI.
- **Fallback if the Printing Press doesn't fix it:** the agent must notice that GraphQL reads are verify-unsafe and re-route through `PostQueryWithParams` by hand (exactly what this run did in `dice_query.go`). Easy to miss → ships a verify-green-but-actually-noop sync.
- **Worth a Printing Press fix?** Yes. Narrow, safe, zero downside — mirror the existing read-path pattern the generated endpoint commands already use.
- **Inherent or fixable:** Fixable.
- **Durable fix:** In `graphql_client.go.tmpl`, route `Query`/`Mutate`(reads)/`PaginatedQuery` through the ungated read path (`doRead` / `PostQueryWithParams`) rather than `Post`. GraphQL queries are reads riding a POST verb — exactly the case `doRead` exists for. (Genuine mutations, if the generator ever emits GraphQL mutations, should keep the gated path.)
- **Test:** positive — generate a GraphQL CLI, run `sync --resources X` under `PRINTING_PRESS_VERIFY=1` against a mock that returns nodes; assert rows land. negative — same under verify with no `LIVE_HTTP`; assert it reaches the (mock) transport rather than the synthetic envelope.
- **Evidence:** This run's `dice_query.go` deliberately used `PostQueryWithParams` instead of the generated `client.Query` specifically to dodge this; the original generated `sync.go` used `c.Query` and would have been verify-broken.
- **Related prior retros:** None (first GraphQL retro).

### 2. GraphQL query/SDL/scaffolding path doesn't serve the common Relay `viewer{edges{node}}` shape (Template gap)
- **What happened:** The first real GraphQL CLI could not use ANY of the generated GraphQL data layer; it was wholly hand-replaced.
- **Scorer correct?** N/A.
- **Root cause:** Three coupled gaps in the GraphQL path:
  1. `graphql_queries.go.tmpl` emits only `query{ field(first,after){ nodes { flat-scalars } } }` — no `viewer`/root wrapper, no Relay `edges { node }`, no nested object selections, no typed `where` inputs. DICE (and GitHub v4, Shopify Admin) use `viewer/connection → edges → node` with nested objects, so the emitted queries are structurally wrong and ship as dead constants.
  2. `internal/graphql/parser.go` `detectConnections` only recognizes connections with a `nodes` field and only walks root `Query` fields — a well-formed SDL with `viewer`-nested `edges`-connections produced **zero resources** ("at least one resource is required").
  3. The generator also emits REST data-source scaffolding (`data_source.go`, `search.go` REST resolution) that GraphQL CLIs never wire, leaving ~26 golangci-lint `unused` functions in DO-NOT-EDIT files.
- **Cross-API check:** The Relay `edges{node}` connection shape is the dominant GraphQL convention; provable the templates can't express it.
- **Frequency:** every GraphQL CLI whose schema uses edges-connections or a viewer/root wrapper (the common case).
- **Fallback:** hand-author the entire GraphQL data layer (this run: `dice_query.go`, `dice_resources.go`, rewritten `sync.go`). Large, recurring per-GraphQL-CLI cost.
- **Worth a Printing Press fix?** Yes, but it's a larger lift; the SKILL already says "GraphQL: scaffold only, build commands in Phase 3," so partial expectation exists. The concrete, safe improvement is: do not emit a *broken* query layer + dead REST scaffolding for GraphQL specs.
- **Inherent or fixable:** Fixable. Two options: (a) support the Relay `edges{node}` + root-wrapper shape in the query template and SDL parser (driven by the parsed schema, not hardcoded); or (b) a documented "GraphQL scaffold-only" mode that emits transport + store + framework + a generic `Query` passthrough, and suppresses the broken per-endpoint query constants and the REST data-source scaffolding.
- **Durable fix:** Prefer (b) as the cheap, safe first step (stop emitting broken/dead code), with (a) as the larger follow-up. Parameterize off the parsed schema — no hardcoded field names.
- **Test:** positive — generate from an `edges{node}` SDL; assert either correct edge-walking queries (a) or no broken query constants + no unused REST scaffolding (b). negative — a `nodes`-style SDL (Linear) still generates as today.
- **Evidence:** This run's spec test-generation produced `queries.go` with empty query fields and `nodes{id}`; the SDL parser rejected a viewer/edges SDL with "at least one resource is required."
- **Related prior retros:**
  - `cli-printing-press` issue #1654 — `extends`. #1654 is a regression in SDL resource-emit *count* against Linear (a `nodes`-style schema); this finding is the structural inability to handle `viewer`/`edges`-shaped schemas at all. Same component area (`internal/graphql` + GraphQL templates), different specific gap.

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | GraphQL transport rides verify-gated POST → sync noops under verify | generator | every GraphQL CLI | low (easy to miss) | small | keep gated path for real mutations |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 2 | GraphQL path can't serve Relay viewer/edges; emits broken queries + dead REST scaffolding | generator | every edges-shaped GraphQL CLI | low | medium (b) / large (a) | preserve nodes-style behavior for Linear-shaped schemas |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| 3 | scorecard `path_validity` is REST-path-literal-based; scores GraphQL CLIs 0/10 | Step B: n=1, low impact — dice still passed (77→81) and ship was never blocked. Real scorer mismatch but not worth maintainer attention yet; will resurface with stronger evidence as more GraphQL CLIs are printed. |
| 4 | README `## Configuration` rendered an empty config-file path | Step B / printed-CLI: the internal YAML spec authored this run omitted an explicit `config:` block; likely a spec-authoring omission rather than a generator default gap. n=1, no cross-CLI evidence. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| scorecard JSON lacks per-dimension detail | `scorecard --json` emits bare integer scores with no `reason`/`detail`, hard to diagnose a 0 programmatically | unproven-one-off (wishlist; mild diagnostic friction, no shipped defect) |
| sync --dry-run not honored | generated/rewritten sync hit the network under --dry-run | iteration-noise (one-line fix applied this run; the rewritten sync owns this, generated REST sync already guards) |

## Work Units

### WU-1: Route generated GraphQL reads through the ungated read path (from F1)
- **Priority:** P2
- **Component:** generator
- **Goal:** Generated GraphQL `Query`/`Mutate`(read)/`PaginatedQuery` reach real transport under `PRINTING_PRESS_VERIFY=1` instead of short-circuiting to the synthetic noop, so generated GraphQL `sync` is verify-correct.
- **Target:** `internal/generator/templates/graphql_client.go.tmpl` (the `Query`/`PaginatedQuery` POST calls), cross-checked against the `doRead`/`PostQueryWithParams` plumbing in `client.go.tmpl`.
- **Acceptance criteria:**
  - positive: generate a GraphQL CLI; under `PRINTING_PRESS_VERIFY=1` (no `LIVE_HTTP`), a `Query`-backed read reaches the (mock) transport and parses nodes.
  - negative: a genuine GraphQL mutation path (if emitted) still short-circuits under verify.
- **Scope boundary:** Only the read-verb routing for GraphQL; does not change query shape or the SDL parser.
- **Dependencies:** none.
- **Complexity:** small.

### WU-2: Stop emitting a broken GraphQL query layer + dead REST scaffolding for edges-shaped schemas (from F2)
- **Priority:** P3
- **Component:** generator
- **Goal:** For GraphQL specs whose connections use the Relay `edges{node}` shape and/or a `viewer`/root wrapper, the machine either emits correct edge-walking queries OR (cheaper first step) a scaffold-only surface — never broken root-`nodes` query constants plus unused REST data-source scaffolding.
- **Target:** `internal/generator/templates/graphql_queries.go.tmpl`, `internal/graphql/parser.go` (`detectConnections`, root-Query-only walk), and the REST data-source scaffolding emission gate for GraphQL specs.
- **Acceptance criteria:**
  - positive: generate from an `edges{node}` / viewer-wrapped SDL; assert no broken query constants and no `golangci-lint unused` REST data-source funcs (scaffold-only), or correct edge-walking queries (full support).
  - negative: a `nodes`-style SDL (Linear) generates exactly as today.
- **Scope boundary:** Schema-driven; no hardcoded field names. Coordinates with #1654 (SDL resource-emit area).
- **Dependencies:** none (independent of WU-1).
- **Complexity:** medium (scaffold-only) / large (full viewer-edges support).

## Anti-patterns
- None observed in this run's process beyond the GraphQL-path gaps above; the lean loop and gates worked.

## What the Printing Press Got Right
- The generated **GraphQL transport's generic `Query(ctx, query, variables)`** is genuinely reusable — hand-authored commands could send arbitrary correct queries through it (once routed via the read path).
- The generic **`resources` store** (resource_type + JSON data + FTS + sync_state) absorbed all 8 DICE entities + a derived `fans` table with zero schema work — ideal substrate for the transcendence analytics commands.
- **`verify-skill` + `validate-narrative`** caught a real bug (sync ignoring `--dry-run`) before ship.
- **Polish** cleanly removed the dead helpers orphaned by deleting the broken generated endpoint commands, rebuilt the README, and scrubbed synthetic PII — moving the scorecard 77→81 (B→A) without gaming.
- The **bearer-auth scaffolding** (config, doctor, client header, README) was correct from the spec's `auth:` block with `verify_query` — no auth hand-patching needed.
