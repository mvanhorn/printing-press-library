# Printing Press Retro: Monarch

## Session Stats
- API: monarch
- Spec source: hand-authored internal YAML (informed by browser-sniff capture against api.monarch.com via browser-use + user's logged-in Chrome profile)
- Scorecard: 80/100 → 83/100 after polish (Grade A)
- Verify pass rate: 100%
- Fix loops: 2 generator failures + 2 manual restoration cycles (polish triggered regen that reverted hand fixes twice)
- Manual code edits: ~12 (5 distinct, applied 2-3× each across regen cycles)
- Features built from scratch: 11 novel commands + GraphQL operation registry (queries.go) + Token auth prefix + endpoint-path const

## Findings

### F1. Generator emits unused `client` imports in param-less endpoint command files (Bug)
- **What happened:** When an endpoint has no `params` and no `body` declared in the spec, the generator emits `import "monarch-pp-cli/internal/client"` at the top of the command file but never references `client.*`. Go fails the build with "imported and not used."
- **Scorer correct?** N/A — this fails `go build` outright.
- **Root cause:** Endpoint command template (`internal/generator/templates/`) emits the import unconditionally. The body of the command uses `flags.newClient()` (which returns `*client.Client`) but never names `client` as a package qualifier — so Go strictness fires.
- **Cross-API check:** Almost universal. Param-less endpoints exist in nearly every spec.
- **Frequency:** every API
- **Fallback if the Printing Press doesn't fix it:** Agent strips the import manually after each regen; `goimports` would also fix but the generator should emit clean code.
- **Worth a Printing Press fix?** Yes. Mechanical, high frequency, currently breaks `go build` on first regen for affected specs.
- **Inherent or fixable:** Fixable. Either (a) emit the import only when `client.X` is referenced in the file body, or (b) reference `client` in the generated body so the import is always used (e.g., a typed return-value expression).
- **Durable fix:** Audit the per-endpoint command template; remove the `client` import when no `client.*` symbol appears in the rendered body, OR add a `var _ = client.Header{}` no-op (less clean). Prefer (a).
- **Test:** Generate a CLI with at least one endpoint that has zero params and zero body. Assert `go build` passes without manual import-stripping. Negative: ensure files that DO use `client.X` still compile.
- **Evidence:** `accounts_list.go:12` — `import "monarch-pp-cli/internal/client"` followed by `flags.newClient()` only; same pattern in `budgets_list.go`, `categories_list.go`, `goals_list.go`, `recurring_list.go`, `tags_list.go`, `transactions_get.go`, `transactions_list.go` (8 files in this run).
- **Related prior retros:** None matched in `~/printing-press/manuscripts/*/proofs/*-retro-*.md`.

### F2. GraphQL-aware client emission is half-implemented for /graphql-only specs (Template gap, Default gap)
- **What happened:** The generator detected that every endpoint in the spec POSTs to `/graphql` and emitted `internal/client/graphql.go` with a `Query()` method, `GraphQLAccessDeniedError` type, and operation-name handling. But (a) `const graphqlEndpointPath = ""` was left empty, breaking every Query() call, (b) the per-endpoint command templates still POST a literal `{}` body to `path` instead of calling `c.Query(QueryConstant, vars)`, and (c) `internal/client/queries.go` emits placeholder GraphQL like `query($first: Int!, $after: String) { (first: $first, after: $after) { nodes { id } pageInfo { hasNextPage endCursor } } }` — note the missing field name after `(`, which is unparseable.
- **Scorer correct?** N/A — verify uses `--dry-run` and doesn't make real HTTP calls, so the half-implementation passes verify but fails any live invocation.
- **Root cause:** Generator partially recognizes the GraphQL shape (it emits the GraphQL helper file) but doesn't follow through. Three template locations need wiring:
  - `internal/client/graphql.go.tmpl` — `graphqlEndpointPath` should be set from the spec's shared endpoint path when all paths converge.
  - `internal/client/queries.go.tmpl` — placeholder body is invalid GraphQL; either skip emission entirely (let the agent hand-author) or emit just the const declarations as empty strings with `// TODO: hand-author` comments.
  - Per-endpoint command templates — when the spec's `path: /graphql` and `method: POST` for every endpoint, command bodies should call `c.Query(<resource>_<op>Query, vars)` instead of `c.Post(path, body)`.
- **Cross-API check:** GraphQL-only specs are a meaningful subclass. Concrete cases:
  - **Linear** (catalog has Linear; their public API is GraphQL-only at `api.linear.app/graphql`).
  - **GitHub GraphQL v4** (`api.github.com/graphql`; widely documented endpoint).
  - **Shopify Admin GraphQL** (`*/admin/api/<version>/graphql.json` — explicitly referenced in the existing graphql.go template's comment as a tenant-versioned graphql endpoint).
- **Frequency:** subclass:graphql-only-spec
- **Fallback if the Printing Press doesn't fix it:** Agent hand-authors queries.go, sets `graphqlEndpointPath`, and refactors per-endpoint commands to call `c.Query()`. Friction multiplies because the agent has to do this in EVERY regen cycle (see F4).
- **Worth a Printing Press fix?** Yes. The generator already emits the helper scaffolding (`graphql.go`); finishing the wiring is high-leverage and unblocks the GraphQL subclass entirely.
- **Inherent or fixable:** Fixable. The signal that a spec is GraphQL-only is mechanical (every endpoint shares one path + method=POST). When that signal fires, the generator should emit GraphQL-aware command bodies and wire the const.
- **Durable fix:**
  1. Detect "GraphQL-only spec" in the spec parser when ≥80% of endpoints share a single `path` with `method: POST`. Set `APISpec.GraphQLEndpointPath` to that path.
  2. Render `graphqlEndpointPath` in `graphql.go.tmpl` from `APISpec.GraphQLEndpointPath`.
  3. Either drop `queries.go` template emission (let SKILL Phase 3 hand-author) or emit empty const declarations with a comment explaining the const is for hand-authoring.
  4. In per-endpoint command templates, when the parent spec is GraphQL-only, emit `c.Query(<ResourceCmd>Query, variables)` against the per-endpoint operation constant instead of `c.Post(path, body)`.
- **Test:** Generate from a GraphQL-only spec (e.g., a fixture mirroring Monarch's shape). Assert `graphqlEndpointPath` is non-empty and matches the spec's shared path. Assert per-endpoint commands compile and reference `c.Query(<Const>Query, ...)`. Negative: REST specs should still emit `c.Post()` and a non-GraphQL `client.go`.
- **Evidence:** Three artifacts in this run — `internal/client/graphql.go:60` (`const graphqlEndpointPath = ""`), `internal/client/queries.go:8-15` (placeholder query with missing field name), `internal/cli/accounts_list.go:33` (`path := "/graphql"; var body map[string]any; ... data, statusCode, err := c.Post(path, body)`). Live testing returned HTTP 400 from Monarch on the empty-body POST.
- **Related prior retros:** None.

### F3. Spec's `auth.prefix` field is ignored by the config template (Bug, Assumption mismatch)
- **What happened:** Internal YAML spec declared `auth: { type: bearer_token, prefix: "Token", env_vars: [MONARCH_TOKEN] }` but the generated `internal/config/config.go` hardcoded `"Bearer " + c.MonarchToken` regardless. Manually replaced both branches with `"Token "`.
- **Scorer correct?** N/A — `--dry-run` paths don't exercise the auth header against a real server.
- **Root cause:** `internal/generator/templates/config.go.tmpl` (or the equivalent for `bearer_token` auth type) appears to render a literal `"Bearer "` string instead of pulling from `APISpec.Auth.Prefix`.
- **Cross-API check:** Non-Bearer prefixes happen on:
  - **Monarch** (`Authorization: Token <session>` — verified live in this run)
  - **GitLab Personal Access Tokens** (`PRIVATE-TOKEN` header, but also `Authorization: Bearer ` for OAuth — split by token type, documented in their auth docs)
  - **GitHub server-to-server tokens** historically use `Authorization: token <pat>` (lowercase `token`), still accepted alongside `Bearer`
- **Frequency:** subclass:non-bearer-prefix
- **Fallback if the Printing Press doesn't fix it:** Agent edits config.go after every regen — same edit, recurring, no per-API insight required.
- **Worth a Printing Press fix?** Yes. The spec already declares the prefix; the template just needs to read it.
- **Inherent or fixable:** Fixable. Replace literal `"Bearer "` with `{{ .Auth.Prefix }}{{ if ne .Auth.Prefix "" }} {{ end }}` (with default fallback to `Bearer ` when `Auth.Prefix` is empty).
- **Durable fix:** Update config.go.tmpl to render the prefix from `APISpec.Auth.Prefix`, defaulting to `Bearer` when unset for backward compatibility.
- **Test:** Generate from a spec with `auth.prefix: "Token"` and assert the rendered `config.go` builds an `Authorization: Token <token>` header. Negative: a spec with `auth.prefix` unset (or `"Bearer"`) should still produce the existing header.
- **Evidence:** `internal/config/config.go:89,93` — both `c.MonarchToken` and `c.AccessToken` branches hardcoded `"Bearer " + ...` despite spec saying `prefix: "Token"`. Direct curl with `Authorization: Token <session>` returned real Monarch data; `Authorization: Bearer <session>` would have 401'd.
- **Related prior retros:** None.

### F4. `printing-press generate --force` preserve-list is too narrow; clobbers internal/client and internal/config (Recurring friction)
- **What happened:** The `--force` flag's documented contract preserves hand-authored `internal/cli/*.go` files. But re-running generate (and polish, which appears to internally re-run templates) clobbered hand-edits to:
  - `internal/client/queries.go` (rewrote real Monarch GraphQL queries back to placeholders)
  - `internal/client/graphql.go` (reverted `graphqlEndpointPath = "/graphql"` back to `""`)
  - `internal/config/config.go` (reverted `"Token "` back to `"Bearer "`)
  - `internal/cli/transactions.go`, `budgets.go`, `accounts.go`, `categories.go`, `recurring.go`, `cashflow.go`, `goals.go` (parent commands lost their `AddCommand(newXxxNovelCmd(flags))` lines for novel children, even though `internal/cli/*.go` IS in the preserve list)
- **Scorer correct?** N/A — these files build cleanly when freshly generated, just with the wrong content.
- **Root cause:** The preserve-list logic (in the generator's force-recreation code) likely matches "hand-authored" by some heuristic that misses parent command files (which the generator "owns" in spirit but agents commonly extend) and excludes `internal/client/` and `internal/config/` entirely.
- **Cross-API check:** Affects every CLI that goes through the polish skill's diagnostic loop OR a manual regen after Phase 3 hand-build. Polish runs on every generated CLI. So this is **every CLI**.
- **Frequency:** every API
- **Fallback if the Printing Press doesn't fix it:** Agent re-applies the same fixes after every regen. Wasteful and error-prone — I had to re-apply the same three edits twice in this run alone.
- **Worth a Printing Press fix?** Yes. Polish makes regen common; the preserve list needs to be right.
- **Inherent or fixable:** Fixable. The fix has two angles:
  1. **Broaden the preserve-list** to include `internal/client/` and `internal/config/` when those files have been modified post-generate. Alternatively, treat any file whose content diverges from a fresh template render as "hand-authored" and preserve it.
  2. **Detect parent-command merging** in `internal/cli/<resource>.go` files: if the existing file has additional `AddCommand` lines that aren't in the regenerated template, merge them rather than overwriting.
- **Durable fix:** Add a `.printing-press-preserve` allow-list (or annotation comments at file headers) that the regenerator respects. Document the contract in the SKILL: which files are owned by the generator vs. extensible by agents. Polish should respect this same contract.
- **Test:** Hand-edit `internal/client/queries.go` and `internal/cli/transactions.go` (adding an `AddCommand` line to the parent). Run `printing-press generate --force` again. Assert both files keep the hand-edits. Negative: a file that the agent didn't touch should still be regenerated.
- **Evidence:** Three system reminders in this session noting that `internal/client/graphql.go`, `internal/config/config.go`, and `internal/cli/root.go` were "modified, either by the user or by a linter" after polish ran — those reminders surfaced because the regen reverted my hand-edits, then I had to re-apply them.
- **Related prior retros:** None matched (`grep -l preserve` against `~/printing-press/manuscripts/*/proofs/*-retro-*.md` returned nothing in the brief sweep).

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Generator emits unused `client` imports in param-less endpoint files | generator | every API | Low (Go fails build) | small | None — pure cleanup of an over-emitted import |
| F2 | Complete GraphQL-aware client emission (graphqlEndpointPath wiring + per-endpoint Query() calls + drop placeholder queries) | generator | subclass:graphql-only-spec (Linear, GitHub GraphQL, Shopify Admin) | Medium (agent has to fix all three locations every time) | medium | Detect via "≥80% of endpoints share path + method=POST"; REST specs unaffected |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F3 | Spec's `auth.prefix` field is ignored by the config template | generator | subclass:non-bearer-prefix (Monarch Token, GitLab PRIVATE-TOKEN, legacy GitHub `token`) | Medium (agent re-edits config.go each regen) | small | Default to `Bearer` when unset |
| F4 | `--force` preserve-list too narrow; clobbers internal/client, internal/config, and parent-command merges | generator | every API (polish triggers regen on every CLI) | Low (loses real fixes silently) | medium | Need to detect "hand-modified" without false positives on whitespace/format diffs |

### Skip
| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---------|-------|--------------------------------------------------|
| F6 (initial candidate) | `traffic-analysis.json` schema mismatch — SKILL says to pass `--traffic-analysis` for browser-sniffed specs but the schema isn't documented; my hand-authored advisory file failed with "missing version" | **Step B**: only one API in evidence (this run); other browser-sniffed runs would have used `printing-press browser-sniff` which writes the right schema. The friction here was self-inflicted (I hand-authored advisory data instead of running browser-sniff). |
| F9 (initial candidate) | Polish skill's strict publish-validate gate marks `phase5-acceptance status:pass tests_failed:1` as FAIL despite SKILL contract saying 5/6 quick-check is acceptable | **Step B**: only one CLI of evidence. Could be a real polish/SKILL drift issue but needs more sightings before filing — the gate behavior may be intentional and the SKILL prose is what's wrong. |
| F10 (initial candidate) | Browser-sniff via browser-use needed manually-written `window.fetch` interceptor; SKILL's reference-file describes browser-use CLI commands but not the JS pattern for capturing GraphQL POST bodies that survive SPA navigation | **Step G case-against stronger**: this is a SKILL recipe addition, not a generator change. The browser-sniff reference could include a default interceptor template, but it's documentation-shaped, not template-shaped. Better as a SKILL PR than a retro finding. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| GraphQL filter type names not derivable | `AccountFilters` (singular) vs `AccountsFilters` (plural) couldn't be guessed without curl iteration; introspection is disabled | API-quirk: the Monarch schema is reverse-engineered, no introspection available; iteration is expected for this class |
| Phase 5 token capture flow | The user had to manually grab a session token from Chrome DevTools because `auth login --chrome` was deferred to v0.2 in this run | printed-CLI: deferring auth login --chrome was a scope decision in this CLI's Phase 3, not a machine gap |
| browser-sniff capture wiped on `location.reload()` | The injected interceptor died when the page hard-reloaded; had to use SPA `pushState` navigation instead | printed-CLI / iteration-noise: a one-time discovery that informed how I wrote the next eval; not a Printing Press fix |

## Work Units

### WU-1: Stop emitting unused `client` imports from param-less endpoint command templates (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** Generated endpoint command files compile without manual import-stripping when the endpoint has no params and no body.
- **Target:** Per-endpoint command template under `internal/generator/templates/` — likely the `<verb>_endpoint.go.tmpl` family or whichever template emits the `import (...)` block.
- **Acceptance criteria:**
  - **Positive:** Generate from a spec where at least one endpoint has empty `params` and empty `body`. Assert `go build` passes without manual edits to the generated files.
  - **Negative:** Endpoints that DO reference `client.X` (e.g., commands that read `client.SomeHelper`) keep the import.
- **Scope boundary:** Only touches the import block in per-endpoint command templates. Does not refactor how `flags.newClient()` is called.
- **Dependencies:** None.
- **Complexity:** small

### WU-2: Complete GraphQL-aware client emission for /graphql-only specs (from F2)
- **Priority:** P1
- **Component:** generator
- **Goal:** When a spec's endpoints all POST to a single shared path, the generator emits a working GraphQL-aware client: `graphqlEndpointPath` set, per-endpoint commands call `c.Query(<Const>Query, vars)`, and `queries.go` is either omitted or filled with hand-authoring placeholders that are valid GraphQL syntax.
- **Target:** Three template locations: `internal/client/graphql.go.tmpl` (const wiring), `internal/client/queries.go.tmpl` (drop or fix placeholder body), and per-endpoint command template (`c.Query` instead of `c.Post`). Plus a spec-parser detector (`internal/spec/` or `internal/openapi/`) that sets `APISpec.GraphQLEndpointPath` when ≥80% of endpoints share a path + method=POST.
- **Acceptance criteria:**
  - **Positive:** Generate from a fixture spec mimicking Monarch's shape (every endpoint POST `/graphql`). Assert `internal/client/graphql.go` has `const graphqlEndpointPath = "/graphql"`, per-endpoint commands call `c.Query(<Const>Query, ...)`, and `queries.go` either doesn't exist or contains valid (if minimal) GraphQL syntax.
  - **Negative:** A REST spec (e.g., the existing loops.yaml fixture) generates exactly as it does today — `graphqlEndpointPath` not declared, commands call `c.Post()`, no `queries.go` emitted.
- **Scope boundary:** Does not implement schema introspection or auto-derive query bodies. The generator emits placeholders; agents hand-author the actual queries (this is the SKILL's existing GraphQL-only contract: "Generate scaffolding only in Phase 2; Build real commands in Phase 3 using a GraphQL client wrapper").
- **Dependencies:** WU-1 (since GraphQL endpoint commands also have the unused-import issue).
- **Complexity:** medium

### WU-3: Honor spec's `auth.prefix` field in the config template (from F3)
- **Priority:** P2
- **Component:** generator
- **Goal:** A spec declaring `auth.prefix: "Token"` produces a generated config.go that builds an `Authorization: Token <token>` header.
- **Target:** `internal/generator/templates/config.go.tmpl` (or equivalent for the `bearer_token`, `api_key`, etc. auth types where a prefix is meaningful).
- **Acceptance criteria:**
  - **Positive:** Generate from a spec with `auth.prefix: "Token"`; assert the rendered `Authorization` header is `"Token " + token`.
  - **Negative:** A spec with `auth.prefix` unset still produces `"Bearer " + token` (default).
- **Scope boundary:** Only touches the prefix string in the rendered header. Doesn't change auth flow or env-var resolution.
- **Dependencies:** None.
- **Complexity:** small

### WU-4: Broaden `--force` preserve list to cover internal/client, internal/config, and parent-command extensions (from F4)
- **Priority:** P2
- **Component:** generator
- **Goal:** `printing-press generate --force` preserves agent-modified files in `internal/client/`, `internal/config/`, and parent-command merge points in `internal/cli/<resource>.go`.
- **Target:** The force-recreation logic in `internal/generator/` (likely a `regen.go` or `merge.go`) plus the file-classification code that decides what counts as "hand-authored."
- **Acceptance criteria:**
  - **Positive:** Hand-edit a generated `internal/client/queries.go` and `internal/config/config.go`, plus add an `AddCommand` line in `internal/cli/transactions.go`. Re-run `printing-press generate --force`. Assert all three edits persist.
  - **Negative:** Untouched generator-emitted files in `internal/cli/` (e.g., a freshly-generated endpoint command) still get regenerated when the underlying template changes.
- **Scope boundary:** Does not touch the polish skill's diagnostic loop. Polish should pick up the broader preserve-list as a natural side effect once the generator respects it.
- **Dependencies:** None (independent of WU-1 through WU-3).
- **Complexity:** medium

## Anti-patterns
- The placeholder GraphQL queries emitted to `queries.go` (`(first: $first, after: $after) { ... }` with a missing field name) look like real GraphQL but are syntactically broken. They mislead agents into thinking the queries.go is closer to working than it is, and pass any non-execution check (verify, scorecard) because the file compiles as a Go string. Either omit the file or emit valid-but-empty queries (e.g., `query Placeholder { __typename }`).
- The generator already emits a sophisticated `graphql.go` helper (with `Query`, `Mutate`, `PaginatedQuery`, typed `GraphQLAccessDeniedError`) but stops short of wiring it into per-endpoint commands. Half-implementations are the worst kind: they look like the work is done.

## What the Printing Press Got Right
- **Auto-detection of GraphQL-shaped specs.** The generator noticed that every endpoint POSTs to `/graphql` and emitted `internal/client/graphql.go` with the right primitives (Query, Mutate, access-denied detection). The wiring is incomplete (F2) but the foundation is there.
- **The browser-sniff gate marker file contract.** Phase 1.7's `browser-browser-sniff-gate.json` enforcement worked perfectly — Phase 1.5 wouldn't have proceeded if the marker were missing, which prevented me from accidentally skipping the user prompt.
- **The `printing-press shipcheck` umbrella.** Running 6 legs in canonical order with a clear summary table made it trivial to spot exactly which leg (validate-narrative) was failing and re-run only that leg standalone. The umbrella's verdict was the source of truth and never lied.
- **Polish skill's diagnostic-fix-rediagnose loop.** Polish caught issues I'd missed (missing `.printing-press.json`, dead helper functions, stale install section in SKILL.md) and applied fixes mechanically. The forked-context execution kept its chatter out of my main flow.
- **Phase 1.5c.5 novel-features subagent.** The customer-model → generate → adversarial-cut pass produced 11 strong novel features grounded in concrete personas (Priya, Marcus, Devon, Sasha) drawn from the brief. The kill-and-keep table forced me to drop weak candidates (transaction velocity, holdings drift) before they hit the manifest.
