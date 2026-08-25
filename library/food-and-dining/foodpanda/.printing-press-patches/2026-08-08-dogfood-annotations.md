# Dogfood annotations (reprint guard)

## internal/cli/promoted_reviews.go — `pp:no-error-path-probe`

**Why:** foodpanda's reviews API returns HTTP 200 with an empty list for unknown
vendor codes, so a bad code is indistinguishable from a vendor with no reviews.
The dogfood error-path probe therefore cannot pass, and inventing a local
"empty means not found" heuristic would be an API-specific fabrication.

**Why it is a hand-patch:** the endpoint-level `no_error_path_probe: true` key is
present in the source spec but is NOT propagated to the generated command's
Cobra annotation. Filed as a Printing Press retro candidate.

**On reprint:** re-apply if the generator still drops the spec flag; delete this
entry once the flag is honored upstream.

## internal/cli/home.go, internal/cli/addresses.go — `pp:requires-tier: session`

**Why:** both read `/api/v5/customers/addresses`, which needs the composed cookie
session. The dogfood runner executes each subprocess in a sandboxed HOME and
overrides `FOODPANDA_*`, so an operator session on the host cannot reach it;
without the annotation these fail rather than skip. Both are verified working on
a host with a session (`auth login --chrome` / `auth set-token`).

**On reprint:** these files are hand-authored and preserved, so the annotation
should survive. Verify it is still present after any regen.

## internal/cli/dish.go, internal/cli/menu_diff.go — `pp:no-error-path-probe`

Both are single-word top-level commands. The dogfood matrix cannot synthesize a
positional for those ("command path [dish] has fewer segments than placeholders"),
so their `Use:` declares no placeholder and input arrives via `--query` /
`--vendor-code` (the positional still works for humans). That in turn makes the
error-path probe skip with "no positional argument", which the promote gate
counts as hollow coverage.

The two commands differ, and the distinction is recorded deliberately:

- **`dish` genuinely has no error path.** A nonsense query returns rc=0 with
  `match_count: 0` and an explanatory `note`, which is the correct answer —
  "no menu item matched" is not an error. Verified:
  `dish --query __printing_press_invalid__` → rc=0.
- **`menu-diff` DOES have a real error path** that the probe cannot reach.
  Verified by hand: `menu-diff --vendor-code __printing_press_invalid__` → rc=1,
  HTTP 404 "The specified vendor was not found". The annotation here reflects a
  harness limitation, not an absent error path.

**On reprint:** re-check whether the matrix can synthesize positionals for
single-word commands; if so, drop the annotation from `menu-diff` first.

---

## SUPERSEDED 2026-08-20 — `home.go` no longer needs the tier annotation

`internal/cli/home.go` does **not** carry `pp:requires-tier: session` and does
not need it. The command now takes explicit coordinates without a session
("Explicit coordinates preview any location without a session"), and its
`pp:happy-args` pass `--latitude` / `--longitude`, so the dogfood matrix
exercises the session-free path and passes. `internal/cli/addresses.go` still
carries the annotation and still needs it.

The `dish` / `menu-diff` / `promoted_reviews` `pp:no-error-path-probe` entries
above are still present and still accurate.
