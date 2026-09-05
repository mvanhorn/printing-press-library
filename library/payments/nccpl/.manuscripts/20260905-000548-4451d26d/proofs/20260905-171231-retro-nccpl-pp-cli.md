# Printing Press Retro: nccpl

## Session Stats
- API: nccpl (National Clearing Company of Pakistan — clearing-layer market data)
- Spec source: browser-sniffed (`spec_source: sniffed`, `http_transport: browser-chrome`)
- Printing Press version: **4.31.7**
- Scorecard: **90/100 (A)** — up from 75/100 (B) at session start
- shipcheck: 6/7 legs PASS; scorecard HOLD only on the structural `live_api_verification`
- `go test ./...`: **0 failures** — down from 20
- Fix loops: 4 (scorer diagnosis → domain patches → infra close-out → root-cause of the 20 tests)
- Manual code edits: substantial (3 dead functions removed, 2 error-handling defects, 2 new
  commands, 1 registration block, 1 learn-config fix, 4 rate-limit call sites)
- Features built from scratch this session: 2 (`search`, `export`) on top of 11 pre-existing

## Findings

### F1. Generator emits a `truncateJSONArray` call site with no definition (Bug)
- **What happened:** Any spec declaring a GET endpoint with a `limit` param gets
  `data = truncateJSONArray(cmd.Context(), data, flagLimit)` emitted under the comment
  `// Honor --limit when the API accepts but ignores ?limit=N.`, but no `func truncateJSONArray`
  is emitted anywhere. `go build ./...` fails module-wide with `undefined: truncateJSONArray`.
- **Scorer correct?** N/A — build break, not a score penalty.
- **Root cause:** Generator templates. The helper emit was dropped from `helpers.go` while the
  call-site emit was retained. This is a **regression on current main**, not a longstanding gap.
- **Cross-API check:** Yes, and it is version-scoped, which is the strongest form of evidence.
  - `foodpanda` (press 4.30.1) — `internal/cli/helpers.go:2720`,
    `func truncateJSONArray(ctx context.Context, data json.RawMessage, n int)` — **emitted**.
  - `peekaboo` (press 4.29.0) — `internal/cli/helpers.go:1902`,
    `func truncateJSONArray(data json.RawMessage, n int)` — **emitted** (2-arg signature).
  - `nccpl` (press 4.31.7) — 2 call sites, **zero generated definitions**; only a
    hand-authored `internal/cli/nccpl_truncate.go` workaround makes the module compile.
  The signature also changed between 4.29.0 (2-arg) and 4.30.1 (3-arg, ctx-first), so the
  template was refactored in that window and the emit was lost by 4.31.7.
- **Frequency:** every API whose spec declares a GET `limit` param, at 4.31.7.
- **Fallback if the Printing Press doesn't fix it:** Poor. The failure is a hard build break, so
  it is always *noticed* — but the natural agent response is to hand-author the helper (as
  happened here), which leaves a duplicate-definition landmine for the next reprint if the
  generator starts emitting it again.
- **Worth a Printing Press fix?** Yes. A generator that emits code referencing a helper it does
  not emit produces a CLI that cannot build.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Emit `truncateJSONArray` into `helpers.go` alongside `isJSONArray`, or guard
  the call-site emit on the same template flag that emits the helper so the two cannot diverge.
  Add a generation-time check that every helper referenced by an emitted call site is emitted.
- **Test:** Positive — generate from a spec with `params: [- name: limit, type: int]` on a GET
  endpoint; `go build ./...` succeeds. Negative — generate from a spec with no `limit` param;
  assert `truncateJSONArray` is either absent or present-and-unused-without-build-error.
- **Evidence:** Pre-existing retro candidate #1, confirmed this session by cross-version census
  of the local library.
- **Related prior retros:** None found.

### F2. `learn.ticker_patterns` is accepted without validating it against the CLI's own embedded playbook examples (Bug)
- **What happened:** `go test ./...` failed **20 tests** on the shipped tree, all on the
  learn/playbook/teach surface (`TestPlaybookInit_*`, `TestTeach*`, `TestLearnNormalizers_*`).
  Every test failed the same way: `playbook init: <name>.json has no query_family_examples;
  skipping (would be unreachable at recall time)` → `expected at least 2 playbook rows; got 0`.
- **Scorer correct?** N/A — test failures, not a score penalty. But note `go test ./...` is a
  generation quality gate, so this blocks a gate.
- **Root cause, established this session (and it CORRECTS the pre-existing candidate):**
  The prior filing said this was "spec-independent … framework code, not emitted from the spec"
  and "likely affects every print at this version." **That diagnosis was wrong.** The cause is
  the CLI's own spec:

      learn:
        ticker_patterns:
          - "^[a-z0-9]{2,12}$"

  `entities.Extract` applies `matchesTicker(tok)` to the **raw token, before lowercasing**, and
  at **higher precedence** than its own ALL-CAPS→entity rule. So a lowercase-anchored pattern is
  *inverted*: it can never match a real PSX symbol (`HUBC`, `OGDC`, `FFC` are ALL-CAPS and were
  already classified as entities by rule 2 without any pattern) and it matches **every ordinary
  lowercase English word**. Every token in a query became a "ticker" entity, so
  `QueryFamily` — documented as "a space-separated bag of non-entity tokens" — returned empty
  for every example, so `families` was empty, so every seeded playbook was skipped.
- **Proof (measured, not inferred):**
  - `internal/learn/playbooks.go`, `internal/cli/playbook_init.go` and
    `internal/cli/playbook_init_test.go` are **byte-identical** between nccpl (4.31.7) and psx
    (4.31.0) apart from the module/binary name — so neither the code under test nor the test was
    the difference.
  - psx registers **no** ticker patterns (`TestNewConfig_NoTickerPatterns`); psx, foodpanda and
    zameen all report **0** failing tests. Only nccpl, the only CLI declaring a lowercase
    ticker pattern, failed.
  - Running the identical test on both: psx `--- PASS`, nccpl `--- FAIL`.
  - Narrowing the pattern to `^[A-Z][A-Z0-9]{2,11}$` → failures 20 → **0**.
  - Removing it entirely (matching psx) → failures 20 → **0**, `gofmt`/`build`/`vet` clean,
    scorecard unchanged at 90. **This is the fix that shipped.**
- **Cross-API check:** The vulnerable code path is framework code present in every printed CLI —
  verified present in `nccpl`, `psx` and `foodpanda`: `Config.matchesTicker` runs on the raw
  token (`internal/learn/entities/config.go:303`) at rule 1, ahead of the ALL-CAPS rule
  (`internal/learn/entities/extract.go`, rules 1–2). Any CLI that declares a lowercase-anchored
  `ticker_patterns` entry silently loses its entire recall surface. Only nccpl exercised it here.
- **Frequency:** subclass — any CLI declaring a `learn.ticker_patterns` regex that can match a
  lowercase dictionary word. Silent and total when it fires.
- **Fallback if the Printing Press doesn't fix it:** Very poor, and this session is the proof.
  The symptom (20 failing generated tests on an unrelated-looking surface) was mis-diagnosed as a
  framework regression and filed as such; the real cause sat one field away in the CLI's own
  spec. The *production* impact — `recall` never matching a playbook — produces **no error at
  all** and would not have been noticed.
- **Worth a Printing Press fix?** Yes. The press generates both halves of this contract: it
  emits the playbook JSON carrying `query_family_examples`, and it generates `newLearnConfig()`
  from the spec. It can check one against the other at generation time.
- **Inherent or fixable:** Fixable, cheaply and API-agnostically.
- **Durable fix (primary):** At generation time, run the generated normalizer over the
  `query_family_examples` of every playbook the generator is about to embed. If any playbook's
  examples all reduce to an empty `QueryFamily`, **hard-fail the generation** with a message
  naming the offending `ticker_patterns` entry and the example it swallowed. This is
  parameterized — it is driven by the CLI's own emitted examples, not by any hardcoded word list.
- **Durable fix (secondary, defence in depth):** Reconsider the precedence in
  `entities.Extract`. Rule 1 (ticker pattern) firing on the raw token ahead of the ALL-CAPS rule
  means a ticker pattern can only ever *widen* entity classification, never usefully narrow it.
  Either apply ticker patterns after the ALL-CAPS rule, or warn at generation time when a
  declared pattern's character class contains lowercase letters and is anchored `^…$`.
- **Test:** Positive — generate with `ticker_patterns: ["^[a-z0-9]{2,12}$"]` and assert the
  generation fails with the new diagnostic. Negative — generate with no ticker patterns, or with
  `^[A-Z][A-Z0-9]{2,11}$`, and assert generation succeeds and `go test ./...` is green.
- **Evidence:** Pre-existing retro candidate #3, **root-caused and corrected this session**.
- **Related prior retros:** None found. Note the pre-existing candidate #3 in this same run's
  `retro-candidates.md` is `contradicts` — it attributed the failure to version-scoped framework
  code affecting every print; the cross-version census disproves that (psx at 4.31.0 and
  foodpanda at 4.30.1 both green, and the code under test is byte-identical).

### F3. The browser-clearance auth path cannot succeed as generated (Bug) — absorbs 3 defects
- **What happened:** On a CLI with `http_transport: browser-chrome`, `auth login --chrome`
  extracted all three required cookies correctly and then **refused to save them**, reporting
  "Found cookies but the session has expired." There is no flag to skip validation, so the CLI
  is unusable without patching it.
- **Scorer correct?** N/A.
- **Root cause — three separate generated defects compounding on one path:**
  1. **`validateComposedAuth` validates over stdlib HTTP.** `internal/cli/auth.go:1258` builds
     `client := &http.Client{Timeout: 5s}` (line 1279) and maps `403` to `fmt.Errorf("HTTP %d")`,
     which the caller renders as "session has expired." But the spec declares
     `http_transport: browser-chrome` *precisely because* stdlib HTTP cannot pass the origin's
     bot protection. Worse: presenting a valid `cf_clearance` from stdlib is treated as token
     abuse and answered with a hard `"Sorry, you have been blocked"` rather than a challenge.
     Measured with an identical cookie jar: stdlib → 403 + hard block; unauthenticated
     `probe-reachability` of the same origin → 403 + `cf-mitigated: challenge`. So the hard block
     is caused by the transport mismatch, not by an expired session. The cookies were valid
     throughout (`cf_clearance` valid to 2027, session cookie ~30 min remaining).
  2. **Chrome cookie discovery uses a substring `LIKE`, so a host-scoped `cookie_domain` misses
     parent-domain cookies.** `discoverChromeProfiles` / `inspectCookiesForDomain` build
     `domainPattern := "%" + strings.TrimPrefix(domain, ".") + "%"` and run
     `SELECT COUNT(*) FROM cookies WHERE host_key LIKE '<pattern>'`. With
     `cookie_domain: "www.example.com"` the pattern `%www.example.com%` cannot match a cookie
     stored under `.example.com`. Cloudflare's `cf_clearance` is *always* a parent-domain cookie
     while app session cookies are host cookies, so a spec naming the full host finds the session
     cookies, misses the clearance, and reports `missing required cookies: cf_clearance` even
     though it is present and valid. Observed exactly that here. **Inconsistency:** the same file
     already contains an RFC-6265-correct matcher, `cookieDomainMatches` (auth.go:1187); only the
     SQL discovery path uses the loose substring form.
  3. **Composed-auth cookie values are not URL-decoded.** Laravel sets `XSRF-TOKEN` URL-encoded
     (`=` arrives as `%3D`) and `VerifyCsrfToken` fails on a still-encoded value.
     `composeAuthFromCookies` (auth.go:1232) does
     `strings.ReplaceAll(composed, "{"+name+"}", cookieMap[name])` with no decoding step, so the
     percent-encoded value goes straight into the header.
- **Cross-API check:** Honest census — the generated functions are present in 2 of 8 local CLIs
  (`nccpl` and `foodpanda` both carry `validateComposedAuth` and the `domainPattern` SQL;
  `cookieDomainMatches` exists in both), and only `nccpl` exercises the browser-clearance path
  today. **This does not clear the strict "three APIs with evidence" bar on library census
  alone.** It is filed anyway because (a) the defects are in *generated framework code*, not
  per-CLI code, (b) `http_transport: browser-*` is a documented spec option so the path is
  offered to every future CLI, and (c) when it fires it fires 100% of the time and blocks the
  entire auth path with a misleading message. The reviewer should weigh (a)–(c) against the
  two-API census.
- **Frequency:** subclass — every CLI with `http_transport: browser-*`. 100% within the subclass.
- **Fallback if the Printing Press doesn't fix it:** Poor. Defect 1 reports a *wrong reason*
  ("session expired") for a valid session, which sends the agent hunting for a cookie problem
  that does not exist. Defect 2 reports a cookie as missing when it is present. Both actively
  mislead.
- **Worth a Printing Press fix?** Yes.
- **Inherent or fixable:** Fixable.
- **Durable fix:**
  1. Validate through the same transport the CLI ships (use the browser/impersonating client
     when `http_transport` is `browser-*`), **or** treat a `403` as *inconclusive* rather than as
     expiry, **or** skip the probe when the credential contains a clearance cookie. Add a
     `--skip-validation` escape hatch regardless.
  2. Derive the registrable domain for the `LIKE` pattern, or replace the SQL filter with a broad
     fetch plus the existing `cookieDomainMatches` in Go — the correct matcher is already there.
  3. URL-decode cookie values in `composeAuthFromCookies` before substitution.
- **Test:** (1) Positive — a browser-transport CLI with a valid clearance cookie saves
  credentials; negative — a genuinely expired session still reports expiry. (2) Positive — a
  spec with `cookie_domain: www.example.com` discovers a cookie stored on `.example.com`;
  negative — an unrelated domain's cookies are not matched. (3) Positive — a percent-encoded
  cookie value is decoded before composition; negative — an already-decoded value is unchanged
  (no double-decode).
- **Evidence:** Pre-existing retro candidates #2, #4 and #6, all confirmed by reading the
  generated code; #6's user-visible effect reproduced this session.
- **Related prior retros:** None found.

### F4. The scorecard is blind to commands registered through the press's own novel-command hook (Scorer bug)
- **What happened:** 11 of nccpl's 12 hand-written commands were **invisible to the entire
  scorecard**. `insight` scored 2/10 and `workflows` 6/10 — nccpl's only two infra outliers
  against the whole library — and the scorer's own `gap_report` printed exactly one line,
  `insight scored 2/10`, pointing at a dimension worth **+2 total points**.
- **Scorer correct?** **No.** The CLI ships 12 working commands; the scorer was measuring a
  different CLI. Confirmed: `nccpl-pp-cli --help` lists all 12 and every behavioural test passes.
- **Root cause:** `pipeline.registeredCommandFiles` (scorecard.go:1843) parses **only**
  `internal/cli/root.go` with `go/parser` + `go/ast`, collects the constructor names passed to
  **literal `AddCommand(...)` calls**, then regex-maps `^func\s+(new[A-Z][A-Za-z0-9_]*Cmd)\s*\(`
  across `internal/cli/*.go` and transitively closes over the result.
  nccpl wired its commands through the press's **own documented extension point** — a separate
  `internal/cli/nccpl_register.go` calling
  `registerNovelCommand(func(root, flags){ addNovelCommandIfAbsent(root, newNCCPLSyncCmd(flags)) … })`
  from `init()`, kept in its own file so `generate --force` preserves the wiring. That file
  defines no `new*Cmd` constructor of its own, so the regex never matches it, so it never enters
  the registered set, so **nothing it references is ever seen**.
  Downstream, `scorecardReachableInternalFiles` returned 118 files and inside `internal/cli`
  returned only the four `infraCoreFiles` (`helpers.go`, `root.go`, `doctor.go`, `auth.go`) plus
  generated files registered literally in root.go. Not one `nccpl_*.go` file was reachable — so
  `insight`, `workflows`, `dead_code`, `breadth`, `mcp_*`, `data_pipeline_integrity` and
  `sync_correctness` all scored against a truncated tree.
- **Proof (measured):** adding literal `rootCmd.AddCommand(...)` lines for the 11 commands while
  **leaving `nccpl_register.go` untouched** (it is idempotent — `addNovelCommandIfAbsent` no-ops
  when the name exists) moved `insight` 2→10 and `workflows` 6→10, total 86→89, with
  `--help` output byte-identical and no command registered twice. Earlier isolation probes:
  renaming a *registered* file to a keyword-matching name moved insight 2→4, while renaming an
  *unregistered* file moved nothing (the gate blocks before the filename rule); adding a real
  `AddCommand` call to root.go moved insight 2→4; comment-only mentions moved nothing; wiring
  1/2/3/4/5 commands produced insight 4/6/8/9/10, confirming the ladder exactly.
- **Cross-API check:** The hook is **machine-emitted**: `func registerNovelCommand` appears in
  the generated `root.go` of **7 of 8** local CLIs. Four of them — `amazon-jobs`,
  `google-maps`, `hubspot`, `zameen` — carry the hook with **zero** `addNovelCommandIfAbsent`
  calls in root.go, so the defect is *latent* in each: the moment any of them adds a novel
  command via the documented separate-file route, that command goes invisible.
  `foodpanda` (10 calls) and `psx` (9 calls) happen to have their wiring inside root.go and are
  therefore unaffected. nccpl is the only CLI that used the documented route, and it is the only
  CLI scoring `insight` below 10.
- **Frequency:** every CLI that uses `registerNovelCommand` from a separate file — i.e. the route
  the press itself documents for surviving `generate --force`.
- **Fallback if the Printing Press doesn't fix it:** Very poor. The scorer's guidance actively
  misdirects: its one printed gap was worth +2 while the actual reachable deficit was worth +12,
  and an unguided polish pass would have spent its whole budget on the wrong dimension and then
  reported 90 as unreachable.
- **Worth a Printing Press fix?** Yes. The press penalises CLIs for using its own extension point.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Seed `reachableCtors` from **every** file in `internal/cli`, or teach
  `addCommandConstructorCalls` to resolve constructors passed to `addNovelCommandIfAbsent` /
  inside `registerNovelCommand` hook bodies, not just literal root.go `AddCommand` arguments.
- **Test:** Positive — a CLI wiring a novel command only via a separate `*_register.go` scores
  the same as one wiring it in root.go. Negative — a file defining a `new*Cmd` that is genuinely
  never registered anywhere stays unreachable.
- **Evidence:** Found this session by two independent agents converging on the same root cause.
- **Related prior retros:** None found.

### F5. `hasNonEmptySyncResources` gates on a hard-coded identifier name instead of testing structure (Scorer bug)
- **What happened:** `sync_correctness` scored 7/10. nccpl fully satisfies the dimension's
  intent — `nccplResources` is a non-empty 22-resource catalogue and `nccplSelectResources("")`
  defaults `--resources` to all of it — but the leg cannot be earned.
- **Scorer correct?** **No.** The CLI has exactly the property being measured.
- **Root cause:** `scoreSyncCorrectness` (scorecard.go:3259) awards its first `+2` via
  `hasNonEmptySyncResources`, which gates on the **literal identifiers**
  `defaultSyncResources` / `syncResources` appearing in a reachable file. Any CLI whose resource
  catalogue carries a domain-appropriate name is unscorable on that leg no matter how correct its
  sync is. Compounded by F4: nccpl's catalogue lives in `nccpl_resources.go`, which is not one of
  the four `infraCoreFiles` and defines no registered constructor, so it is unreachable anyway.
- **Proof (measured):** appending a single **uncalled** stub
  `func syncResources() []string { return []string{"probe"} }` to the reachable `helpers.go`
  moved `sync_correctness` 7→10 and total 75→78. Pure string bait, no behaviour. (`dead_code`
  fell 2→1 because the stub is uncalled, which is itself a nice illustration that the two legs
  disagree.) The probe copy was deleted; the shipped tree contains no such identifier.
  Separately established: the pagination leg the pre-existing notes blamed was **already earned**
  via generated `paginatedGet` in `helpers.go` — the missing leg was this one.
- **Cross-API check:** Every CLI whose sync catalogue is not literally named
  `syncResources`/`defaultSyncResources`. Verified present in the scorer, which is shared by all.
- **Frequency:** every CLI with a domain-named resource catalogue.
- **Fallback if the Printing Press doesn't fix it:** The "fallback" is to rename a correctly-named
  identifier to match a scorer string — which is gaming, was **explicitly declined this session**,
  and left 5 points on the table. That is the wrong incentive to put in front of every future run.
- **Worth a Printing Press fix?** Yes.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Test structure, the way the sibling
  `hasSyncPaginationStructureInFiles` already does with a real `go/ast` walk — look for a
  non-empty string-slice catalogue consumed by the sync command — rather than matching a name.
  At minimum, require the identifier to be *referenced* so an uncalled stub cannot satisfy it.
- **Test:** Positive — a CLI with a non-empty domain-named catalogue consumed by sync earns the
  leg. Negative — an uncalled stub named `syncResources` does **not** earn it.
- **Evidence:** Found this session; the declining agent measured the counterfactual and refused
  to take the points.
- **Related prior retros:** None found.

### F6. `scorecard --write-manifest` changes the score it reports (Scorer bug)
- **What happened:** On a byte-identical tree, `scorecard --dir L` reported **75/100 Grade B**
  while `scorecard --dir L --write-manifest L/.printing-press.json` reported **80/100 Grade A** —
  with a **dimension-for-dimension identical** 27-value vector and an identical
  `unscored_dimensions` set. Reproduced twice, including on an isolated scratch copy, and with
  `--no-live-check`.
- **Scorer correct?** **No.** A flag documented as "update `.printing-press.json` with scorecard
  summary and built novel features" must not change the score it reports.
- **Root cause:** Not isolated. The total is demonstrably **not a pure function of the printed
  dimension vector** when this flag is present. Candidate causes an implementer should
  disambiguate: (a) the manifest write path re-reads `novel_features` /
  `novel_features_built` and re-derives a dimension after printing the table; (b) a
  transcendence/novel-feature credit applied only on the write path; (c) `dead_code` recomputed
  against manifest-declared novel features. The +5 delta exactly matches domain raw 21→24, i.e.
  three domain points, i.e. the full `dead_code` range — which points at (c), but that is a
  hypothesis, not a finding.
- **How it was found:** shipcheck's scorecard leg reported 80 while a standalone run on the same
  tree reported 73/75. Reading `shipcheck --json`'s `legs[].command` showed the only difference
  was `--write-manifest`. This matters operationally: **shipcheck and standalone `scorecard`
  publish two different numbers for the same CLI**, and the manifest records the higher one.
- **Notable:** the divergence **closed** once the underlying deficits were genuinely fixed — at
  the final 90-state both invocations agree at 90. So the flag inflates only when there is a real
  deficit in the affected dimension, which is the worst shape for a measurement bug: it hides
  exactly the gap you are trying to measure.
- **Cross-API check:** The flag and the scoring path are shared by every CLI; shipcheck passes it
  on every run while operators and skills frequently call `scorecard` bare. Any CLI with a
  non-maxed affected dimension gets two different published scores.
- **Frequency:** every CLI whose affected dimension is below max.
- **Fallback if the Printing Press doesn't fix it:** Poor and silent. Nothing warns that the
  number moved; the operator simply sees a different score depending on which command they ran.
  It also skews the polish skill's `ship` gate (`scorecard >= 75`) in one direction.
- **Worth a Printing Press fix?** Yes — a measurement tool whose observer effect is 5 points
  undermines every other finding in this document.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Compute the scorecard once, then write the manifest from that computed result.
  Add a regression test asserting the JSON `total` is identical with and without
  `--write-manifest` on a fixture tree with at least one non-maxed dimension.
- **Test:** Positive — a fixture CLI scores identically both ways. Negative — the manifest is
  still correctly written and still contains the summary and novel-feature list.
- **Evidence:** Found this session by the orchestrator; measured before any agent ran.
- **Related prior retros:** None found.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | `truncateJSONArray` call site emitted without definition | generator | every API with a GET `limit` param at 4.31.7 | Always noticed (build break) but "fixed" by hand-authoring, creating a reprint landmine | small | None needed |
| F2 | `learn.ticker_patterns` not validated against the CLI's own embedded playbook examples | generator | subclass: any CLI declaring a lowercase-matching pattern; silent and total when it fires | Very poor — mis-diagnosed as a framework regression this session; production impact is completely silent | small | Only hard-fail when a playbook the generator is itself embedding becomes unreachable |
| F3 | Browser-clearance auth path cannot succeed as generated (3 defects) | generator | subclass: every `http_transport: browser-*` CLI, 100% within it | Poor — reports a wrong reason ("session expired") for a valid session, and a present cookie as missing | medium | Keep genuine-expiry detection working; don't double-decode cookie values |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F4 | Scorecard blind to commands registered via the press's own `registerNovelCommand` hook | scorer | every CLI using the documented separate-file route; latent in 4 of 8 local CLIs | Very poor — the scorer's own `gap_report` misdirected effort onto a +2 dimension while +12 sat unreachable | medium | A `new*Cmd` genuinely never registered anywhere must stay unreachable |
| F5 | `hasNonEmptySyncResources` gates on a hard-coded identifier name | scorer | every CLI with a domain-named sync catalogue | The only "fallback" is to rename a correct identifier to match a scorer string — i.e. gaming | small | An uncalled stub must not satisfy the leg |
| F6 | `scorecard --write-manifest` changes the score it reports | scorer | every CLI with a non-maxed affected dimension | Poor and silent — two different published numbers for one tree | small | Manifest contents must remain unchanged |

### Skip
| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---------|-------|--------------------------------------------------|
| S1 | `terminal_ux` penalises a hand-authored `<command>_test.go` | **Step G: case-against is stronger.** Measured real (a 6-line `export_test.go` cost 1 point; `export_shape_test.go` also cost it; `nccpl_export_test.go` and `zzz_probe_test.go` did not) — the press reads `internal/cli/<command>_<sub>.go` as a subcommand file, which is its own generator convention (`fipi.go` + `fipi_data.go`), so once `export` is a real command any `export_*.go` is scored as a phantom `export test` subcommand with no UX surface. But the workaround is a filename, the repo's hand-authored tests already follow the `nccpl_*_test.go` convention, and a maintainer would reasonably close this as working-as-designed. Recorded here rather than filed. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Pre-existing #5 | "dogfood reports implemented novel commands as TODO stubs" | `printed-CLI` — already withdrawn by its author as operator error; the generator emitted scaffolds and the implementations were written to parallel files, leaving orphaned scaffolds. dogfood was correct. |
| Pre-existing #7 | Surf cannot replay NCCPL's Cloudflare clearance | `API-quirk` — a reachability finding about one origin, not a press defect. |
| Pre-existing #8 | A fully replayable FIPI/LIPI surface exists at scstrade | `API-quirk` — a valuable data-source discovery, not a press defect. Belongs in the CLI's research notes, which is where it lives. |
| macOS clamps `--window-position` | `--window-position=-3000,-3000` is clamped to `left:0`, so a headed window cannot be hidden that way | `API-quirk` (platform behaviour) — nothing the press controls. |
| `LSUIElement` on a copied Chrome.app | Editing `Info.plist` breaks the bundle code signature | `API-quirk` (platform behaviour) — dead end, recorded in the CLI's proofs. |
| `git apply --directory=. -p1` fails | git 2.50.1 rejects the `./`-prefixed paths that flag synthesises, for any patch | `iteration-noise` — a local git quirk that cost three agents a detour; worth knowing, not a press defect. |

## Work Units

### WU-1: Emit `truncateJSONArray` (or guard its call site) (from F1)
- **Stable ID:** WU-1
- **Priority:** P1
- **Type:** bug
- **Component:** generator
- **Goal:** A spec with a GET `limit` param generates a CLI that builds.
- **Target:** Generator templates in `internal/generator/` — the `helpers.go` emit and the
  endpoint-command `--limit` call-site emit.
- **Acceptance criteria:**
  - positive test: generate from a spec with `params: [- name: limit, type: int]` on a GET
    endpoint; `go build ./...` exits 0.
  - negative test: generate from a spec with no `limit` param; build still exits 0 and no
    unused-helper vet failure appears.
  - regression guard: a generation-time assertion that every helper referenced by an emitted
    call site is itself emitted.
- **Scope boundary:** Does not change `--limit` semantics or the helper's signature; only
  restores the coupling between the two emits.
- **Dependencies:** None
- **Complexity:** small

### WU-2: Validate `learn.ticker_patterns` against the generator's own embedded playbook examples (from F2)
- **Stable ID:** WU-2
- **Priority:** P1
- **Type:** bug
- **Component:** generator
- **Goal:** A `ticker_patterns` entry that would make the CLI's own seeded playbooks unreachable
  fails generation loudly instead of shipping a silently dead recall surface and 20 failing tests.
- **Target:** Generator, where playbook JSON is embedded and `newLearnConfig()` is emitted;
  secondarily `internal/learn/entities/extract.go` rule precedence.
- **Acceptance criteria:**
  - positive test: generating with `ticker_patterns: ["^[a-z0-9]{2,12}$"]` and any embedded
    playbook fails with a diagnostic naming the pattern and the swallowed example.
  - negative test: generating with no ticker patterns, or with `^[A-Z][A-Z0-9]{2,11}$`, succeeds
    and the generated `go test ./...` is green.
  - the check must be driven by the CLI's own emitted `query_family_examples`, never a hardcoded
    word list.
- **Scope boundary:** Does not remove the `ticker_patterns` spec field or change `QueryFamily`'s
  contract. The precedence change in `extract.go` is optional defence in depth, not required.
- **Dependencies:** None
- **Complexity:** small

### WU-3: Make the browser-clearance auth path work as generated (from F3)
- **Stable ID:** WU-3
- **Priority:** P1
- **Type:** bug
- **Component:** generator
- **Goal:** `auth login --chrome` succeeds on a `http_transport: browser-*` CLI with valid
  cookies, and reports expiry only when the session is actually expired.
- **Target:** Generator templates emitting `internal/cli/auth.go` — `validateComposedAuth`,
  `discoverChromeProfiles` / `inspectCookiesForDomain`, `composeAuthFromCookies`.
- **Acceptance criteria:**
  - positive: a browser-transport CLI holding a valid clearance cookie persists credentials.
  - negative: a genuinely expired session still reports expiry (don't fail open).
  - positive: a spec with `cookie_domain: www.example.com` discovers a cookie stored under
    `.example.com`; negative: an unrelated domain's cookies are not matched.
  - positive: a percent-encoded cookie value is decoded before header composition; negative: an
    already-decoded value is not double-decoded.
  - a `--skip-validation` (or equivalent) escape hatch exists.
- **Scope boundary:** Does not attempt to make stdlib HTTP pass bot protection, and does not
  change the browser transport itself.
- **Dependencies:** None
- **Complexity:** medium
- **Note for the reviewer:** this WU's Step B census is 2 local APIs, not 3 — see F3's
  cross-API check for why it is filed anyway. Close it as too-narrow if you disagree; the
  evidence is stated plainly rather than padded.

### WU-4: Teach the scorecard to see novel commands registered via `registerNovelCommand` (from F4)
- **Stable ID:** WU-4
- **Priority:** P2
- **Type:** bug
- **Component:** scorer
- **Goal:** A CLI that wires novel commands through the press's own documented extension point
  scores the same as one that wires them literally in `root.go`.
- **Target:** `pipeline.registeredCommandFiles` / `addCommandConstructorCalls` /
  `scorecardReachableInternalFiles` in the scorecard implementation.
- **Acceptance criteria:**
  - positive test: a fixture CLI registering a novel command **only** via a separate
    `*_register.go` + `registerNovelCommand` hook produces the same `insight`, `workflows`,
    `dead_code` and `breadth` scores as an equivalent fixture wiring it in `root.go`.
  - negative test: a file defining a `new*Cmd` that is genuinely never registered anywhere
    remains unreachable and still counts toward `dead_code`.
- **Scope boundary:** Does not change any dimension's scoring formula — only which files the
  reachability walk can see.
- **Dependencies:** None
- **Complexity:** medium

### WU-5: Make `hasNonEmptySyncResources` structural rather than name-matched (from F5)
- **Stable ID:** WU-5
- **Priority:** P2
- **Type:** bug
- **Component:** scorer
- **Goal:** A correct, domain-named sync resource catalogue earns the leg; an uncalled stub does not.
- **Target:** `hasNonEmptySyncResources` in the scorecard implementation.
- **Acceptance criteria:**
  - positive test: a CLI with a non-empty string-slice catalogue consumed by its sync command
    earns the leg regardless of the identifier's name.
  - negative test: an **uncalled** `func syncResources() []string` stub does **not** earn it
    (this is the exact bait that currently works, measured at +3 raw / +5 total).
- **Scope boundary:** Does not alter the other four legs of `scoreSyncCorrectness` or the
  `"/{"`-derived cap.
- **Dependencies:** WU-5|wu:WU-4 — the catalogue file must be reachable before a structural
  check can find it, so WU-4 lands first.
- **Complexity:** small

### WU-6: Make `--write-manifest` observation-free (from F6)
- **Stable ID:** WU-6
- **Priority:** P2
- **Type:** bug
- **Component:** scorer
- **Goal:** `scorecard` reports the same total with and without `--write-manifest`.
- **Target:** The scorecard command's manifest-write path and `recomputeScorecardTotals`.
- **Acceptance criteria:**
  - positive test: on a fixture tree with at least one non-maxed affected dimension, the JSON
    `total`, `percentage` and full 27-value vector are identical with and without
    `--write-manifest`.
  - negative test: the manifest is still written correctly and still carries the scorecard
    summary and the built novel-feature list.
  - the implementer should first isolate which of the three candidate causes in F6 applies; the
    +5 delta matching domain raw 21→24 points at `dead_code` recomputation, but that is
    unconfirmed.
- **Scope boundary:** Does not change any dimension's scoring rules — only removes the
  flag's effect on the reported number.
- **Dependencies:** None
- **Complexity:** small

## Anti-patterns
- **Trusting the scorer's own `gap_report` as a work list.** It printed exactly one item
  (`insight scored 2/10`), which was worth **+2 total points** and would have consumed the most
  effort of anything on the board. The actual route to the target ran through three dimensions it
  never mentioned. Reverse-engineer the arithmetic before planning against a score.
- **Reporting a score without stating the invocation.** Three defensible numbers existed for the
  same tree on the same afternoon: 75 (bare), 73 (`--spec`), 80 (`--write-manifest`). Any
  before/after delta that doesn't name the invocation is meaningless.
- **Assuming "verified" is better than "unverified."** Passing `--spec` moved `path_validity`
  and `auth_protocol` out of unverified and **lowered** the total 75→73, because unscored
  dimensions are removed from the denominator. Two of the three unverified dimensions were
  actively helping.
- **Diagnosing a test failure from its symptom's neighbourhood.** 20 failures on the
  learn/playbook/teach surface were filed as a framework regression affecting every print at
  4.31.7. A cross-version census (psx 4.31.0 green, foodpanda 4.30.1 green, code under test
  byte-identical) disproved that in minutes; the cause was one regex in the CLI's own spec.
- **Renaming a correct identifier to satisfy a string match.** Available and declined twice
  (`nccplResources` → `syncResources`, and a content-free `internal/cli/jobs.go`). Worth
  +5 and +1 total respectively. Both would have degraded the CLI to move a number.
- **Batching four max-effort agents into one spend window.** All four Workstream-2 agents were
  killed by the spend limit mid-flight after 142 tool calls. Their findings were fully
  recoverable from the on-disk transcripts — but only because the work had been logged to files
  rather than held in agent context.

## What the Printing Press Got Right
- **The arithmetic is honest and legible once read.** Two independently normalised halves of 50,
  no hidden weight table, no calibration fudge. A Python replica reproduced all 8 local CLIs'
  totals with zero misses, and every one of ~30 probe mutations landed on its predicted value.
- **`--spec` lowering the score is correct behaviour, not a bug.** Verifying more dimensions
  enlarges the denominator. That is the right incentive; it just needs to be understood.
- **The scorer's structural checks are better than its string checks.**
  `hasSyncPaginationStructureInFiles` does a real `go/ast` walk and correctly credited nccpl's
  generated `paginatedGet` without being told to. The two legs that misfired
  (`hasNonEmptySyncResources`, the reachability seed) are both string/name-matched — the fix
  direction is already demonstrated inside the same file.
- **`scoreAgentWorkflow` is honestly cheap and it doesn't pretend otherwise.** Four `os.Stat`
  calls, no content read. All 8 local CLIs score exactly 9/10 and none ships
  `internal/cli/jobs.go`, so 9 reads as the practical ceiling for a non-marketplace API rather
  than as a per-CLI miss. A dimension that is transparently presence-only is easier to decline
  than one that pretends to measure quality.
- **`addNovelCommandIfAbsent` is genuinely idempotent**, which is why the F4 fix could add
  literal registrations in `root.go` while leaving the init hook untouched — restoring
  scorer visibility *and* keeping the reprint-survival property, with byte-identical `--help`.
- **shipcheck's `--json` envelope exposes each leg's exact command line.** That is the single
  artifact that made F6 findable; without it the 7-point discrepancy between shipcheck and
  standalone `scorecard` would have stayed a mystery.
- **The press source ships in the module cache.** `scorecard.go`, `scorecard_structural.go` and
  `climanifest.go` are readable at
  `~/go/pkg/mod/github.com/mvanhorn/cli-printing-press/v4@<ver>/internal/pipeline/`. One agent
  spent an afternoon on `objdump` before another found this. Worth a line in the retro skill.
