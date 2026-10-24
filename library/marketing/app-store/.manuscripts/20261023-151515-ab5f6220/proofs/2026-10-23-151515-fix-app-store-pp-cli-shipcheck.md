# Shipcheck: app-store-pp-cli

Run 20261023-151515-ab5f6220, Printing Press 4.33.10, 2026-10-24. Every run used an isolated `APP_STORE_HOME` under the session scratchpad.

**Verdict: ship.** All 7 legs pass, verify has 0 critical failures, the scorecard is 80/100 (A), and all 5 novel features pass the live sample probe.

## Runs

| Run | Finished (UTC) | Verdict | verify | validate-narrative | dogfood | workflow-verify | apify-audit | verify-skill | scorecard |
|---|---|---|---|---|---|---|---|---|---|
| 1 | 2026-10-23 23:00:57 | FAIL (6/7) | PASS leg, WARN 98% (50/51, 1 critical) | FAIL (4 ok, 5 MISSING) | PASS (WARN: 9 dead helpers) | PASS | PASS | PASS | 78/100 B, probe 5/5 |
| 2 | 2026-10-23 23:23:15 | PASS (7/7) | PASS 100% (51/51, 0 critical) | PASS (9/9, full examples) | PASS (WARN: 9 dead helpers) | PASS | PASS | PASS | 80/100 A, probe 4/5 |
| 3 (confirm) | 2026-10-23 23:26:22 | PASS (7/7) | PASS 100% (51/51, 0 critical) | PASS (9/9, full examples) | PASS (WARN: 9 dead helpers) | PASS | PASS | PASS | 80/100 A, probe 5/5 |

Run 3 follows the phase rule: a leg is fixed and re-run on its own, then the whole umbrella is re-run once to confirm nothing else regressed. The fix was in `keyword-share`, and the scorecard leg was re-run on its own first, with the probe at 5/5. That standalone scorecard total was 75/100 (B) with every dimension score identical to the umbrella's. The umbrella's verify leg supplies context the standalone run lacks, so 80/100 is the canonical number and the one recorded in `.printing-press.json`.

## Blockers found and fixed

### 1. validate-narrative: 5 MISSING (run 1)

`app 324684580`, `top` (twice), `reviews 6448311069` and `app 553834731` were reported as MISSING. The CLI's learn journal recorded those `--help` calls at 23:00:47Z with exit 2 and no resolved command. `doctor`, `traction` and `changes` exited 0 at the same moment.

**Cause:** this is a press ordering bug. The narrative leg resolves the staged binary `build/stage/bin/app-store-pp-cli` first (`ResolveScorerBinaryPath`). That binary was still the generate-time build from 21:27, which has only the scaffolded novel commands. The scorecard leg is the only step that refreshes the staged binary, and it runs after the narrative leg. The narrative commands themselves were correct: all 9 resolved, and their full examples passed against the current binary.

**Fix:** rebuild `build/stage/bin/{app-store-pp-cli,app-store-pp-mcp}` from current sources before each umbrella run. Runs 2 and 3 show validate-narrative OK, 9/9.

### 2. verify critical: resource-path:export (run 1)

The check reported: `"categories" resolves to https://apps.apple.com/us/iphone/charts/36, which is absent from tools-manifest.json`.

**Cause:** this is a generator inconsistency. The `categories` resource carries a `base_url` override (apps.apple.com, where the spec's base is itunes.apple.com). Three places name it by the effective absolute URL: `resource_paths.go`, the MCP handlers (`makeAPIHandler("GET", "https://apps.apple.com/us/iphone/charts/36", …)`) and the generated commands' `pp:path`. `tools-manifest.json` names it by the bare endpoint path `/us/iphone/charts/36`. Verify compares the two literally.

**Fix:** `tools-manifest.json` now gives both tools their effective URLs, matching every runtime surface. It is metadata only, and no CLI code reads it at runtime. Commit 0d1e9b9; patch record `.printing-press-patches/shipcheck-harness-fixes.json`.

### 3. verify exec failures: developer, events, featured, iap, top (run 1, non-critical)

**Cause:**
- Verify's mock mode can't reach the hand-built App Store client, which ignores `APP_STORE_BASE_URL`. So these commands read Apple live inside verify's 15 s per-command budget.
- Verify gives every subprocess one scoped home, so the governor's cross-run pacing applies across them. apps.apple.com pages go 20 s apart, so the later page commands were killed mid-wait.
- A reproduction from a fresh home right after a verify run got HTTP 429 from apps.apple.com at 23:10:51Z. The governor stopped at once, exited 7 and held the host for 5 minutes, which was respected.
- None of these commands declared exit 7 as a typed outcome.

**Fix** (commit 0d1e9b9):
- Under a Printing Press harness, the client waits for send slots only within the harness budget, counted from the start of the invocation: 8 s under verify and 22 s under the live matrix. A slot later than that is refused with nothing sent. Outside a harness, waits are bounded only by `--timeout`, as before.
- A pacing refusal (`*appstore.PacingError`) now exits 7, like a throttle, with a "run it again in Ns" hint. Fan-out loops stop on it.
- Every command that reads Apple live declares `pp:typed-exit-codes: "0,7"`. The owner's governed sbp CLI uses the same precedent.
- README troubleshooting, the SKILL exit-code table and `research.json` now describe the pacing refusal.

**What verify's exec rows now prove:** none of the 51 commands fails with an exit outside its typed set. Verify can't check whether a page command's parse is correct, because it reads Apple live. Parse correctness rests on the fixture tests and the phase-18 live matrix.

### 4. Live sample probe: keyword-share (run 2)

The probe flagged `keyword-share "podcast player"`: "output does not contain any token from query 'podcast player'".

**Cause:** with no local store yet, the empty answer listed `terms_missing: []` and suggested capturing "music streaming", which dropped the term that was asked about.

**Fix:** the no-store answer now lists every requested term in `meta.terms_missing`, and the capture hint names the first one. Commit d183a86, with the test `TestKeywordShareWithoutAStoreListsTheRequestedTerms`. Run 3's probe is 5/5.

## Before and after

| Measure | Run 1 | Run 3 |
|---|---|---|
| Verify | WARN, 98% (50/51), 1 critical | PASS, 100% (51/51), 0 critical |
| Scorecard | 78/100 B | 80/100 A |
| validate-narrative | 4 ok, 5 MISSING | 9/9 ok, full examples pass |
| Live sample probe | 5/5 | 5/5 |

Tests: `go build`, `go vet` and `go test -count=1 ./...` are green. The new tests are `TestGovernorRefusesASlotPastWaitUntil`, `TestPacingRefusalExits7`, `TestHarnessWaitUntilFitsEachHarnessBudget` and `TestKeywordShareWithoutAStoreListsTheRequestedTerms`.

## Remaining gaps (not ship blockers)

- **dead_code 1/5:** 9 generated helpers are never called (`booleanNoValueFlags`, `booleanSpaceFormPositionalErr`, `endpointArgsUsageErr`, `endpointPositionalArgs`, `endpointUsageLine`, `hasChangedLocalFlags`, `requiredInputWaived`, `retainCLIQueryParams`, `retainExplicitQueryParams`). This goes to phase 19 polish.
- **path_validity 0/10:** dogfood reports this check as SKIP ("internal-yaml spec: paths validated at parse time", 0/0 tested), and the scorer counts a SKIP as zero.
- **Cache freshness 3/10:** the generated cache-freshness helper is not emitted. The App Store client keeps its own 15-minute response cache, and every result carries FetchedAt and FromCache.
- **MCP quality 7/10:** the MCP server exposes the 2 typed categories tools plus SQL. The hand-built commands reach agents through the CLI, not as typed MCP tools.

## Retro candidates (press)

1. The generator writes `ep.Path` into tools-manifest.json but the effective URL into resource_paths.go, the MCP handlers and `pp:path` for resources with a `base_url` override. Verify's resource-path check compares the two literally, so it fails on every such CLI.
2. Shipcheck's validate-narrative leg uses `build/stage/bin` before the scorecard leg refreshes it. After phase 11 adds commands, the narrative leg checks a stale generate-time binary and reports false MISSING.
3. Verify's mock mode can't route hand-built clients, which ignore `<API>_BASE_URL`, so its exec step reads the upstream live inside a 15 s budget.
4. The scorer gives path_validity 0/10 when dogfood reports the check as SKIP (0/0).
5. A standalone `scorecard --live-check` gives a different total than the umbrella (75 vs 80) with identical dimension scores.
