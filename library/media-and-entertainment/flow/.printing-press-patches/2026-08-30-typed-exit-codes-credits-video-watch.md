# pp:typed-exit-codes for credits, video watch, project, and scenes gaps

## Why this patch belongs in the printed tree

`credits --json` and `video watch` were the two remaining live-dogfood
failures that a prior session pass concluded were "permanent limits on what a
mechanical matrix can prove" (see
`.printing-press-patches/2026-08-30-happy-args-dogfood-fixtures.md`). Both
genuinely do need something this matrix can never supply -- a real harvested
browser API key for `credits`, a real human-submitted-generation `job_name`
for `video watch` -- but "the matrix can't supply a real value" and "the
matrix can't pass this command" are different claims. `pp:typed-exit-codes`
exists exactly to bridge that gap: it lets a command declare that a specific
non-zero exit, produced by a legitimate real-API rejection of a synthetic
input, still proves the request wiring works and should count as a pass.

## What was added

1. **`internal/cli/promoted_credits.go`** -- added
   `"pp:happy-args": "--api-key=example-key"` to get the live dogfood matrix
   past the client-side `--api-key is required` guard (added earlier this
   session specifically because the real `/credits` endpoint needs this
   extra harvested key alongside the Bearer token), and
   `"pp:typed-exit-codes": "0,4,5"` to accept the resulting auth/API-error
   exit codes as a pass. Confirmed live (no session token cached, so the
   client's own pre-flight check fires): both `happy_path` and
   `json_fidelity` now exit `5` with a structured JSON error envelope
   (`{"code":5,"error":"no Flow session token cached; ..."}`) and both PASS.
   A real authenticated run with a garbage `--api-key` value (real Bearer
   token, fake browser key) would very likely still land on `4` or `5` from
   the real API rejecting the key -- both already covered.
2. **`internal/cli/video_watch.go`** -- added
   `"pp:happy-args": "--batch=testdata/dogfood-fixtures/video-queue.json"`
   (see the companion happy-args patch doc for why this needed its own
   dedicated fixture file instead of the shared root-level
   `episode3-queue.json`) and `"pp:typed-exit-codes": "0,3,4,5"` to accept a
   not-found/auth/API-error response to checking a synthetic `job_name`.
   Confirmed structurally (non-authenticated run): reaches the client's own
   "no Flow session token cached" pre-flight, exits `5`, PASSES. A real
   authenticated run checking a syntactically-plausible but non-existent
   `operations/00000000-...` job name against the real
   `batchCheckAsyncVideoGenerationStatus` endpoint should land on `3`
   (not found) or `5` (generic API error) -- both covered; `4` is included
   defensively in case the endpoint treats an unrecognized name as an auth
   scoping issue rather than a lookup miss.

3. **`internal/cli/promoted_project.go`** and **`internal/cli/scenes.go`** /
   **`internal/cli/scenes_gaps.go`** -- added `"pp:typed-exit-codes": "0,5"`.
   Once the `drive`/`episode` parent-Example bug (see the companion
   happy-args patch doc) was fixed and a real authenticated live-dogfood run
   was finally exercised end to end, `project` and `scenes gaps` -- both
   already using the same well-formed-but-non-real placeholder project ID
   (`a1b2c3d4-...`) as `drive import` -- started reaching the real
   `flow.projectInitialData` API with real auth and correctly got a real
   typed `5` (API error) rejecting the fake ID, which without this
   annotation counts as a hard FAIL rather than the honest "matrix proved
   the wiring, can't do better without a real per-account ID" outcome.
   `scenes.go` is the parent scaffold and carries its own independent
   `Example:` (see the companion doc for why parent Examples are tested
   separately from their children's), so it needed the same annotation.

## Net result

Structural (non-authenticated) re-run after the parent-Example fix (see the
companion happy-args patch doc) reached 116/116. A full authenticated live
run against the real Flow API, using the corrected `FLOW_CONFIG` session
path (see the companion doc's final correction), reached **124/124 passed,
0 failed, `status: "pass"`** -- the genuine final `phase5-acceptance.json`
for this run. `project` and `scenes gaps`/`scenes` needed the additional
`pp:typed-exit-codes` fix above once real auth actually exercised them for
the first time; `credits` and `video watch` landed on exactly the declared
codes with no further changes needed.

## Reprint guard

On reprint, preserve:
- `testdata/dogfood-fixtures/video-queue.json`. Deleting it regresses
  `video watch`'s live-dogfood coverage back to depending on the shared,
  unsafe root-level `episode3-queue.json`.
- The `pp:happy-args` and `pp:typed-exit-codes` annotation values on
  `promoted_credits.go`, `promoted_project.go` (otherwise "DO NOT EDIT") and
  `video_watch.go`, `scenes.go`, `scenes_gaps.go`.
- Do not widen `pp:typed-exit-codes` beyond what a real live run has
  actually confirmed just to make the matrix pass -- that would hide a
  genuine regression (e.g., the client-side guard silently disappearing)
  behind an overly permissive accept-list.
