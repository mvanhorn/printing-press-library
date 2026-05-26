# Printing Press Retro: adminbyrequest

## Session Stats
- API: adminbyrequest (endpoint privilege management; 5 sub-APIs)
- Spec source: hand-authored internal YAML (docs-mode generation failed on Windows, see F6)
- Scorecard: 90/100 (Grade A)
- Verify pass rate: 100%
- Fix loops: 1 (verify-skill + validate-narrative caught two SKILL.md/research.json recipe drifts; one quick fix loop)
- Manual code edits: 7 novel-feature files (~700 LOC, expected) + 3 wiring edits (2 `Hidden:` removals + 1 `correlate` SQL fallback)
- Features built from scratch: 7 novel features (correlate, requests repeat-offenders, requests denied-reasons, inventory drift, inventory risk-score, quota show/forecast, report compliance)

## Findings

### F1: Multi-endpoint resource parents emit `Hidden: true` and disappear from root --help (template gap)

- **What happened:** The internal-YAML spec declares `resources.auditlog`, `resources.inventory`, `resources.requests` — each with 2+ endpoints. The generator emits each parent with `Hidden: true`, which removes the parent from the root `--help` "Available Commands" listing. Agents discovering capabilities via `--help` cannot see that `inventory` or `requests` exists; only `events` (which got promoted to root because it had only one endpoint) is visible. Subcommand discoverability collapses for resources that don't qualify for promotion.
- **Scorer correct?** N/A — this is a generator behavior, not a score-driven finding.
- **Root cause:** Generator templates for the parent-resource command set `Hidden: true` by default for multi-endpoint resources. The promotion-to-root flow only fires for single-endpoint resources. Multi-endpoint resources end up "registered but invisible."
- **Cross-API check:** Reproduces on any internal-YAML spec with multi-endpoint resources. Most real APIs have multi-endpoint resources, so this affects most generated CLIs.
- **Frequency:** every multi-endpoint resource in every CLI
- **Fallback if the Printing Press doesn't fix it:** Agent must read the generated `internal/cli/<resource>.go` after generation and remove `Hidden: true`. Easy to miss — the binary still WORKS for `<cli> inventory list` typed directly, just doesn't appear in help.
- **Worth a Printing Press fix?** Yes. Either (a) unhide multi-endpoint parents by default, or (b) only hide when a same-named root-promoted command exists.
- **Inherent or fixable:** Fixable. One template change.
- **Durable fix:** In the resource-parent template, drop `Hidden: true` unconditionally for multi-endpoint resources. Single-endpoint resources continue to be hidden because their `list` is already promoted to root.
- **Test:** Positive — generate a CLI with a multi-endpoint resource; assert `<cli> --help` lists the resource. Negative — generate a single-endpoint resource; assert it stays promoted to root and the (hidden) parent doesn't add noise.
- **Evidence:** During AdminByRequest's Phase 3 wiring, both `inventory` and `requests` parents had `Hidden: true`. I had to hand-edit both to unhide them so my new transcendence sub-commands (`inventory drift`, `inventory risk-score`, `requests repeat-offenders`, `requests denied-reasons`) would appear in `<cli> --help`. Without that edit, agents using only `--help` for discovery would not see them.
- **Related prior retros:** None (no local retros; gh API unauthenticated)

### F2: Store's `id` TEXT column inconsistent with JSON `$.id` for int-typed top-level id fields (bug, root cause uncertain)

- **What happened:** My hand-written `correlate` command queried `SELECT data, ... FROM auditlog WHERE id = ?` with the audit id `50461167` as a string. The query returned zero rows even though `export auditlog` showed the row was present with that id. Switching the predicate to `CAST(json_extract(data, '$.id') AS TEXT) = ?` resolved it. Spec declared id as `type: int`; store schema is `id TEXT PRIMARY KEY`.
- **Scorer correct?** N/A.
- **Root cause (uncertain — two candidates):**
  1. **Store upsert doesn't populate the `id` TEXT column from a JSON-int `$.id`.** The column ends up NULL for that row, so `WHERE id = '50461167'` matches nothing, but `json_extract(data, '$.id')` still resolves through the JSON blob.
  2. **SQLite type affinity.** With `id TEXT PRIMARY KEY` and an integer-looking value, SQLite still stores under integer affinity. Query `WHERE id = '50461167'` (string parameter) doesn't auto-cast to integer-affinity comparison without an explicit `CAST`.
  Disambiguation: open the synced SQLite file in `sqlite3` and run `SELECT id, typeof(id) FROM auditlog LIMIT 3`. If `typeof(id) = 'null'`, root cause is (1). If `typeof(id) = 'integer'`, root cause is (2).
- **Cross-API check:** Any spec where the top-level resource id is typed `int`. Common: GitHub (issue/PR/repo numbers), Linear (some IDs are numeric), Discord (Snowflakes), AdminByRequest, most legacy REST APIs.
- **Frequency:** subclass:int-id — every CLI whose top-level id is integer-typed.
- **Fallback if the Printing Press doesn't fix it:** Every hand-written novel command querying the store by id must use the `json_extract` fallback. Easy to forget — silent "0 rows" instead of an error.
- **Worth a Printing Press fix?** Yes. Even the short-term mitigation (a `cliutil.LookupByID` helper) compounds across novel features in every int-id CLI.
- **Inherent or fixable:** Fixable.
- **Durable fix (two-step):**
  1. *Cheap, ships now:* emit `cliutil.LookupByID(ctx, db, table, idStr) (json.RawMessage, error)` into every printed CLI; performs CAST + json_extract fallback; returns a typed `ErrNotFound` on miss. Update the SKILL's novel-feature template to point at it.
  2. *Root-cause fix after disambiguation:* patch the store-upsert path or the schema to ensure `id` is queryable as text regardless of input type.
- **Test:**
  - positive: helper finds the row by id whether the spec declared `type: int` or `type: string`.
  - negative: looking up a non-existent id returns `ErrNotFound`, not a zero-value row.
- **Evidence:** correlate.go's initial query against `id` column returned `sql.ErrNoRows` for known-good IDs. After switching to `CAST(json_extract(data, '$.id') AS TEXT) = ? OR id = ?`, the query matched. Reproducible at `~/printing-press/library/adminbyrequest/internal/cli/correlate.go`.
- **Related prior retros:** None locally.

### F3: FTS5 `search` command does not quote/escape query strings, hyphens parse as negation (bug)

- **What happened:** Running `<cli> search "LAPTOP-CHRISC"` returns `SQL logic error: no such column: CHRISC (1)`. FTS5 treats `-` in unquoted MATCH expressions as the binary negation operator (`A - B` = "A but not B"), and `CHRISC` is then parsed as a column reference. Any hyphenated identifier (`API-keys`, `feature-branch`, `LAPTOP-X`, `pull-request-N`) breaks search.
- **Scorer correct?** N/A — never surfaced in scorecard.
- **Root cause:** The `search` command template builds the MATCH clause from raw user input without quoting. FTS5 requires the entire query to be `"<phrase>"`-wrapped to disable operator parsing.
- **Cross-API check:** Any CLI that ships `search` will hit this on hyphenated input. Examples with evidence:
  - GitHub CLI (in catalog) — branch names like `feature-x`, label names like `good-first-issue` — hyphens common.
  - Linear CLI (likely in catalog) — issue IDs like `LIN-1234`, project labels with hyphens.
  - HubSpot CLI (in catalog) — domain identifiers like `acme-corp.com`.
- **Frequency:** every CLI's `search` command, triggered whenever a user pastes a hyphenated identifier.
- **Fallback if the Printing Press doesn't fix it:** None — the user gets a SQL error with no guidance. Workaround "wrap your query in double quotes" is non-obvious.
- **Worth a Printing Press fix?** Yes. Cheap template change, broad impact.
- **Inherent or fixable:** Fixable. Quote at the SQL builder.
- **Durable fix:** In the `search` template's SQL builder, wrap the user-supplied MATCH input in `"…"` after escaping any internal `"` (replace `"` with `""`). Document the FTS5 special-syntax escape hatch (e.g. `--fts-raw` flag) for power users who actually want negation.
- **Test:** Positive — `search "LAPTOP-CHRISC"` returns matching rows. `search "foo-bar"` returns hits without SQL error. Negative — backslash and quote characters in input still escape cleanly; the FTS5 query plan still uses the FTS index, not a table scan.
- **Evidence:** Phase 5 live test attempted `<cli> search "LAPTOP-CHRISC" --data-source local --json` and got `Error: search inventory failed: SQL logic error: no such column: CHRISC (1)`.
- **Related prior retros:** None

### F4: `export --data-source local` still calls the live API (bug — confirmed during retro)

- **What happened:** Running `<cli> export <resource> --data-source local` with no API key in the environment retried HTTP 500s from upstream three times. The `--data-source` flag's documented value `local` says "synced data only," but the export resolver does not actually short-circuit; it still tries to fetch from the API.
- **Scorer correct?** N/A — runtime failure, no scoring signal.
- **Root cause:** The `export` command's `--data-source local` branch either (a) doesn't dispatch reads to the store path at all, or (b) dispatches but falls through to live when the resource isn't in the local store. The bug is in the generator template that emits `export.go` (or the shared `data_source.go` resolver it imports).
- **Cross-API check:** Every generated CLI emits `export` and `--data-source` from the same template, so the bug ships everywhere.
- **Frequency:** every CLI; user-visible whenever someone passes `--data-source local` (intent: offline read).
- **Fallback if the Printing Press doesn't fix it:** Users notice when offline use fails; they sync first or supply credentials. The "offline export from a stale store" workflow silently doesn't work as advertised.
- **Worth a Printing Press fix?** Yes — documentation and behavior disagree.
- **Inherent or fixable:** Fixable.
- **Durable fix:** In the `export` template's `--data-source local` branch, dispatch reads exclusively to the store path. If the store has no rows for the requested resource, return a clear "no local data for `<resource>` — run sync first" error with exit code 2 (usage). Never make an HTTP call when `local` was explicit.
- **Test:**
  - positive: with `unset <CLI>_API_KEY` and a synced store, `export <resource> --data-source local --format jsonl` returns rows without any HTTP request. Verify with a process tracer or by running offline.
  - negative: `--data-source auto` and `--data-source live` continue to fetch from the API as today.
- **Evidence:** During the retro, confirmed via `unset ADMINBYREQUEST_API_KEY; <cli> export auditlog --format jsonl --limit 1 --data-source local` → returned three rounds of `server error 500, retrying...`. The same call with `--data-source live` correctly hit the live API; the call with `--data-source local` should have short-circuited but didn't.
- **Related prior retros:** None locally.

### F5: Internal-YAML spec format reference does not document `auth.verify_path`, so `doctor` always reports "Credentials: present (not verified)" (skill instruction gap)

- **What happened:** `<cli> doctor` reports `WARN Credentials: present (not verified — set auth.verify_path in spec for an API acceptance check)`. The doctor template clearly expects an `auth.verify_path` field, and the spec parser presumably supports it (the message names it directly), but `skills/printing-press/references/spec-format.md` does not list this field anywhere. Without doc support, the agent writing a spec from scratch has no reason to add it. Every CLI authored from the internal-YAML format ships with the WARN. The portal-facing CLI looks like it half-works.
- **Scorer correct?** Partially — the scorecard correctly notes that doctor is informationally complete; it doesn't penalize for the WARN because the spec is missing data. The WARN itself is fair, but the format ref's silence about the field guarantees it always fires.
- **Root cause:** Documentation gap in `skills/printing-press/references/spec-format.md`. The Complete Schema Reference enumerates `auth.{type, header, format, env_vars, scheme, in}` but omits `verify_path`.
- **Cross-API check:** Affects every internal-YAML-authored CLI. Three named with evidence:
  - producthunt-spec.yaml (in `catalog/specs/`) — no `auth.verify_path`, just `auth.type: none`.
  - shopify-2026-04-wrapper.yaml (in `catalog/specs/`) — would similarly omit the field unless the format ref mentioned it.
  - AdminByRequest (this run) — caught the WARN at doctor time.
- **Frequency:** every internal-YAML spec written by an agent reading the format reference (i.e. most non-OpenAPI generations going forward).
- **Fallback if the Printing Press doesn't fix it:** Agent has to read the doctor source code to discover the field, then back-port it to the spec, then regen. The WARN message names the field, which helps if an agent reads the doctor output carefully.
- **Worth a Printing Press fix?** Yes. One-line addition to the format reference, plus a one-sentence "When to set it" note.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Add to `references/spec-format.md` Section 1:
  ```yaml
  auth:
    verify_path: "/v1/me"   # string optional; doctor calls GET <verify_path> with auth headers to confirm the credential is valid. Omit if no cheap-to-call probe exists.
  ```
- **Test:** Add the field to a sample spec; assert `<cli> doctor` reports `OK Credentials: verified (HTTP 200 from /v1/me)` instead of the WARN.
- **Evidence:** `doctor` output during Phase 5: `WARN Credentials: present (not verified — set auth.verify_path in spec for an API acceptance check)`.
- **Related prior retros:** None

### F6: `generate --docs` silently degrades on Windows when the LLM CLI subprocess fails to spawn (recurring friction / platform support)

- **What happened:** `printing-press generate --docs <url>` invokes the local `claude` (or `codex`) CLI to convert docs to a spec. On Windows the `claude.cmd` shim spawn failed with `fork/exec C:\Users\ChrisCoombes\AppData\Roaming\npm\claude.cmd: The filename or extension is too long.`. The code fell back to a regex parser, which then reported `no endpoints found in <url>` and the generation aborted. The user has to author the spec by hand (which is what I did). Phase 4.85 also reported `unable: true` because `live-check` couldn't detect the binary as executable on Windows — a separate Windows-detection gap in scorecard.
- **Scorer correct?** Partially — the scorecard live-check `unable:true` outcome was correct (it couldn't run the binary), but the root cause (Windows executability detection in `internal/cli/scorecard.go`) is a fixable gap, not an inherent property of the system.
- **Root cause:** Two related Windows-platform gaps:
  1. The `generate --docs` LLM-spawn path uses Go's standard `os/exec` invocation against `.cmd` shims via long PATH; on Windows long-PATH `.cmd` spawn hits a 256-char limit in `CreateProcess`. The graceful fallback (regex) is too weak for real docs HTML (MadCap-generated AbR docs include 1MB of CSS scaffolding for ~50 lines of API content).
  2. `scorecard --live-check` probes binary executability with a check that returns `unable:true` for Windows `.exe` files in some configurations, silently skipping the output-review pass.
- **Cross-API check:** Affects every Windows user generating from `--docs`. The fallback regex would weakly hit any docs source that's small and clean (e.g., a single-page OpenAPI viewer), but most real docs are MadCap/GitBook/Docusaurus shells with low signal-to-noise.
- **Frequency:** subclass: Windows-only docs-mode runs. ~all when triggered.
- **Fallback if the Printing Press doesn't fix it:** Agent authors spec by hand from research (what I did). It works but loses the productivity promise of `--docs`.
- **Worth a Printing Press fix?** P3 — Windows platform support is bounded, but docs-mode is a named feature.
- **Inherent or fixable:** Fixable, two distinct work units:
  1. Better `claude`/`codex` spawn on Windows — use cmd.exe wrapper or shorter PATH copy; or detect the long-PATH failure and emit a clearer error including the `npm config get prefix` workaround.
  2. Scorecard live-check Windows detection — accept `.exe` extension and `os.IsExecutable` equivalent for Windows attributes; surface "executability check skipped" reason.
- **Durable fix:** Trace both invocations to `internal/generator/docs.go` (LLM spawn) and `internal/cli/scorecard.go` (live-check). Add Windows-specific spawn path. Add a clear error when the LLM CLI is unavailable instead of silently falling through to regex.
- **Test:** Mock the failure on Linux too (PATH=/dev/null, mock `claude` returns non-zero); assert the user sees an actionable error, not "no endpoints found."
- **Evidence:** Generate output included `warning: claude failed (fork/exec ...: The filename or extension is too long.), trying codex` → `warning: LLM doc-to-spec failed, falling back to regex` → `Error: generating spec from docs: no endpoints found`. Polish output: `output review SKIPped: scorecard live-check returned unable: true because the binary failed Windows executability detection`.
- **Related prior retros:** None

### F8: Narrative recipe commands (research.json `narrative.recipes`) drift from the actual CLI flag shape unless the author consults `<cli> sync --help` first (skill instruction gap)

- **What happened:** During Phase 1.5e I authored a recipe `<cli> sync --auditlog --events && <cli> quota show --agent` in `research.json`. The real flag shape is `<cli> sync --resources auditlog,events`. `validate-narrative` caught it in Phase 4 (full-example mode dry-run errored on `--auditlog: unknown flag`), forcing me to swap recipes mid-shipcheck. The system worked (the gate caught the bug) but the cost of the catch was a fix loop, and the underlying cause is the narrative-authoring step had no awareness of sync's flag shape.
- **Scorer correct?** Yes — validate-narrative caught the bug as designed.
- **Root cause:** `skills/printing-press/SKILL.md` Step 1.5e instructs the agent to author `narrative.recipes` from product intuition but does not say "before authoring a recipe that references a generated framework command (sync, search, doctor, export, import, analytics, tail, workflow), run `<cli> --help` on the in-progress binary or read the relevant template stub for its real flag shape."
- **Cross-API check:** Generalizes to every CLI where the narrative authoring step happens before binary build. The agent can only intuit framework command shapes from the SKILL examples, which are illustrative, not exhaustive.
  - producthunt-spec.yaml — likely had similar drift for sync.
  - Any CLI whose narrative recipes use framework commands without per-recipe `--help` verification.
- **Frequency:** every CLI's first authoring pass.
- **Fallback if the Printing Press doesn't fix it:** validate-narrative catches it before publish. Fix loop cost is one cycle.
- **Worth a Printing Press fix?** P3 — improving up-front catches a minor friction; the validator already prevents shipping bad recipes.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Add a one-paragraph instruction to `skills/printing-press/SKILL.md` Step 1.5e: "Before authoring any recipe whose command starts with a generated framework command (`sync`, `search`, `doctor`, `export`, `import`, `analytics`, `tail`, `workflow`, `auth`, `which`), check the actual flag shape — either read the canonical example in `references/framework-flags.md`, or run `<cli> <cmd> --help` once the binary has been built."
- **Test:** Author a recipe with a wrong sync flag in a test research.json; assert the skill instruction would have caught it before validate-narrative had to.
- **Evidence:** Phase 4 validate-narrative output: `FAILED [recipes]: <cli> sync --auditlog --events: full example failed → unknown flag: --auditlog`.
- **Related prior retros:** None

### F9: `cliutil` has no day/week/month duration parser; every novel feature with a `--window` flag reimplements it (missing scaffolding)

- **What happened:** Five of the seven AdminByRequest novel features take a `--window` flag (`requests repeat-offenders --window 30d`, `correlate --window 5m`, `report compliance --since 2026-01-01`, etc.). Go's `time.ParseDuration` does not accept `d` / `w` / `mo`. I had to hand-roll `parseDurationExtras` in `internal/cli/duration_extras.go` to support these natural units. Every CLI with time-window novel features faces the same gap.
- **Scorer correct?** N/A.
- **Root cause:** `internal/cliutil/` exports `AdaptiveLimiter`, `FanoutRun`, `CleanText`, `ExtractNumber`, `ExtractInt`, etc., but no `ParseDuration` that accepts day-or-larger units.
- **Cross-API check:** Affects every CLI with windowed novel features. Three with evidence:
  - Linear CLI — sprint windows ("last 30 days") are natural; `--window 30d` is the obvious flag.
  - GitHub CLI — `issues opened-since 7d`, `prs stale-after 14d` — day-windows everywhere.
  - Stripe CLI — `events --since 24h`, `charges in-window 7d` — finance dashboards use day-windows.
- **Frequency:** every windowed novel feature on every CLI.
- **Fallback if the Printing Press doesn't fix it:** Each novel feature author hand-rolls the parser, which is small but copy-pasted. Worse, parsers drift in their edge cases (does `1mo` = 30 days or calendar-month? does `1y` work?) so consumers get inconsistent behavior across CLIs.
- **Worth a Printing Press fix?** Yes. Cheap addition (≈25 LOC), broad reuse.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Add `cliutil.ParseDuration(s string) (time.Duration, error)` that wraps `time.ParseDuration` and additionally accepts `Nd` (days = 24h), `Nw` (weeks = 7d), `Nmo` (months = 30d). Document the calendar-month approximation. Surface in `internal/cliutil/text.go` or new `duration.go`.
- **Test:** Table-driven: `30d → 720h`, `2w → 336h`, `1mo → 720h`, `5m → 5m`, `1h → 1h`, edge cases (`""`, `"abc"`, `"30"`, `"30dx"`).
- **Evidence:** I wrote `internal/cli/duration_extras.go` in AdminByRequest (~32 LOC) to handle `--window 30d` / `--window 7w` / `--window 1mo`. The repeat-offenders, denied-reasons, and correlate commands all use it.
- **Related prior retros:** None

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Multi-endpoint resource parents hidden from root --help | generator | every multi-endpoint resource | low — Claude must read generated code, easy to miss | small | none |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | Store id column inconsistent with JSON `$.id` for int-typed ids | spec-parser | subclass:int-id | medium — silent zero-rows | small (helper) / medium (root fix) | subclass:int-id only |
| F3 | FTS5 search lacks query quoting; hyphens break | generator | every search invocation w/ hyphenated input | none — fails hard with SQL error | small | document `--fts-raw` for advanced operators |
| F4 | `export --data-source local` still calls live API | generator | every CLI | low — silent until offline | small | none |
| F9 | cliutil missing day/week/month duration parser | generator | every windowed novel feature | each author reimplements | small | none |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F5 | spec-format.md missing auth.verify_path | skill | every internal-YAML spec | doctor message names the field | small | none |
| F6 | Windows: --docs LLM spawn + scorecard live-check | generator + scorer | Windows users only | manual spec authoring works | medium | Windows-platform-gated |
| F8 | narrative recipes drift from framework cmd flags | skill | every CLI's first authoring | validate-narrative catches before publish | small | none |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|----------------------|
| (none) | | All six survivors filed; no candidate survived Phase 2.5 but failed Phase 3. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| sync `resource_not_incremental` warnings | every endpoint emits `sync_warning resource_not_incremental` | works-as-designed — the warning is informational, sync still pulled all rows; not a bug |
| validate-narrative pipe-chain interpretation on Windows | pipe through `jq -r` confused the validator's `--dry-run` append | printed-CLI — the workaround is "don't author pipe-chain recipes" |
| multi-data-center base URL discovery | doctor doesn't probe dc1..dc6 to find tenant's region | API-quirk — only AdminByRequest has 6-DC topology; not generalizable |

## Work Units

### WU-1: Unhide multi-endpoint resource parents (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** Generated CLIs with multi-endpoint resources surface those resource parents in root `--help` so agents and humans discover the surface via `--help` alone.
- **Target:** Generator templates in `internal/generator/` that emit resource-parent commands (`<resource>.go` template).
- **Acceptance criteria:**
  - positive test: regen a CLI from a spec with a multi-endpoint resource; assert `<cli> --help` lists the resource in `Available Commands`.
  - negative test: regen a CLI from a spec with a single-endpoint resource; assert the resource is promoted to root and the hidden parent isn't visible in `Available Commands` (existing behavior preserved).
- **Scope boundary:** Don't change MCP-tool exposure (cobratree walker already mirrors regardless of `Hidden`). Don't change subcommand ordering or grouping.
- **Dependencies:** none
- **Complexity:** small

### WU-7: Store-lookup helper for int-typed top-level id (from F2)
- **Priority:** P2
- **Component:** spec-parser
- **Goal:** Hand-written novel commands look up a row by id with a single helper, whether the spec declares id as `int` or `string`.
- **Target:**
  - Short-term: emit `cliutil.LookupByID(ctx, db, table, idStr) (json.RawMessage, error)` into every printed CLI. The helper runs `WHERE CAST(json_extract(data, '$.id') AS TEXT) = ? OR id = ?` and returns typed `ErrNotFound`.
  - Long-term: after disambiguation (open SQLite, check `typeof(id)`), fix the upsert path so the `id` TEXT column is queryable as text regardless of spec-declared type.
- **Acceptance criteria:**
  - positive test (short-term): a generated CLI with `type: int` top-level id exposes `cliutil.LookupByID` that resolves rows. Test with both JSON int and JSON string id inputs.
  - positive test (long-term): `SELECT id FROM <resource> WHERE id = '<stringified-int>'` matches, no helper needed.
  - negative test: looking up a non-existent id returns `ErrNotFound`, never silent zero-rows.
- **Scope boundary:** Does NOT change how `id` is sent over the wire to the upstream API.
- **Dependencies:** Disambiguation step described in F2 must come first for the root-cause fix; the helper can ship without it.
- **Complexity:** small (helper); medium (root-cause fix).

### WU-8: `export --data-source local` short-circuits to store, never calls API (from F4)
- **Priority:** P2
- **Component:** generator
- **Goal:** `--data-source local` is truly local-only; offline use works as documented.
- **Target:** The `export` command template (or shared `--data-source` resolver) in `internal/generator/`.
- **Acceptance criteria:**
  - positive test: with the API key env var unset and a synced store, `<cli> export <resource> --data-source local --format jsonl` returns store rows without any HTTP request. Verify by removing the env var entirely (not just emptying it).
  - negative test: `--data-source auto` and `--data-source live` continue to fetch live as today.
- **Scope boundary:** Does NOT change the default `--data-source` value (`auto`). Does NOT change other commands' `--data-source` behavior unless they have the same bug (audit `sql`, `analytics`, `search` for the same pattern as a follow-up).
- **Dependencies:** none
- **Complexity:** small

### WU-2: FTS5 query quoting in search template (from F3)
- **Priority:** P2
- **Component:** generator
- **Goal:** `<cli> search "<anything with hyphens or operators>"` returns matches, never SQL errors.
- **Target:** Search command template in `internal/generator/` that builds the SQLite FTS5 MATCH clause.
- **Acceptance criteria:**
  - positive test: `search "LAPTOP-CHRISC"`, `search "feature-branch"`, `search "API-keys"` all return rows (or empty results) without SQL error.
  - negative test: legit FTS5 power-user invocations (with `--fts-raw` if introduced) still parse operators correctly; non-hyphenated queries continue to work identically.
- **Scope boundary:** Don't change the FTS5 index schema. Don't change how `search` resolves resource-name targets.
- **Dependencies:** none
- **Complexity:** small

### WU-3: cliutil.ParseDuration with d/w/mo support (from F9)
- **Priority:** P2
- **Component:** generator
- **Goal:** Novel-feature authors don't reimplement a day-aware duration parser; every CLI gets `cliutil.ParseDuration("30d")` for free.
- **Target:** `internal/cliutil/duration.go` (new) or appended to an existing cliutil file.
- **Acceptance criteria:**
  - positive test: `cliutil.ParseDuration("30d") == 720*time.Hour`, `cliutil.ParseDuration("2w") == 336*time.Hour`, `cliutil.ParseDuration("1mo") == 720*time.Hour`, `cliutil.ParseDuration("5m") == 5*time.Minute` (forwards to time.ParseDuration).
  - negative test: empty string, non-numeric prefix, garbage suffix return descriptive errors.
- **Scope boundary:** Don't change `time.Duration` itself; don't introduce calendar-month arithmetic (use 30-day approximation, document it).
- **Dependencies:** none
- **Complexity:** small

### WU-4: spec-format.md documents auth.verify_path (from F5)
- **Priority:** P3
- **Component:** skill
- **Goal:** Agents authoring internal-YAML specs know to declare `auth.verify_path` so doctor can verify credentials end-to-end.
- **Target:** `skills/printing-press/references/spec-format.md` Section 1 ("Complete Schema Reference") and Section 2 ("Annotated Example").
- **Acceptance criteria:**
  - positive test: a spec authored from the reference that includes a credential-checkable endpoint declares `auth.verify_path`; resulting `<cli> doctor` reports `OK Credentials: verified`.
  - negative test: a no-auth spec (`auth.type: none`) doesn't reference `verify_path` and continues to report `OK Auth: configured` without WARN.
- **Scope boundary:** Documentation-only; no code change to the parser (it presumably already supports the field given the doctor message).
- **Dependencies:** Verify the spec parser actually does support `auth.verify_path` (read `internal/spec/parser.go`); if not, this expands to a generator change.
- **Complexity:** small

### WU-5: Windows: docs-mode spawn + scorecard live-check (from F6)
- **Priority:** P3
- **Component:** generator + scorer (primary: generator)
- **Goal:** Windows users running `--docs` get either a clean spec or an actionable error; scorecard live-check runs against `.exe` binaries on Windows.
- **Target:**
  - `internal/generator/docs.go` (LLM spawn) — handle `claude.cmd` long-PATH spawn; surface clearer error when both LLM CLIs are unavailable.
  - `internal/cli/scorecard.go` (live-check executability detection) — recognize Windows `.exe` files and Windows attribute semantics.
- **Acceptance criteria:**
  - positive test: on Windows with `claude.cmd` installed, `--docs <real-API-docs-URL>` produces a spec.
  - negative test: on Windows without `claude` or `codex`, the command exits with `Error: docs-mode requires the 'claude' or 'codex' CLI; install instructions: ...` instead of `no endpoints found`.
  - positive test (scorecard): scorecard `--live-check` against a Windows `.exe` produces samples (not `unable: true`).
- **Scope boundary:** Don't rewrite the regex fallback parser to handle MadCap; that's a separate larger work item.
- **Dependencies:** May need Windows CI runner for testing.
- **Complexity:** medium

### WU-6: SKILL.md instructs framework-flag verification before authoring narrative recipes (from F8)
- **Priority:** P3
- **Component:** skill
- **Goal:** Agents writing `research.json` narrative recipes catch framework-command flag drift before validate-narrative does.
- **Target:** `skills/printing-press/SKILL.md` Step 1.5e (narrative-authoring instructions).
- **Acceptance criteria:**
  - positive test: a new agent authoring narrative recipes for a CLI with sync (or any framework command) reads the instruction, runs `<cli> sync --help` (or consults a canonical reference doc), and writes a flag-correct recipe.
  - negative test: existing valid recipes don't need to change.
- **Scope boundary:** Don't add an automated lint step (validate-narrative already does that); this is purely a workflow instruction.
- **Dependencies:** none
- **Complexity:** small

## Anti-patterns
- I initially proposed F4 ("export --data-source local still hits API") in the Phase 5 acceptance report. Triage re-checked the evidence and showed the original failing trace was missing `--data-source local`, not because the flag was broken. Dropping was correct.
- The retro's tendency to file every observed friction would have produced 9–11 findings; ruthless Phase 2.5/Phase 3 triage cut that to 6.

## What the Printing Press Got Right
- **End-to-end shipcheck umbrella ran in <40s** across 6 legs and gave a clear PASS/FAIL signal. The umbrella's per-leg summary table is excellent.
- **validate-narrative caught my bogus recipes** before they shipped — the `--dry-run --strict --full-examples` mode is doing real work, not just box-checking.
- **dogfood synced novel_features_built from the verified set** automatically; I didn't have to hand-edit research.json after building. The "sync from verified set" pattern is one of the strongest scorer features.
- **Polish skill's forked context** kept its diagnostic-fix-rediagnose loop out of the main session. The skill's `context: fork` frontmatter is doing exactly what it should.
- **Scorecard 90/100 Grade A on a hand-authored internal-YAML spec for an obscure API** — the generator's defaults are strong enough that minimal hand authoring lands a Grade-A CLI.
- **Anti-reimplementation enforcement** (the AGENTS.md rule) made me think clearly about which novel features were "read-from-store aggregations" (allowed) vs "hand-rolled API mimics" (forbidden). The carve-outs (`// pp:client-call`, `// pp:novel-static-reference`) are well-scoped.
