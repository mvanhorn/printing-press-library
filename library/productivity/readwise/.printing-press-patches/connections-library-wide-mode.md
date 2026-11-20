# Connections Library-Wide Mode

## Why this local patch exists

The generated `connections` command originally required a single `--seed-doc`.
This local patch adds a library-wide discovery mode for engaged Reader documents
so the CLI can surface document pairs without a seed while preserving the
existing seeded output contract.

## Reprint guard

On reprint, preserve the `internal/cli/connections.go` library-wide path unless
the upstream Printing Press catalog has equivalent behavior:

- Keep `--library-wide` and `--all` dispatch separate from seeded mode.
- Keep bare `connections` as a usage error, not a silent empty result.
- Keep the engaged-document corpus based on tags, `last_opened_at`, or
  `reading_progress > 0`; do not use highlights for Reader documents.
- Keep term DF filtering, and also apply the same `df/total > 0.15` cutoff to
  tags and authors for both indexing and shared-signal scoring.
- Keep the inverted-index guard that skips postings lists above the cap and
  surfaces skipped signals in the output note.
- Keep library-wide near-duplicate suppression using normalized URL or valid
  normalized title plus author, and surface the hidden count in the note with a
  `dedupe` pointer.
- Keep library-wide default output aligned with seeded mode: JSON by default,
  with the markdown renderer covered separately by tests rather than selected
  only because stdout is a TTY.
- Keep `--all` scoped to `--library-wide`; seeded `connections --seed-doc X
  --all` must return a usage error.
- Preserve seeded JSON as `{seed, matches}` and library-wide JSON as `{pairs,
  scanned, note}`.
- Preserve the library-wide tests in `internal/cli/connections_test.go`,
  especially the tag/author DF cutoff, exact DF boundary, engaged filter,
  dispatch, skip-signal, pair uniqueness, ranking order, near-duplicate, and
  determinism cases.

Library-wide candidate work uses precomputed identities and sorted signal intersections, uint64 pair keys and a bounded top-K heap. Signals over min(2000, N/20) are skipped with a floor of three postings to preserve small-library connections and the existing 15% boundary fixture. --all requires a positive --top. Meaningful URL query parameters remain distinct; burst saves need the same author or host.
