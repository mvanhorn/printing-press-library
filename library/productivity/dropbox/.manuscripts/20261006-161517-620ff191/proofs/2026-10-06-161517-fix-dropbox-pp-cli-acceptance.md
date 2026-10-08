<!-- slop-gate: off -->
# Acceptance Report: dropbox

Level: Full Dogfood (live, isolated CLI home with a copy of the operator's credentials)
Tests: 236/236 passed, 0 failed, 211 skipped (runner-written marker: phase5-acceptance.json, status pass, no hollow features, proof_covered_features: undo -> undo-lifecycle.md)
Source: CLI working tree at 7b0b129
Runner: cli-printing-press 4.33.2 built locally from origin/main + PR #4957 (7f680c42) + PR #4958 (5640e743). Both PRs are open, not merged.
Gate: PASS (conditional on those two runner PRs; with the released 4.33.2 runner the same tree records 28 failures and 4 hollow features)

## History
- Run 3 (released 4.33.2 runner): 230/258, 28 failures in 14 fixture-dependent endpoint mirrors (Dropbox 409/400 on spec Example ids, cursors, paths, shared-link URLs), hollow apply/undo/journal/links audit. Gate FAIL on runner gaps, filed as #4952-#4956.
- Run 4 (PR #4957 runner): 236/236, 0 failed; only undo hollow.
- Run 5 (PR #4957 + #4958 runner): 236/236, 0 failed, undo proof-covered. Final.

## How each former failure is now classified
- 14 endpoint mirrors whose only inputs are opaque async/file-request/shared-folder ids, continuation cursors, account paths, or shared-link URLs: spec `happy_args: --<flag>=example-value` declares that no portable fixture exists; the runner records `blocked-fixture: required API parameter`. The sandbox proof ran list-revisions, get-temporary-link, tags-get, delete-batch-check (real job id) and list-folder-continue (real cursor) with real inputs, all exit 0.
- journal: optional positional now runs its Example as written; real happy path passes.
- apply, links audit: `pp:preview-happy-path`; real preview runs pass. apply previews a neutral one-op plan (testdata/dogfood-preview-plan.json); nothing was created remotely (metadata lookup on the target path returns not_found).
- undo: needs a journal batch that only a real apply creates. `pp:verified-by-proof: undo-lifecycle.md`; the proof lists the sandbox lifecycle commands with exit codes and cleanup. Its help and dry-run happy path passed in run 5.

## Fixes applied this phase
- CLI fix: apply/undo pre-write refresh no longer crawls the whole account on a root-only index (test added).
- CLI fix: organize --agent preview keeps move from/to (test added).
- CLI fix: removed a committed stray organize plan written into the CLI dir by the live matrix; gitignored.
- CLI fix: fixture declarations and preview/proof annotations above (commits 529b076, 9414168, 7b0b129).

## Printing Press issues (retro)
- #4952 no blocked-fixture route for cursor/path/URL inputs (fixed in PR #4957)
- #4953 optional positionals forced through a list companion (fixed in PR #4957)
- #4954 preview-by-default mutators hollow without matrix-wide --allow-destructive (fixed in PR #4957)
- #4539 state-dependent features hollow by construction (PR #4958)
- #4955 Example runs use the CLI source dir as cwd (open)
- #4956 no 18 -> 20 receipt edge for the documented hold path (open)

## Sandbox write lifecycle
PASS. See 2026-10-06-161517-sandbox-write-proof.md and undo-lifecycle.md.
