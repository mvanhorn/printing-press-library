# Goodreads Dynamic Collection Model

Generated: 2026-05-22

## Rule

Do not hardcode the observed account's current shelves, friend sections, message folders, or list pages as the Goodreads model.

Goodreads exposes a mix of:

- global route templates, such as `/review/list/:user?shelf=:shelf`;
- account-specific collections, such as custom shelf names/counts;
- page-specific pagination, such as `/review/list/:user?page=2&shelf=read`;
- optional modules, such as friends, messages, Kindle notes, recommendations, and Amazon purchases.

The CLI should discover the account/page inventory first, then paginate only the collections that prove they need pagination.

This matters for shelves, friends, lists, messages, notes, and profile modules. The observed Goodreads account state is just one observed account state, not the product model.

## Shelf Inventory

`to-read`, `currently-reading`, and `read` are common Goodreads shelves, but the current account also has shelves such as `did-not-finish`, `for-the-aesthetic`, and `want-to-read-again`. Another account can differ.

Implementation rule:

1. Load `/review/list/:user`.
2. Extract shelf links from the page/sidebar.
3. Treat the extracted `shelf=<slug>` values and counts as this account's shelf inventory.
4. Offer common shelf aliases as convenience only after inventory discovery.
5. When moving books, require the target shelf to exist in the discovered inventory or show a separate shelf-create write plan.

## Pagination

The main bookshelf pages matter most for pagination because they contain the user's actual library.

Observed in the sanitized account capture:

- `read`: declared 40 books; HTML table pages 1-2 yielded 40 unique review ids.
- `to-read`: declared 132 books; HTML table pages 1-5 yielded 132 unique review ids.
- public RSS for `to-read` returned only 100 items, so RSS is not enough for complete large-shelf export.

Implementation rule:

1. Parse `#reviewPagination` links from the first shelf page.
2. Follow each numbered page once.
3. Deduplicate by `review_id` first, then `book_id`.
4. Compare parsed unique count against the declared shelf count when available.
5. Mark export incomplete if the counts do not match.

Subpages like message folders, people discovery tabs, genre indexes, and notes detail pages should still check for pagination or load-more controls, but they do not need exhaustive crawling by default unless the user asks for a full export of that collection.

## Social, Lists, Messages, And Profile Modules

Friends, friend requests, message folders, comments, quotes, profile modules, Listopia pages, reading challenge/year-in-books pages, and recommendation pages are not guaranteed to be the same across accounts or dates.

Implementation rule:

1. Start from the current account/profile nav and extract links/forms actually present.
2. Classify each discovered route as account-owned, public profile, global discovery, or write/action.
3. Treat message folders and friend/list tabs as discovered UI inventory, not hardcoded constants.
4. For each collection, record the count/pagination/load-more controls if present.
5. Paginate account-owned export surfaces by default only when they are the primary dataset, such as bookshelves. For social/discovery subpages, expose pagination but require an explicit full-export mode before exhaustive crawling.

## Printing Press / CLI Shape

Recommended read commands:

```text
goodreads shelves discover
goodreads books list --shelf <slug> --paginate auto
goodreads books export --all-shelves --paginate auto
goodreads messages folders
goodreads messages list --folder inbox --paginate auto
goodreads notes list --paginate auto
goodreads friends list --paginate auto
```

Recommended write-plan commands:

```text
goodreads books move --book-id <id> --to-shelf <slug> --dry-run
goodreads shelves create --name <name> --dry-run
```

Writes remain disabled or dry-run until an approved write-capture pass exists for that exact route.
