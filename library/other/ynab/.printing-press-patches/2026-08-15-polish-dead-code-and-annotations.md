# Polish patch: 2026-08-15

Hand-edits applied to generator-owned (`DO NOT EDIT`) files during a
`/printing-press-polish` pass. Recorded here so a reprint carries the intent
forward instead of silently dropping it. None of these change generated
templating logic; they are narrow, low-risk fixes surfaced by `dogfood`/
`verify`/`publish validate`.

## internal/cli/helpers.go — removed dead function `collectionItemsForOutput`

`dogfood` flagged `collectionItemsForOutput` as a dead function (defined,
never called). Verified via repo-wide grep: no call sites anywhere in
`internal/cli`. Its sibling helpers (`extractPaginatedItems`,
`paginatedCollectionEnvelopeField`, `extractPaginatedItemsFromObject`) are
still used elsewhere and were left in place. Removed the function and its
doc comment; no behavior change since nothing called it.

## internal/cli/{learnings,teach_playbook,channel_workflow,profile}.go — added `pp:typed-exit-codes: "0,2"`

`accounts`, `categories`, and `category-groups` (generated `pp:api-resource`
parent groups) already carry `"pp:typed-exit-codes": "0,2"` on their
`Annotations` map, documenting that a bare-parent invocation intentionally
exits 2 (`parentNoSubcommandRunE`) rather than 0. `learnings`, `playbook`,
`workflow`, and `profile` use the exact same `parentNoSubcommandRunE` helper
and the exact same bare-invocation behavior, but were missing the
annotation, which `verify` reads to decide whether a non-zero bare-parent
exit is expected. Added the matching annotation to bring all four in line
with the existing sibling convention; no runtime behavior changed.

## Retro candidates (not hand-patched here)

- `--max-age` global flag (`internal/cli/root.go`) is registered
  unconditionally by the generator but only does anything when a
  "freshness helper" is emitted, which only happens for CLIs with a sync
  command. This CLI has no sync command (`verify`'s
  `freshness.detail: "cache freshness helper not emitted"`), so the flag is
  dead by generator design gap, not by omission in this printed tree.
  Recommend the generator conditionally skip registering `--max-age` when
  no freshness helper is emitted. Filed as a retro candidate rather than
  hand-patched here since it's a public, user-visible global flag and the
  real fix belongs in the template.
