# Goodreads Web Route Reference

Generated: 2026-05-22T08:49:00Z

## Scope

This is the first local `api-map/` artifact for the Goodreads venue. It is derived from:

- `goodreads/proofs/network-seed-2026-05-22.json`
- `goodreads/proofs/user-pages-map-2026-05-22.json`
- `goodreads/proofs/user-pages-map-batch2-2026-05-22.json`
- `goodreads/proofs/nav-bookshelves-links-2026-05-22-v3.json`
- `goodreads/proofs/genres-page-links-2026-05-22.json`
- `goodreads/proofs/discovery-pages-map-2026-05-22.json`
- `goodreads/proofs/bookshelves-read-move-map-2026-05-22.json`
- `goodreads/proofs/bookshelf-edit-popover-map-2026-05-22-v2.json`
- `goodreads/proofs/bookshelf-script-routes-2026-05-22.json`
- `goodreads/proofs/notes-publicize-writes-2026-05-22.json`
- `goodreads/proofs/notes-publicize-verify-2026-05-22.json`
- `goodreads/research/current-api-sweep-2026-05-22.md`

Raw proofs are private local evidence. They can include account-visible labels and Kindle highlight text. This route reference is intentionally route-level and does not include cookies, headers, response bodies, or highlight text.

## Auth Model

Goodreads web uses a browser cookie session. Login can redirect through Amazon SSO. The local session artifact is:

```text
sessions/goodreads.json
```

The CLI should not print cookie values. For replay, import cookies into a jar from a local `0600` file or use a live browser session. Goodreads forms include Rails-style CSRF tokens, so write requests need token extraction from the current page unless a route is proven tokenless.

## Observed Routes

| Method | Route | Surface | Source | CLI use |
|---|---|---|---|---|
| GET | `/` | home/feed | network seed | feed/activity parser |
| GET | `/user/show/:user_slug` | profile | browser map | `profile show` |
| GET | `/user/delayable_user_show/:user_id` | profile XHR | browser map | delayed profile sections |
| GET | `/review/list/:user` | shelves/books | browser map | `books list`, `shelf show`, `export shelves` |
| GET | `/review/list_rss/:user` | public shelf RSS XML | external + curl proof | `books rss`, public fallback |
| GET | `/tooltips` | book metadata XHR | shelf/list pages | batch book metadata enrichment |
| GET | `/book/show/:book_slug` | book detail | browser map | `book show` |
| GET | `/opensearch.xml` | search descriptor | curl proof | canonical search template |
| GET | `/search/search` | book search | opensearch proof | `search books` |
| GET | `/quotes/list` | user quotes | browser map | `quotes list` |
| GET | `/quotes/widget/:user_slug` | user quotes widget | browser map | alternate quote source |
| GET | `/comment/list/:user_slug` | comments/posts | browser map | `comments list` |
| GET | `/notes/:user_slug` | notes/highlights index | browser map | `notes list` |
| GET | `/notes/:user_id/load_more` | notes pagination XHR | browser map | notes pagination |
| GET | `/notes/:book_slug/:user_slug` | notes book detail | browser map | `notes show --book-id` |
| PUT | `/notes/:book_id/share` | notes publicize write | approved write proof | `notes publicize --execute` |
| POST | `/notes/:book_id/:annotation_pair_id/note` | per-note thought write | DOM only, unsubmitted | disabled until capture |
| GET | `/notifications` | notifications | browser map | `notifications list` |
| POST | `/notifications/track` | analytics | browser map | omit from CLI |
| GET | `/friend/requests` | friend requests | browser map | `friends requests` |
| GET | `/friend` | friends | browser map | `friends list` |
| GET | `/topic` | discussions/groups | browser map | `topics list --scope groups` |
| GET | `/message/inbox` | messages | browser map | `messages inbox` |
| GET | `/message/:folder` | message folders | discovery map | `messages list --folder` |
| GET | `/message/show/:message_id` | message detail | discovery map | `messages show` |
| POST | `/message/move_batch` | message folder/read action | DOM only, unsubmitted | dry-run only |
| GET | `/message/mark_all_as_read` | mark all inbox read | DOM only, unclicked | dry-run only |
| GET | `/user/year_in_books/:year/:user_id` | yearly reading summary | browser map | `year-in-books show` |
| GET | `/list/show/:list_id` | public lists | browser map | `lists show` |
| GET | `/recommendations` | recommendations | browser map | `recs list` |
| GET | `/recommendations/to_me` | friends' recommendations | nav capture | `recs friends` |
| GET | `/amazon_purchases` | Amazon purchases import page | discovery map | `amazon-purchases inspect` |
| GET | `/shelf` | top public shelves | discovery map | `discovery shelves` |
| GET | `/shelf/show/:shelf_slug` | public shelf landing | discovery map | `discovery shelf show` |
| GET | `/genres` | all genres | genre capture | `genres list` |
| GET | `/genres/list` | all genre shelves | discovery map | `genres list --all` |
| GET | `/genres/search` | genre finder | discovery map | `genres search` |
| GET | `/genres/:genre_slug` | genre page | genre capture | `genres show` |
| GET | `/user/top_readers` | people discovery | discovery map | `people top-readers` |
| GET | `/user/top_reviewers` | people discovery | discovery map | `people top-reviewers` |
| GET | `/user/best_reviewers` | people discovery | discovery map | `people popular-reviewers` |
| GET | `/user_following/most_followed` | people discovery | discovery map | `people most-followed` |
| GET | `/review/stats/:user` | reading stats | nav capture | `reading-stats` |
| GET | `/review/drafts` | review drafts | nav capture | `reviews drafts` |
| GET | `/review/duplicates` | duplicates tool | nav capture | `books duplicates` |
| GET | `/review/import` | import/export page | nav capture | `export shelves` fallback |
| POST | `/user_shelves` | create custom shelf | DOM only, unsubmitted | dry-run only |
| POST | `/shelf/update/:shelf_id` | shelf table settings | DOM only, unsubmitted | dry-run only |
| POST | `/review/update_list/:user_id` | batch add/remove selected reviews to shelf | DOM/script only, unsubmitted for shelves | dry-run only |
| POST | `/shelf/add_to_shelf` | single-book shelf add | script-mining proof | dry-run only |
| POST | `/shelf/remove_book` | single-book shelf remove | script-mining proof | dry-run only |
| POST | `/shelf/move_batch` | reorder shelf positions | script-mining proof | dry-run only |
| POST | `/shelf/move_to_position` | move one shelf/book position | script-mining proof | dry-run only |
| POST | `/review/update/:book_id` | inline review/date/note update | script-mining proof | dry-run only |

## Shelf Routes

The bookshelf UI accepts numeric and slugged user forms:

```text
/review/list/100000000
/review/list/100000000-example-user
```

Observed shelf query values for the current logged-in account:

```text
#ALL#
to-read
currently-reading
read
did-not-finish
for-the-aesthetic
want-to-read-again
```

These are account inventory values, not a global enum. The CLI should discover shelves from `/review/list/:user` before listing or moving books. Common shelves can be aliases, but the discovered sidebar/form inventory is the source of truth.

The first parser target should be `/review/list/:user?shelf=<shelf>`, because it is central to books, reviews, export, and tooltip resource ids.

Public RSS fallback:

```text
/review/list_rss/100000000?shelf=read
```

The RSS route returned `200 application/xml` without browser cookies in a live probe. It includes structured book/user fields and is a good public fallback, but it is not a full authenticated shelf export. The logged-in page can expose a private `key=` RSS URL in `link rel="alternate"`; that value must stay redacted and local.

Important cap: public shelf RSS appears limited to 100 items. A live count returned 40 RSS `<item>` entries for the 40-book `read` shelf and 100 entries for the 132-book `to-read` shelf. Use RSS as the first public fallback, but use authenticated HTML/table parsing for full large-shelf export.

Authenticated shelf HTML pagination must also be followed. Fixture parsing proved `read` spans 2 pages and `to-read` spans 5 pages for this account; unique parsed review ids matched the declared shelf counts after pagination.

## Shelf Move/Add Map

Current shelf names from live authenticated table capture:

```text
to-read
currently-reading
read
did-not-finish
for-the-aesthetic
want-to-read-again
```

Batch move/add/remove surface:

```text
POST /review/update_list/100000000
```

Important fields:

```text
authenticity_token
view=table
edit[shelf]=<shelf-slug>
reviews[<review_id>]=<review_id>
```

Single-book shelf chooser:

```text
POST /shelf/add_to_shelf
parameters: book_id, name, a
```

`a=remove` is used by `ShelfChooser.submitShelf()` for removal. The Want-to-Read button helper also switches to `/shelf/remove_book` when toggling off.

Safety contract:

- Default to dry-run.
- Resolve book/review id and target shelf before showing the write plan.
- Require explicit approval/`--execute` before POSTing.
- Reload `/review/list/:user?shelf=<target>` and verify the book moved.

## Notes/Highlights Writes

Approved write endpoint:

```text
PUT /notes/:book_id/share
```

Observed successful book ids:

```text
218134959
48526390
56798941
19269993
```

One pasted page was already fully visible:

```text
220329192
```

Safety contract:

- Default to dry-run.
- Execute only with explicit user approval or `--execute`.
- Do not add shelves automatically if the shelf gate appears.
- Reload after the PUT and verify `.js-readingNote[data-visible=true]` equals total count.
- Never log raw Kindle highlight text by default.

## Parser Priority

1. Shelves/books: `/review/list/:user` plus `/tooltips`.
2. Public fallback reads: `/review/list_rss/:user`, `/opensearch.xml`, `/book/show/:book_slug` JSON-LD and `__NEXT_DATA__`.
3. Notes/highlights: `/notes/:user_slug`, `/notes/:user_id/load_more`, `/notes/:book_slug/:user_slug`.
4. Profile/social: `/user/show/:user_slug`, `/user/delayable_user_show/:user_id`, `/friend`, `/friend/requests`, `/notifications`.
5. Quotes/comments: `/quotes/list`, `/quotes/widget/:user_slug`, `/comment/list/:user_slug`.
6. Discovery: `/recommendations`, `/genres`, `/genres/list`, `/shelf`, `/list/show/:list_id`, `/user/top_readers`, `/user/best_reviewers`, `/book/show/:book_slug`.

## Open Questions

- Exact payload for `POST /notes/:book_id/:annotation_pair_id/note` needs an approved write capture.
- Spoiler checkbox route/method is still unsubmitted.
- Delete routes are intentionally unmapped beyond DOM class names.
- `/review/import` may expose Goodreads' own export flow and should be checked before implementing a custom bulk exporter.
- Mobile API surfaces remain unverified.
- Legacy XML API routes (`/review/list.xml`, `/shelf/list.xml`) still respond but require old API credentials; do not build around them unless a user supplies a working old key/OAuth token.
