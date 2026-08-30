# Acceptance Report: rundown-pp-cli

**Level:** Full Dogfood (live)
**Tests:** 106/106 passed, 0 failed, 79 skipped (skips are mutation/positional probes that do not apply to a read-only CLI)
**Gate: PASS**

Full depth was chosen without prompting because the CLI is read-only against a
public API: no credentials, no billing, no outbound messages, no mutation. There
is no real-world side effect that full live testing could trigger.

## Failures found and fixed

Three failures surfaced on the first live run. All three were fixed in-session
rather than deferred.

### 1. `stack` returned exit 0 for an unknown tool slug
`error_path` probe: `stack __printing_press_invalid__` exited 0.

A slug that appears nowhere in the catalogue is bad input, not an empty result.
Fixed by distinguishing the two cases explicitly:

- slug absent from the whole catalogue -> `notFoundErr` (exit 3), plus
  "did you mean" suggestions from a Levenshtein pass over known slugs
- slug valid but no workflows inside `--since` -> exit 0 with a note

Declared `pp:typed-exit-codes: "0,3"` so the live matrix counts exit 3 as a pass.

Verified: `stack zzzznotatool` -> 3; `stack n8n` -> 0; `stack n8n --since 1h`
(valid slug, empty window) -> 0.

### 2. `use-cases` returned exit 0 for a no-match topic
`error_path` probe: `use-cases __printing_press_invalid__` exited 0.

This one was correct behaviour being mis-probed. There is no such thing as a
malformed free-text search query, so a topic that matches nothing is a valid
empty result, not bad input. Inventing an error for it would have made the
command worse to satisfy a test.

Fixed by declaring the opt-out the framework provides for exactly this case:
`pp:no-error-path-probe: "true"`.

### 3. `feedback --help` had no Examples section
A generated framework command, not hand-written code. Editing
`internal/cli/feedback.go` would be reverted by `generate --force`, so the
examples are attached through the `registerNovelCommand` hook in the
hand-authored `internal/cli/rundown_framework_help.go`, which regeneration
preserves. The hook defers if a future generator version supplies its own
examples.

### 4. Regression introduced by fix (1), caught by the scorecard live probe
Fixing `stack` to return exit 3 for an unknown slug immediately broke the
scorecard's sample-output probe:

```
Tool co-occurrence stacks: exit 3: Error: unknown tool slug "claude-code"
```

`claude-code` is the single most-used tool in the corpus. The probe runs in a
sandboxed HOME with **no synced mirror**, so `allSlugs` was empty and the new
`unknownSlug` test could not distinguish "this slug does not exist" from
"nothing has been synced yet". The fix turned a missing-sync state into a
spurious input error.

Corrected by requiring evidence before making the claim:

```go
unknownSlug := usingTool == 0 && len(posts) > 0 && !allSlugs[target]
```

plus a distinct empty-mirror note pointing at `sync`. Verified across all three
states: empty mirror + valid slug -> 0; populated + valid -> 0; populated +
bogus -> 3.

Worth stating plainly: the typed-exit fix was correct in isolation and wrong in
the state the probe actually runs in. A negative claim ("this slug is unknown")
needs positive evidence that the data to judge it was present.

## Process note worth recording

The first two re-runs after fixing (1) and (2) still reported the same three
failures. The fixes were correct; **dogfood was executing the stale staged
binary** at `build/stage/bin/rundown-pp-cli.exe` while only the repo-root binary
had been rebuilt. The giveaway was that the reported output showed pre-fix
behaviour despite passing manual checks.

When live dogfood results contradict a manual check, compare
`build/stage/bin/` mtime against `internal/` sources before re-diagnosing the
code.

## Coverage

- All 6 novel commands: help, happy path, JSON fidelity, error path
- All 4 generated endpoint commands
- Framework surface: sync, search, doctor, export, workflow, learn loop, profile
- Output modes: `--json`, `--agent`, `--select`, `--csv`, `--human-friendly`
- Sample output probe: 6/6 novel commands returned real, relevant data

No API key involved; auth type is `none`.
