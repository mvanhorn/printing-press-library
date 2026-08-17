# book-goat CLI Brief

## API Identity
- Domain: open book APIs — Open Library (metadata spine), Gutendex/Project Gutenberg (full public-domain texts), Google Books (optional enrichment).
- Users: a reader who wants a daily reading *practice* — one book/passage to sit with, a reflection journal, a shelf — grounded in real public-domain texts, kept local.
- Data profile: works, authors, public-domain texts; personal reading log, shelf, reflections.

## Reachability Risk
- None. All keyless. Open Library + Gutendex + Google Books respond to programmatic GETs (need a real `User-Agent`; Gutendex/OL rate-limit anonymous ~1 req/s).

## Top Workflows
1. "Give me something to read today" → an opinionated public-domain pick.
2. "Sit with a passage and capture a reflection" → the contemplative core.
3. "Find a book" → federated search (OL metadata + which have free full text).
4. "Track what I'm reading / want to read" → shelf + log.
5. "Read back my reflections / what to read next."

## Table Stakes (from libro, bookcut, goodreads-cli, pgberg)
- search books (libro/bookcut) · shelves add/list (libro/goodreads-cli) · review+rate (libro) · fetch free ebook text (bookcut/pgberg).

## Data Layer
- Corpus: books (OL works + Gutendex texts), authors — synced to SQLite, FTS search.
- Personal: reading_log (status/rating/dates), shelf, reflections (the journal). These are the soul; they compound.

## Sources & Auth (all keyless)
| Source | Base | Role | Notes |
|---|---|---|---|
| Open Library | openlibrary.org | search + work/author metadata | `/search.json?q=&fields=&limit=`; needs `User-Agent`; ~1-3 req/s |
| Gutendex | gutendex.com | **public-domain full texts** | `/books?search=&mime_type=text/plain`; `formats["text/plain; charset=utf-8"]` → a gutenberg.org .txt (2nd fetch, 300KB-1MB, stream+slice) |
| Google Books | googleapis.com/books/v1 | optional enrichment (descriptions) | `?q=&country=US`; anonymous OK, `GOOGLE_BOOKS_API_KEY` optional |
| ~~Wikiquote~~ | — | **dropped v1** | wikitext quotes are unstructured markup — not worth the parsing misses |

## Product Thesis
- Name: `book-goat` — a contemplative daily reading practice.
- Why it should exist: reading trackers (libro, Goodreads) log what you read; ebook CLIs (bookcut, pgberg) fetch files. None make a *daily practice* out of real public-domain texts — a `today` pick you `sit` with and journal. That's the art-goat contemplative loop for books.

## Build Priorities
1. Data layer + OL substrate (generated) + Gutendex source adapter.
2. Absorbed: `search` (federated), `show`, `shelf`, `passage`.
3. Transcendence (the soul): `today`, `sit`, `journal`, `next`, `stats`.

## Deferred (honest, fast-follows)
- Google Books enrichment (descriptions), Wikiquote quotes, Goodreads CSV import, `path --theme`.
