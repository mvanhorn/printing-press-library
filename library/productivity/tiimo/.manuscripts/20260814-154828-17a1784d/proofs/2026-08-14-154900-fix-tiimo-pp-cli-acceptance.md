# Acceptance Report: tiimo

    Level:  Full Dogfood
    Tests:  207/207 passed (0 failed, 101 skipped as not-applicable)
    Gate:   PASS

Run against a real Tiimo account with a live bearer token. Personal schedule
content is deliberately not quoted below; findings are described structurally.

## Live data exercised

| Resource | Records mirrored |
|---|---|
| activities (occurrences) | 602 over 30 days / 1190 over 90 days |
| distinct activities | 14 |
| to-do tasks | 13 |
| tags | 14 |
| linked calendars | 2 |
| to-do lists | 1 |
| routines | 0 (account has none) |

Write path exercised on self-created, self-deleted fixtures only. Every
fixture was removed and absence re-verified against the API; the account
finished in exactly the state it started in.

## Failures found and fixed (15 -> 7 -> 4 -> 3 -> 1 -> 0)

1. **`sync` could not reach any nested resource.** The generated
   `syncResourcePath()` registers only flat paths, and every Tiimo resource
   except `/api/profiles` is scoped under `/api/profiles/{profile_id}/...`, so
   sync reported "unknown sync resource" for all of them. The local mirror
   could never be hydrated, which silently emptied every offline command.
   Replaced with a hand-written syncer that enumerates profiles and fans out.
   *Generator defect - filed for retro.*

2. **Upsert errors were swallowed.** The first syncer version reported
   `activities 0 ok` -- success with zero records -- because it `continue`d
   past store failures. Now counted, surfaced, and fatal when nothing lands.
   *My defect, and the exact silent-failure pattern the conventions forbid.*

3. **Every activity id collapsed onto 14 rows.** Recurring activities share
   one `activityId` across all dates, so 1190 occurrences overwrote each other
   last-write-wins, destroying the per-occurrence history that `drift`,
   `adherence` and `stalls` exist to read. Rows are now keyed by
   `activityId + NUL + occurrenceDate`, using the store's own composite-key
   delimiter so `BareResourceID()` strips it correctly.

4. **Timezone skew emptied `today` and `gaps`.** `parseTiimoTime` used
   `time.Parse` (UTC) while window bounds used `time.Local`. On a UTC-5
   machine that shifted every comparison, and since Tiimo stores
   bucket-scheduled activities at `T00:00:00`, they fell off the front of the
   window: `today` returned nothing on a day with 14 activities. Now parsed
   with `ParseInLocation`.

5. **`overlaps` reported 91 false positives on one ordinary day.** Every
   bucket-scheduled activity sits at midnight, so all of them "collided".
   Added `ClockScheduled()` and restricted overlap and gap arithmetic to
   activities that genuinely occupy clock positions. Result: 91 -> 0.

6. **`drift` counted every occurrence as "started".** Tiimo pre-populates
   `durationActual` with the planned duration even for untouched occurrences,
   so keying off it marked all 1190 as run and reported a confident zero
   drift. `Started()` now requires real evidence (completion, recorded pause,
   or an actual start diverging from plan), and activities with no execution
   history are omitted rather than shown as rows of zeros.

7. **`done` reported success while doing nothing.** Writing `completedAt`
   through the activity update endpoint returns HTTP 200 and silently
   discards it; seven candidate completion endpoints all 404'd. Recaptured
   the web app, clicking a completion circle on a throwaway activity, and
   found the real contract:

       POST /api/profiles/{profileId}/activityactions
       {actionTime, actionType: "Completed"|"Reset", instanceDate, activityId}

   `instanceDate` makes completion per-occurrence, independently confirming
   the occurrence-keying in fix 3. `done` now posts this and verifies the
   returned state rather than trusting the status code; `--undo` posts
   `Reset`. Verified against server state: `state=Completed` / `completedAt`
   set, then both cleared on undo.

8. **`feed --out - --json` emitted a raw ICS document on stdout**, which is
   not JSON. Machine modes now return the document inside the envelope.

9. **`add --at` implied a precision the API does not have** (see Known Gaps).
   Reworked around `--bucket`.

10. **Live-matrix fixture gaps.** Profile-scoped generated commands were
    probed with no arguments and correctly exited 2. Rather than weaken their
    validation to make tests pass, real ids are injected as `pp:happy-args`
    from a hand-authored file.

11. **`feedback --help` failed its own check** for lacking an `Examples:`
    section -- a generator-emitted command failing a generator check. Proven
    by fixing the identical failure on this CLI's `todo` parent purely by
    adding an Example. Supplied from a hook rather than editing a DO-NOT-EDIT
    file. *Generator defect - filed for retro.*

## Known gaps (disclosed, not fixed)

- **Tiimo activities are not clock-scheduled.** The API normalizes any start
  time to midnight; activities are ordered inside Morning/Afternoon/Evening/
  Anytime buckets. Verified structurally: 0 of 1190 mirrored activities carry
  a clock time. `gaps` and `overlaps` therefore have little to find among
  native activities, and both now disclose bucket-scheduled committed time
  instead of implying an empty day. Only imported calendar events carry real
  times, and those are read-only.
- **One linked calendar returns HTTP 500** (`ArgumentNullException`) from
  Tiimo's own server on every request. An upstream fault, not a client error.
  Sync treats it as a partial failure, mirrors the other calendar, and
  reports it rather than hiding it.
- **`sync` does not reconcile deletions.** Items removed upstream linger in
  the local mirror until it is rebuilt.
- **No mood, energy, or focus-timer endpoints** were found; those features
  appear not to be exposed to the web client.
- **`adherence` and `stalls` currently report zero completion** for this
  account. That is accurate, not a defect: `completed_at` is null across all
  1190 occurrences.

## Printing Press issues for retro

1. `syncResourcePath()` omits parent-scoped resources, making sync unusable
   for any API whose resources are nested under a path parameter.
2. `extractObjectID()` does not recognize the `<resource>Id` naming
   convention, so records fail to store, and resources that happen to carry a
   `name` field are silently keyed by name instead -- collision-prone.
3. Generated commands with subcommands ship without an `Examples:` section
   and fail the live-dogfood help check (`feedback`).
4. Generated `go.mod` pinned `golang.org/x/text` v0.38.0 with a reachable
   advisory (GO-2026-5970).
5. Scorecard's live sample probe intermittently dies with SIGBUS at a
   page-aligned address, on a different command each run; never reproducible
   outside the harness.
