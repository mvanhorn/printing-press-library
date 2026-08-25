# Planning Workflow Reference

Shared-plan preview, clone, fill, and empty-source handling. Default inspect and auth rules live in `SKILL.md`.

## Core Workflow

1. `wanderlog-pp-cli doctor`.
2. Shared source: `plan preview --source-url URL --agent`.
3. New private copy only when the user wants a copy: dry-run `plan clone --source-url URL`, then `--apply` after approval.
4. Existing trip: `trips home` → `plan outline --target-key KEY`, then dry-run `plan fill` if filling from a source.
5. `--force` on fill only after confirming overwrite/merge intent.

Editing the shared URL in place is not a clone. See `SKILL.md`.

## Commands

- `plan preview`: Inspect a shared/public plan without writing. Reports dates, sections, block counts, clone warnings.
- `plan clone`: New private trip filled from a source. Auth + `--apply` to write.
- `plan fill`: Fill an existing target from a source. Auth, target approval, dry-run first.
- `guides get`: Public guide/plan structure. `--select` is fine here for place metadata — not a substitute for `plan outline` on a trip.
- `trips home`: Account trip list; `key` is the 16-char editable key.
- `plan outline` / `plan inspect`: Slim itinerary for a trip.

## Empty Shared Plan

Do not assume a shared plan has useful stops. `plan preview` and `plan clone --dry-run` expose `blocks`, `place_blocks`, and `note_blocks`. If those are zero, the source is a date/section skeleton — say so.

1. Confirm dates, day count, and geo from `plan preview`.
2. Create/fill the target only if the user still wants the skeleton.
3. Candidate pool: `places autocomplete`, `places card/details`, public guides, user constraints.
4. `itinerary-drafting.md` before turning candidates into days.
