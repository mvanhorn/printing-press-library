# Readwise API drift changes

Applied the documented update-spec items; no no-action items applied.

| Drift ID | Change |
|---|---|
| A1 | Added `review.complete`, POST `/review/complete/`, with completion response description. |
| A2 | Added `books.update` with title/author overrides, category enum, and cover image handling. |
| A3 | Added `highlight-tags.get` with highlight and tag positional IDs. |
| A4 | Added `book-tags.get` with book and tag positional IDs. |
| B1 | Added `last_highlight_at__lt` / `last-highlight-before`. |
| B2 | Removed `location_type` only from highlight UPDATE. |
| B3 | Added the four-category highlight CREATE enum. |
| B4 | Described all eight CREATE field limits, including source_type length and space restrictions. |
| B5 | Described the four-field dedupe key and highlight_url text upsert rule. |
| B6 | Added `pagination.type: page` to v2 highlight/book lists; described the wrapper and following next until null. |
| B7 | Removed note only from the Reader save category enum. |
| B8 | Added save language, parent_id, content; made url optional with its conditional requirement and highlight creation restrictions described. |
| B10 | Described up to five repeated tag query parameters, AND semantics, and empty-value behavior. |
| B12 | Added update language and reading_progress, with 0–1 and PDF restrictions. |
| B13 | Described highlight-only notes/tags updates, 400 rejection, and updating notes through their highlight. |
| B15 | Added bulk item language, reading_progress, published_date, image_url, category enum and tag replacement description; documented 50-item maximum, results wrapper and 200/207/400 handling. |
| B16 | Added cascade warning, 204 response description and dry-run/confirmation guidance. |
| C1 | Added five Document fields, content opt-in rules and ephemeral raw_source_url storage warning. |
| C3 | Described parent_id as a Document self-reference for highlights and notes. |
| C4 | Added Highlight external_id and end_location. |
| C5 | Added Book asin, resurface_weighting, source_syncs_all_books_together and CREATE-only modified_highlights IDs. |
| C6 | Added ExportBook external_id and ExportHighlight with all listed fields; described the nested array relationship. |
| C7 | Added Review and ReviewHighlight types, connected review responses, and described 404 as no review today. |
| D1 | Documented the v2 default client budget of 240 requests/min per access token. |
| D3 | Documented all six Reader endpoint rate limits. |

## Format limitations

- B10: no repeatable-query construct is documented. The existing string flag remains; repetition, maximum five and AND semantics are described. This does not implement repeated CLI flag serialization.
- B8: no conditional-required construct is documented; url is optional in the schema and its dependency is described.
- B4/B12/B15: no length, numeric-range or array-count constraint keys are documented; limits are descriptions.
- B5/B8/B13/B16/C1/C3/C5: dedupe, cross-field restrictions, lifecycle/storage rules, self-reference and response-specific field availability are descriptive semantics.
- B15/C7: no status-specific response or runtime policy construct is documented. Partial-success/error printing, 404 handling and delete confirmation are documented guidance, not client implementation changes.
- C6/C7: no typed array-element reference is documented for TypeField. ExportHighlight and ReviewHighlight are named field-list types, connected to their containing arrays through descriptions.
- D1/D3: no rate-budget construct is documented; limits are descriptions.
- B6: the reference explicitly mentions page pagination; next-link traversal is described because no full-URL next-link traversal construct is documented.

## Verification

The required generator dry run exited 0. Baseline: resource_count 7, endpoint_count 22. Updated: resource_count 7, endpoint_count 26. A method/path comparison retained all 22 original endpoints and found exactly books.update, review.complete, highlight-tags.get and book-tags.get as additions. Export remains hand-built.

No new tests shipped. Verification used the generator dry run and an ad hoc endpoint comparison; no live API writes or long gate ran.
