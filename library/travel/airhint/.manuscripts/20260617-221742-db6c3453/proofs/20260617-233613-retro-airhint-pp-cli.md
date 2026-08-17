# Printing Press Retro: AirHint

## Session Stats
- API: airhint (flight price prediction — "buy or wait" recommendations)
- Spec source: browser-sniffed (HAR via agent-browser, then complete manual rewrite)
- Scorecard: 83/100 (Grade A)
- Verify pass rate: 100% (22/22)
- Fix loops: 4 (3× phase5-acceptance.json schema, 1× generate resource name collision)
- Manual code edits: 4 (spec YAML full rewrite, root.go XHR headers, workflow_transcendence.go new file, channel_workflow.go additions)
- Features built from scratch: 3 (workflow predict-sweep, workflow compare-routes, workflow cheapest-window)

## Findings

### F1: Spec parser generates hardcoded path segments for compact identifiers (spec-parser)

- **What happened:** `cli-printing-press browser-sniff` processed a HAR where the only observed URL for the prediction endpoint was `/predict/FR/STN/DUB/2026-08-16`. The traffic analysis output named this endpoint cluster `list_2026_08_16` with resource `2026_08_16` — treating the date as a resource identifier and all four path segments as literals. The correct parameterized form is `/predict/{airline}/{origin}/{destination}/{date}`. This required a complete manual rewrite of the entire spec YAML (6 resources, all type definitions). The same parser correctly identified `/search/c7fa65f9-a457-4acb-b55f-70193933d745` as `/search/{search_id}` — UUIDs are recognized as parameters. The path `/cheapest-deal-month/one-way/STN/DUB/8` was partially parameterized as `/cheapest-deal-month/one-way/STN/DUB/{dub_id}` — only the last segment, named after the preceding segment `DUB`.

- **Scorer correct?** N/A — no score penalty; this is a spec-parser output quality issue.

- **Root cause:** The spec parser's parameterization heuristic recognizes UUIDs (36-char hex with hyphens) as parameter candidates, but cannot identify other compact identifier patterns: ISO dates (`YYYY-MM-DD`), short uppercase codes (IATA airline codes like `FR`, airport codes like `STN`/`DUB`), or small integers used as enumerations (month number `8`). Without cross-request variance signal (only one URL per endpoint from a single browser session), the parser defaults to treating all non-UUID segments as literal.

- **Cross-API check:** Any browser-sniffed CLI where endpoints embed data values in path segments would exhibit this. Travel and transportation APIs consistently use IATA codes and dates in paths. The pattern is not travel-specific — sports APIs embed league/team codes, finance APIs embed ticker symbols, health APIs embed ICD codes.

- **Frequency:** Subclass: browser-sniffed CLIs with coded path segments. High frequency within browser-sniff workflow; all other spec sources (OpenAPI, Postman) provide parameterized paths directly.

- **Fallback if the Printing Press doesn't fix it:** Agent must completely rewrite the auto-generated spec YAML — every endpoint, every parameter, all type definitions. This is the highest-effort manual step in the browser-sniff workflow, error-prone (agent may miss some endpoints), and not parallelizable. Fallback reliability: poor (agent-written spec rewrite costs many minutes and several error cycles).

- **Worth a Printing Press fix?** Yes. The improvement is bounded — extend the parameterization heuristic with concrete identifier patterns — and the benefit compounds across every browser-sniffed CLI with coded paths.

- **Inherent or fixable:** Partially inherent (no variance signal without multiple requests), but fixable for common patterns. ISO dates (`\d{4}-\d{2}-\d{2}`), 2–3 uppercase letters at likely code positions (short enough to be codes, uppercase convention), and small integers following static path segments are identifiable heuristically. False positive risk (over-parameterizing a genuinely static segment) is recoverable via agent review; false negative risk (leaving a parameter as a literal) requires a full spec rewrite — current behavior is strictly worse.

- **Durable fix:** Extend the spec parser's path normalization step with a set of parameterization heuristics that fire when the path has only one observed sample:
  1. ISO date pattern (`\d{4}-\d{2}-\d{2}`) → `{date}`
  2. Short uppercase string at a position following a similar-length uppercase segment → likely a code (`{code}` or contextual name); flag as "low-confidence parameter candidate" in the traffic analysis output
  3. Small integer (`\d{1,4}`) following a long static segment chain → `{id}` or contextual name
  These should produce a "low-confidence parameterization" flag in the traffic analysis so the agent knows which segments were inferred vs. directly observed. The fix is purely additive — existing UUID detection is unchanged.

- **Test:**
  - Positive: HAR containing `/predict/FR/STN/DUB/2026-08-16` should produce spec with `/predict/{segment_0}/{segment_1}/{segment_2}/{date}` (date recognized; short uppercase segments flagged as candidates)
  - Negative: HAR containing `/v1/users/me` should NOT parameterize `me` (not an uppercase code, not a date)

- **Evidence:** `research/airhint-browser-sniff-spec-traffic-analysis.json` → `endpoint_clusters[*].path` shows `"/predict/FR/STN/DUB/2026-08-16"` and `"/cheapest-deal-month/one-way/STN/DUB/{dub_id}"`. The generated `candidate_commands` include `"name": "list_2026_08_16"` and `"resource": "2026_08_16"`.

- **Related prior retros:** None found.

---

### F5: phase5-acceptance.json validation emits opaque errors, requiring 4 sequential fix loops (scorer)

- **What happened:** Writing the phase5-acceptance.json file required 4 distinct round-trips with the scorer before the file validated. Each loop surfaced one previously-hidden error: (1) `status: "accepted"` rejected — valid value is `"pass"`; (2) `level: "quick-check"` rejected — valid value is `"quick"`; (3) field `matrix_size` was missing and required; (4) field `tests_passed` was missing and required. None of these constraints were discoverable before writing the file. The scorer emitted one error per run, not all errors at once.

- **Scorer correct?** Yes — the validation is correct. The scorer is checking the right things. The UX is the problem: one error per pass, no schema reference, no valid-values list, no example file to copy.

- **Root cause:** The scorer validates `phase5-acceptance.json` against an internal schema but does not surface the schema in error output, does not enumerate valid field values, and halts on the first failure rather than collecting all validation errors. The skill instructions do not include a canonical example or a link to the schema.

- **Cross-API check:** Applies to every generated CLI — `phase5-acceptance.json` is produced during Phase 5 for every run. The 4-loop discovery pattern would repeat for any agent starting from scratch without having seen a prior valid example.

- **Frequency:** Every browser-sniffed CLI, every OpenAPI CLI, every run. Universal.

- **Fallback if the Printing Press doesn't fix it:** Agent writes the file from memory, iterates through validation errors sequentially. Time cost: 4+ edit-save-rerun cycles. Fallback reliability: mediocre — will always eventually succeed, but 4 loops is avoidable waste that occurs every run.

- **Worth a Printing Press fix?** Yes. This is universal (every CLI), the fix is small (emit all errors at once + include valid values), and the benefit is a single-pass write.

- **Inherent or fixable:** Fixable. The validator already knows the schema — surfacing it requires adding error detail to existing validation output and possibly adding a `--schema` flag that prints the JSON schema.

- **Durable fix:** Two options (either resolves the issue):
  1. **Scorer: enumerate valid values in error output.** When a field value is invalid, emit: `status: invalid value "accepted" — valid values: ["pass", "fail"]`. When a required field is missing, emit all missing fields at once rather than halting after the first. Complexity: small.
  2. **Skill: include a filled-in example in the Phase 5 instructions.** The skill template for Phase 5 already tells the agent to write this file; add a complete, commented example (with all required fields and their valid values) directly in the skill instructions. Agent copies, modifies the test matrix, submits once. Complexity: trivial. This is the cheaper fix and could be done without a binary change.
  Option 2 is the fastest implementation; Option 1 is the more durable machine improvement. Both together eliminate the issue completely.

- **Test:**
  - Positive: Writing `"status": "accepted"` should produce `status: invalid value "accepted" — valid values: ["pass", "fail"]` in a single validation pass that also reports all other field errors
  - Negative: A fully valid `phase5-acceptance.json` produces no validation output (passes silently or with a success message)

- **Evidence:** Session conversation: four sequential write-validate-fix cycles for the file, each fixing one undiscovered constraint. Final accepted structure required `schema_version`, `api_name`, `run_id`, `status: "pass"`, `level: "quick"`, `matrix_size`, `tests_passed`, `tested_at`.

- **Related prior retros:** None found.

---

### F4: Skill doesn't instruct agent to verify HAR response body quality before proceeding (skill)

- **What happened:** The browser-sniff HAR captured 16 requests to `www.airhint.com` but all response bodies were empty (`size_class: "empty"`, `response_shape: {}` in the traffic analysis). The generated spec had no type information for any endpoint. The skill instructions did not flag this as a known risk or provide a fallback path. The agent discovered the empty-body problem indirectly (when type definitions were missing from the generated spec) and resolved it manually with curl calls to reconstruct response shapes.

- **Scorer correct?** N/A.

- **Root cause:** The browser-sniff skill section does not mention HAR response body quality as a check point. No instruction tells the agent to inspect `size_class` / `response_shape` fields in the traffic analysis JSON after browser-sniff completes.

- **Cross-API check:** Whether `agent-browser` captures response bodies appears to depend on the server's content encoding (gzip responses may not be decoded) and how the browser tool saves the HAR. This likely recurs for any API behind a CDN with compressed responses.

- **Frequency:** Subclass: browser-sniffed CLIs where the server uses response compression or the browser tool doesn't capture bodies. Estimated: common but not universal.

- **Fallback if the Printing Press doesn't fix it:** Agent eventually notices empty type definitions and uses curl — but the trigger is indirect and may not happen until generate produces skeleton types. A direct instruction makes the check deliberate rather than accidental.

- **Worth a Printing Press fix?** Yes, as a skill instruction — trivial to add, eliminates an indirect discovery path. Not worth a generator or binary change.

- **Inherent or fixable:** Fixable as a skill instruction. The check itself is cheap: look for `response_shape: {}` in the traffic analysis output immediately after `browser-sniff`.

- **Durable fix:** Add to the browser-sniff section of the skill: "After browser-sniff, inspect the traffic analysis JSON for `size_class: 'empty'` and `response_shape: {}` on endpoint clusters. If all clusters have empty response shapes, use curl to call each discovered endpoint directly to capture response structure before writing the spec."

- **Test:**
  - Positive: A session where HAR has empty bodies should include curl calls for each endpoint to discover types, before spec is written
  - Negative: A session where HAR has populated `response_shape` fields should proceed directly to spec generation

- **Evidence:** `research/airhint-browser-sniff-spec-traffic-analysis.json` — every `endpoint_clusters[*].response_shape` is `{}` or empty. Type definitions in the final spec were written from curl calls, not from HAR evidence.

- **Related prior retros:** None found.

---

### F2: Browser-sniff can produce resource names that collide with reserved template names (spec-parser)

- **What happened:** The browser-sniff spec auto-generated a resource named `search` (from the `/search/{id}` endpoint pattern). Running `generate` failed immediately: "resource name 'search' collides with reserved Printing Press template 'search'". The error message was clear; the fix was renaming `search` to `flights` in the spec YAML. Total cost: ~2 minutes.

- **Scorer correct?** N/A — generate refusal, not a score penalty.

- **Root cause:** The browser-sniff command produces resource names from path structure without checking against the list of reserved Printing Press template names. The collision is detected at generate time (correct behavior) but not at spec-generation time (missed opportunity).

- **Cross-API check:** Any API with a `/search/...` endpoint structure would produce a `search` resource. GitHub API (`GET /search/repositories`), Spotify API (`GET /search`), Elasticsearch-backed APIs — all would hit this collision.

- **Frequency:** Subclass: CLIs for APIs with `/search` as a top-level resource. Common in SaaS APIs.

- **Fallback if the Printing Press doesn't fix it:** `generate` fails with a clear error message, agent renames the resource. Fallback reliability: excellent — the error is specific and the fix is trivial. This is the lowest-impact finding in the "Do" set.

- **Worth a Printing Press fix?** Marginally yes. The improvement is moving the check earlier (spec-generation time) so the agent produces the right name the first time rather than encountering a generate failure.

- **Inherent or fixable:** Fully fixable. The list of reserved template names is known at spec-generation time; the browser-sniff command could check against it and either warn or auto-suffix conflicting names (e.g., `search` → `flight-search` based on parent path context).

- **Durable fix:** In the browser-sniff command, after computing resource names from path clustering, check each name against the reserved template name list. For collisions, either: (a) auto-suffix with a contextual term derived from the path (e.g., `search` under `/flights/search/` → `flight-search`), or (b) emit a warning in the traffic analysis output: "resource name 'search' may collide with reserved template — consider renaming." Option (a) is cleaner but risks wrong naming; option (b) is safer and costs zero complexity.

- **Test:**
  - Positive: A HAR with `/search/{id}` produces traffic analysis with a warning about the reserved name, or auto-names the resource `{context}-search`
  - Negative: A HAR with `/predict/{id}` (non-reserved name) produces no warning

- **Evidence:** Session: `generate` hard-failed with "resource name 'search' collides with reserved Printing Press template 'search'". Fixed by renaming resource to `flights` in the spec YAML before re-running generate.

- **Related prior retros:** None found.

---

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Spec parser treats compact identifiers as literal path segments | spec-parser | Every browser-sniffed CLI with coded paths | Poor (full spec rewrite required) | Medium | Only applies when browser-sniff produces single-sample endpoints |
| F5 | phase5-acceptance.json validation emits opaque per-field errors | scorer | Every CLI run | Mediocre (4+ loops, always eventually succeeds) | Small | None needed — affects all runs |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F4 | Skill missing HAR body quality check + curl fallback instruction | skill | Browser-sniffed CLIs with CDN compression | Mediocre (indirect discovery) | Trivial | Only add instruction to browser-sniff section |
| F2 | Browser-sniff resource name may collide with reserved template at generate time | spec-parser | APIs with `/search` as top-level resource | Excellent (clear error, trivial fix) | Small | Check against reserved name list at spec-generation time |

### Skip
*(None — all candidates either survived to Do or were dropped.)*

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| C3: XHR header not propagated | AirHint API returns 403 without `X-Requested-With: XMLHttpRequest`, not emitted by generated root.go | `API-quirk`: Legacy AJAX detection pattern deprecated in most frameworks since Django 3.1/Rails 5. Step B could not name 3 modern APIs that enforce this header. Step G case-against: adding this header as a browser-sniff default would carry noise into CLIs that don't need it. |

---

## Work Units

### WU-1: Extend spec-parser path parameterization heuristics for compact identifier patterns
- **Priority:** P2
- **Component:** spec-parser
- **Goal:** Browser-sniffed specs correctly identify ISO dates, short uppercase code strings, and small integers as path parameter candidates rather than literal segments, reducing full spec rewrites
- **Target:** Spec parser path normalization logic in `internal/spec/` (path clustering / parameter detection code)
- **Acceptance criteria:**
  - Positive test: A HAR containing `/predict/FR/STN/DUB/2026-08-16` produces a spec with `/predict/{segment_0}/{segment_1}/{segment_2}/{date}` — ISO date recognized, short uppercase segments flagged as low-confidence parameter candidates
  - Negative test: A HAR containing `/v1/users/me` does NOT parameterize `me` — non-uppercase, non-date, non-integer
- **Scope boundary:** Does not change UUID detection (already works). Does not attempt to infer semantic names for codes (the agent supplies names). Produces a "low-confidence" flag in traffic analysis for heuristically-identified parameters so the agent can verify.
- **Dependencies:** None
- **Complexity:** Medium

### WU-2: Improve phase5-acceptance.json validation UX — emit all errors at once with valid values
- **Priority:** P2
- **Component:** scorer
- **Goal:** Phase 5 acceptance file validation emits all errors in a single pass with valid field values enumerated, collapsing 4+ fix loops to 0–1
- **Target:** Scorer validation logic for `phase5-acceptance.json` (likely `cli-printing-press shipcheck` or a dedicated phase5 subcommand)
- **Acceptance criteria:**
  - Positive test: A file with `status: "accepted"`, `level: "quick-check"`, missing `matrix_size`, missing `tests_passed` produces a single output listing all four errors simultaneously, with valid values: `status: invalid "accepted" — valid: ["pass","fail"]`, `level: invalid "quick-check" — valid: ["quick","full"]`, `matrix_size: required`, `tests_passed: required`
  - Negative test: A valid `phase5-acceptance.json` produces no output (or explicit "valid" message)
- **Scope boundary:** Does not change what the schema validates — only the error output format and completeness. Consider also adding the canonical file schema (or a filled example) to the printing-press SKILL.md as a companion change.
- **Dependencies:** None
- **Complexity:** Small

### WU-3: Add HAR body quality check to browser-sniff skill instructions
- **Priority:** P3
- **Component:** skill
- **Goal:** Skill explicitly instructs agent to inspect HAR response body quality after browser-sniff and fall back to curl when bodies are empty, preventing empty-type specs
- **Target:** Browser-sniff section of `skills/printing-press/SKILL.md`
- **Acceptance criteria:**
  - Positive test: A browser-sniff session with `response_shape: {}` on all clusters triggers curl calls for each endpoint before spec is written
  - Negative test: A browser-sniff session with populated `response_shape` fields proceeds directly to spec generation without extra curl calls
- **Scope boundary:** One or two sentences in the skill's browser-sniff section only. Does not change the generator or spec parser.
- **Dependencies:** None
- **Complexity:** Trivial

### WU-4: Warn on reserved template name collisions during browser-sniff spec generation
- **Priority:** P3
- **Component:** spec-parser
- **Goal:** Browser-sniff command warns when a derived resource name conflicts with a reserved Printing Press template name, so the collision is caught at spec-generation time rather than generate time
- **Target:** Resource name derivation step in browser-sniff command or spec parser (`internal/spec/` or `cmd/cli-printing-press/browser_sniff.go`)
- **Acceptance criteria:**
  - Positive test: A HAR with `/search/{id}` produces a warning in the traffic analysis or CLI output: "resource name 'search' conflicts with reserved template — consider renaming (e.g., 'flight-search', 'query')"
  - Negative test: A HAR with `/predict/{id}` produces no warning
- **Scope boundary:** Warning only (no auto-rename) to keep the behavior transparent. Does not change the reserved-name list itself.
- **Dependencies:** None (can be implemented before or after WU-1)
- **Complexity:** Small

---

## Anti-patterns

- **Treating `size_class: empty` as acceptable and proceeding to spec generation.** Always check response shape quality in the traffic analysis JSON before writing the spec — empty shapes mean no type information from the HAR.
- **Using the traffic analysis `candidate_commands` as a source of truth for resource names.** When paths contain real data values (dates, codes), the candidate commands will be named after the data (e.g., `list_2026_08_16`) rather than the resource structure. These names are signals about what was observed, not suggestions for spec resource names.
- **Writing phase5-acceptance.json from memory without an example.** The schema has undocumented constraints (specific enum values for `status` and `level`, non-obvious required fields). Copy a prior example or wait for the validator to enumerate all errors in a single pass.

---

## What the Printing Press Got Right

- **`surf` HTTP client with browser impersonation** handled Cloudflare protection transparently — no TLS fingerprinting issues, no Cloudflare challenges despite the traffic analysis flagging protection signals at 0.9 confidence. The generated client worked for all discovered endpoints.
- **Async search pattern handling** — while the retry loop was hand-built, the fact that the generate step produced two separate commands (`create-search` and `get-search`) that correctly mapped to the two-step async pattern meant the workflow code could build directly on generated primitives.
- **Transcendence workflow structure** — the three transcendence features (predict-sweep, compare-routes, cheapest-window) were built quickly using the generated client methods as a base. The generated type stubs (`predictResponse`, `searchInitResponse`, etc.) could be embedded directly in the workflow types. The generator's output quality here was solid.
- **Verify 100% (22/22)** — despite the manual spec rewrite and custom workflow code, the structural quality of the generated CLI was high enough that all verify checks passed without targeted fixes.
- **No auth complexity** — the generator correctly identified `auth_type: none` from the HAR (no auth headers on any API call) and emitted a config-file-only CLI with no key management overhead.
