# book-goat-pp-cli Absorb Manifest

Landscape: reading trackers (libro, goodreads-cli) log what you read; ebook CLIs (bookcut, pgberg) fetch files. book-goat matches their search/shelf/read surface AND adds the art-goat *contemplative practice* loop none of them has: a daily pick you sit with and journal.

## Absorbed (match the table-stakes surface)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search books | libro / bookcut | `search <q>` (federated Open Library + Gutendex) | flags which results have free full text; `--json`/`--select` |
| 2 | Book detail | goodreads-cli | `show <work>` | Open Library work metadata + subjects |
| 3 | Track shelves (want/reading/read) | libro / goodreads-cli | `shelf add <work> --to <want\|reading\|read>` / `shelf list` | local SQLite, agent-native |
| 4 | Log + rate a read | libro | `log <work> --status --rating` | folds into stats + next |
| 5 | Read free full text | bookcut / pgberg | `passage <gutenberg-id>` | pulls a real Gutendex plain-text excerpt |

## Transcendence (the contemplative practice — only a local reading-practice model enables)
| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|--------------------------|
| 1 | Opinionated daily pick — a public-domain book/passage to read today, rotated against your recent sits | `today` | Requires anti-repeat against your local sit history; no catalog API "picks for you" |
| 2 | Sit with a real passage + capture a reflection | `sit <gutenberg-id>` | Requires fetching a real Gutenberg text, excerpting it, and persisting a reflection locally |
| 3 | Your reflections over time | `journal` | Requires the local reflections table — the journal is yours, not any API's |
| 4 | What to read next, from your shelf + log | `next` | Requires ranking your want-shelf against your reading history locally |
| 5 | Reading stats — pace, top subjects, ratings | `stats` | Requires aggregating your local reading_log; no single API returns it |

Honest edges:
- **Gutendex full text is a second fetch to gutenberg.org** (300KB–1MB); `sit`/`passage` stream and slice an excerpt, they don't cache whole books.
- **`today` picks from the public-domain (Gutendex) pool** so it can always serve a real passage to sit with — not the entire Open Library catalog.
- Sources need a real `User-Agent` (set) and are rate-limited (~1 req/s) — the client backs off.

## Deferred (fast-follows, not stubs in the surface)
- Google Books description enrichment · Wikiquote quotes (unstructured wikitext, dropped) · Goodreads CSV import · `path --theme` walk.
