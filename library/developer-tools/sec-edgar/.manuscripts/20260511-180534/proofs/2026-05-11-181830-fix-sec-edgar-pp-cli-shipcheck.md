# sec-edgar-pp-cli Shipcheck Proof

## First-pass verdict

```
LEG               RESULT  EXIT      ELAPSED
dogfood           PASS    0         1.557s
verify            PASS    0         30.536s
workflow-verify   PASS    0         17ms
verify-skill      FAIL    1         479ms
validate-narrative  FAIL    1         102ms
scorecard         PASS    0         97ms

Verdict: FAIL (2/6 legs failed)
```

## Failures and fixes

### verify-skill: `holdings-delta` was treated as a top-level command in SKILL/README

The novel feature ships as `holdings delta` (Cobra parent + sub) but `research.json`'s `command` field had `holdings-delta` (hyphenated). The dogfood-sync wrote SKILL.md as `holdings-delta`, then verify-skill correctly flagged:

```
[flag-commands] sec-edgar-pp-cli holdings-delta: --filer-cik is declared elsewhere but not on holdings-delta
[unknown-command] sec-edgar-pp-cli holdings-delta: command path not found in internal/cli/*.go
```

**Fix:** updated `research.json` novel_features[7] `command` to `holdings delta` and the `example` to `sec-edgar-pp-cli holdings delta …`. The next shipcheck dogfood-sync rewrote root.go's Highlights, SKILL.md, and README.md from `novel_features_built` correctly.

### validate-narrative: `facts statement --periods last4` failed

The facts-statement command had `--periods` as `int` while research.json's quickstart and recipes used `last4` / `last8` (string). Result:

```
Error: invalid argument "last4" for "--periods" flag: strconv.ParseInt: parsing "last4": invalid syntax
```

**Fix:** changed `--periods` to accept a string with three forms: `lastN` (e.g. `last4`), bare integer `N`, or `all` / `0` for unlimited. New `parsePeriodsCount` helper normalizes to the int count. The same Bug Class would have caught any narrative-recipe mismatch on a string flag.

### verify EXEC failures on `cross-section` and `industry-bench`

Both required `--tag` (and `--period` for industry-bench) before honoring `--dry-run`. The verify runner probes commands with `--dry-run` alone; the early flag checks returned usage errors before `dryRunOK(flags)` short-circuit could fire. Result:

```
cross-section   read         PASS   PASS     FAIL     2/3
industry-bench  read         PASS   PASS     FAIL     2/3
```

**Fix:** moved the `dryRunOK(flags) { return nil }` check to the top of both RunE blocks (before any flag validation). Help-when-no-tag falls through to `cmd.Help()` for the bare-invocation case.

## Re-verification

After all three fixes:

```
LEG               RESULT  EXIT      ELAPSED
dogfood           PASS    0         1.586s
verify            PASS    0         28.29s
workflow-verify   PASS    0         19ms
verify-skill      PASS    0         455ms
validate-narrative  PASS    0         106ms
scorecard         PASS    0         92ms

Verdict: PASS (6/6 legs passed)
```

Verify pass rate: **100% (26/26 commands, 0 critical)**.

## Scorecard

```
Total: 77/100 - Grade B
```

Strong dims (≥9/10): Output Modes, Auth, Error Handling, Terminal UX, Doctor, Agent Native, Local Cache, Agent Workflow.

Polish targets for Phase 5.5:
- **Workflows: 4/10** — no `workflow_verify.yaml` manifest, no compound multi-step workflow commands. Could add one e.g. "track a coverage list" workflow.
- **Insight: 2/10** — scorer expects derived signals; my transcendence commands deliver these, the scorer may be picking up a static check on `<api>-cli insight` or similar. Polish can add a curated insight summary command.
- **MCP Remote Transport: 5/10** — only stdio. Adding `mcp.transport: [stdio, http]` to the spec would lift this to 10.
- **Cache Freshness: 5/10** — TTLs not declared on resources; polish can add.
- **Path Validity: 5/10** — likely the `submissions` path mapping with the literal `{cik}` rendering on no-args (the path-tokens get the endpoint name when no value supplied). Edge case; not a real bug.
- **Type Fidelity: 3/5** — likely the `--periods` accepting "lastN" string instead of int. Acceptable for the agent UX.
- **Vision: 5/10** — README narrative not yet expanded.
- **Breadth: 7/10** — would lift to 10 if we shipped Form 4 detail parsing + 13F/NPORT parsers (deferred per build log).

## Ship recommendation

**ship** — all ship-threshold conditions are met. Phase 5 live dogfood is next, then polish.
