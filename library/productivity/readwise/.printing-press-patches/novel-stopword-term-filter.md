# Novel Stopword Term Filter

## Why this local patch exists

The generated `connections` and `ask` implementations share `novelTokens` for
term extraction. The original generated stopword set was small enough that
common function words such as `they`, `their`, `through`, and `instead` could
become shared connection terms and link unrelated Reader documents.

## Reprint guard

On reprint, preserve the package-level `novelStopwords` table in
`internal/cli/ask.go` and keep `novelTokens` using it before emitting terms:

- Keep the existing `len(tok) < 4` and numeric-token filters.
- Keep meaningful content words available for `ask` dossiers and
  `connections` overlap.
- Do not change the `connections --seed-doc` requirement.
- Do not change the document-frequency cutoff in
  `novelSignificantConnectionTerms`.
- Preserve `TestNovelConnectionsNoStopwordOverlap` as a corpus-level
  connections-path regression: shared stopwords such as `they`, `their`, and
  `through` must stay below the DF cutoff so the stopword filter, not the DF
  filter, is what prevents unrelated documents from connecting.
- Keep a positive low-DF content-word guard in that test so real shared terms
  such as `leadership` still produce a connection.
