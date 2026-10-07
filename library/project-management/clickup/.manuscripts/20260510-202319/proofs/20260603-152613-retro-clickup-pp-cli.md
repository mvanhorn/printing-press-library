# Printing Press Retro: clickup

## Session Stats
- API: clickup
- Spec source: catalog (OpenAPI, ClickUp v2 + v3)
- Scorecard: n/a (published-library CLI; scorecard/dogfood artifacts empty in this run)
- Verify pass rate: n/a (no workflow manifest)
- Fix loops: n/a (retro run on an installed library CLI, not a fresh generation)
- Manual code edits: 1 (local hotfix: 7 files, `team-id` float64 -> string)
- Features built from scratch: 0
- **Evidence basis:** generated CLI source (`~/printing-press/library/clickup/`) + live API repro. Session-independent — the defects are in committed generated code, not iteration history.

## Findings

### 1. Numeric query-param flags are float64 + serialized with `%v`, producing scientific notation (bug)
- **What happened:** `clickup-pp-cli task get <id> --custom-task-ids true --team-id 1234567` failed with `HTTP 400 SHARD_024 Invalid workspace id: 1.234567e+06`. The generator typed `--team-id` as a Go `float64` and serialized it into the query string with `fmt.Sprintf("%v", flagTeamId)`. For `float64` magnitudes >= ~1e6, `%v` (i.e. `%g`) switches to scientific notation, so `1234567` went out as `1.234567e+06`, which the API can't parse.
- **Scorer correct?** n/a — not a score-penalty finding (dogfood/verify did not exercise a custom-task-id lookup with a large workspace id, which is exactly why it shipped).
- **Root cause:** Generator (`internal/generator/`). Two compounding template decisions: (a) OpenAPI params typed `type: number` map to Go `float64` (confirmed: ClickUp's spec types `team_id` as `number`; petstore's `integer/int64` `petId` correctly maps to `int64`, so the mapping is spec-type-driven); (b) the query-param serialization template emits `fmt.Sprintf("%v", <floatFlag>)`. `%v` on a float64 is the wrong formatter for any identifier- or timestamp-shaped value.
- **Two failure modes from one cause:**
  - *Scientific notation* for values >= ~1e6 (IDs, epoch-millisecond timestamps). ClickUp has 42 float64 query flags; beyond `team-id` (x21) these include epoch-ms params `start-date`, `end-date`, `with-message-since` (always >= 1e12 -> always scientific notation) and numeric IDs `space-id`, `list-id`, `folder-id`.
  - *Silent precision loss* for integer values > 2^53 — float64 cannot represent them exactly. Vendor IDs are trending past this; the value would be corrupted, not just misformatted, with no error.
- **Cross-API check:** The `%v`-on-float64 code path is generic generator output, not ClickUp-specific. Breadth in the local library is honestly **thin**: only ClickUp exhibits it, because petstore uses `int64` IDs and readwise/youtube type their IDs as `string` (`StringVar`, where `%v` is harmless). So this is a **first sighting** in the 4-CLI local set. The generalizing argument is the *param shape*, not the API: any spec that types an ID or an epoch-millisecond timestamp as `number`/`integer` and carries it as a query param hits this, and epoch-ms timestamp query params (`*_after`, `*_since`, `due_date`) are common across REST APIs. That argument is reasoning, not three spec citations — see Step G.
- **Frequency:** subclass — APIs with `number`/`integer`-typed query params holding values >= ~1e6. Within such an API it is pervasive (ClickUp: 42 flags).
- **Fallback if the Printing Press doesn't fix it:** Low reliability. The failure is invisible at generation time (dogfood uses small/mocked values that format identically under `%v`), and the scientific-notation rejection only appears when a real large value is passed at runtime. The precision-loss mode produces no error ever. An agent reasoning over the CLI will not catch either unless it happens to test a >1e6 value live.
- **Worth a Printing Press fix?** Yes. The serialization half of the fix is provably safe (see Step C) and removes a silent failure class.
- **Inherent or fixable:** Fixable in the generator.
- **Durable fix:** Two levels, prefer both:
  1. **Serialization (high-confidence, zero-risk):** never serialize numeric query params with `%v`. Use `strconv.FormatFloat(f, 'f', -1, 64)` (exact, never scientific) for float-typed flags. This is a one-line template change with no downside.
  2. **Type mapping (higher-leverage, needs care):** map OpenAPI `type: integer` (and `number` with `format: int*`) to Go `int64`, not `float64`. For params named like identifiers (`*_id`, `*Id`), prefer `string` pass-through to dodge the 2^53 ceiling entirely. petstore already proves the generator can emit `int64`, so this is closing a `number` gap rather than new machinery.
- **Test:**
  - positive: a `number`-typed query param flag set to `1234567` serializes as `1234567`, and `1779431746395` (epoch ms) as `1779431746395`.
  - negative: a genuine fractional value (e.g. a `rate`/`score` param = `3.5`) still serializes as `3.5`, not `3` or `3.50000`.
- **Evidence:** `internal/cli/task_get.go:16,42,96` (float64 decl, `%v` serialization, `Float64Var` registration); live `400 SHARD_024`; `--dry-run` showed `team_id=1.234567e+06`; empirical Go check: `fmt.Sprintf("%v", 1234567.0)` -> `1.234567e+06`, `strconv.FormatFloat(1234567, 'f', -1, 64)` -> `1234567`.
- **Related prior retros / open issues:** **#2513** (`bug,retro,P1,comp:generator` — "MCP numeric path/query params rendered in scientific notation (makeAPIHandler %v on float64)") — `same` root cause, different emitter. #2513 covers the **MCP handler** (`makeAPIHandler`); this finding covers the **CLI command** serialization (`internal/cli/*.go`, `fmt.Sprintf("%v", flagX)`). Action: comment on #2513 to extend it to the CLI path + the precision-loss and `number`->float64 type-mapping dimensions, rather than file a duplicate. (youtube retro 20260529 unrelated.)
- **Step G case-against / why it survives:** Case-against — "only one local CLI exhibits it; the other three don't, so this looks like a ClickUp-spec quirk, P3 at most." Why that fails: the `%v` serialization is generic generator code, the *serialization* fix carries zero risk to any API (Step C), and it closes a **silent** failure (precision) plus a **hard** failure (rejection) that generation-time scoring structurally cannot catch. A safe fix for a silent runtime failure beats the thin-breadth objection. Filed; priority tempered to P2 (not P1) to reflect the breadth caveat honestly.

### 2. Query-param flags registered but never wired into the request on non-GET commands (bug)
- **What happened:** `task_delete.go` and ~20 sibling action/sub commands (`task_update`, `task_checklist_create`, `task_guest_add-to-task`, `task_tag_*`, `task_time_*`, `team_time-entries_*`, etc.) register `--custom-task-ids` and `--team-id` flags but **never reference them in `RunE`** — the values are dropped on the floor. Consequence: you cannot target a task by its custom ID on delete/update/and most write operations; the flags are accepted and silently ignored.
- **Scorer correct?** n/a — not score-derived. dogfood doesn't assert that a registered flag actually reaches the request.
- **Root cause:** Generator (`internal/generator/`). The query-param-wiring code is emitted for read/GET command templates (`task_get.go` *does* wire `team_id`) but omitted for the non-GET / action command templates, even though the same flags are still registered. A registered-but-unread flag is a template inconsistency between command shapes.
- **Cross-API check:** Confirmed concrete in ClickUp across ~21 files. **Not verified in other CLIs** — I did not establish that another catalog API registers a query-param flag on a non-GET command and fails to wire it. The mechanism (per-template wiring divergence) is plausibly systemic, but I have one CLI's evidence only.
- **Frequency:** this API confirmed; cross-API unproven.
- **Fallback if the Printing Press doesn't fix it:** Low — silently-ignored flags are invisible unless someone diffs registration against usage. No error is ever raised.
- **Worth a Printing Press fix?** Probably, but evidence is one-CLI. Filed at P3 pending cross-API confirmation rather than dropped, because it is unambiguously generator-emitted code and severe (silent no-op on write targeting), and a maintainer with generator access can confirm the template divergence in minutes.
- **Inherent or fixable:** Fixable — emit the same query-param wiring block across all command templates that register the flag, or (cleaner) fail-fast if a registered flag has no consumer.
- **Durable fix:** In the generator, unify query-param serialization so any registered query param is wired into the request regardless of HTTP method/command template. Add a generation-time assertion: every registered non-global flag must be read at least once (catches the whole class).
- **Test:**
  - positive: `task delete <id> --custom-task-ids true --team-id <ws>` issues `DELETE /task/<id>?custom_task_ids=true&team_id=<ws>`.
  - negative (assertion): generation fails if a command registers a flag that no codepath reads.
- **Related prior retros:** None.
- **Step G case-against / why it survives:** Case-against — "one CLI, no cross-API evidence; could be a ClickUp template path nobody else triggers; Step B fails the 3-API bar." That mostly holds, which is why it is **P3, not P2**. It survives (rather than Skip) only because it is provably generator-emitted, the failure is silent, and the proposed generation-time assertion is a cheap general guard that would catch the entire class regardless of how many APIs currently trip it.

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Numeric query params: float64 + `%v` -> scientific notation / precision loss | generator | subclass: numeric query params >= ~1e6 | low (silent) | small (serialization) / medium (type mapping) | serialization fix needs none; type-mapping fix scoped to integer/number formats |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | Query-param flags registered but not wired into non-GET requests | generator | this API confirmed; cross-API unproven | low (silent) | medium | unify wiring across command templates; add registered-flag-must-be-read assertion |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| — | — | (none — both Phase-3 candidates filed, at tempered priorities) |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| local hotfix to the 7 ClickUp files | Changed `team-id` float64 -> string in the installed CLI | printed-CLI (helps only this CLI; the durable fix is F1 in the generator) |

## Work Units

### WU-1: Serialize numeric query params without scientific notation / precision loss (from F1)
- **Priority:** P2
- **Component:** generator
- **Goal:** Numeric query-param flags reach the API as plain integers/decimals, never scientific notation, never precision-lossy.
- **Target:** Generator query-param serialization template(s) in `internal/generator/`, plus the OpenAPI `type` -> Go-type mapping.
- **Acceptance criteria:**
  - positive: a `number`/`integer` query param = `1234567` serializes as `1234567`; an epoch-ms value `1779431746395` serializes as `1779431746395`.
  - negative: a fractional value `3.5` still serializes as `3.5`.
- **Scope boundary:** Two independently shippable layers. Layer 1 (replace `%v` with `strconv.FormatFloat(f,'f',-1,64)`) is the safe minimum and should land regardless. Layer 2 (map `integer`/`number+format:int*` -> `int64`, identifier-named params -> `string`) is the fuller fix; gate it behind the profiler's detected type/format so non-integer numerics are untouched.
- **Dependencies:** none.
- **Complexity:** small (Layer 1) / medium (Layer 2).

### WU-2: Wire every registered query-param flag into the request, on all command shapes (from F2)
- **Priority:** P3
- **Component:** generator
- **Goal:** A registered query-param flag is never silently ignored; non-GET/action commands serialize their query params like GET commands do.
- **Target:** Generator command templates in `internal/generator/` (the non-GET/action template path that omits query-param wiring).
- **Acceptance criteria:**
  - positive: `task delete <id> --custom-task-ids true --team-id <ws>` sends both as query params.
  - negative: generation fails (or warns loudly) if a command registers a flag that no codepath reads.
- **Scope boundary:** Does not include redesigning flag registration; only closes the register-without-wire gap and adds the guard. Cross-API generality to be confirmed by the maintainer against the generator.
- **Dependencies:** none.
- **Complexity:** medium.

## Anti-patterns
- `fmt.Sprintf("%v", <float64>)` for anything that must round-trip as an integer. `%v`/`%g` is display formatting; query params need value-preserving formatting.
- Treating opaque identifiers as quantities (`float64`/`number`) at all. IDs are strings that happen to look numeric.
- Registering a CLI flag without a generation-time check that something consumes it — silent no-op flags are worse than a missing flag.

## What the Printing Press Got Right
- The `--dry-run` flag made root-causing instant: it printed `team_id=1.234567e+06` without sending, turning a vague `400` into an obvious formatting bug in one command.
- The spec-type-driven mapping already emits `int64` correctly for petstore's `integer/int64` IDs — the machinery to do the right thing exists; `number` is the gap.
- `doctor`, `--agent`, `--select`, and the local SQLite `sync`/`search` all worked exactly as documented and made the rest of the session friction-free.
