# zoom-pp-cli reprint — Phase 4 shipcheck (2026-07-28)

Binary: cli-printing-press v4.29.0
Dir: `.runstate/cli-printing-press-186005b2/runs/20260728-120506-5644192a/working/zoom-pp-cli`

## Run 1 (baseline, immediately after Phase 3)

```
  LEG                 RESULT  EXIT   ELAPSED
  verify              PASS    0      40.167s
  validate-narrative  PASS    0      1.302s
  dogfood             PASS    0      8.387s
  workflow-verify     PASS    0      57ms
  apify-audit         PASS    0      161ms
  verify-skill        PASS    0      19.607s
  scorecard           PASS    0      2.025s

Verdict: PASS (7/7 legs passed)
Scorecard: 96/100 — Grade A
Sample Output Probe: 10/15 (67%)
```

Blocker found (not a leg failure, but a real regression I introduced): five
sample-probe error strings and the README/SKILL auth narrative still pointed at
`zoom-pp-cli auth set-token`, which in this reprint is the *framework* command
that stores a pre-exchanged token. The Zoom S2S credential exchange moved to
`auth s2s-token` (see build log, adaptation 2), so every "you need credentials"
hint sent the user to the wrong command.

## Fixes applied

- `internal/cli/zoom_schedule.go` — auth hint -> `auth s2s-token`
- `internal/cli/zoom_notes.go` — auth hint -> `auth s2s-token`
- `internal/config/zoom_auth_config.go` — doc comment -> `auth s2s-token`
- `README.md` (2 sites), `SKILL.md` (1 site) — auth narrative -> `auth s2s-token`
- `research.json` `narrative.auth_narrative` updated so a future regen emits the
  corrected text rather than reintroducing the stale hint.

`doctor.go` and `client/client.go` references to `auth set-token` were left
alone: those are framework messages about the framework command and are correct.

## Run 2 (after fixes)

```
  LEG                 RESULT  EXIT   ELAPSED
  verify              PASS    0      36.949s
  validate-narrative  PASS    0      1.274s
  dogfood             PASS    0      7.264s
  workflow-verify     PASS    0      54ms
  apify-audit         PASS    0      157ms
  verify-skill        PASS    0      21.074s
  scorecard           PASS    0      2.16s

Verdict: PASS (7/7 legs passed)
```

- **verify** (standalone rerun): Pass Rate 98% (195/198), 0 critical, verdict
  PASS. Data Pipeline: PASS (sync completed; sql table validation skipped —
  `sql` unavailable in the mock harness).
- **workflow-verify**: `workflow-pass` (no workflow manifest — skipped).
- **verify-skill**: all checks pass (flag-names, flag-commands, positional-args,
  shell-var-quotes, unknown-command) + canonical-sections.
- **dogfood**: `novel_features_check {planned: 15, found: 15}`, no
  `built_with_stub`; `reimplementation_check {checked: 15, exempted_via_store: 14}`.
- **scorecard**: **96/100 — Grade A** (unchanged).

### Scorecard detail (run 2)

Perfect 10/10: Output Modes, Auth, Error Handling, Terminal UX, README, Doctor,
Agent Native, MCP Remote Transport, MCP Tool Design, MCP Surface Strategy,
Local Cache, Breadth, Vision, Workflows. Domain Correctness all 10/10 (Path
Validity, Auth Protocol, Data Pipeline Integrity, Sync Correctness),
Type Fidelity 5/5, Dead Code 5/5.

Below cap: MCP Quality 8/10, **Cache Freshness 5/10**, Insight 7/10,
Agent Workflow 9/10. Omitted from denominator: MCP Desc Quality, MCP Token
Efficiency, Live API Verification (no S2S credentials available this run).

## Remaining sample-probe failures (5/15) — all environmental

| feature | error | cause |
|---|---|---|
| Schedule + bookmark | `no Zoom S2S OAuth token available` | no S2S credentials in this environment |
| Speaker-time analytics | `no local recording matching "meeting-2026-05-12-1400"` | probe uses a synthetic ID; needs `recordings local sync` against a real `~/Documents/Zoom` |
| Export a recording bundle | same | same |
| Notes — AI Companion summary | `requires Zoom S2S OAuth` | no S2S credentials |
| Notes — AI Companion transcript | `requires Zoom S2S OAuth` | no S2S credentials |

None of these are code defects: each is a correct, actionable error from a
command that cannot succeed without credentials or a populated local store. Per
the run brief, no S2S OAuth credentials are available, so the cloud surface
cannot be live-tested here.

## Open / carried items

1. **Systemic (retro candidate):** generate hard-blocks when a flat endpoint and
   a same-named sub-resource derive the same output path
   (`GET /users/email` vs `PUT /users/{userId}/email`). Worked around by
   renaming the `operationId` in the vendored spec. The generator should
   auto-disambiguate.
2. **Systemic (retro candidate):** no generator hook exists for a CLI-specific
   credential resolver, so the Zoom token-cache lookup in
   `internal/config/config.go` must be hand-reapplied to a generated file on
   every reprint. Prime source of silent auth breakage.
3. **Polish, deferred:** Cache Freshness 5/10 and Insight 7/10 — the largest
   remaining scorecard headroom. Not blocking.
4. Cloud surface unverified against the live API (no credentials).

## Recommendation

**SHIP.** 7/7 shipcheck legs pass, verify 98% with 0 critical, scorecard 96/100
Grade A, all 15 transcendence rows built and reachable, and the notes docs
family — the highest-value carried feature — is intact with all five of its unit
tests passing.
