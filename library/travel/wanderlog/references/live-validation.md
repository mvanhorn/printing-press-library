# Live validation fixture contract

The CLI supports optional Printing Press `pp:happy-args` annotations for exercising commands that need an existing block, expense or reservation. These annotations supply real test inputs to the runner. They do not create data, replace API responses, skip validation, or change normal command flag defaults.

Set these variables only for validation against a disposable plan you own:

- `WANDERLOG_DOGFOOD_PLAN_KEY`: the fixture plan's editable key.
- `WANDERLOG_DOGFOOD_NOTE_BLOCK_ID`: the actual stable ID of its day-one note. Without it, block-get retains its ordinary examples.
- `WANDERLOG_DOGFOOD_CHANGES_FILE`: path to a real reviewed JSON changes file using actual fixture IDs. Without it, plan-edit retains its ordinary examples. The runner still reads and validates that file.

The fixture must already contain:

| Location | Required content |
| --- | --- |
| Day 1, block index 0 | Note with an attachment at index 0 |
| Day 1, block index 1 | Place block, such as Eiffel Tower |
| Day 1, block index 2 | Checklist with at least one item |
| Day 2, block index 0 | Flight reservation |
| Day 8 | Empty day section |
| Budget | An expense at index 0 and payment at index 0 |

Fixture annotations override only attachment removal, block-get, block rename, expense edit/removal, payment removal, checklist item add/check/removal, place replacement, reservation edit/removal, section deletion and plan-edit. Boolean flags are deliberately absent from annotations. Each mutator retains `--dry-run` in its real help example, and short help exposes inherited `--dry-run` so the audited Printing Press runner can inject its supported preview flag. A passing preview validates current target data and constructs operations; it does not establish that a live write succeeded. Fixture construction and any explicit write round trip require separate authorization.

Lodging-search annotations always supply UTC calendar dates 30 and 37 days after command-tree creation. These are runner inputs only: a user's lodging command still uses the dates they supplied. Archived help examples cannot thereby force the live runner to search past dates.

Printing Press v4.31.1 parses `pp:happy-args` as semicolon-separated `--flag=value` entries, not shell syntax. A semicolon in a file path is escaped as `\;`; paths with spaces remain literal values. No shell executes these strings. The v4.31.1 runner expands boolean annotation entries such as `--dry-run=true` into separate `--dry-run true` arguments; the latter leaves a stray positional argument under Cobra. Do not put `--dry-run`, `--markdown`, `--checked`, or other booleans into fixture annotations. Keep real boolean flags in command examples (including `--checked` for checklist checking), and retain the runner’s audited dry-run injection. Strict argument checking remains enabled. The annotations are visible to runtime discovery when fixture environment variables are set, so do not publish captured command metadata containing a private plan key or local path. Credentials are never added to annotations.

Unset the three fixture variables after validation. With no plan fixture key, existing plan examples and all normal command execution remain unchanged. Missing optional ID/file inputs are not invented and do not receive passing placeholder results.

To build a new fixture, create a private eight-day trip with `trips create` using a real geo ID and future dates. On day 1, append a note, a real place and a checklist in that order. Add one attachment to the note. On day 2, append a synthetic flight reservation. Add one synthetic budget expense and settlement record. Label all records as tests and leave day 8 empty. Preview each setup command, then apply it only to the disposable target. These are actual fixture-setup writes, unlike the subsequent dry-run matrix.

Read the note's stable ID from the add response or refreshed outline and save a changes file proposing a different note or schedule. Export the three fixture variables and run `cli-printing-press dogfood --dir . --live --level full --auth-tier authenticated --write-acceptance <manuscript-proof-path> --json`. Use the CLI's existing authenticated configuration. Keep the raw output private because the runner includes resolved target keys. Then run `cli-printing-press publish validate --dir . --json` without fixture variables. Only a passing live marker for the unchanged source satisfies publication.

The optional `WANDERLOG_DOGFOOD_BLOCKS_FILE` supplies a real synthetic array for `plan block add-batch` previews. Overview and multi-day runner inputs reuse the private fixture key. These values only configure runner arguments; normal command behavior and validation are unchanged. Keep raw reports and fixture files private.
