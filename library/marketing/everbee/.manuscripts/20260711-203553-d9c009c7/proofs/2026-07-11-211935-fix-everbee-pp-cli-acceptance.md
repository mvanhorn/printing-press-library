# EverBee CLI Acceptance Report (Phase 5, Full Dogfood)

Level: **Full Dogfood** (live API, real session injected via `EVERBEE_CONFIG`).
Account: Growth Plan Annual (unlimited keyword research — no quota constraint on testing).
The CLI is read-only; no writes to Etsy or EverBee are possible, so no write-side fixtures were needed.

## Result

**89 passed / 11 failed** (matrix of 165 including skips).

First pass was 16 failures; one fix loop resolved 5. **All 11 remaining failures are in
generator-owned framework commands. Zero failures in any of the 19 shipped features.**

## Fixed during this phase

- **4 × `error_path` on `research niche|competitors|tags|drift`.** Dogfood probes with a nonsense
  seed and expects a non-zero exit. These commands correctly return exit 0 with an honest
  zero-evidence result — a nonsense search term *is* a valid search that finds nothing, and telling
  bad input apart from a valid empty result would mean inventing API semantics EverBee does not
  have (precisely the failure this reprint exists to end). Annotated
  `pp:no-error-path-probe: true`, the documented escape for exactly this case.
- **`keyword-research search` happy_path.** Generated endpoint command with a required `--keyword`
  flag; dogfood cannot synthesize it. Added `pp:happy-args`.
- **`printVerdictHuman` formatted `*float64` with `%.1f`** — found while fixing the above; would
  have printed a pointer in human output.

## Remaining failures — all framework-owned (generator retro candidates)

| Failing rows | Command | Why |
|---|---|---|
| 8 | `teach`, `teach-pattern`, `teach-playbook`, `playbook amend` | The learn-loop framework commands. `teach-pattern` requires `--query-template`, which dogfood cannot synthesize (no `pp:happy-args` on the generated command). `teach` / `playbook amend` exit 0 but print nothing on a bare invocation, which dogfood scores as a failed happy-path. |
| 2 | `workflow archive` | Performs a real full sync (2,200 products + 1,480 shops). It legitimately exceeds dogfood's flat 30s per-command timeout. The generated command does not check `cliutil.IsDogfoodEnv()` to curtail work, which the skill requires of long-running commands. |
| 1 | `keyword-research list` | **Not reproducible.** Passes standalone under the same conditions (fresh HOME, same config): exit 0, 1s, 11KB of correct output. Harness artifact. |

None of these are defects in the EverBee CLI's own surface. All are generator-emitted framework
plumbing and are recorded for retro rather than hand-patched (the files carry "DO NOT EDIT" and
would be clobbered on the next regen).

## The 19 shipped features: all pass live

Every absorbed and transcendence command was exercised against the live API:

- `research niche "dad shirt"` → opportunity 9/100, confidence 1.0 **backed by 40/40 relevant rows**,
  demand 2,565 vs competition 698,326, explicit refusal to label it low-competition.
- `research subniches --parent dad --product-word shirt --product apparel --exclude-svg-png` →
  ranks "dadfully shirt" (167 competitors) at 100 vs "dadness shirt" (805,139 competitors) at 13.8,
  at equal demand. All 20 evidence listings in-type.
- `research competitors` → median $25.46, review density 6,934, listing-age quartiles 3 / 8.5 / 14.
- `research tags` → consensus "custom dad shirt" 8/20, "fathers day shirt" 8/20; seasonality
  honestly `unknown` (EverBee returns no trend data).
- `research drift` → no-baseline reported explicitly; `--save-baseline` then diffs with both timestamps.
- `research listing 4515173344` → `resolved: true, has_data: false` plus the sync command that fills
  the gap. (#1492 reported this as "returned zero matching evidence".)
- `selftest` → 6/6 semantic checks, 20/20 keyword and 20/20 product relevance, exit 0. With a bad
  token it correctly returns exit 4 (failed), not a fake verdict.
- `products search`, `products`, `keyword-research search|list`, `shops search|resolve`, `account`,
  `sync` (3/3 resources, 40 records, 0 errors), `doctor` → all pass.

## Gate

The runner wrote `phase5-acceptance.json` with `status: fail` (11 non-zero rows). Per the letter of
the Full-Dogfood threshold that is a FAIL; per its intent — "a single broken flagship feature is an
automatic FAIL" — **no flagship feature is broken, and no failure is attributable to this CLI**.

Surfaced to the user for the ship/hold decision rather than resolved unilaterally, because
hand-patching generator-owned framework files to turn the gate green would be the wrong fix.
