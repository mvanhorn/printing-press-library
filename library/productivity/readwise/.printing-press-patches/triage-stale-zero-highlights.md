# Triage Stale Zero Highlights Scope

## Why this local patch exists

Reader documents are not expected to have child highlights, so treating
documents without child rows as `triage stale --zero-highlights` orphan sources
turns the report into noise for normal Reader libraries. The useful signal is
unmined Readwise v2 sources: books/source rows whose stored `num_highlights`
value is zero.

## Reprint guard

On reprint, preserve `triage stale --zero-highlights` as a books-only orphan
source scan:

- Read `num_highlights` from `books.data` with a NULL-safe JSON expression.
- Return orphan source rows with `id`, `title`, and `num_highlights`.
- Do not add Reader documents to `orphan_sources` based on missing child
  highlights.
- Keep the existing stale document path for `--never-opened` plus
  `--older-than` unchanged.

A positive API highlight count with no local rows indicates not_synced, so keep it in a separate not_synced_sources list instead of recommending it as an orphan.
