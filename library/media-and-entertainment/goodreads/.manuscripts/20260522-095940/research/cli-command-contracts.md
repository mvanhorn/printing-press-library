# Goodreads CLI Command Contracts

Generated: 2026-05-22

## Contract Principles

- Discover account inventory first; never hardcode one user's shelves, folders, friends, lists, or page counts.
- Treat shelves, friends, lists, messages, profile modules, and recommendation sections as account/page inventory because they vary by user and over time.
- Default to read-only commands.
- Treat RSS, HTML, JSON-LD, and `__NEXT_DATA__` as separate source lanes with confidence labels.
- Keep account-visible raw fixtures local/private.
- Make every write command a dry-run plan unless an approved write-capture exists for that exact route.

## Shared Output Fields

Every command should include:

```json
{
  "source": "goodreads-web",
  "accountUserId": "100000000",
  "accountUserSlug": "100000000-example-user",
  "generatedAt": "ISO-8601",
  "confidence": "high|medium|low",
  "warnings": []
}
```

For paginated collections:

```json
{
  "pagination": {
    "mode": "auto",
    "pagesSeen": [1, 2],
    "declaredCount": 40,
    "parsedCount": 40,
    "complete": true
  }
}
```

## `shelves discover`

Purpose: build the account-specific shelf inventory.

Primary source:

```text
GET /review/list/:user
```

Parser evidence:

```text
goodreads/fixtures/parsed/shelf-to-read.parsed.json -> shelfInventory
```

Output shape:

```json
{
  "shelves": [
    {
      "slug": "to-read",
      "displayName": "Want to Read",
      "count": 132,
      "href": "/review/list/100000000-example-user?shelf=to-read",
      "kind": "account_shelf",
      "isObservedForThisAccount": true
    }
  ]
}
```

Rules:

- `to-read`, `currently-reading`, and `read` can be convenience aliases, but only after they appear in discovered inventory.
- Custom shelves such as `for-the-aesthetic` are first-class.
- `#ALL#` is an account all-books view, not a normal shelf write target.

## `books list --shelf <slug> --paginate auto`

Purpose: list books/reviews on one discovered shelf.

Primary source:

```text
GET /review/list/:user?shelf=:shelf
```

Fallback source:

```text
GET /review/list_rss/:user?shelf=:shelf
```

Rules:

- Use authenticated HTML for full export.
- Use RSS as public fallback and mark `complete=false` when item count is exactly 100 or below discovered shelf count.
- Follow `#reviewPagination` page links until all numbered pages have been parsed once.
- Deduplicate by `review_id` first, then `book_id`.
- Compare unique parsed review ids against discovered/declared shelf count.

Fixture proof:

```text
read: pages 1-2 -> 40 unique review ids, declared 40
to-read: pages 1-5 -> 132 unique review ids, declared 132
to-read RSS: 100 items, incomplete for full export
```

## `books export --all-shelves --paginate auto`

Purpose: export the account library.

Flow:

1. Run `shelves discover`.
2. For each discovered shelf except `#ALL#`, run `books list --shelf <slug> --paginate auto`.
3. Merge by `book_id` and retain all shelf memberships.
4. Report per-shelf completeness.

Output should include:

```json
{
  "books": [],
  "shelves": [],
  "perShelf": [
    {
      "shelf": "read",
      "complete": true,
      "parsedCount": 40,
      "declaredCount": 40
    }
  ]
}
```

## `book show <slug-or-id>`

Purpose: normalize one public book page.

Primary source:

```text
GET /book/show/:book_slug
```

Rules:

- Extract JSON-LD first.
- Extract `__NEXT_DATA__` only into normalized fields.
- Do not store raw public review bodies by default.

Fixture proof:

```text
goodreads/fixtures/parsed/book-gate-of-the-feral-gods.parsed.json
```

## `notes list --paginate auto`

Purpose: list Kindle notes/highlights metadata without raw highlight text.

Primary sources:

```text
GET /notes/:user_slug
GET /notes/:user_id/load_more
```

Rules:

- Emit book-level note/highlight metadata.
- Do not emit raw highlight text by default.
- Detect load-more/pagination controls and follow them only for full export mode.

## `notes show --book <book_slug>`

Purpose: inspect visibility/state for a book's notes.

Primary source:

```text
GET /notes/:book_slug/:user_slug
```

Fixture proof:

```text
notes-mr-whisper: 12 notes, spoiler toggles present, no raw highlight text emitted
```

## `messages folders`

Purpose: discover account message folders.

Primary source:

```text
GET /message/inbox
```

Observed folder routes:

```text
/message/inbox
/message/saved
/message/sent
/message/trash
```

Rules:

- Treat folder names as discovered account UI inventory.
- Do not mark messages read as part of discovery.

## `messages list --folder <folder> --paginate auto`

Purpose: list message metadata.

Primary source:

```text
GET /message/:folder
```

Rules:

- Emit message ids and structural metadata only by default.
- Do not emit sender labels, subjects, or bodies unless an explicit user-owned export mode is added.
- Check for pagination but do not exhaustively crawl unless requested.

## `friends list --paginate auto`

Purpose: list account-visible friend/following sections without assuming every account has the same tabs.

Primary source:

```text
GET /friend
GET /friend/requests
```

Rules:

- Discover friend-related tabs/sections from the current page before listing.
- Check for pagination or next links but do not exhaustively crawl unless full export is requested.
- Emit profile ids, profile hrefs, and relationship/action metadata only by default.

## `lists discover` / `lists show <id>`

Purpose: handle Listopia/global lists and account-visible list links separately.

Primary sources:

```text
GET /list
GET /list/show/:id
```

Rules:

- Treat global popular lists and account-specific list links as different inventories.
- Discover list ids, names, counts, and pagination from the current page.
- Do not vote/add/remove books unless a write plan is explicitly approved.

## `profile inventory`

Purpose: map the current user's profile modules and links without hardcoding them.

Primary source:

```text
GET /user/show/:user
```

Rules:

- Extract links/modules that actually appear: shelves, notes/highlights, quotes, comments, friends, groups, discussions, reading challenge, year in books, favorite genres, and account settings.
- Mark missing modules as absent for this account/page, not as unsupported globally.
- Keep module inventory separate from collection exports.

## Write-Plan Commands

These commands should default to dry-run:

```text
books move --book-id <id> --to-shelf <slug> --dry-run
shelves create --name <name> --dry-run
notes publicize --book-id <id> --dry-run
messages move --message-id <id> --folder saved --dry-run
messages mark-read --message-id <id> --dry-run
```

Dry-run output must include:

- route;
- method;
- form/page source;
- CSRF requirement;
- target ids;
- verification URL;
- explicit warning that `--execute` mutates the account.

Only `PUT /notes/:book_id/share` has an approved write proof in this run, and only for the pasted note pages.
