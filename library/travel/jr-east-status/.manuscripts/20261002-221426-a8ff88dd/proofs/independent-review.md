# Independent JR East Status review

Initial review completed 2026-10-03 JST. Review path: one authorized fresh-context reviewer, covering correctness, security, maintainability, source contracts, SKILL semantics, document correctness and output plausibility. No subagents, publication, global configuration changes, browser/CDP sessions or external messages were used. Findings below describe the initial reviewed implementation; builder fixes require a subsequent verification entry.

Project: `/Users/zjsng/Projects/Personal/Coding/jr-east-status-cli`.

## Review evidence

- Read the batch brief, source brief, absorb manifest and research.json, the actual Press phase 14–17 instructions, and the output-review skill. All five verified novel commands match research.json and resolve in live binary help. The anonymous source/auth description and anti-triggers fit the shipped scope.
- Independently read the native jreast parser/client/policy/handoffs and command implementations, generated CLI output/context wiring, the source-guide/client path and MCP registration. Generated framework-only cache/maxAge/sync scaffolding was not treated as an operational status-cache claim.
- Ran `go test -count=1 ./internal/jreast ./internal/cli ./internal/mcp`: all passed. Independently examined fixture assertions and real regional, express, certificate, planned and partial-itinerary outputs.
- Assessed eligible `status: pass` scorecard samples for impact, planned and coverage. The complete planned and coverage samples have plausible source facts, canonical links, explicit unknown calendar year and correct reporting rollover. The two-component impact sample preserves relevant details; Press's bounded evidence sample truncation is distinct from the runtime field loss below.
- Independently ran a real three-component anonymous `impact --agent` read. Outside the shell DNS sandbox it succeeded in 0.639 seconds, made two requests and emitted 4,182 bytes. Sobu remained suspended, Yamanote had a general normal label, and the invalid component remained explicit with a stderr warning. Full evidence: `independent-live-impact.out` / `.err` in this proofs directory. Initial default-shell DNS failure was an execution restriction, not provider failure.
- Added review-only Go overlay probes outside the project source tree. `reviewer-overlay.json` and `reviewer-probes-*.go.txt` reproduce the closed-source, clipping, Japanese normalization and observed Tokaido-heading cases without changing shipped code. `independent-boundary-tests.log` records the four failures and a passing native 20 ms timeout probe.
- Read the current first-party [planned-work page](https://www.jreast.co.jp/suspend/) to verify the Tokaido heading and mixed omitted/explicit years. The current source calls the selected route 東海道本線 while line discovery calls it 東海道線.

## Findings

### F1 — P1: recommended agent itinerary output loses disruption facts

Location: `internal/jreast/types.go:104` (`Notices` uses `omitempty`), `internal/cli/impact.go:119`; interaction with generated compact projection in `internal/cli/helpers.go:2294`.

Reproduction: `impact --lines kanto:sobuline,kanto:yamanoteline,kanto:not-a-real-line --agent --no-learn`. In `independent-live-impact.out`, Sobu has `notice_fact_count: 8` but no `notices` field, and `meta.truncated` is false. The other two rows lack notices, so the generated compact frequency rule drops the sparse field. The existing three-component acceptance output also lacks Sobu notices.

Impact: the recommended itinerary mode omits every affected section, direction, cause and replacement-transport fact for a disrupted component. The two-component happy example does not expose this bug.

Fix: preserve the bounded native notices field consistently for every itinerary component, including empty/failed components, or explicitly preserve domain fields at the native print boundary. Add a three-or-more-component serialization regression. Keep the shared generator helper unchanged for this local fix.

### F2 — P2: an explicit closed source becomes a missing line

Location: `internal/cli/impact.go:80`.

Reproduction: the `TestReviewerClosedSourceClassification` overlay returns the recognized Japanese and English closed-hours messages during a locally open reporting window. Both parsed source states are `outside_reporting_hours`, with zero rows. The command emits `line_not_found` and returns API exit 5. The proof log records the parsed source states and error.

Impact: a cached/explicit source closure near reopening is confused with invalid line identity; status and planned already inspect source reporting state. This breaks the requirement to distinguish closed reporting from source/identity failure.

Fix: use parsed source reporting state when handling an empty snapshot; preserve an explicit closed/unknown result for every requested component without inventing a line-not-found error. Add the observed-source-closed/local-clock-open regression.

### F3 — P2: construction parser clipping is hidden from the envelope

Location: `internal/jreast/handoffs.go:71`, `internal/jreast/handoffs.go:77`, `internal/jreast/handoffs.go:81`, `internal/jreast/handoffs.go:91`; `internal/cli/planned.go:64`.

Reproduction: `TestReviewerPlannedTruncationSignal` supplies seven matching headings with thirteen table rows per heading, then calls planned with `--limit 6`. Six notices and twelve rows per notice remain, but `meta.truncated` is false. The parser also clips cell text and date-expression counts without surfacing that clipping.

Impact: agents instructed to inspect truncation cannot tell that returned planned-work evidence is incomplete. The README explicitly promises that result truncation is reflected in `meta.truncated`.

Fix: return parser truncation metadata for matching-section, table-row, cell/date limits and propagate it into the command envelope, independently of the CLI result limit. Keep request/body/row caps intact.

### F4 — P2: native Tokaido discovery cannot find the source's construction heading

Location: `internal/jreast/handoffs.go:51`.

Reproduction: the current first-party planned source has the heading `東海道本線 線路切換工事に伴う列車の時刻変更について`; the discovered `tokaidoline` name is `東海道線`. `TestReviewerObservedTokaidoPlannedHeading` uses the observed heading and resolves the bundled native ID. ParsePlanned returns zero matches without an error.

Impact: a supported native line misses an actual published construction notice because the source uses a different established label. A handoff note protects against asserting no disruption but does not make the promised line-specific discovery work.

Fix: apply a narrow, source-evidenced native-ID-to-planned-heading alias for Tokaido; do not use unbounded fuzzy matching. Add a regression using the observed source label.

### F5 — P2: Japanese voicing marks collapse during exact-name resolution

Location: `internal/jreast/policy.go:41`–45 and `internal/jreast/policy.go:57`.

Reproduction: `TestReviewerJapaneseVoicingDistinct` shows `Normalize("わかしお・さざなみ") == Normalize("わかしお・ささなみ")`; both become `わかしお・ささなみ`. NFD plus removal of every Unicode Mn deletes Japanese dakuten/handakuten along with Latin macrons.

Impact: an advertised exact Japanese name resolver accepts a different Japanese spelling and can collapse distinct names into false equality/ambiguity. Source-native identifiers and Japanese meaning should remain distinct.

Fix: retain Japanese voicing marks while folding width and Latin accents; verify both voiced/unvoiced Japanese distinction and the existing Tōkaidō/macron examples.

### F6 — P2: one explicit date year is assigned to a mixed-year notice

Location: `internal/jreast/handoffs.go:56` and analogous `internal/jreast/parser.go:357`.

Reproduction/source evidence: the current Tokaido notice contains year-omitted November/October expressions and an explicit ending expression in January 2027. The initial implementation finds the first `YYYY年` anywhere in the section and assigns that year to the whole returned planned object. The original selection bug in F4 obscures this for the normal native-ID invocation, but it occurs as soon as the heading resolves.

Impact: `calendar_year: 2027` can wrongly lend a year to separate dates for which the source omitted it. A source page timestamp must not supply the year either.

Fix: keep the shared calendar year unknown for mixed explicit/omitted or conflicting-year expressions, retaining each original date expression. Apply the same conservative rule to regional notice facts. The builder independently identified this issue while the review was underway.

### F7 — P3/document warning: several generated statements do not fit the verified scope

Locations and fixes:

- `README.md:150` and `SKILL.md:62` assert that the capabilities are unavailable in every other tool. The brief records no relevant maintained integration found and explicitly avoids superiority claims. Replace the absolute exclusivity assertion with a neutral capability introduction.
- `README.md:202` and `SKILL.md:114` document `sources guide`; actual binary help and the registered promoted command expose `sources` with no guide subcommand. The extra positional currently runs only because the command ignores args. Document `sources`.
- `SKILL.md:184` advertises offline sync/search commands, while current root help exposes neither command. Replace this boilerplate with the actual local learning capability and the live-read requirement; keep the status/cache distinction already present.

These are documentation correctness/semantic warnings, not claims that runtime status uses a cache. No additional account or network features are requested.

## Phase statuses at initial review

- **14 / SKILL semantics: NEEDS FIX.** Trigger phrases, the five-command verified set, feature descriptions, anonymous auth narrative, limitations and worked-example intents match the implementation. F1 affects a runtime recipe promise; F7 contains an unsupported exclusivity claim.
- **15 / README, SKILL and AGENTS correctness: NEEDS FIX.** Main source policy, certificate semantics, read-only scope and anti-triggers are accurate. F3's explicit-truncation claim and F7's command/boilerplate statements need correction. AGENTS adds no new scoped error.
- **16 / output plausibility: WARN.** Eligible passing samples were actually inspected; a further hands-on itinerary invocation exposes F1. Sources/components themselves remain explicit, links/encoding are plausible and no ranking claims require assessment.
- **17 / local code review: FINDINGS.** F1–F6 are scoped corrective findings. No new security finding, source-auth bypass, unbounded live request loop, external mutation or missing native timeout boundary was found. Maximum request/body/deadline controls are real. Review-only overlay files do not alter the project.

Initial output-review outcome: WARN for F1. The current structured result appears after fix verification below.

## Fix verification

**Final outcome: PASS — scoped findings cleared.** The same independent reviewer verified the corrections; no additional reviewer or source edits by this reviewer were used.

F1–F7 cleared across two builder correction passes. The first pass fixed the sparse notices, explicit closed-source classification, section/table/date caps, Tokaido alias, Japanese voicing and mixed-year inference. Its independent probes exposed the remaining cell clipping; the second pass flags cell/title clipping too. The source-guide MCP surface was corrected without editing the generated handler: the raw typed endpoint is hidden, while the preserved CLI command is exposed as a structured command mirror. This preserves the approved source-link capability and all seven domain commands/five novel rows.

The final document audit additionally corrected `config.toml` to the actual `config.json` and replaced the inapplicable stored-secret/auth-write paragraph with the anonymous-source contract. The final runner metadata correction removes `--agent=true` from happy-argument annotations because the installed Press converts that into a positional `true`; required value flags remain. Neither correction expands operational scope.

### Independent verification evidence

- `independent-round1-live-impact.out`: the real three-component agent invocation now retains all eight Sobu notices, including Japanese `direction: both`, the 八街–成東 section and Typhoon cause. Yamanote's general normal label and the invalid component remain explicit. Two requests, 0.622 seconds, 10,876 stdout bytes.
- `independent-round1-live-tokaido-planned.out`: the real native-ID invocation resolves the source's formal Tokaido heading, preserves source date expressions and keeps the common calendar year null. Three requests, 1.266 seconds, 2,677 stdout bytes.
- `independent-round2-all-tests.log`: independently ran the complete jreast/cli/mcp package tests with the review overlay. All pass, including original regressions and the reviewer's separate closed-source, section/table/cell clipping, voiced Japanese, observed heading, mixed-year, native timeout, invalid budget and exact eighteen-request-cap checks. Review overlay files live only in this proofs directory.
- `independent-final-mcp-live.json`: independently compiled a temporary MCP binary from current source, initialized it over actual stdio, inspected the registered inventory and called the live `sources` mirror. It returned six canonical first-party URLs in a JSON collection, 2,174 text bytes in 0.335 seconds. The raw `sources_guide` tool is absent; 24 tools are registered. The accompanying runtime registration probe verifies `sources` and all seven domain commands use the child CLI mirror.
- `independent-final-metadata-tests.log`: the final happy-argument/source-annotation and MCP registration checks pass after the metadata-only adjustment.
- The eligible passing scorecard coverage/planned samples were inspected, and supplemented by the corrected live itinerary, planned and MCP behavior. No unsupported ranking or exact-train claims were introduced. A reviewer fixture attempting to replace the generated client's default transport was unsuitable for that constructor; it is not source-failure evidence and was superseded by the actual stdio/live MCP check.

### Template-shaped retro candidates

These are generated-surface metadata/style issues, separately recorded rather than hidden by edits to generator-owned code. They do not represent an outstanding native runtime finding.

- **P3 — `internal/cli/platform_client.go:517`**, thin Short `List client profiles`. Origin: DO-NOT-EDIT shared platform/profile command emission under the Press generator's template set; precise upstream template filename was not resolved in this review. The command's runtime purpose is clear from its longer help; fix descriptive template text upstream.
- **P3 — `internal/cli/teach.go:868`**, thin Short `List recorded learnings`. Origin: DO-NOT-EDIT shared learning command emission under the generator template set; precise template filename was not resolved. Improve the generic Short upstream rather than changing generated code in this CLI.
- **P3 — `tools-manifest.json` hidden-endpoint inventory**, the installed Press manifest still inventories `sources_guide` even though `mcp.endpoint_tools=hidden` suppresses it at runtime. Origin: generated tools-manifest metadata. Actual runtime inventory and the final stdio output are authoritative; the preserved source mirror supplies the approved feature. Reconcile hidden-endpoint inventory in the generator.
- The generic typed HTML handler did not apply CLI link extraction. The local sanctioned hidden-endpoint/command-mirror configuration now resolves that output mismatch. General HTML-extraction parity remains an upstream generator improvement rather than a local handler patch.

### Final phase outcomes

- **14 / SKILL semantics: PASS — no scoped findings.** Trigger phrases, verified-set alignment, command descriptions, anonymous auth, limitations and recipes match shipped behavior. The unsupported exclusivity sentence is removed.
- **15 / README, SKILL and AGENTS correctness: PASS — scoped correctness verified.** Current command names, configuration path, read-only/no-auth contract, live-source provenance, limits and MCP surface are accurate. Installer text remains explicitly conditional on publication. Generated style/inventory retro candidates are explained above.
- **16 / output plausibility: PASS — no findings.** Eligible passing samples were assessed; corrected live output is relevant, preserves every requested component and disruption facts, and provides canonical handoffs with explicit unknowns.
- **17 / local code review: PASS — findings cleared.** The corrections introduce no new scoped correctness/security/maintainability issue. Timeout/request/body boundaries remain effective; no account, booking, messaging, notification or publication action occurred. Generated retro candidates remain separately documented.

Raw source HTML captures are no longer needed for this independent review. Preserve the source identity catalogue, capture URL/hash/status metadata and compact live/test proofs; full captures can be omitted from the shippable archive.

## Closing-hours lifecycle follow-up

The same reviewer independently verified the later, real post-02:00 JST lifecycle correction. Captured source metadata is in `discovery/closure-capture-metadata.jsonl`; private captures show zero Japanese Shinkansen rows plus its explicit closure message, while the English page has zero rows and no closure message. They are provider lifecycle evidence, not access-denial evidence.

`internal/jreast/client.go:110` now returns the authoritative Japanese closed snapshot before attempting English translation. The guard requires a successfully parsed original page, zero rows and explicit parsed closed reporting state. Japanese request, parse/challenge and typed-throttle failures still return their errors; the new path does not convert an arbitrary empty/error page to a successful closure. The returned snapshot has no line operational claims; catalogue fallback carries its own identity-source tag. The source contract and README remain accurate.

### C1 — P2, found and cleared: express details were still requested after original closure

Initial follow-up location: `internal/cli/impact.go:89`. After the closed regional snapshot was replaced with catalogue identities, the express branch still called `jrExpress`. Catalogue URLs point to the regional pages, so it made two unneeded requests and could replace a known closure with `detail_source_error`.

Independent reproduction: `TestReviewerClosedExpressImpactStopsAtOriginalSource` supplies an explicit original closure and rejects English requests. Before the final guard it made three requests, emitted `detail_source_error` and exited 5 (`independent-closing-boundary-tests.log`). The actual 02:18 JST three-region itinerary reproduced the consequence with the provider's empty English express shell: five requests and an express fetch failure, while Shinkansen/Sobu remained closed. See `independent-postclosing-closing-impact.out/.err`. At that same time, Shinkansen status and line discovery each passed with one Japanese request, catalogue identity, empty statuses and null actual delay.

The builder guards `jrExpress` in both status and impact with `!jrSnapshotClosed(s)`. Original closure remains authoritative, all requested components remain explicit, and the detail/translation request is skipped. The fix preserves open-hours detail behavior and the reporting/error distinction.

### Final closing verification

- `independent-closing-final-tests.log`: the complete jreast/cli/mcp package tests with the independent overlay pass. They include the original-source closure test at a fixed locally open/cache-closed time, challenge/error-page rejection, the denied-English closed-express regression and prior bounds/semantic regressions.
- `independent-postclosing-final-impact.out/.err`: independently compiled a temporary CLI from the latest source and performed the actual three-region itinerary read at `2026-10-03T02:31:24+09:00`. Express, Shinkansen and Sobu all returned `outside_reporting_hours`, catalogue identities, empty operational labels and null train delay. Exactly three original Japanese requests; no fetch failures or stderr warnings. Runtime 1.634 seconds, stdout 4,303 bytes.
- The published certificate, planned-work, normal-label, reporting threshold and service-day meanings are unchanged. The correction introduces no new source/auth transport or external mutation.

**Current phase outcomes remain 14 PASS, 15 PASS, 16 PASS, 17 PASS; C1 is cleared and no scoped finding remains.** Raw closure captures can stay outside the shippable archive; their compact metadata and these independent proofs suffice for this review.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---
