# Printing Press Retro: exchangerate-api

## Session Stats
- API: exchangerate-api
- Spec source: hand-authored internal YAML from docs (no OpenAPI published; docs are authoritative)
- Scorecard: 81/100 (Grade A)
- Verify pass rate: 90.9%
- Fix loops: 1 (post-initial-shipcheck) + 1 (post-Phase-5 live dogfood)
- Manual code edits: ~12 files patched after generation (7 spec-derived commands for auth UX, 2 client.go security fixes, 1 dry-run masking fix, ~10 novel-feature files added)
- Features built from scratch: 10 transcendence commands + 1 cliutil helper (`parseDurationOrDate`) + 1 store extension (`exrate_tables.go`)

## Findings

### F1. Generated client/error templates leak secrets in URL paths (Bug, defensive security)

- **What happened:** When the API embeds a credential in the URL path (`/v6/{KEY}/...`), three template-emitted surfaces leaked the full credential to user-visible output:
  1. `dryRun()` printed the URL line with the unmasked key (only the query-string copy was masked)
  2. `APIError.Error()` embedded the unmasked URL path in every error message
  3. Any future log/diagnostic that prints `req.URL.String()` would do the same
  
  The same `maskToken` helper already exists in the template and is called for query-string auth. It's just not called consistently in the URL-path code paths.
- **Scorer correct?** N/A — this isn't a scoring finding, it's a code defect.
- **Root cause:** `internal/generator/templates/client.go.tmpl` calls `maskToken(authHeader)` only for the query-string preview line in `dryRun()`. `APIError` stores the path verbatim. Neither template path covers credentials embedded in the URL path.
- **Cross-API check:** Path-embedded secrets are common across catalog candidates:
  - ExchangeRate-API: `/v6/{KEY}/latest/{BASE}` — direct evidence this run; both `dryRun` and `APIError` leaked the key in plaintext
  - Slack webhooks: `https://hooks.slack.com/services/{TEAM}/{CHANNEL}/{SECRET}` — any CLI calling Slack webhook URLs would print the secret token in dry-run + errors
  - Mailgun: HTTP Basic auth credentials in the URL (`https://api:{KEY}@api.mailgun.net/...`) — any CLI using their URL-credential auth would leak
  - Query-string-keyed APIs (OpenWeatherMap, NewsAPI, Polygon.io, Twelve Data, many others) — secrets in `?api_key=` are already masked in the query preview, but ANY error message or log that prints `req.URL.String()` would include the unmasked query string with the key
- **Frequency:** Every CLI whose auth credential ends up in any part of the URL (path or query) — a large slice of the catalog. The mask helper exists; it just isn't invoked everywhere a URL is rendered for display.
- **Fallback if the Printing Press doesn't fix it:** Agent has to read every code path that prints a URL and add masking. ~30% catch rate (security failures of this kind are silent — they only surface if the agent reviews error messages and dry-run output deliberately). Means ~70% of affected CLIs ship with credential leaks in error messages.
- **Worth a Printing Press fix?** Yes. Defense-in-depth security. The cost is "add one helper, call it in 2-3 places in the template." The benefit is every future CLI inherits the masking by default.
- **Inherent or fixable:** Fixable. Small change.
- **Durable fix:** Add a `displayURL(rawURL string) string` helper to `client.go.tmpl` that masks every known credential field from `c.Config` (api key, bearer, basic password, etc.). Call it everywhere a URL is rendered for display:
  1. `dryRun()` URL line
  2. `APIError.Error()` (mask the `Path` field at construction time, before the error propagates)
  3. Any debug/verbose log that prints `req.URL.String()`
  
  The helper should be conservative — only mask the *value* of known credential fields, not arbitrary path segments. If a field is empty, the helper is a no-op (zero overhead for unauthenticated APIs).
- **Test:** 
  - positive: spec with `auth.in: query` → request a non-existent endpoint, assert error message contains `****<last4>` not the full key
  - positive: same as above for `auth.in: path` declarations
  - negative: spec with `auth.in: header` (key never in URL) → dryRun and error output identical before/after the change (no false-positive masking)
- **Evidence:** This run, `exchangerate-api-pp-cli rates enriched USD EUR` returned `Error: GET /v6/****37ac/enriched/USD/EUR returned HTTP 403...` with the full key in the error path. I patched the template-equivalent code in this CLI's `client.go` to mask: `displayURL := strings.ReplaceAll(targetURL, c.Config.ExchangerateApiKey, maskToken(c.Config.ExchangerateApiKey))`. The same patch would be needed verbatim on every future CLI with URL-embedded credentials.
- **Case-against (Step G):** "This is per-CLI; the agent should review their own generated code for credential leaks during shipcheck." — Counter: `maskToken` is *already* called for query-string auth in the same template. Not calling it consistently for the URL-path render and APIError is an internal inconsistency in the template, not per-CLI work. Defense-in-depth is the right default for security.
- **Related prior retros:** None found (no `*-retro-*.md` files in the manuscripts directory yet — this may be the first formal retro in this user's history).

### F2. Shipcheck doesn't rebuild the stage binary before scorecard --live-check (Scorer bug)

- **What happened:** `scorecard --live-check` invokes `build/stage/bin/<cli>-pp-cli` to run each `novel_features[].example` from `research.json`. The stage binary is built once at generation time and is never rebuilt by shipcheck or any other downstream tool. Any post-generation source edit — i.e., every Phase 3 hand-built novel command — causes the stage binary to be missing those commands. Result: `scorecard --live-check` returns `0/10 passed` with "unknown command" errors for every novel command, silently penalizing the Insight, Agent-Workflow, and MCP scorecard dimensions.
- **Scorer correct?** No. The scorer is testing the wrong binary. The CLI source has the commands; the stage binary doesn't because nothing rebuilt it after Phase 3 work landed.
- **Root cause:** `printing-press scorecard --live-check` invokes the stage binary at `<cli-dir>/build/stage/bin/<cli>-pp-cli`. There's no rebuild step in shipcheck or in scorecard before this binary is invoked. The stage binary's mtime is fixed at generation time; the SKILL Phase 3 instructions for novel commands don't mention rebuilding the stage binary.
- **Cross-API check:** Affects every CLI with Phase 3 hand-built work — and the SKILL's Phase 3 explicitly mandates Priority 2 transcendence commands as hand-built. Concrete evidence:
  - **exchangerate-api** (this run): scorecard sample probe returned 0/10 on first invocation with all 10 novel commands reporting "unknown command"; passed 7/10 after manual `go build -o build/stage/bin/...` rebuild.
  - **blu-ray** (catalog): stage binary at `build/stage/bin/blu-ray-pp-cli` mtime is `2026-05-17 03:43:53`; newest source file (`internal/cli/drift_test.go`) is `2026-05-17 03:55:06` — **11 minutes newer**. A scorecard `--live-check` against Blu-Ray right now would fail every `drift`, `editions`, `history` (15 hand-written files in `internal/cli/`) as "unknown command" for exactly the same reason.
  - Architectural argument: the staleness is structural — any post-generation edit, whether by an agent, a polish pass, or a manual fix, leaves the stage binary out of sync. The current behavior makes the scorecard's `--live-check` essentially useless for any CLI with hand-built features.
- **Frequency:** every CLI with Phase 3 hand-built work (per the SKILL, almost all CLIs).
- **Fallback if the Printing Press doesn't fix it:** Agent has to remember to `go build -o build/stage/bin/<cli>-pp-cli ./cmd/<cli>-pp-cli` before every shipcheck rerun. This isn't documented anywhere. Catch rate without documentation: ~10%. Even with a documented step, agents will forget regularly because it's invisible work (the failure mode is "novel commands score 0/10" which looks like "novel commands are broken").
- **Worth a Printing Press fix?** Yes. The fix is structural and tiny.
- **Inherent or fixable:** Fixable. Trivially.
- **Durable fix:** Two options, both small:
  1. **scorecard rebuilds the stage binary before live-check** if `<cli-dir>/cmd/<cli>-pp-cli/*.go` (or any `.go` file under `internal/`) is newer than `build/stage/bin/<cli>-pp-cli`. Idempotent — no-op when already up-to-date.
  2. **shipcheck umbrella rebuilds the stage binary first** before any leg runs. Slightly broader (helps verify-skill and others if they use the stage binary too) and simpler to reason about.

  Option 2 is the more defensive fix. The rebuild costs ~5 seconds; running `go build` in the CLI dir is well-tested and won't fail unexpectedly (if it does, shipcheck would have failed regardless).
- **Test:**
  - positive: generate a CLI, add a novel command (`internal/cli/foo.go` with `newFooCmd` registered in root.go), run shipcheck. Sample probe in scorecard should find `foo --help` and pass.
  - negative: generate a CLI, run shipcheck without any edits, verify the rebuild is a no-op (no-rebuild branch fires).
- **Evidence:** This run, see Phase 4 shipcheck logs in `proofs/2026-05-17-025552-fix-exchangerate-api-pp-cli-shipcheck.md`: initial `Sample Output Probe (live command sample) Passed: 0/10`; jumped to `7/10` after manual `go build -o build/stage/bin/exchangerate-api-pp-cli ./cmd/exchangerate-api-pp-cli`. Blu-Ray library state: `ls -lT /Users/vinnypasceri/printing-press/library/blu-ray/build/stage/bin/blu-ray-pp-cli` shows mtime older than `internal/cli/drift.go` and 14 other hand-written files.
- **Case-against (Step G):** "Agents should know to rebuild the stage binary after edits. SKILL could document this." — Counter: the SKILL doesn't currently mention `build/stage/bin/` or stage-binary rebuilds anywhere. Even adding a SKILL note would be lossy — agents miss SKILL notes regularly, especially ones about infrastructure that isn't visible during normal use. The cost of "scorecard rebuilds stage binary first" is trivial (~5s amortized); the cost of "every retro re-discovers this" compounds.
- **Related prior retros:** None.

### F3. verify-skill flags prose mentions of the CLI name as unknown commands (Scorer bug)

- **What happened:** verify-skill scanned `SKILL.md` and `README.md` and reported `[unknown-command] exchangerate-api-pp-cli wraps: command path not found in internal/cli/*.go (no matching Use: declaration)` for the natural-English prose line:
  > exchangerate-api-pp-cli wraps the official ExchangeRate-API with typed commands...
  
  The token after the CLI name was a verb ("wraps"), not a flag or subcommand. The scorer's heuristic mistook prose for a command invocation.
- **Scorer correct?** No. The CLI does not have a `wraps` subcommand; the scorer correctly identifies "no Use: declaration matches," but the underlying claim that "exchangerate-api-pp-cli wraps" is a command invocation is wrong — it's a sentence in a paragraph.
- **Root cause:** verify-skill appears to grep for `<cli-name> <next-token>` patterns in README.md/SKILL.md and treats every match as a candidate command invocation, regardless of context (code fence, inline code backticks, or plain paragraph). Markdown context matters; the scorer doesn't read it.
- **Cross-API check:** Every CLI has narrative.value_prop or README intro prose that's natural to write as "<cli-name> wraps/handles/provides/queries/exposes/connects/manages...". I rewrote my narrative to start with "This CLI exposes..." rather than "exchangerate-api-pp-cli wraps..." — a stylistic concession driven by the scorer, not by readability.
- **Frequency:** Common — almost any README that mentions the CLI name in a sentence. The fix in this CLI was to reword every "exchangerate-api-pp-cli <verb>" prose construction. Future CLIs will face the same friction.
- **Fallback if the Printing Press doesn't fix it:** Agent rewords prose to avoid leading sentences with the CLI name + verb. ~70% catch rate when running verify-skill, but it adds noise to the report every run and pushes agents toward awkward narrative phrasing.
- **Worth a Printing Press fix?** Yes. The fix is small and improves both signal quality and narrative-writing freedom.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Make verify-skill's command detection context-aware. Require ONE of:
  - The match is inside a fenced code block (```` ``` ```` or `~~~`)
  - The match is inside an inline code span (backticks)
  - The next token is a known shape that looks like a CLI invocation: starts with `--` (a flag), is a subcommand the CLI declares (i.e., would resolve in the cobra tree), or is followed by a known flag pattern
  
  Any "<cli-name> <word>" outside code context where `<word>` isn't a real subcommand should be treated as prose and dropped (or warned at a much lower severity level — info, not error).
- **Test:**
  - positive: SKILL/README with `` `cli-name latest USD` `` in backticks where `latest` exists → no warning
  - positive: SKILL/README with `cli-name latest USD` in a fenced bash block where `latest` exists → no warning
  - negative: SKILL/README with `cli-name wraps the API` in a plain paragraph → no warning (currently fires)
  - negative: SKILL/README with `cli-name fhfh --foo` in plain paragraph where `fhfh` isn't a subcommand AND `--foo` is a flag-shape → WARN (this is the real false-positive case: gibberish command in prose-shape)
- **Evidence:** First shipcheck run for this CLI's `verify-skill` flagged "exchangerate-api-pp-cli wraps" alongside legitimate command-mismatch findings. I worked around by rewording the value_prop's opening to "This CLI exposes every ExchangeRate-API endpoint..." Stylistic loss is small per CLI; cumulative noise across the catalog is real.
- **Case-against (Step G):** "verify-skill is correctly cautious — better to false-positive on prose than miss a wrong-command. Rewording is cheap." — Counter: the false positive is a recurring drag on every CLI's verify-skill pass. The smarter heuristic (require code-context or recognized subcommand) is strictly better — it doesn't lose the wrong-command detection (the legitimate cases in this run, like `sync --base USD` and `mcp serve --stdio`, all caught real bugs). The improvement is "filter noise, keep signal."
- **Related prior retros:** None.

## Prioritized Improvements

### P2 — Medium priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Mask credentials in URL paths consistently | generator | Every CLI with auth credential reachable via URL (path or query); a large slice of the catalog | ~30% — security findings of this shape are silent failures only caught on deliberate review of error/dry-run paths | small | None — no-op when `c.Config.<CredField>` is empty |

### P3 — Low priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | Shipcheck rebuilds stage binary before scorecard --live-check | scorer | Every CLI with Phase 3 hand-built work (most CLIs); confirmed direct evidence on this run + Blu-Ray | ~10% without SKILL note; ~50% even with one | small | Skip rebuild if binary is newer than newest `.go` under `internal/` and `cmd/` |
| F3 | verify-skill context-aware command detection | scorer | Every CLI whose README/SKILL has prose mentioning the CLI name + verb | ~70% (agent reads verify-skill output and reluctantly rewords prose) | small | Don't drop the wrong-command-shape detection; only suppress prose-context matches |

### Skip

| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---------|-------|--------------------------------------------------|
| S1 | First-class `auth.in: path` support in the spec format | Step B: only 1 catalog API (ExchangeRate-API) has direct evidence of this pattern; TomTom and others are speculation without library presence. The workaround (`auth.in: query` placeholder + client patch + scorer Auth Protocol penalty of 4/10) is acceptable as a per-CLI cost given the rarity. If 2+ more path-based-key APIs land in the catalog, revisit. |
| S2 | Scorecard sample probe runs `novel_features[].example` through a shell so pipes/redirects work | Step G: case-against ("examples in `novel_features[].example` should be the minimal-invocation form; pipelines belong in `narrative.recipes` where `validate-narrative --full-examples` does shell-eval") is stronger than the case-for. Agent-author can simply avoid shell metacharacters in `example`, and the SKILL effectively already implies this by separating examples from recipes. |

### Dropped at triage

| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Bogus `apikey_placeholder=<KEY>` query param on live requests | Direct consequence of the path-based-key workaround | printed-CLI (the placeholder was my choice in this spec; fixed by 1-line patch) |
| `mcp serve` subcommand wasn't on the main CLI by default | The MCP binary is a separate `cmd/<api>-pp-mcp` per the generator; I chose to add a wrapper subcommand on the main CLI to satisfy the novel-feature `command: "mcp serve"` claim | printed-CLI (specific to my novel-features list; the generator's default of a separate MCP binary is correct) |
| Narrative had `sync --base USD` (sync doesn't accept --base; sync-rates does) | I wrote the wrong command path in `research.json`; validate-narrative caught it | iteration-noise / agent-author error |
| Narrative had `pair USD EUR` (top-level pair doesn't exist; it's `rates pair`) | Same shape as above | iteration-noise / agent-author error |
| Narrative had `mcp serve --stdio` (no `--stdio` flag) | Same shape | iteration-noise / agent-author error |
| Narrative time-traveling example uses a date older than any captured snapshot | Sample probe runs on fresh DB; my example doesn't work fresh | agent-author error |
| `convert-batch` example referenced `amounts.txt` (doesn't exist) | I authored the example; live dogfood ran it literally | agent-author error (fix was to use `--input -` for stdin) |
| `history-cache` with 1 positional arg exited 0 (fell through to `cmd.Help()`) | I wrote the RunE without explicit usage error on 1 arg | agent-author error (the verify-friendly RunE template in SKILL already says to validate inside RunE) |
| `mcp serve --json` blocks indefinitely | I didn't handle `--json` in mcp serve | agent-author error |
| Polish pass reported +0/+0 on scorecard and verify | Polish has nothing to fix on a clean shipcheck | working as designed |
| Skipped a couple of `kind: read` verify probes because the mock harness can't synthesize valid currency codes for novel commands | Mock harness limitation; doesn't reflect reality (commands work with real input, 88/88 in Phase 5 live dogfood) | API-quirk-shaped (novel commands have domain-specific positional types) |

## Work Units

### WU-1: Mask credentials in URL paths in generator templates (from F1)
- **Priority:** P2
- **Component:** generator
- **Goal:** Defense-in-depth so any credential carried in URL paths or query strings is masked in every user-visible URL render (dryRun, APIError, debug logs).
- **Target:** Generator templates that emit `internal/client/client.go` — extend the existing `maskToken` usage from "query-string preview only" to "every URL render path."
- **Acceptance criteria:**
  - positive test 1: Generate a CLI from a spec with `auth.in: query`. Invoke a known-failing endpoint. Assert the error message contains `****<last4>` not the full credential value.
  - positive test 2: Same, but with credentials embedded in URL path (e.g., a basic-auth-in-URL spec or a hand-authored `/{key}/` shape). Same mask appears.
  - negative test: Generate a CLI from a spec with `auth.in: header`. dryRun and APIError output is byte-identical before and after the template change.
- **Scope boundary:** Does NOT change spec format or parser. Does NOT add new credential-detection logic — only masks `c.Config.<known-cred-field-values>` wherever URLs are printed. Only masks the *value* of known fields (not arbitrary path segments).
- **Dependencies:** none
- **Complexity:** small

### WU-2: Shipcheck rebuilds stage binary if source is newer (from F2)
- **Priority:** P3
- **Component:** scorer
- **Goal:** Make `scorecard --live-check` (and any other tool depending on the stage binary) reflect the current source. Eliminate the silent "0/10 on novel commands" failure mode.
- **Target:** `shipcheck` umbrella in the printing-press binary, OR `scorecard` itself before the `--live-check` step runs.
- **Acceptance criteria:**
  - positive test: Generate a CLI. Add a novel command (`internal/cli/foo.go` + register in root.go). Run shipcheck. The scorecard sample probe finds and runs `foo --help`. No manual rebuild required.
  - negative test: Generate a CLI. Run shipcheck without any edits. The rebuild step is a no-op (binary mtime already newer than newest source). Confirmable via log or strace; rebuild should not run.
- **Scope boundary:** Only the stage binary. Does not change how the stage binary is initially built at generation time. Does not move stage binaries elsewhere.
- **Dependencies:** none
- **Complexity:** small (one mtime check + conditional `go build`)

### WU-3: verify-skill context-aware command detection (from F3)
- **Priority:** P3
- **Component:** scorer
- **Goal:** Stop flagging natural-English prose as unknown commands. Preserve detection of actual wrong-command and wrong-flag invocations in code fences and inline backticks.
- **Target:** verify-skill's regex/heuristic that matches `<cli-name> <word>` patterns in `SKILL.md` and `README.md`.
- **Acceptance criteria:**
  - positive test 1: SKILL.md with `cli-name latest USD` inside a fenced bash block where `latest` is a real subcommand → no warning.
  - positive test 2: SKILL.md with `` `cli-name latest USD` `` in an inline code span → no warning.
  - positive test 3: SKILL.md with `cli-name wraps the API and provides...` in a plain paragraph → no warning (currently fires).
  - negative test 1: SKILL.md with a real wrong-command invocation inside a code fence (e.g., `cli-name fakecommand --foo` where `fakecommand` is not declared) → WARN (preserve existing behavior).
  - negative test 2: SKILL.md with `cli-name sync --base USD` in plain prose where `--base` is not declared on `sync` → WARN (the original real false-positive from this run that the scorer correctly caught).
- **Scope boundary:** Only the prose-vs-code disambiguation. Does NOT change which commands or flags are considered "known"; does NOT relax the wrong-command detection inside actual code context.
- **Dependencies:** none
- **Complexity:** small

## Anti-patterns

- **Don't auto-pivot during Phase 3 when the generator's spec-derived commands have awkward UX.** I correctly stayed with the AST-merge-preserves-hand-edits pattern (patched the spec-derived `rates_*.go` files in-place; the regen step preserved my edits as expected). The temptation was to author the commands from scratch in new files — that would have left both old (broken) and new (correct) commands in the build until cleanup.
- **Don't trust `research.json` narrative claims without running the resulting examples.** Three separate command-path errors in my narrative (`sync --base`, top-level `pair`, `mcp serve --stdio`) survived authoring and only surfaced when `validate-narrative` and `verify-skill` ran. The discipline is to run every example I write before declaring the narrative ready.

## What the Printing Press Got Right

- **AST merge preserved every hand-edit through three regen cycles.** I regenerated twice mid-Phase-4 (to refresh README/SKILL from updated `research.json`) and the AST merger correctly preserved my 7 patched spec-derived commands, the security patches in `internal/client/client.go`, and all 10 hand-authored novel commands. This is the single largest workflow win — without AST merge I'd have lost work or avoided regenerating.
- **The browser-sniff gate's "skip-silent" decision matrix made the right call automatically.** ExchangeRate-API's docs are authoritative for all 7 endpoints; no traffic capture was needed. The marker-file requirement forced me to record the decision even on the silent-skip path, which is the right contract.
- **The shipcheck umbrella's per-leg verdict table was the right shape.** Showing `dogfood / verify / workflow-verify / verify-skill / validate-narrative / scorecard` as six independent pass/fail signals (rather than a single roll-up) made it easy to see which leg needed which fix and to iterate quickly.
- **`dogfood --live --write-acceptance` produced the gate marker without ceremony.** Phase 5.6's check for `phase5-acceptance.json` worked exactly as the SKILL describes — no per-step plumbing required.
- **Polish skill's "no further polish needed" verdict on a clean shipcheck was an honest no-op.** Polish ran, evaluated the structural-deficit findings (auth_protocol, mcp_remote_transport), correctly identified them as out-of-scope-for-polish, and reported `ship_recommendation: ship` + `further_polish_recommended: no` with reasoning. This is exactly the right contract for the polish skill — it doesn't manufacture work to look busy.
- **The `mcp:read-only` annotation contract for hand-built commands was easy to follow.** I added `"mcp:read-only": "true"` to every novel query command. The cobratree mirror picked them up automatically and ran clean through `mcp-audit`.
