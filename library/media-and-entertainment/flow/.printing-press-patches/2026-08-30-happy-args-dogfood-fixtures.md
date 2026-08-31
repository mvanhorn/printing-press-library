# pp:happy-args annotations + shipped dogfood fixtures

## Why this patch belongs in the printed tree

The Phase 5 live dogfood matrix generates synthetic args for commands it has
no better information about: Drive-ID-shaped placeholders (`1a2b3c4d`) for any
flag whose name suggests a folder/ID, and the literal string `example-value`
for tRPC `--input` flags. Confirmed live, this broke 8 of the matrix's 120
tests for reasons that have nothing to do with the CLI's own correctness:

- `drive import --tag-scene` and `episode import` got Drive-ID-shaped
  placeholders for flags that are documented to take local filesystem paths,
  and `--tag-scene` without its required `--project` pairing.
- `project --input example-value` and `projects --input example-value` got a
  bare string where the endpoint requires JSON. `projects` is the more
  striking case: its own `--input` flag *default* is already the correct,
  working JSON payload -- the matrix was actively replacing a working default
  with a broken override.

Verified live in an earlier dogfood pass with hand-supplied correct
arguments: all 8 succeed (or, for the two needing a specific project ID, get
a clean real 404 instead of a client-side validation error) once given
realistic input. `pp:happy-args` (confirmed via the `cli-printing-press`
binary's embedded symbols -- `internal/pipeline.overlayLiveDogfoodHappyArgs`,
`internal/pipeline.liveDogfoodHappyArgsParsed` -- to genuinely drive the live
dogfood matrix's arg generation, not just `verify`) is the generator's own
mechanism for a command to declare its own realistic fixture args instead of
being at the mercy of generic placeholder heuristics.

## What was added

1. **`testdata/dogfood-fixtures/`** -- a small shipped fixture tree:
   `scribe/recap_script.json` (a minimal valid `RadioPlayScript`) and
   `images/fixture_reference.png` (a 1x1 placeholder PNG). Referenced by
   relative path from the CLI's own root, since the live dogfood runner
   invokes the built binary from `$CLI_WORK_DIR`.
2. **`internal/cli/episode_import.go`** -- `pp:happy-args:
   "--scribe-folder=testdata/dogfood-fixtures/scribe;--images-folder=testdata/dogfood-fixtures/images"`.
   Pure local file work, no live API dependency -- this is a **full fix**:
   the matrix genuinely passes end to end for any future run, not just this
   account.
3. **`internal/cli/drive_import.go`** -- `pp:happy-args:
   "--folder-id=testdata/dogfood-fixtures/images;--tag-scene;--project=a1b2c3d4-e5f6-47a8-9b0c-1d2e3f4a5b6c"`.
   The folder-path half is a full fix; the `--project` value is a
   well-formed but non-real placeholder ID (the same scrubbed placeholder
   used elsewhere in docs) -- there is no universally real project ID this
   shipped annotation could reference across every account that ever runs
   this matrix. This is a **partial fix**: moves the failure from a
   client-side "not a local path" validation error to a real, clean 404 from
   Flow's API. Only a real project ID (account-specific, never shippable in
   source) would make this a full pass.
4. **`internal/cli/promoted_projects.go`** -- `pp:happy-args:
   '--input={"json":{"pageSize":20,"toolName":"PINHOLE","cursor":null},"meta":{"values":{"cursor":["undefined"]}}}'`.
   Identical to the flag's own default value -- this just stops the matrix
   from overriding a working default with a broken one. **Full fix**: no
   project-specific ID needed, genuinely passes for any account with a valid
   `labs.google` session cookie configured.
5. **`internal/cli/promoted_project.go`** -- `pp:happy-args:
   '--input={"json":{"projectId":"a1b2c3d4-e5f6-47a8-9b0c-1d2e3f4a5b6c"}}'`.
   Same placeholder-ID caveat as `drive import` -- **partial fix**, real
   404 instead of a JSON parse error.

`credits --json` and `video watch` are NOT addressed by this patch. Neither
has a synthesizable fix: `credits` needs a real harvested browser API key
(never shippable in source), and `video watch` needs a `job_name` that only
exists after a human manually submits a generation through Flow's
reCAPTCHA-gated UI (costs real credits). These remain genuine, permanent
limits on what the mechanical matrix can prove for this CLI.

## Confirmed live: results, two wrong theories, and the actual root cause

Re-ran the full live dogfood matrix multiple times after adding the
annotations. This section is left as a record of two successive wrong
diagnoses before the actual bug was found, since the debugging path is
itself a useful lesson: **both `drive`/`episode` failures were caused by a
printed-CLI authoring mistake, not by any Printing Press generator or
dogfood-runner defect.**

**`project` / `projects` (generated/promoted commands): the `pp:happy-args`
fix genuinely worked**, no correction needed here. `agent-context --json`
confirmed the compiled binary's runtime command tree carries the annotation
verbatim, and the live matrix's test args for these two commands changed
from `--input example-value` to the exact `pp:happy-args` value.

**`drive import` / `episode import`: two wrong theories, then the real
cause.** The matrix consistently showed two separate test-row labels for
these commands: `"drive import"`/`"episode import"` (passing) and
`"drive"`/`"episode"` (failing with `1a2b3c4d`/`5e6f7g8h` placeholders and a
`--tag-scene` without `--project` error). Two prior corrections in this
file, in order, both turned out to be wrong:

1. *First theory (wrong):* `pp:happy-args` reading is gated on
   `pp:endpoint`/`pp:method`/`pp:path` ("promoted" vs "novel" commands).
   Disproven once `queue`/`script`/`video` (also novel, no promoted
   annotations) were checked and behaved differently from each other.
2. *Second theory (wrong):* a "parent-command fallback" path fires for any
   Cobra parent with exactly one child, using generic name-based heuristics
   and ignoring both `Example:` and `pp:happy-args`. This looked right until
   cross-checked against **three other library CLIs** (`shopper-pp-cli`'s
   `cashback optimize` -- single-child parent, flag-only `Example`, no
   `pp:happy-args`; `shopper-pp-cli`'s `checkout` -- two-child parent, for
   contrast; `fedex-pp-cli`'s `rate shop` -- single-child parent, flag-only
   `Example`, no `pp:happy-args`): **none of them produced a duplicate
   parent-labeled row at all**, single-child or not. The "single child"
   condition was a coincidence, not the trigger.
3. *Actual cause (confirmed):* `internal/cli/drive.go` and
   `internal/cli/episode.go` -- the hand-written **parent** scaffold files,
   separate from the child command files -- each declared their **own**
   `Example:` field, written once during initial scaffolding and never
   updated: `"flow-pp-cli drive import --folder-id 1a2b3c4d --tag-scene"`
   and `"flow-pp-cli episode import --scribe-folder 1a2b3c4d
   --images-folder 5e6f7g8h"`. The live dogfood matrix correctly and
   mechanically tests **every command that declares an `Example:` field**,
   parent or child, generated or hand-written -- there is no special-casing
   and nothing to fix upstream. It parsed each parent's own stale Example
   verbatim and ran it as a real invocation, which of course failed: those
   placeholder paths never existed on disk, and the annotation lived on the
   *child* command, which is a separate Cobra command with its own,
   independent `Example:`/`Annotations` -- not something the parent's test
   row reads at all. `queue.go`/`script.go`'s parent `Example:` fields
   happened to reuse the *same* literal filenames as their real root-level
   fixtures, so once those fixtures were restored, the parent-level test
   passed by coincidence -- which is exactly what made this took three
   passes to diagnose. `video`'s parent (`promoted_video.go`, a *generated*
   file) has no relation to `video watch` at all; its own bare-`video` test
   row exercises the unrelated generated `POST /video` operation and
   correctly reports `unsynthesizable-body` for that.

   **Fix applied in this CLI:** `drive.go`/`episode.go`'s parent `Example:`
   fields were pointed at their own child's `--help` (e.g.
   `"flow-pp-cli drive import --help"`) -- always a safe, always-successful
   invocation to actually execute, while still being genuinely useful
   documentation. Confirmed live: both `drive`/`episode` parent-labeled rows
   now pass.

**Lesson for future retros on this pattern:** before concluding "the matrix
ignores my annotation," check whether the *parent* Cobra command has its own,
separately-authored `Example:` field. The dogfood matrix has no
parent/child-aware logic here at all -- it just walks every command with an
`Example:` and runs it, which is exactly the mechanical, non-special-cased
behavior you'd want from a matrix like this.

**A second, unrelated bug found during the same investigation (real, printed-
CLI-side, already fixed): a shared fixture file getting clobbered.**
`script draft-prompts`'s own happy-path test (unrelated to the parent-Example
issue above) writes real, non-dry-run output to `episode3-queue.json` when
that specific command lacks a `--dry-run`-safe test path for its own
mutating write -- and `video watch`'s test, which runs later in the same
matrix pass, was sharing that same root-level file for its `job_name`
fixture. **Fix applied in this CLI:** gave `video watch` its own
`pp:happy-args` pointing at a dedicated fixture
(`testdata/dogfood-fixtures/video-queue.json`) that nothing else writes to.

**Net result:** with the parent-Example fix, the companion
`pp:typed-exit-codes` fixes (see
`.printing-press-patches/2026-08-30-typed-exit-codes-credits-video-watch.md`),
and the video-queue fixture, the full live dogfood matrix reaches a clean
pass with no remaining CLI-attributable failures. See
`proofs/phase5-acceptance.json` for the final authenticated result.

## Reprint guard

On reprint, preserve:
- `testdata/dogfood-fixtures/` (both files). Deleting this tree silently
  regresses `episode import`'s live-dogfood coverage back to a hard failure.
- The five `pp:happy-args` annotation values above, on the four listed files.
  `promoted_project.go` and `promoted_projects.go` are otherwise "DO NOT
  EDIT" generated files -- this is a hand-edit tracked here per the standard
  reprint-guard convention, not a claim that the generator emits these
  annotations from spec metadata (this API has no true OpenAPI spec with
  `x-happy-args`; these were authored directly in the generated Go).
- Do not "fix" the `--project` / `projectId` placeholder values by
  hardcoding a real project ID from any specific account. That would leak
  account-specific data into shipped CLI source and would still only work
  for that one account's future dogfood runs, not the CLI generally.
