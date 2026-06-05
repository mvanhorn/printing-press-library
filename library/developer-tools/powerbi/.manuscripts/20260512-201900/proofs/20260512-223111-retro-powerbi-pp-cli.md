# Printing Press Retro: Power BI

## Session Stats
- API: powerbi (Power BI REST API)
- Spec source: hand-crafted internal YAML (no OpenAPI exists for the user-facing Power BI REST API; only ARM specs in azure-rest-api-specs)
- Scorecard: 87/100 (Grade A) after polish (85 pre-polish)
- Verify pass rate: 100%
- Fix loops: 1 (narrative recipe flag fixes after first shipcheck)
- Manual code edits: 6 new files (auth_login.go, auth_doctor.go, dax.go, report_export.go, refreshes_failures.go, dataset_describe.go) + small AddCommand registrations in auth.go, datasets.go, root.go
- Features built from scratch: 7 transcendence features (the entire novel-feature set — all hand-built since the generator emits API endpoints, not workflow/orchestration commands)

## Findings

### 1. verify-skill crashes on Windows console with default cp1252 codec (scorer bug)
- **What happened:** `printing-press shipcheck` reports `verify-skill: FAIL` on Windows because the Python tool crashes printing a ✓ U+2713 char: `UnicodeEncodeError: 'charmap' codec can't encode character '✓' in position 26`. With `PYTHONIOENCODING=utf-8` the same tool succeeds and reports "All checks passed." Every Windows user generating any CLI will see a false-FAIL verdict from shipcheck.
- **Scorer correct?** No — pure tool bug.
- **Root cause:** `internal/scorer/verify-skill` (or equivalent) calls `print(format_human(report))` without setting UTF-8 on `sys.stdout`. Windows defaults `sys.stdout.encoding` to cp1252 (or the active code page), which can't encode `✓`, `─`, em-dashes, or any common report-formatting glyph.
- **Cross-API check:** Universal — affects every Windows user generating every CLI. The retro skill itself hit this and worked around it.
- **Frequency:** every-Windows-API.
- **Fallback if not fixed:** Every Windows shipcheck reports red even when everything passes. Agents reading the shipcheck verdict will treat passable CLIs as failed and either drop scope or refuse to promote. Already cost ~3 minutes this session diagnosing what was actually a tool bug.
- **Worth a Printing Press fix?** Yes — universal Windows scorer bug.
- **Inherent or fixable:** Fixable in one line. The Python tool needs to either `import sys; sys.stdout.reconfigure(encoding='utf-8')` at startup, or wrap the print in `print(..., flush=True)` with errors='replace', or write through a UTF-8 codecs.getwriter wrapper. Same fix shipcheck-wide.
- **Durable fix:** In `internal/scorer/verify-skill.py` (and any other Python scorer that prints unicode), call `sys.stdout.reconfigure(encoding='utf-8', errors='replace')` immediately on startup. This is a no-op on POSIX where stdout is already UTF-8.
- **Test:** On Windows with `chcp 1252` active, run `printing-press shipcheck --dir <any-cli>` — verify-skill leg should report "All checks passed", not a Python traceback.
- **Evidence:** Shipcheck output line 1 of `verify-skill` block in the session showed `UnicodeEncodeError: 'charmap' codec can't encode character '✓' in position 26: character maps to <undefined>` while the same tool with `PYTHONIOENCODING=utf-8` printed `✓ All checks passed`.
- **Related prior retros:** None — first retro for this user. Adjacent issue [#1255](https://github.com/mvanhorn/cli-printing-press/issues/1255) exists for a different verify-skill defect (regex coverage); this finding is unrelated.

### 2. "0 results (live)" framework prefix leaks into stdout before JSON output
- **What happened:** Every list command in the generated CLI prints `0 results (live)` to stdout BEFORE the JSON body, even when `--json` is set. Real session output:
  ```
  PS> .\powerbi-pp-cli.exe groups list --json --select value.id,value.name
  0 results (live)
  {
    "meta": { "source": "live" },
    "results": { "value": [...] }
  }
  ```
  The `0 results (live)` line is from the data-source-auto count display. It's emitted because the local cache is empty (no `sync` has run), but the JSON shows the live results that were just fetched. Two problems: (a) the count is wrong — it says 0 but the live response has results, and (b) the count is emitted to stdout instead of stderr, so it corrupts JSON piping.
- **Scorer correct?** N/A (no score penalty fired). But agent piping `| jq` will fail because stdout has plain text before the JSON. Dogfood's json_fidelity heuristic apparently missed it because the count line doesn't break JSON parsing if a tolerant parser starts at the `{`.
- **Root cause:** Generator template for list commands emits the count summary unconditionally before the JSON output handler. Should be either suppressed entirely under `--json` / `--quiet` / `--compact` / `--csv`, or routed to stderr.
- **Cross-API check:** Every CLI the Printing Press emits that uses `--data-source auto` (which is the default). That's every printed CLI shipped after the auto-mode feature landed.
- **Frequency:** every-API.
- **Fallback if not fixed:** Every agent piping the CLI to jq or another JSON parser sees a malformed mixed text/JSON stream. Defensive parsers handle it; strict ones break. False json_fidelity passes silently ship broken CLIs.
- **Worth a Printing Press fix?** Yes — universal.
- **Inherent or fixable:** Fixable. Either gate the count line on `isTerminal(stdout) && !flags.asJSON && !flags.csv && !flags.quiet && !flags.compact`, or route the count line to stderr (better — keeps the human-readable signal but doesn't pollute the pipe).
- **Durable fix:** In the generator's list-command template (and the data-source auto fall-through), route the "X results (source)" diagnostic to stderr unconditionally, or gate it on `!isStructuredOutput(flags)`. Add a dogfood check that asserts stdout starts with `{` or `[` whenever `--json` is set.
- **Test:** Run any generated CLI's list command with `--json` and pipe to `jq .` — should succeed without parse error. Run without `--json` — should still show the count for humans.
- **Evidence:** Session output above. Reproduced on every list command run during dogfood.
- **Related prior retros:** None. Adjacent issue [#1254](https://github.com/mvanhorn/cli-printing-press/issues/1254) covers workflow archive's NDJSON-in-JSON mixing — similar shape (text leaking into structured stream) but different code path.

### 3. API error response bodies truncated mid-message in stderr
- **What happened:** When an API returns a 400/404 with a useful error body, the CLI's error wrapper truncates at ~200 chars. Power BI returned:
  ```
  HTTP 404: {"error":{"code":"PowerBIEntityNotFound","pbi.error":{"code":"PowerBIEntityNotFound","parameters":{},"details":[{"code":"DetailsMessage","detail":{"type":1,"value":"You cannot query the dataset 'c6643...
  ```
  The actually-useful "You cannot query the dataset 'c6643...' because..." sentence cuts off exactly at the diagnostic detail. User has no actionable information.
- **Scorer correct?** N/A.
- **Root cause:** Generator's error wrapper in the client or per-command code does naive string truncation: `truncate(string(raw), 200)` style. Doesn't preserve start+end, doesn't preserve the deepest `detail.value` field, doesn't pretty-print.
- **Cross-API check:** Universal — every CLI's API error path. Notably bad for Microsoft (deeply nested `pbi.error.details[].detail.value`), Google (`error.details[].@type`), AWS (nested `__type` payloads). All three put diagnostic detail in the latter half of the body.
- **Frequency:** every-API.
- **Fallback if not fixed:** Users see meaningless prefixes. Auth doctor commands and equivalent help, but for runtime errors the trail goes cold. Forces every user to `curl` the same endpoint with the same body to read the full error.
- **Worth a Printing Press fix?** Yes — universal and easy.
- **Inherent or fixable:** Fixable. Three options: (a) emit the full body unconditionally (cheapest, fine for most error paths), (b) truncate from the middle keeping first 200 + last 200 chars + `...`, (c) recursively extract the deepest non-empty `message` / `detail.value` / `errorDescription` field and surface that on its own line. Option (a) is the simplest improvement; option (c) is the agent-friendly version.
- **Durable fix:** Generator's client error wrapper: stop truncating bodies. Emit the full response body (cap at ~4KB to prevent runaway logs; that's still 20x current). Optionally add a small `extractDeepestMessage` helper for the human-readable line.
- **Test:** Run any generated CLI command against a real API endpoint that returns a nested error body. Verify the actually-useful message text appears in stderr (not just the prefix).
- **Evidence:** Session log line `Error: POST /groups/.../executeQueries returned HTTP 404: {"error":{"code":"PowerBIEntityNotFound","pbi.error":{"code":"PowerBIEntityNotFound","parameters":{},"details":[{"code":"DetailsMessage","detail":{"type":1,"value":"You cannot query the dataset 'c6643...`
- **Related prior retros:** None.

### 4. Internal YAML spec body field can't express nested-object request bodies
- **What happened:** Power BI's executeQueries endpoint takes a body of shape `{queries: [{query: "DAX"}], serializerSettings: {includeNulls: bool}, impersonatedUserName: "upn"}` — array of objects + nested options object. The internal YAML spec's `body:` field is `[]Param` — a flat list of name+type+description rows. Tried to express the nested shape with `body.schema.properties.queries` (OpenAPI-style); parser rejected with `yaml: unmarshal errors: cannot unmarshal !!map into []spec.Param`. Had to drop the endpoint from the spec entirely and hand-write the entire `dax run` command, including the Go struct types matching the request body.
- **Scorer correct?** N/A.
- **Root cause:** Spec parser at `internal/spec/` defines `Endpoint.Body` as `[]Param` for the internal YAML format. Works for simple flat bodies (most REST POST). Breaks for: GraphQL (`{query, variables}`), Power BI executeQueries, Slack `chat.postMessage` with blocks/attachments arrays, Stripe POST with nested address/metadata, any RPC-style API.
- **Cross-API check:** Three named with evidence: GitHub GraphQL (`POST /graphql` body `{query: "...", variables: {...}}`), Slack chat.postMessage (`{channel, text, blocks: [...]}`), Stripe POST /v1/customers (nested `address: {line1, city, ...}` + `metadata: {key: value, ...}`). All three are real APIs in common use that need hand-written bodies under the current spec format.
- **Frequency:** subclass:RPC-or-rich-body-POST APIs. Most read-only REST APIs (where the user said this would be needed) don't hit it. POST/PUT-heavy APIs hit it routinely.
- **Fallback if not fixed:** Agent has to drop the endpoint from the spec entirely and hand-build the command + Go struct types + JSON encoding. That's exactly what happened this session — 270 lines of dax.go was the cost. Generator gives no scaffolding because it can't represent the input shape.
- **Worth a Printing Press fix?** Yes — multiple APIs in the catalog already hit this.
- **Inherent or fixable:** Fixable. Two options: (a) add a `body_schema` field on `Endpoint` that takes an inline JSON Schema and emits typed Go structs — replicates what OpenAPI parsers do but for internal YAML; (b) support `body: object` with a `properties:` map (looks like the user's first attempt with the syntax). Option (b) matches OpenAPI conventions and would have made the user's first spec parse correctly.
- **Durable fix:** Extend internal YAML spec parser to accept `body:` as either `[]Param` (current) OR an `object` shape with `properties:` (new). When body is a typed object, emit a typed Go struct + JSON serialization in the generated command. This is parser work in `internal/spec/`, generator template work for POST commands.
- **Test:** Add a fixture spec with a GraphQL-shaped body (`{query: string, variables: object}`). Generate; verify the POST command emits a typed struct and the user can pass nested fields via flags or `--body-file query.json`.
- **Evidence:** Session error: `Error: parsing spec ... yaml: unmarshal errors: line 175: cannot unmarshal !!map into []spec.Param, line 211: cannot unmarshal !!map into []spec.Param`. Forced full hand-build of the executeQueries surface as `internal/cli/dax.go`.
- **Related prior retros:** None. Adjacent issue [#1240](https://github.com/mvanhorn/cli-printing-press/issues/1240) covers compile OOM on deeply nested OpenAPI types — different direction (output too big), but same general territory (request body schemas).

### 5. Windows `.exe` validation gate inside `generate` reports false FAIL
- **What happened:** `printing-press generate` builds the CLI successfully on Windows (verified by `go build` and the absence of compile errors) but the final post-generate validation step fails with: `Error: gate "powerbi-pp-cli --help" failed: exec: "C:\\...\\powerbi-pp-cli-validation": executable file not found in %PATH%`. The validator looks for a binary without the `.exe` suffix on Windows. Build is fine; only the validator is broken.
- **Scorer correct?** No — exact duplicate of an already-filed scorer bug (see Related prior retros).
- **Root cause:** Validator inside `generate` resolves the expected binary path without `runtime.GOOS == "windows"` → add `.exe` extension. Filed prior as [#1150](https://github.com/mvanhorn/cli-printing-press/issues/1150).
- **Cross-API check:** Every Windows user generating any CLI. Universal.
- **Frequency:** every-Windows-API.
- **Fallback if not fixed:** Every Windows user gets a non-zero exit from `generate` even when the actual code generation succeeded. Looks like a hard failure; isn't. Already filed at P3 in #1150 — bumping with new recurrence evidence.
- **Worth a Printing Press fix?** Yes, but already filed.
- **Durable fix:** As described in #1150 — append `.exe` to the binary name on Windows in the validation gate.
- **Test:** Generate any CLI on Windows; the `printing-press --help` validation gate should pass with exit 0.
- **Evidence:** Session log: `FAIL powerbi-pp-cli --help / Error: validating generated project: gate "powerbi-pp-cli --help" failed: exec: "...\\powerbi-pp-cli-validation": executable file not found in %PATH%`. Exit code was non-zero from generate; everything had actually succeeded. Hit immediately after a clean `go build -o powerbi-pp-cli.exe ./cmd/powerbi-pp-cli`.
- **Related prior retros:** [#1150](https://github.com/mvanhorn/cli-printing-press/issues/1150) — `aligned`. Same defect, P3 priority, evidence base from prior retro. This session adds a second occurrence; commenting rather than filing new per dedup scan.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | verify-skill UnicodeEncodeError on Windows console | scorer | every-Windows-API | poor — false FAIL every time | small | reconfigure stdout encoding; no-op on POSIX |
| F2 | "0 results (live)" prefix leaks to stdout under --json | generator | every-API | poor — silently corrupts agent pipes | small | route diagnostic to stderr, or gate on output mode |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F3 | API error body truncation cuts diagnostic detail | generator | every-API | medium — users see useless prefix | small | raise cap to ~4KB; optionally extract deepest message |
| F4 | Internal YAML body can't express nested-object POST shapes | spec-parser | subclass:rich-body-POST | medium — agent hand-builds the command | medium | accept `body:` as either []Param or object-with-properties |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| Spec format docs gap (flat `body:[]` vs OpenAPI nested) | Step G: case-against stronger. The real fix is supporting nested bodies (F4 above), not better docs on the current limitation. Subsumed by F4. |
| PowerShell quote-stripping hint on `--query` | Step G: case-against stronger. This is a Windows-shell ergonomic issue, not a Printing Press concern. README/SKILL note suffices. The mitigation (use `--file`) was already a documented alternative. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Power BI not in catalog | Catalog is opt-in; not a generator gap | normal-growth |
| My singular `dataset describe` vs plural `datasets describe` in narrative | Agent authoring error; validate-narrative caught it correctly | iteration-noise |
| Hand-built 7 transcendence features | Transcendence features are explicitly hand-built by design | by-design |
| ARM specs in azure-rest-api-specs are management not user-facing | Per-API research finding, not a machine issue | per-CLI |

## Work Units

### WU-1: Make scorer Python tools UTF-8-safe on Windows console (from F1)
- **Priority:** P1
- **Component:** scorer
- **Goal:** verify-skill (and any other Python scorer that prints unicode glyphs) succeeds on Windows console with default cp1252 codepage.
- **Target:** Python scoring tools — primarily `verify-skill` but check all Python-based scorers under `internal/scorer/` (or wherever the embedded Python scripts live).
- **Acceptance criteria:**
  - positive test: on Windows with `chcp 1252` active, `printing-press verify-skill --dir <any-fixture>` exits 0 and prints "All checks passed" without a Python traceback.
  - negative test: existing POSIX behavior unchanged — stdout still UTF-8, no extra reconfigure noise.
- **Scope boundary:** Does NOT include rewriting verify-skill's check logic or changing what it reports. Pure encoding fix.
- **Dependencies:** None.
- **Complexity:** small.

### WU-2: Route "X results (source)" data-source diagnostic to stderr, not stdout (from F2)
- **Priority:** P1
- **Component:** generator
- **Goal:** Every generated CLI's list command produces a stdout stream that is clean structured output under `--json` / `--csv` / `--quiet` / `--compact`. The human-readable "0 results (live)" line goes to stderr.
- **Target:** Generator template for list commands (and the data-source auto fall-through helper) under `internal/generator/`.
- **Acceptance criteria:**
  - positive test: `<any-cli> <resource> list --json | jq .` succeeds without parse errors.
  - positive test: `<any-cli> <resource> list --csv | head -1` shows a CSV header, no count prefix.
  - positive test: `<any-cli> <resource> list` (human mode) still shows the count for readability.
  - negative test: stderr still carries the count diagnostic (verifiable via `2>&1 >/dev/null`).
  - regression check: dogfood gets a new check asserting stdout starts with `{` or `[` (or is empty) under `--json`.
- **Scope boundary:** Does NOT include changing what the count says (the "0 vs N" mismatch when local cache is empty is a separate freshness question). Just where the line goes.
- **Dependencies:** None.
- **Complexity:** small.

### WU-3: Stop truncating API error response bodies in client error wrapper (from F3)
- **Priority:** P2
- **Component:** generator
- **Goal:** Agents and humans see the full diagnostic detail in API errors (especially for APIs that put the useful information deep in the response — Microsoft, Google, AWS).
- **Target:** Generator's client error wrapper template — the place that produces `apiErr(fmt.Errorf("... returned HTTP %d: %s", status, truncate(string(raw), 200)))` or equivalent.
- **Acceptance criteria:**
  - positive test: when the API returns a deeply-nested error body (e.g., Power BI `pbi.error.details[].detail.value`), the full message text appears in stderr.
  - positive test: a runaway 1MB error body is capped (~4KB cap) but the cap is high enough to surface real diagnostic detail.
  - negative test: stderr is still single-line for short error bodies; doesn't suddenly wrap or pretty-print.
- **Scope boundary:** Does NOT include adding a recursive "extract deepest message" helper as a hard requirement. That's a nice-to-have agent improvement. Core scope is "don't truncate at 200 chars."
- **Dependencies:** None.
- **Complexity:** small.

### WU-4: Accept nested-object request bodies in internal YAML spec format (from F4)
- **Priority:** P2
- **Component:** spec-parser
- **Goal:** Power BI executeQueries, GraphQL endpoints, Slack chat.postMessage with blocks, Stripe nested-address POSTs, and similar rich-body APIs can be expressed in internal YAML specs and have their typed Go struct emitted automatically.
- **Target:** Spec parser in `internal/spec/`. Generator's POST/PUT command template under `internal/generator/`.
- **Acceptance criteria:**
  - positive test: A fixture spec declaring `body: { properties: { query: string, variables: object } }` parses successfully and the generated command emits a typed Go struct + JSON encoding.
  - positive test: Existing `body: [{name, type}]` flat-list syntax still works (backward compatibility).
  - positive test: A nested body containing an array of objects (`{queries: [{query: string}]}`) emits a slice of typed structs.
  - negative test: Malformed body shapes produce a clear parser error pointing to the line, not "cannot unmarshal !!map into []spec.Param".
- **Scope boundary:** Does NOT include full OpenAPI-equivalence with $refs and discriminators — just the common nested-object and array-of-objects shapes. Composition via $ref can be a future increment.
- **Dependencies:** None.
- **Complexity:** medium.

## Anti-patterns
- Searching for `--groupId` in narrative recipes assumed camelCase, but Cobra defaults to kebab-case (`--group-id`). The narrative author (the agent writing research.json) needs to pre-resolve flag names from the generated CLI before authoring recipes. (This was my mistake, not a generator bug — already in Dropped.)
- Trying to use `&&` in a single `command:` field in narrative recipes — validate-narrative treats each recipe as a single binary invocation and rejects chains.

## What the Printing Press Got Right
- **Auth scaffolding via `bearer_token` type was perfect for OAuth2.** The generator emitted a complete `config.Config` with `AccessToken`, `RefreshToken`, `TokenExpiry`, `ClientID`, `ClientSecret` fields and a working `SaveTokens()` method. Made the hand-built `auth login` command trivial — I just had to handle the OAuth token-exchange HTTP and call `cfg.SaveTokens()`.
- **dogfood's `novel_features_built` sync is excellent.** After Phase 3, dogfood detected that 7 of the planned novel features were actually built, synced them to README/SKILL/root help, and surfaced the delta in research.json. Saved a manual cleanup pass.
- **The cli-printing-press skill nailed the URL-detection disambiguation.** When the user pasted `app.powerbi.com/groups/.../reports/...`, the skill correctly asked "is this the website or the API?" rather than blindly trying to browser-sniff the report page. That single question saved a 5-minute wrong-path detour.
- **Reachability gate's HTTP-403-with-no-auth case worked exactly right.** Probed `api.powerbi.com/v1.0/myorg/groups` without a token, got 403, recognized this as "needs auth" not "blocked at network," and PASSED.
- **Validate-narrative's `--full-examples` mode caught two real recipe bugs** (`--groupId` vs `--group-id` and the `sync &&` chain) before they shipped. The fact that it ran every recipe through `--dry-run` against the actual built binary made the difference.
- **printing-press-polish improved the scorecard 85→87 and cleared 8 verify-skill findings and 2 tools-audit pending items in one pass.** This is real cross-cutting value from a single skill invocation.
