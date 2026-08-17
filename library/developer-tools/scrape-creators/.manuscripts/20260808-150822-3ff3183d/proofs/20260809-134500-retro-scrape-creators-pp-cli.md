# Printing Press Retro: scrape-creators (reprint + publish session)

## Session Stats
- API: scrape-creators
- Spec source: public docs OpenAPI (docs.scrapecreators.com/openapi.json, 175 paths)
- Scorecard: 93/100 (A)
- Verify pass rate: 100% (live)
- Fix loops: 4 CI rounds on the publish PR (mvanhorn/printing-press-library#1680, ended 5/5 Greptile, 7/7 checks green)
- Manual code edits: 10 (6 from pre-publish multi-agent code review, 1 live-gate-found bug, 3 library-CI reconciliations)
- Features built from scratch: 6 novel commands (prior generation session, carried by absorb)
- Session type: reprint publish + drive-to-green. Generation happened in a prior session; this retro covers the publish/review tail with full conversation history, plus generation-session discoveries carried via patch catalog.

## Findings

### 1. Generator's novel-command registration is invisible to the library's SKILL verifier (Bug)
- **What happened:** The library CI check "Validate SKILL.md against shipped CLI source" hard-failed the publish PR: `account: --posts is declared elsewhere but not on account`. The 4.30 generator emits `addNovelCommandIfAbsent(cmd, newNovelAccountBudgetCmd(flags))`, but the library's `verify_skill.py` command-tree parser only recognizes `.AddCommand(newXxxCmd(` (ADDCMD_CHILD_RE). When the parent group also has generated `.AddCommand` children, the strict tree wins over the legacy fallback, so novel subcommands vanish and their documented flags fail CI.
- **Scorer correct?** Split-brain: the press's own `verify-skill` passes; the library's verifier fails. The library verifier has the blind spot, but the press generator changed shape (4.27 emitted direct `cmd.AddCommand(newNovelAccountBudgetCmd(flags))` — verified against the published base tree) without the library verifier following.
- **Root cause:** Generator template changed novel registration to a helper call the downstream verifier's regex cannot see.
- **Cross-API check:** Deterministic for every 4.30+ print that has a novel command under a group with generated children. Evidence: scrape-creators `account` group (CI fail reproduced locally with the downloaded `verify_skill.py`: 4 errors, then all-pass after switching to direct AddCommand); the same template registers novels in every printed CLI; the previously *published* scrape-creators tree used the direct form and passed.
- **Frequency:** every 4.30+ CLI with grouped novel commands.
- **Fallback if not fixed:** every publisher hand-edits generated `DO NOT EDIT` group files and records a patch — or fails CI without understanding why (empty error message names the parent group, not the registration).
- **Worth a fix?** Yes — hard CI failure, deterministic, self-inflicted drift between generator and downstream verifier.
- **Inherent or fixable:** Fixable at template level.
- **Durable fix:** Either (a) generator emits `cmd.AddCommand(newNovelXxxCmd(flags))` directly (collision-safety can live inside the constructor or at print time — generated and novel names are known when printing), or (b) coordinate a `verify_skill.py` regex update in the library and keep the helper. (a) is self-contained in this repo.
- **Test:** print a CLI with a novel command under a group with generated children; run the library's `verify_skill.py` — zero flag-commands errors (positive); a CLI without novels remains unchanged (negative).
- **Evidence:** PR #1680 CI round 1 fail → pass after commit 121a4e5c0; local repro with `.github/scripts/verify-skill/verify_skill.py`.
- **Related prior retros:** None found for registration shape. Adjacent open issues: #3957 (novel reserved-name gate), #3849 (novels invisible to tools-manifest) — `extends`: same "novel commands are second-class to downstream tooling" family, different mechanisms.

### 2. Live dogfood JSON reports embed the live API key in output samples (Bug, security)
- **What happened:** The publish live gate's full-matrix report (`publish-live-gate-full-supplementary.json`) contained the real ScrapeCreators API key in 10 `output_sample` fields (CLI error output echoing request context). The publish flow stores gate reports under `.manuscripts/<run>/proofs/` — which ships to the PUBLIC library repo. The key was briefly pushed to the fork branch before an exact-value scan caught it; the branch was deleted and recreated, but orphaned blobs can outlive branch deletion, so the key had to be treated as compromised. Redacted form on the wire: `<REDACTED:vendor-api-key:28ch>` — the value itself is intentionally not quoted here.
- **Scorer correct?** N/A (not a score penalty). The dogfood report writer is the component.
- **Root cause:** `dogfood --json` copies raw command output into `output_sample` without redacting the value of the env var it was itself given via `--auth-env` (or any vendor-prefix token).
- **Cross-API check:** Any printed CLI whose error paths echo request URLs/params or whose commands print auth diagnostics. The report format and `--auth-env` plumbing are shared by every CLI. The publish skill's own docs tell agents to commit these reports as proofs — the leak path is paved by the machine.
- **Frequency:** every CLI, whenever a probe fails in a way that echoes request context.
- **Fallback if not fixed:** each publisher must remember to grep proofs for the key before committing — this failed once already in this session despite an experienced flow; two prior PII leaks to the public library are documented in the publish skill itself.
- **Worth a fix?** Yes — small change, high blast radius, defense-in-depth for a public repo.
- **Inherent or fixable:** Fixable: the report writer knows the exact secret value (it received the env var name); replacing every occurrence with a redaction marker before serializing is lossless for the report's purpose.
- **Durable fix:** In dogfood's report serialization, scrub the resolved `--auth-env` value (and vendor-prefix token patterns) from `output_sample`/stderr fields, writing `<REDACTED:auth-env>` instead.
- **Test:** run live dogfood with a deliberately failing probe whose error echoes the key; report contains the marker, not the value (positive); reports without secrets are byte-identical (negative).
- **Evidence:** This session — 10 redactions applied across three copies; remote branch delete/recreate; key rotation recommended to the operator.
- **Related prior retros:** None on redaction. #3893 (`live dogfood: unclassified write commands fail open`) — `extends`: both are dogfood-safety findings; different surface.

### 3. `publish package` cannot parse the release ledger the library itself writes (Bug) — CONFIRMED, already filed as #4039
- **What happened:** `publish package` hard-errored: `parsing .printing-press-release.json: json: cannot unmarshal object into Go struct field CLIReleaseManifest.changes of type string`. The library's post-merge release workflow now writes `changes` as `[]object` (title/pr/url/commit); the 4.30.1 binary expects `[]string`. Workaround discovered: stash the ledger out of the source dir during `publish package`, restore after (the publish skill's own copy step re-installs the public ledger anyway).
- **Scorer correct?** N/A — binary parse bug.
- **Root cause:** Binary's `CLIReleaseManifest` schema lags the library workflow's output.
- **Cross-API check:** Every reprint/update of any CLI released since the library workflow started writing rich `changes` (scrape-creators 2026.8.1 observed; the same workflow stamps every release).
- **Frequency:** every published-CLI update.
- **Durable fix:** accept both shapes (custom unmarshaler), never require parsing fields publish doesn't use.
- **Evidence:** This session; independently hit and filed by another user (aborruso) on 2026-08-08 as #4039 — this retro adds a second reproduction plus the stash-restore workaround as interim guidance.
- **Related prior retros:** #4039 — `aligned` (same bug, same diagnosis). Action: comment, don't duplicate.

### 4. Reprint drops the base tree's runtime-version shape in MCP main, tripping the library's release-ledger guard (Assumption mismatch)
- **What happened:** The 4.30 generator emits `var version = "..."` in `cmd/<cli>-pp-mcp/main.go` and passes it to `server.NewMCPServer`. The published base tree hardcodes the literal (library release automation owns stamping). The library's Verify guard hard-failed the PR: "release-ledger files/runtime version changed with normal CLI files. Remove cmd/scrape-creators-pp-mcp/main.go from this PR." Greptile independently flagged it P1. Manual reconciliation (restore the base literal, drop the var) fixed it.
- **Scorer correct?** The library guard is correct by its contract; the generator emits a shape that violates it on every reprint.
- **Root cause:** The reprint pipeline (absorb/regen-merge) reconciles novel features and patches but not the runtime-version *declaration shape* against the published base. The publish skill documents the manual reconciliation — the machine could do it.
- **Cross-API check:** Every reprint of every published CLI with an MCP main (all recent CLIs ship one: spotify, youtube, substack-reader, movie-goat — all stamped by the same release workflow; any reprint hits the same guard).
- **Frequency:** every reprint of a published CLI.
- **Fallback if not fixed:** publish skill's manual "version declaration reconciliation" step, which agents must notice and execute correctly per surface (root.go, version.go, MCP main); it was executed for root.go in this reprint but missed for MCP main until CI failed.
- **Worth a fix?** Yes — the manual step demonstrably half-fires (root.go survived, MCP main didn't), and the failure costs a CI round per reprint.
- **Inherent or fixable:** Fixable — regen-merge already diffs against the base tree; extend it to preserve the base's version-declaration layout in the three runtime surfaces.
- **Durable fix:** In the reprint/absorb path, detect version declarations in root.go/version.go/MCP main of the base tree and carry their exact shape+value into the fresh print (same rule the publish skill states as manual guidance today).
- **Test:** reprint a stamped CLI; `git diff base -- <3 files> | grep 'var version\|"20'` is empty (positive); fresh prints keep the generator default (negative).
- **Evidence:** PR #1680 Verify fail round 1 + Greptile P1, fixed in 121a4e5c0; patch `scrape-creators-publish-ci-reconciliation`.
- **Related prior retros:** #3949 (acceptance marker fingerprint, publish gate) — `extends`: publish-gate contract family, different mechanism.

### 5. Compact/verbose stripping still deletes the sole payload array; CLI-synthesized sidecar arrays retrigger it (Bug, regression)
- **What happened:** Two-part. (a) The 4.30 generator re-introduced the exact bug fixed by amend on the published CLI (patch `scrape-creators-compact-payload-array`, PR #1624/F7): `--agent`/`--compact` strips a blocklisted key (`comments`) even when it is the response's ONLY payload array — paid call, empty output. The fix (`resolveVerboseObjectFields`: never strip the only non-empty array-of-objects) had to be re-ported by hand during the reprint. (b) The pre-publish code review then found the fix is defeated by the CLI's own partial-failure bookkeeping: `fetch_failures` (a synthesized array-of-objects sidecar) counts as a competing payload because `envelopeMetadataArrayKeys` lists only errors/warnings — one failed sub-fetch strips the entire paid payload again (patch `scrape-creators-compact-fetch-failures-sidecar`).
- **Scorer correct?** N/A.
- **Root cause:** Generator's compact template lacks (a) the sole-payload protection and (b) the rule that CLI-synthesized bookkeeping arrays are envelope metadata, never domain collections.
- **Cross-API check:** The compact helpers template is identical across every printed CLI; the blocklist (`comments`, `description`, ...) plus the "is the payload" collision occurs on any API whose domain object IS a blocklisted key (the original bug hit six platforms' comments commands). `fetch_failures` is the generator's own fan-out scaffold vocabulary.
- **Frequency:** subclass: every CLI with a blocklisted-key payload; every CLI using the fan-out failure scaffold.
- **Fallback if not fixed:** each affected CLI re-discovers silent data loss in agent mode (the worst failure shape: exit 0, empty data, credits spent), then re-ports the same fix on every reprint.
- **Worth a fix?** Yes — data-loss class, already cost one amend + one re-port + one review finding on a single CLI.
- **Inherent or fixable:** Fixable at template level; the patch entries contain the exact durable rules.
- **Durable fix:** Adopt in the helpers template: (1) `resolveVerboseObjectFields` sole-payload semantics (competitor = non-empty array-of-objects only); (2) `envelopeMetadataArrayKeys` includes generator-synthesized sidecars (`fetch_failures`/`FetchFailures`, and any future scaffold-emitted bookkeeping arrays).
- **Test:** compact a `{scalars..., comments:[...]}` response — array kept (positive); compact a post object with `images` payload + `comments` sidecar — sidecar still stripped (negative); add non-empty `fetch_failures` next to the payload — payload kept.
- **Evidence:** amend F7 history, reprint re-port, review finding #7 validated + regression tests `amend_compact_comments_test.go`.
- **Related prior retros:** Closed #3089/#2950 — `extends`: compact projection family; the sole-payload rule is new.

### 6. Live-gate signal quality: fixture-bound arg failures drown real regressions; publish skill mandates a level that cannot pass (Scorer bug + Skill instruction gap)
- **What happened:** The full-matrix live gate fails 137 probes on this CLI (139 at generation) — all HTTP 400s on generator-synthesized happy-args the live API rejects (mock handles, fabricated URLs). Consequences observed this session: (1) a REAL shipped bug (`account estimate` parsed the live balance as 0 → spurious exit 7 on every plan) was indistinguishable inside the noise at generation time and only surfaced when publish reran the gate; (2) the publish skill's Step 4.5 mandates `--level full` + stop-on-fail, which is structurally unpassable here — the accepted phase5 marker contract is level=quick, so the operator must deviate from the skill to publish at all (3 full-matrix runs were spent discovering this).
- **Scorer correct?** Partially. The probes genuinely fail — but "the API rejected an impossible fixture argument" and "the CLI is broken" are different facts reported identically.
- **Root cause:** (scorer) dogfood classifies arg-infeasible 4xx as `fail`; (skill) publish gate level contradicts the marker contract dogfood/validate actually accept.
- **Cross-API check:** Any CLI with required exotic-format params (URLs, opaque IDs) — most large printed CLIs; evidence beyond this CLI is inferential, which caps the priority.
- **Frequency:** most multi-endpoint CLIs at full level; the skill mismatch affects every publish that reruns the gate.
- **Fallback if not fixed:** operators learn to ignore full-level output (masking regressions) and to deviate from the skill (undocumented judgment call in every publish).
- **Worth a fix?** Yes at P3 — high value but single-CLI hard evidence.
- **Durable fix:** (scorer) classify happy-path 4xx-on-fixture-args as `unverifiable-args` distinct from `fail`, so behavioral failures stand out; (skill) align Step 4.5's required level with the marker contract (quick as gate, full as advisory evidence).
- **Test:** run full on a CLI with fixture args — summary separates unverifiable-args from fail (positive); a genuine 500/behavioral failure still reports `fail` (negative).
- **Evidence:** publish-live-gate-full-supplementary.json (497 pass/137 fail, all `HTTP 400` samples); estimate bug found only at publish; PR #1680 body documents the deviation.
- **Related prior retros:** #3973 (live-check penalizes cold mirror) — `extends`: scorer-fidelity family.

### 7. Novel-command test scaffolds are six identical `--help` smoke tests (Template gap)
- **What happened:** All six new novel commands shipped with byte-identical `--help`-only `*_test.go` files. The pre-publish review found every behavioral contract untested (budget stop, exit-code-7 boundary, route selection), and one untested contract WAS a shipped bug (estimate's balance parsing). The publish session had to hand-build a testable seam (`sweepBudget` type + 6 unit tests) to satisfy review.
- **Scorer correct?** N/A.
- **Root cause:** The novel scaffold emits a vacuous test; there is no scaffolded seam (fake client/injection point) making RunE logic testable, so agents default to leaving the smoke test.
- **Cross-API check:** Scaffold-driven, so every 4.30 CLI's novel commands. Evidence beyond this CLI is the template itself.
- **Frequency:** every CLI with novel commands.
- **Fallback if not fixed:** review-time findings (when a review runs) or shipped behavior bugs (when it doesn't — estimate shipped).
- **Worth a fix?** Yes at P3 — floor-raise, adjacent issues already track other novel-scaffold gaps.
- **Durable fix:** scaffold novels with a client-injection seam (the command builder takes a doer interface) and a test skeleton that exercises RunE against a fake, replacing the pure `--help` test.
- **Test:** scaffolded novel test compiles and exercises one RunE path against a fake (positive); no live network in unit tests (negative).
- **Evidence:** six identical test files at review time; `sweepBudget` extraction was required to make the budget contract testable.
- **Related prior retros:** #3854 (non-runnable Example), #3965 (dry-run contract), #3845 (missing pp:data-source) — `extends`: novel-scaffold quality family; this adds the test dimension.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Novel registration invisible to library SKILL verifier | generator | every 4.30+ CLI with grouped novels | Low — CI error names the wrong thing; agents patch DO-NOT-EDIT files | small | none needed (4.27 shape restored) |
| F2 | Dogfood reports embed the live API key | scorer | every CLI, any echoing failure | Low — manual grep failed once already | small | redact only report fields, never CLI behavior |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F4 | Reprint drops base version shape in MCP main | generator | every reprint of a published CLI | Medium — skill documents it; half-fired this run | medium | reprint-only; fresh prints unchanged |
| F5 | Compact strips sole payload; synthesized sidecars retrigger | generator | subclass: blocklisted-key payloads + fan-out scaffold | Low — silent data loss in agent mode | medium | sidecar-strip semantics preserved for true sidecars |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F6 | Fixture-arg fails drown real regressions; skill gate level unpassable | scorer | most large CLIs at full level | Medium | medium | genuine failures must stay `fail` |
| F7 | Novel test scaffolds are vacuous --help smoke tests | generator | every CLI with novels | Medium (review catches when run) | medium | keep scaffold compiling without live network |

### Comments on existing issues (not new filings)
| Finding | Existing issue | Contribution |
|---------|---------------|--------------|
| F3 | #4039 (ledger `changes` parse) | independent second reproduction + stash-restore workaround |
| — | #4022 (auth_env_vars wrong var) | observed downstream cost: 2 wasted full live-gate runs; probes ran keyless with misleading exit-5s |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|----------------------|
| S1 | Exotic cursor names (continuationToken/paginationToken) not in the generator's recognized-cursor table | Step B: all 13 affected feeds live in ONE spec (this CLI's platforms); cannot name 3 library APIs with direct evidence. 4.30 already auto-detects undeclared next_max_id. Adjacent: #4034. Next reprint of a cursor-heavy CLI will re-surface with better evidence. |
| S2 | phase5 acceptance marker desync between CLI-local `.manuscripts` and central manuscripts (validate read fresh, package read stale) | Step G: uncertain root cause — dual location may be intentional copy semantics; single observation; my flow synced copies manually, possibly self-inflicted. Adjacent: #3949. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Greptile summary edited in place broke merge-watch monitoring | library process quirk, not the press | API-quirk |
| Key-extraction regex bugs, monitor syntax error | operator/agent slips during the session | iteration-noise |
| `account estimate` creditCount balance bug | real bug, but fixed in the CLI and its kernel is covered by F6+F7 | printed-CLI |
| sweep/thread silent-success and error-envelope fixes | review findings fixed in this CLI; envelope-in-200 already tracked as #3956 | printed-CLI |
| `--max-credits` hard-ceiling semantics (Greptile 3-round loop) | resolution was per-CLI design (sweepBudget); no machine surface | printed-CLI |

## Work Units

### WU-1: Emit novel-command registration the downstream verifier can see (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** Novel commands under generated groups pass the library's `verify_skill.py` command-tree parser without hand-edits.
- **Target:** novel registration emission in `internal/generator/` (group-file wiring of `addNovelCommandIfAbsent`).
- **Acceptance criteria:**
  - positive: printed CLI with a novel under a generated group → library `verify_skill.py` flag-commands/positional-args pass.
  - negative: CLIs without novels produce byte-identical output.
- **Scope boundary:** does not change the library verifier; press-side shape only.
- **Dependencies:** none.
- **Complexity:** small

### WU-2: Redact the auth secret from dogfood JSON reports (from F2)
- **Priority:** P1
- **Component:** scorer
- **Goal:** No dogfood/live-gate report can carry the resolved `--auth-env` value (or vendor-prefix tokens) into proofs destined for the public library.
- **Target:** dogfood report serialization (`output_sample`, stderr capture).
- **Acceptance criteria:**
  - positive: failing probe echoing the key → report contains `<REDACTED:auth-env>`, not the value.
  - negative: reports without secrets byte-identical; CLI runtime output untouched.
- **Scope boundary:** report writer only; no change to probe execution.
- **Dependencies:** none.
- **Complexity:** small

### WU-3: Reprint preserves the base tree's runtime-version declaration shape (from F4)
- **Priority:** P2
- **Component:** generator
- **Goal:** A reprint of a published CLI carries the base's version expression in root.go/version.go/MCP main automatically, keeping the library's release-ledger guard green.
- **Target:** absorb/regen-merge reconciliation in `internal/generator/`.
- **Acceptance criteria:**
  - positive: reprint of a stamped CLI → `git diff base` shows no version-declaration change in the three runtime surfaces.
  - negative: fresh prints keep the generator default (`0.0.0-dev` / var form).
- **Scope boundary:** version declarations only; no other main.go reconciliation.
- **Dependencies:** none.
- **Complexity:** medium

### WU-4: Compact projection: sole-payload protection + synthesized-sidecar metadata (from F5)
- **Priority:** P2
- **Component:** generator
- **Goal:** `--agent`/`--compact` can never strip a response's only payload array, and generator-synthesized bookkeeping arrays never count as competing payloads.
- **Target:** compact/verbose helpers template.
- **Acceptance criteria:**
  - positive: `{scalars, comments:[...]}` keeps comments; `{payload:[...], fetch_failures:[...]}` keeps payload.
  - negative: true sidecars next to a payload array are still stripped.
- **Scope boundary:** projection semantics only; blocklist contents unchanged.
- **Dependencies:** none.
- **Complexity:** medium

### WU-5: Separate arg-infeasible failures from behavioral failures; align publish gate level (from F6)
- **Priority:** P3
- **Component:** scorer
- **Goal:** Full-level live dogfood distinguishes "fixture arg rejected by API (4xx)" from real failures, and the publish skill's required gate level matches the marker contract it validates.
- **Target:** dogfood classification + `skills/printing-press-publish` Step 4.5 wording.
- **Acceptance criteria:**
  - positive: fixture-arg 400s report as `unverifiable-args`; a behavioral failure still reports `fail` and gates.
  - negative: quick-level behavior unchanged.
- **Scope boundary:** classification and gate wording; no happy-args synthesis changes.
- **Dependencies:** none.
- **Complexity:** medium

### WU-6: Novel scaffolds ship a testable seam instead of a vacuous --help test (from F7)
- **Priority:** P3
- **Component:** generator
- **Goal:** Scaffolded novel commands include a client-injection seam and a behavioral test skeleton, so credit-spend/branch logic is testable without live calls.
- **Target:** novel-command scaffold templates.
- **Acceptance criteria:**
  - positive: scaffolded test exercises one RunE path against a fake client and fails when the logic breaks.
  - negative: no network in unit tests; scaffold still compiles untouched.
- **Scope boundary:** scaffold only; existing CLIs untouched.
- **Dependencies:** none.
- **Complexity:** medium

## Anti-patterns
- Arguing with a deterministic reviewer instead of removing the finding: two Greptile rounds were spent defending an estimator in prose; the score moved only when the contract became a tested type. Findings die by construction, not by rebuttal.
- Committing live-gate reports into proofs without a report-side redaction layer (see WU-2) — the manual scan is the last line, and it triggered after a push.

## What the Printing Press Got Right
- The patch catalog worked exactly as designed: `.printing-press-patches/` entries from the amend era let the reprint re-port two fixes deliberately instead of losing them, and this session extended the catalog for the next reprint.
- `publish package`'s mandatory vendor-prefix scan and module-path rewrite were flawless; `contributors add` idempotency held.
- 4.30's cursor auto-detection is a real improvement over 4.27 (undeclared `next_max_id` recognized without patches).
- The publish skill's divergence guard, ledger preservation, and PR-body template drove a clean Reprint/replace with full provenance — the PR reached 5/5 with the manuscripts trail intact.
