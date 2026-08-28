# Rightbrain CLI — shipcheck proof

## Final result

    LEG                 RESULT  EXIT   ELAPSED
    verify              PASS    0      21.4s
    validate-narrative  PASS    0      1.6s
    dogfood             PASS    0      16.4s
    workflow-verify     PASS    0      68ms
    apify-audit         PASS    0      200ms
    verify-skill        PASS    0      180ms
    scorecard           PASS    0      6.4s

    Verdict: PASS (7/7 legs passed)

- **verify pass rate:** 100% (51/51, 0 critical)
- **scorecard:** 96/100, Grade A  (only weak dimension: `insight` 4/10)
- **dogfood:** 0 dead flags, 0 dead functions, 10/10 commands have examples,
  novel features 7/7 survived, MCP surface mirrors the Cobra tree
- **workflow-verify:** workflow-pass
- **verify-skill:** clean (flag-names, flag-commands, positional-args,
  shell-var-quotes, unknown-command)

## Iterations

Three shipcheck runs were needed.

**Run 1 — FAIL (1/7 legs).** `validate-narrative` failed: the quickstart and a
troubleshooting entry used generic resource names (`tasks`, `task_runs`) but
this CLI's syncable resources are `project_task` / `project_task_run`. Fixed at
the source in `research.json` and in the rendered README/SKILL.

**Run 2 — PASS (7/7).** Used as the baseline for the review phases.

**Run 3 — PASS (7/7).** Re-run after the review-phase fixes below.

## Fixes applied between runs

### Code (from the Phase 4.95 review — 9 findings, all fixed)

1. **high** — `eval-flake` counted an errored eval result toward `runs` but never
   toward `fails`, and never consulted the error count, so a case that errored in
   every run was reported as `stable-pass` with `fail_rate: 0.00` and sorted to
   the bottom. A permanently broken test case read as healthy. Now errors are
   treated as non-passes with a dedicated `errored` classification ranked above
   `stable-pass`.
2. **medium** — scope injection wrapped `scope use` itself, so a bare `scope use`
   silently pinned the env-derived scope to disk instead of printing help. The
   `scope` subtree is now excluded from injection.
3. **medium** — `scope use` checked arity before `dryRunOK`, so `--dry-run` exited
   2 while every sibling exited 0.
4. **medium** — `rollout` substituted `0` for an unparseable (spec-nullable)
   revision weight, which inflated every other revision's configured share and
   made a correctly-split canary look starved. Unparseable weights are now
   excluded from the denominator and marshal as `null`.
5. **low** — `eval-flake` ignored `--timeout` on the mirror read.
6. **low** — a corrupt `scope.json` was swallowed silently; now warns.
7. **low** — the circular "may be omitted" note on `scope use --help` (fixed by 2).
8. **low** — `approvals` dropped records with an unparseable `created` from both
   output sections while still counting them, so the JSON's counts disagreed with
   its rows.
9. **low** — `drift` counted task-run rows into headline totals that no mover row
   could account for under `--group-by agent`.

### Documentation (from the Phase 4.8/4.9 audit — 4 errors, all fixed)

1. `gate` was described as comparing against "what is live right now" / "the
   revision currently taking traffic". It actually compares against the most
   recent completed eval run recorded in the **local mirror** for a **different**
   revision. Corrected at the source so every rendered surface follows.
2. README referenced `auth-status`; the real command is `auth status`.
3. SKILL claimed `which` exits 2 on no match, but under `--json`/`--agent` — which
   the recipes mandate — a no-match is exit 0 with an empty `matches` array.
4. Seven executable examples contained the placeholder `mock-value` as a
   positional org id, which would 404, and which contradicted the README's own
   promise that you never paste UUIDs. They now rely on scope resolution.

Warnings also fixed: exit code 6 was missing from both exit-code tables; code 3
now notes it is also returned when `gate` does not clear; the OAuth
client-credentials claim was overstated (the CLI consumes an already-minted
token and performs no exchange); the env-var table omitted `RB_ORG_ID` and
`RB_PROJECT_ID`; a raw Python docstring leaked from the upstream spec into the
`project delete` description; the `--deliver` claim was scoped to
envelope-emitting commands; the `changelog` tamper-evidence claim now notes it
requires `--verify`.

### Manifest

Five absorbed rows claimed commands that were never built (`auth login`,
`auth token`, `env list`/`env use`, `sql`, and `project use`). None appeared in
any shipped document, so nothing user-facing lied, but the manifest overstated
parity with the incumbent npm CLI. Corrected to say **NOT BUILT** with the
reason, rather than left standing.

## Known gaps

- **`insight` scores 4/10** on the scorecard — the weakest dimension.
- **Parity gaps vs the incumbent `rightbrain` npm CLI:** no browser OAuth
  `login`, no `token` print (deliberate — printing secrets to stdout is a leak
  vector), no environment switching. Credentials are supplied via `RB_API_KEY`
  or `auth set-token`.
- **`eval-flake` and `rollout` were not exercised against real eval or run
  history**, because the test workspace has none. They are covered by unit tests
  and by a seeded synthetic mirror with known values, not by live data.
- **A confirmed data race exists in generated framework code** (`internal/store/learnings.go`,
  the learn subsystem). It is a Printing Press template defect, not printed-CLI
  code, so it was filed as a retro candidate rather than patched here. See
  `retro-candidates.md`. It is reachable under the MCP server, which services
  concurrent tool calls, and "concurrent map read and map write" is a fatal Go
  runtime error.

## Verdict

**ship** — 7/7 legs pass, verify 100%, scorecard 96/100 Grade A, all 7 approved
transcendence features built and behaviorally verified, and every review finding
fixed in-session rather than deferred.
