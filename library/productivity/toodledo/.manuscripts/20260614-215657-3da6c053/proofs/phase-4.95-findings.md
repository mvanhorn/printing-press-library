# Phase 4.95 Local Code Review — Toodledo CLI

Reviewer verdict: **0 error, 4 warning, 11 info — no blockers.** Clean SQL (all table/column names are string literals; user values flow through `?` placeholders / `strconv.Atoi`), NULL-safe scanning throughout, no credential leakage, OAuth Basic-auth blocks correct (gated on non-empty secret, dual-send with form fallback, `expires_in:0` guard present).

## Fixed in place
- **dashboard "due today" timezone mismatch** (warning): due dates are stored at noon UTC (`parseDueDate`), but the today-window was computed in local time → far-east/west undercount. `startOfTodayUnix` now uses UTC. (`toodledo_common.go`)
- **sync-cost swallowed mirror-open error** (warning): `openLocalMirror`'s error was discarded, so a corrupt mirror looked identical to "no mirror." Now surfaced as a stderr warning. (`sync_cost.go`)
- **nondeterministic fuzzy name resolution** (info): `resolveRefID` substring match returned the first map hit (random order); now picks shortest-then-lexical deterministically. (`toodledo_common.go`)
- **availableNames unsorted** (info): doc promised sorted; added `sort.Strings`. (`toodledo_common.go`)
- **capture partial-batch error** (info): errors now report how many tasks were already created, so a retry isn't a blind double-create. (`capture.go`)
- **`--star` registration idiom** (info): unified `BoolP("star","",...)` → `Bool("star",...)`. (`tasks_write.go`)

## Accepted as-is (documented / expected)
- goal-progress `done` counts read 0 unless the user runs `sync --param comp=-1` — documented in the command's Long help.
- `within_budget` in sync-cost is a lower bound (>1000 changed rows page) — documented in the result `note`.
- Inbox/Waiting/Someday review buckets are independent axes (a task can appear in two) — intended multi-axis review.

## Retro candidate (generator-owned, out of scope for the printed CLI)
- **`truncate` in `internal/cli/helpers.go` is not UTF-8-safe** — it slices by byte (`s[:max-3]`), which can split a multibyte rune in non-ASCII task titles, violating the AGENTS.md "UTF-8-safe string truncation" rule. The novel commands depend on this generator helper. File against the machine.
