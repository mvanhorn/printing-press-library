# Read Command Defaults And Doctor Counts

## Why this local patch exists

The generated `readwise-pp-cli` tree had human-facing read/analytics commands
that advertised useful defaults but returned help on bare invocation. The same
print also labeled `sync_state.total_count` as cache rows in `doctor`, which can
diverge from the actual local store after incremental syncs.

## Reprint guard

On reprint, preserve these CLI-surface behaviors unless upstream Printing Press
has equivalent fixes:

- Bare `backlog`, `digest`, `dedupe`, `read-next`, and `triage stale`
  invocations execute their default computations instead of printing help.
- Bare `triage stale` defaults both `--never-opened` and `--zero-highlights`
  only when neither toggle was explicitly changed by the user.
- `doctor` keeps staleness timestamps from `sync_state.last_synced_at` but
  reports actual stored resource row counts via the same store count source used
  by `workflow status`, not `sync_state.total_count`.
