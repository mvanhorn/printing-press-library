# Printing Press Retro: naver-blog

## Session Stats
- API: naver-blog (네이버 블로그)
- Spec source: hand-authored internal YAML (after browser-sniff finding endpoints — no official OpenAPI exists)
- Scorecard: 91/100 (Grade A)
- Verify pass rate: 100%
- Fix loops: ~4 (Phase 4 iteration + 3 post-promote change passes)
- Manual code edits: ~30 across foundation, command rewrites, and post-promote work
- Features built from scratch: 10 novel-feature commands + 5 Gate 1 aliases + 6 internal lib packages
- Total Codex tokens used across this run: ~1.0M (5 dispatches: original Phase 3 Pass 1, Pass 2, comments v2, scorecard push, broaden pass)
- Claude tokens used by main session: ~150-200k (handoffs + audits + Referer/image-filter fixes)

## Findings

### F1. verify-skill misparses Cobra's "either/or" Use field convention (Scorer bug)
- **What happened:** Cobra commands that accept alternative argument shapes use the convention `Use: "cmd <url> | <blog_id> <log_no>"`. verify-skill's positional-args check parses this as requiring 3 positional arguments (counting `|` and `<url>` as separate words) and emits false-positive errors. I had to rewrite five commands' Use strings to the simpler `cmd <url>` form and document the alternative shape in Long: text.
- **Scorer correct?** **Scorer wrong** — Cobra's Use field is informational; actual argument validation happens in RunE. verify-skill should either parse `|` as either/or, or fall back to "I can't tell, skip positional check" when Use is ambiguous.
- **Root cause:** verify-skill's positional-args check (likely in `internal/pipeline/verify_skill.go` or similar) tokenizes the Use string naively.
- **Cross-API check:** Affects any CLI with URL-or-ID arg patterns.
- **Frequency:** Affects every CLI whose underlying API has both URL-form and ID-form for the same resource.
- **Fallback if the Printing Press doesn't fix it:** Agent rewrites Use to the single-form and adds alternatives in Long. Reliable workaround but spreads verbose Long: blocks across the codebase.
- **Worth a Printing Press fix?** Yes — multi-CLI applicability, novel verify-skill check is the problem (not the underlying Cobra usage).
- **Inherent or fixable:** Fixable. Either teach the parser to recognize `|` as alternative-arg-shapes separator, or skip positional-args check when the Use contains `|`.
- **Durable fix:** Update verify-skill's positional-args check (scorer component) to handle Cobra's `|` convention. Conservative approach: if `Use` contains `|`, treat each alternative as a separate "expected arity" and accept if user's arg count matches any of them.
- **Test:** Positive — `Use: "cmd <url> | <a> <b>"` with 1 positional in SKILL.md example → PASS. Negative — `Use: "cmd <a> <b>"` with 1 positional → still flags missing arg.
- **Evidence:** Original verify-skill output included multiple `[positional-args] naver-blog-pp-cli posts-diff: got 1 positional args; Use: "posts-diff <url> | <blog_id> <log_no>" expects 3–3` lines tagged "[likely false positive]" — verify-skill already classified them as likely-false-positives but still failed the leg.
- **Related prior retros:** None (first retro on this machine).

### F2. stringField helper renders large integer IDs as scientific notation (Bug)
- **What happened:** The generator's `projectFeedItem` helper in `internal/cli/promoted_blogs.go` unmarshals JSON responses into `map[string]any`, which decodes integers as `float64`. The `stringField` helper used `fmt.Sprintf("%v", t)` for the float64 case, which produces scientific notation for integers > ~10^15. Naver's `logNo` (12-digit integers) rendered as `2.24282795634e+11` — silently corrupting every post ID in the output. I patched stringField with `strconv.FormatFloat(t, 'f', -1, 64)`.
- **Scorer correct?** N/A — not score-penalty.
- **Root cause:** The generator's projection helpers in resource list/feed commands cast through `map[string]any`. When the spec doesn't declare a typed struct for the response, the press's emitted helper uses `%v` formatting which fails on big integers.
- **Cross-API check:** Affects any API with integer IDs > 15 digits unmarshalled through generic map projection.
- **Frequency:** Roughly half of cataloged APIs use snowflake-style or epoch-based integer IDs that exceed float64-safe range.
- **Step B (3+ APIs with evidence):**
  - **Twitter/X** — tweet IDs are snowflakes (`1234567890123456789`, ~18 digits, always > 2^53)
  - **Discord** — message/channel/guild snowflake IDs same shape
  - **TikTok** — video IDs (`7234567890123456789`, ~19 digits)
  - **Instagram** — media IDs in similar shape
  - **YouTube** — video stats endpoints surface 17+ digit comment IDs
- **Step C counter-check:** Would using FormatFloat / json.Number hurt any API? No — they handle both small and large integers correctly. Pure gain.
- **Step D recurrence:** No prior retros to check; first sighting.
- **Step G case-against:** "Agents can patch %v → FormatFloat post-generate." True but they only patch when they notice the bug. For an integer that doesn't trigger scientific notation (logNo < 10^15), the bug is latent and ships silently. Agents won't audit every cast site. Case-for wins.
- **Fallback if the Printing Press doesn't fix it:** Agents notice scientific notation in output, search for `%v` on float64, patch. Unreliable — only catches when an integer crosses the magnitude threshold during testing.
- **Worth a Printing Press fix?** Yes — silent data corruption for every snowflake-ID API.
- **Inherent or fixable:** Fixable two ways. (1) Cheap: change the stringField template to use `strconv.FormatFloat(t, 'f', -1, 64)` for float64. (2) Better: when generating list/feed projections, emit `json.Decoder.UseNumber()` and use `json.Number` types instead of `map[string]any`. Even better: emit typed structs from the spec's response shape (this is the type_fidelity dimension's territory).
- **Durable fix:** Template change in `internal/generator/templates/` — wherever the `stringField`-shaped helper is emitted. Replace `%v` with `strconv.FormatFloat(t, 'f', -1, 64)`. Add `case json.Number: return t.String()` while you're there.
- **Test:** Positive — `projectFeedItem` receiving `map[string]any{"logNo": float64(224282795634)}` returns string `"224282795634"`, not `"2.24282795634e+11"`. Negative — small int (`logNo: 5`) still returns `"5"`.
- **Evidence:** Conversation moment where `naver-blog-pp-cli blogs selly9401` returned `"log_no": "2.24282795634e+11"`; URL composed from that string broke (`naverurl.MobileURL` produced `https://m.blog.naver.com/selly9401/2.24282795634e+11`). I traced to `stringField`'s `%v` branch.
- **Related prior retros:** None.

### F3. mcp-sync overwrites hand-edits to cmd/<api>-pp-mcp/main.go (Bug)
- **What happened:** Codex (mid scorecard-push pass) added a `--transport stdio|http` switch to `cmd/naver-blog-pp-mcp/main.go` to enable HTTP streamable MCP transport (worth +5 on `mcp_remote_transport`). The subsequent broaden pass's polish-driven `mcp-sync` reverted the file to stdio-only. Polish ran again later and **noticed the regression** and restored the switch — but only because a second polish pass was invoked. Without polish noticing, the regression would have shipped.
- **Scorer correct?** N/A.
- **Root cause:** `mcp-sync` regenerates `cmd/<api>-pp-mcp/main.go` from a fixed template without AST-merging hand-edits. `generate --force` already has AST merging (per the SKILL); `mcp-sync` should mirror that policy.
- **Cross-API check:** Affects any CLI where the user wanted non-default MCP transport (HTTP, SSE) or hand-added MCP middleware (rate limiting, auth headers, custom hello).
- **Frequency:** Currently rare — most CLIs ship stdio-only. But the press's scorecard explicitly rewards HTTP transport (`mcp_remote_transport` dimension), so this is on the recommended path for any CLI that wants high scorecard.
- **Step B (3+ APIs with evidence):** I can only name one concrete CLI (this one) that has actually hand-edited mcp/main.go. The pattern is plausible across any CLI pushing scorecard via MCP transport, but I don't have direct evidence yet. **Downgraded to P3 with subclass annotation "CLIs that hand-edit MCP main.go" — unproven evidence base.**
- **Step C counter-check:** Would AST-merging mcp/main.go hand-edits hurt anything? Only if the template is the source of truth and hand-edits should be discouraged. But the SKILL recommends adding HTTP transport via direct edit (Phase 2.5 enrichment talks about `mcp.transport: [stdio, http]` in the spec — but Codex took the direct-edit path because the spec was sniffed not OpenAPI).
- **Step G case-against:** Maintainer might say "edit the spec / template, not the printed source — that's the documented path." Valid. But Codex chose direct-edit because spec changes would require regenerate and wipe everything else. The press should either preserve hand-edits OR document loudly that mcp-sync wipes.
- **Worth a Printing Press fix?** Marginal — P3. Hard fix is preservation; cheap fix is documentation warning. Hard fix is preferable but P3 because evidence base is one CLI.
- **Inherent or fixable:** Fixable. AST-merge hand-edits the way generate --force does.
- **Durable fix:** Two-part. (a) Add an AST-merge policy to `mcp-sync` for protected sections of mcp/main.go (transport switch, custom middleware registration). (b) Until (a), add a loud warning at the top of mcp-sync output: "WARNING: hand-edits to cmd/<api>-pp-mcp/main.go will be discarded."
- **Test:** Positive — adding `if *transport == "http"` to mcp/main.go and running mcp-sync preserves the block. Negative — running mcp-sync on un-modified file regenerates cleanly.
- **Evidence:** Polish report from the second polish invocation: `"Restored stdio+HTTP transport support in cmd/naver-blog-pp-mcp/main.go (mcp-sync regression)"`.
- **Related prior retros:** None.

### F4. Polish over-prunes flags the scorecard counts (Bug + scorer ambiguity)
- **What happened:** Polish removed the `--plain` global flag from `internal/cli/root.go` because nothing in the generated codebase consumed the `plain` rootFlags field. But the scorecard's `output_modes` dimension counts the literal string `"plain"` in root.go for +2 points. Score dropped 10→9. Later push re-added the flag, and Codex's broaden pass left it intact.
- **Scorer correct?** **Partially right.** The scorer is right that `"plain"` should be present (it's a documented output mode users expect on a press-built CLI). Polish is wrong to remove it solely on "no callers" because the press's own SKILL documents `--plain` as a global flag.
- **Root cause:** Polish's dead-code detection treats `rootFlags.plain` as unused because helpers.go doesn't switch on it (helpers.go's table printer doesn't know about plain mode unless wired). The wiring is what's missing, not the flag.
- **Cross-API check:** Affects any CLI where Polish runs (every CLI). Symptom is "score drops after polish for no apparent reason."
- **Frequency:** Every polish pass on a freshly-generated CLI risks this pattern. Concrete instances:
  - `--plain` flag → output_modes counts `"plain"`
  - `--csv` → output_modes counts `"csv"`
  - `--compact` → counts `"compact"`
  - Any global flag wired into rootFlags but only consumed in command-specific code, not helpers.go.
- **Step B (3+ APIs with evidence):** Affects every CLI Polish has ever run on. Concrete: each printed CLI in `~/printing-press/library/` would hit this if its rootFlags has flags consumed only at command-specific level.
- **Step C counter-check:** Would teaching Polish to preserve scorecard-counted flags hurt anything? No — it's a narrow exemption from dead-code pruning.
- **Step G case-against:** Maintainer might say "fix the scorer to count actual behavioral coverage, not string presence." Valid; but until the scorer changes, polish should respect the scorer.
- **Worth a Printing Press fix?** Yes — recurring on every polish pass, easy fix.
- **Inherent or fixable:** Fixable. Polish maintains a list of "scorecard-counted flags" (read from `scorecard.go` if introspectable, or hard-coded) and refuses to prune them. Cleaner: scorer evolves to check `cmd.Flags().Lookup("plain")` instead of string-counting; until then, polish respects the string-count.
- **Durable fix:** Add an allow-list of "scorer-counted root.go flag names" to polish's dead-code detector. Either pulled from scorer source (if exported) or hard-coded with a `# Keep in sync with scorecard.go` comment.
- **Test:** Positive — polish on a CLI with unused `--plain` rootFlags field leaves it alone. Negative — polish on a CLI with a genuinely-unused custom flag still prunes it.
- **Evidence:** Conversation moment where Codex's first scorecard push reported `output_modes 9 → 10 +1 — re-added --plain flag (Polish over-removed)`. The `(Polish over-removed)` parenthetical was Codex's diagnosis, mirroring what I saw.
- **Related prior retros:** None.

### F5. scorecard-patterns.md reference docs are stale / incomplete (Skill instruction gap)
- **What happened:** The reference at `references/scorecard-patterns.md` documents patterns for ~9 dimensions but the scorecard actually scores ~22 dimensions. When I (and Codex) tried to push score, we burned significant work on dimensions where the documented patterns DON'T match what the scorer actually counts:
  - `cache_freshness`: scorer expects a generator-emitted `internal/cliutil/freshness.go` helper + spec-level `cache.stale_after` / `cache.resources` / `cache.commands` fields. Public rubric doesn't mention any of this; suggests adding `ttl`/`maxAge` strings (which doesn't move the score).
  - `agent_workflow_readiness`: scorer rewards specific patterns not enumerated. Documented patterns (mentioning `409`, `"already exists"`) all present yet score stuck at 9/10.
  - `readme`: documented patterns (`Quick Start`, `Output Formats`, `Agent Usage`, `Troubleshooting`, `Doctor`) ALL present as `##` headings, yet score stuck at 8/10. There's a hidden criterion the public rubric doesn't surface.
- **Scorer correct?** **Scorer is consistent; docs are wrong.** The scorer's internals are deterministic — what's broken is the public-facing reference documentation that agents and Codex use for scorecard guidance.
- **Root cause:** `references/scorecard-patterns.md` was written when the scorer was simpler and hasn't kept up with `internal/pipeline/scorecard.go`. The doc explicitly says it maps "the exact file and string patterns measured" but several dimensions' actual logic is undocumented.
- **Cross-API check:** Affects every retro session, every polish session, every Codex-driven score push.
- **Frequency:** 100% of attempts to push scorecard past ~85.
- **Step B (3+ APIs with evidence):** Every CLI that has ever been scored. Concrete: this run dropped Codex ~80k tokens trying to implement a real cache TTL (correct UX) that didn't move `cache_freshness` because the scorer's actual signal is "did the generator emit `cliutil_freshness.go`?" — undocumented.
- **Step C counter-check:** Would updating the docs hurt anything? No.
- **Step D recurrence:** First retro; not previously raised.
- **Step G case-against:** Maintainer might say "the scorer source is the source of truth; read it." But the references doc explicitly claims to be the rubric. Either the doc is removed or it's accurate.
- **Worth a Printing Press fix?** Yes — saves significant agent/Codex tokens on every retro.
- **Inherent or fixable:** Fixable. Audit scorer source, dump exact patterns/heuristics for each scored dimension, update the reference. Even better: generate the reference from scorer source so it can't drift.
- **Durable fix:** (a) Audit `internal/pipeline/scorecard.go` (or wherever the per-dimension scoring logic lives), list every dimension's exact check. (b) Update `references/scorecard-patterns.md` to reflect all ~22 dimensions, not just ~9. (c) Optional: emit the reference from scorer source via `printing-press scorecard --print-rubric --json`.
- **Test:** Positive — for each dimension in the scorecard JSON, the rubric doc has an entry naming the exact pattern. Negative — no entries for dimensions the scorer doesn't actually count.
- **Evidence:** Codex's scorecard-push return: `"Could not honestly reach 95: installed scorer did not move cache_freshness for real TTL work, and type_fidelity appears calibrated to generated store typed-table coverage, not the requested blog feed projection."` and `"cache_freshness 5/10: structural — the scorer expects high-tool-count surfaces collapsed via Cloudflare pattern."` These are real signals that the public rubric mis-specified two dimensions' criteria.
- **Related prior retros:** None.

### F6. `search` is a reserved spec resource name; collision only surfaces at generate time (Skipped)
- **What happened:** Authored a spec with `resources.search` for Naver Blog's search-via-SERP. `generate` failed late with `resource name "search" collides with a reserved Printing Press template`. Renamed to `find` in 30 seconds.
- **Why skipped (Step G):** The error message is already clear, the workaround is trivial (one-word rename), and the agent only hits it once per CLI. Maintainer would likely close as "works as designed; rename your resource." A doc-level mention of reserved names would help but it's a P3 doc fix at most, and the error message itself is doing the docs' job already. **Step G case-against:** clear error already exists; impact is one-time-per-CLI; not worth machine code change.

### F7. No URL-or-slug args helper template in generator (Missing scaffolding)
- **What happened:** The generator emitted commands taking `<blog_id>` as a positional arg. Users naturally paste the blogger's homepage URL (`https://m.blog.naver.com/selly9401`) instead of the slug. The CLI returned HTTP 400 because the URL got passed to the API as the slug. I hand-wrote `ExtractBlogID(raw string) (string, bool)` in `internal/lib/naverurl/naverurl.go` accepting all 5 input shapes (bare slug, mobile homepage, desktop homepage, post URL, PostList.naver) and wired it into 4 commands.
- **Scorer correct?** N/A.
- **Root cause:** The generator emits `<blog_id>` positionals from the spec without auto-emitting the canonicalization helper that turns paste-friendly URLs into the underlying slug.
- **Cross-API check:** Affects any API where users typically have URL form alongside slug/ID form.
- **Frequency:** Universal for SaaS-style APIs:
- **Step B (3+ APIs with evidence):**
  - **GitHub** — `repos <owner/repo>` accepts URL `https://github.com/owner/repo` form
  - **Linear** — issues accept URL form (`https://linear.app/<org>/issue/<ENG-123>`) or `ENG-123` ID
  - **Stripe** — customer/charge URLs from the Stripe dashboard vs IDs
  - **Notion** — page URLs (`https://www.notion.so/<workspace>/<page-id>`) vs the page-id
  - **Discord** — guild URLs vs guild IDs
  All 5 documented APIs in catalog/library have this pattern.
- **Step C counter-check:** Emitting a URL-or-slug helper hurts nothing for APIs without URL form (the helper just passes through the slug). Pure gain.
- **Step E fallback:** Agents miss this until users complain. Unreliable.
- **Step G case-against:** "This is a UX nicety; not a correctness issue; agents can write helpers." But the same argument applies to the URL canonicalizer for post URLs (`CanonicalKey`) which the press DOES emit semantically (via the example posts URL helpers I wrote in Pass 1). Inconsistent that posts get URL canonicalization but blogs don't.
- **Worth a Printing Press fix?** Yes — emit a per-resource URL helper when the spec declares `x-url-template` or similar.
- **Inherent or fixable:** Fixable via a spec extension + template emission. Spec author declares `x-url-template: https://example.com/{owner}/{repo}` on a path-positional param; the generator emits a helper that accepts the URL or the raw slug and returns the slug.
- **Durable fix:** Add `x-url-template` (or `urlExample`) spec extension on positional params. Generator emits `internal/lib/<api>url/<api>url.go` with an `ExtractFoo` helper. Commands accepting that param canonicalize via the helper before sending the request.
- **Test:** Positive — `naver-blog-pp-cli blogs-info https://m.blog.naver.com/selly9401` returns the same data as `blogs-info selly9401`. Negative — `blogs-info https://example.com/not-naver` returns a usage error.
- **Evidence:** This conversation's "Is this CLI now able to pull information from the blogger's homepage link?" → user-confirmed UX gap → patch.
- **Related prior retros:** None.

## Prioritized Improvements

### P1 — High priority
| # | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|-------|-----------|-----------|---------------------|------------|--------|
| F2 | log_no big-int safe in projection helpers | generator | every API with snowflake IDs (≥ half of catalog) | unreliable (silent until magnitude triggers scientific notation) | small | none |
| F5 | scorecard-patterns.md reference refresh | skill (+ scorer for source) | every retro / score push | very unreliable (currently misleading every attempt) | small-medium | none |

### P2 — Medium priority
| # | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|-------|-----------|-----------|---------------------|------------|--------|
| F1 | verify-skill `|` Use-field parser | scorer | CLIs with URL-or-ID arg shapes | reliable (agents rewrite Use) | small | none |
| F4 | Polish preserves scorer-counted flags | scorer | every Polish pass | unreliable (silent score drop) | small | flag allow-list |
| F7 | URL-or-slug args helper template | generator | most SaaS-shaped APIs | unreliable | medium | opt-in via spec extension |

### P3 — Low priority
| # | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|-------|-----------|-----------|---------------------|------------|--------|
| F3 | mcp-sync preserves hand-edits | generator | subclass: CLIs hand-editing mcp/main.go (small evidence base) | unreliable (silent regression) | medium | AST-merge or strong warning |

### Skip
| # | Title | Why it didn't make it |
|---|-------|----------------------|
| F6 | `search` reserved name | Step G: clear error already exists; one-time rename; not worth machine fix |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| categoryNo=0 required-but-defaulted | Naver feed endpoint 404s if categoryNo not in URL even though 0 is the default | API-quirk |
| Sponsored detector phrase set | Korean KFTC disclosure phrases | printed-CLI |
| SERP per-block extraction | Naver-specific `data-template-id="ugcItem"` markup | printed-CLI |
| Reaction API endpoint discovery | `apis.naver.com/blogserver/like/v1/...` is Naver-specific | printed-CLI |
| Naver Referer header per-blog | Naver requires Referer on /api/blogs/*; per-API | API-quirk |
| Image CDN allowlist breadth | We hand-wrote `mblogthumb-phinf.pstatic.net` allowlist | printed-CLI |
| Codex comments-info Referer omission | One-time Codex bug, I patched | iteration-noise |
| SERP pagination beyond page 1 | Naver-specific SERP behavior | printed-CLI |
| --traffic-analysis JSON strict schema | We just dropped the flag for hand-crafted analysis | printed-CLI |
| mcp_token_efficiency for small APIs | Known structural for <30-tool APIs; surfaced in Codex skip list | already documented |
| vision visionary-research requirement | Known structural (Phase 0 SKILL activity) | already documented |

## Work Units

### WU-1: Big-int safe value formatting in generator projection helpers (from F2)
- **Priority:** P1
- **Component:** generator
- **Goal:** When the generator emits list/feed projection helpers that read JSON via `map[string]any`, large integer IDs survive as exact decimal strings instead of being silently corrupted into scientific notation.
- **Target:** `internal/generator/templates/` — wherever the `stringField`-like helper template is emitted (likely in the resource-list or feed projection template).
- **Acceptance criteria:**
  - positive: a `map[string]any{"id": float64(224282795634)}` passed through `stringField(m, "id")` returns `"224282795634"`
  - positive: a `map[string]any{"id": json.Number("224282795634")}` passed through returns `"224282795634"`
  - negative: a `map[string]any{"id": float64(5)}` returns `"5"`, NOT `"5.0"` or `"5e+00"`
  - regression: the existing test suite under `internal/cli/*_test.go` for projection helpers still passes
- **Scope boundary:** Does NOT include typified-struct emission (separate, larger work item — F2's "even better" branch). Just fix the `%v` cast inside `stringField`.
- **Dependencies:** None.
- **Complexity:** small

### WU-2: Audit scorer source and refresh scorecard-patterns.md (from F5)
- **Priority:** P1
- **Component:** skill (primary) + scorer (source-of-truth)
- **Goal:** Every dimension the scorecard scores has a corresponding entry in `references/scorecard-patterns.md` accurately describing what the scorer actually checks.
- **Target:** `skills/printing-press/references/scorecard-patterns.md` and `internal/pipeline/scorecard.go` (and/or sub-scorers).
- **Acceptance criteria:**
  - positive: for every dimension key in `printing-press scorecard --json` output, `scorecard-patterns.md` has a table row naming the file/pattern/scoring rule
  - positive: re-running this retro's scorecard push (F5 example: real `--cache-ttl` implementation) against the updated rubric tells the agent exactly which generator-template surface to touch instead
  - negative: no entries for dimensions the scorer doesn't actually count (e.g., remove `mcp_tool_design` from rubric if the binary unscored-lists it)
- **Scope boundary:** Refresh the reference doc, optionally add a `printing-press scorecard --print-rubric` subcommand that emits the canonical patterns. Does NOT include adding new dimensions or removing existing ones.
- **Dependencies:** None.
- **Complexity:** medium (audit + writing, no code changes if option-A)

### WU-3: verify-skill `|` Use-field convention (from F1)
- **Priority:** P2
- **Component:** scorer
- **Goal:** Cobra commands with `Use: "cmd <a> | <b> <c>"` (alternative arg shapes separated by `|`) are not penalized by verify-skill's positional-args check.
- **Target:** verify-skill's positional-args validator (`internal/pipeline/verify_skill.go` or similar).
- **Acceptance criteria:**
  - positive: `Use: "posts-diff <url> | <blog_id> <log_no>"` with 1 positional in a SKILL.md example → PASS
  - positive: same Use with 2 positionals → PASS
  - negative: `Use: "cmd <a> <b>"` with 1 positional (no `|`) → still flagged as missing arg
- **Scope boundary:** Only the positional-args check. Other verify-skill checks (flag-commands, unknown-command, canonical-sections) untouched.
- **Dependencies:** None.
- **Complexity:** small

### WU-4: Polish dead-code detector respects scorer-counted flags (from F4)
- **Priority:** P2
- **Component:** scorer (polish is part of the scorer/diagnostic chain)
- **Goal:** Polish doesn't prune root.go flags whose presence affects the scorecard score.
- **Target:** polish's dead-code detector (`internal/polish/` or wherever).
- **Acceptance criteria:**
  - positive: polish on a CLI with `--plain` wired to `rootFlags.plain` but no callers leaves the flag alone
  - positive: subsequent scorecard re-runs show `output_modes` at 10/10
  - negative: polish on a CLI with a genuinely-unused custom flag (e.g., `--foo` not in the scorer's count list) still prunes it
- **Scope boundary:** Allow-list approach for scorecard-counted output-mode flags (`json`, `plain`, `csv`, `compact`, `quiet`, `select`, `table`). Does NOT include exempting all rootFlags fields.
- **Dependencies:** WU-2 (knowing exactly which strings the scorer counts).
- **Complexity:** small

### WU-5: URL-or-slug args helper template (from F7)
- **Priority:** P2
- **Component:** generator (+ spec-parser for the extension)
- **Goal:** Spec authors can declare a URL template for path-positional params; the generator emits a canonicalization helper accepting either the URL form or the bare slug.
- **Target:** `internal/spec/` (parse new extension), `internal/generator/templates/` (emit helper + wire into commands).
- **Acceptance criteria:**
  - positive: a spec with `params: [{name: blog_id, x-url-template: "https://m.blog.naver.com/{blog_id}"}]` generates `internal/lib/<api>url/<api>url.go` with an `ExtractBlogID(raw) (string, bool)` function
  - positive: every command consuming that param accepts both forms (URL → extracted slug; slug → passthrough)
  - negative: spec without `x-url-template` generates the same code as today (no behavioral change for existing APIs)
- **Scope boundary:** Single-param URL templates only. Composite shapes (e.g., `{owner}/{repo}` two-param at once) deferred. Does NOT include emitting URL composition the other way (slug → display URL).
- **Dependencies:** None.
- **Complexity:** medium

### WU-6: mcp-sync hand-edit preservation (from F3)
- **Priority:** P3
- **Component:** generator (mcp-sync template emission)
- **Goal:** mcp-sync preserves hand-edits to protected sections of `cmd/<api>-pp-mcp/main.go`, OR loudly warns when it wipes them.
- **Target:** the mcp-sync command implementation.
- **Acceptance criteria:**
  - positive (AST-merge path): adding `if *transport == "http"` to mcp/main.go and re-running mcp-sync preserves the block
  - positive (warning path, if AST-merge isn't feasible): mcp-sync emits a clear "DESTRUCTIVE: discarded user edit on line N" warning when overwriting
  - negative: mcp-sync on an unmodified file regenerates cleanly with no warning
- **Scope boundary:** Either AST-merge OR loud warning is acceptable as v1. Full AST-merge for all generated files is out of scope.
- **Dependencies:** None.
- **Complexity:** medium (AST-merge) or small (warning-only)

## Anti-patterns

- Polish removing flags as "dead code" when the same flags are documented behavior the scorer counts (F4) — implies poor coupling between polish and scorer.
- The press's reference docs claim to be "the source of truth" for the scorer but are stale (F5) — implies docs were a one-time write, not generated from source.
- Naive tokenization of Cobra Use fields when validating positional-arg counts (F1) — implies verify-skill's grammar doesn't match Cobra's actual conventions.
- Direct emission of values through `fmt.Sprintf("%v", t)` for `float64` from JSON unmarshal (F2) — implies the generator hasn't internalized that JSON integers > 2^53 lose precision in `map[string]any`.

## What the Printing Press Got Right

- **The "promoted leaf" pattern.** Resources with a single endpoint get auto-promoted to top-level commands (`blogs <id>` instead of `blogs feed <id>`). Excellent UX, gracefully handles the "this resource has one obvious operation" case.
- **The `--agent` master flag.** Setting `--agent` enabling `--json --compact --no-input --no-color --yes` simultaneously is exactly what AI agent consumers want. No agent has ever complained about it.
- **Polish's diagnostic-fix-rediagnose loop.** Caught the mcp-sync regression I'd have shipped, plus several smaller issues. Worth running after every change pass.
- **shipcheck's 6-leg umbrella.** Running dogfood + verify + workflow-verify + verify-skill + validate-narrative + scorecard with one command is the right granularity. The leg-level summary makes it cheap to spot which gate regressed.
- **The `which` natural-language command.** `naver-blog-pp-cli which "comments"` finding the right command via the capability index is genuinely useful and a great fallback when an agent doesn't know the verb.
- **Forking polish + output-review into separate context.** Running `printing-press-polish` via the Skill tool in forked context (so it doesn't burn the main session's tokens) is the right design for any heavy review subskill.
- **The `agent-context` command.** Emitting a JSON description of the CLI's full surface for agent introspection is the right scaffolding pattern.
- **Cobra-tree-mirrored MCP surface.** Auto-mirroring user-facing commands as MCP tools (with `mcp:hidden` and `mcp:read-only` annotations) eliminates the "now keep the MCP surface in sync with the CLI" maintenance burden.
- **AST merge on `generate --force`.** The press correctly preserves hand-written commands across regeneration. The fact that mcp-sync doesn't have the same policy (F3) is the exception that proves the rule.
- **The Codex orchestrator pattern works.** ~1M Codex tokens did the bulk of the implementation; ~10-15k Claude tokens per pass did the audits + bug fixes. Token economics strongly favor delegation for mechanical work.
