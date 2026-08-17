# Open Library CLI

`open-library-pp-cli` is a read-only Printing Press CLI for Open Library book, author, edition, work, and subject metadata. It returns compact `--agent` JSON for bibliography and discovery workflows while preserving Open Library source URLs, rate-limit posture, and non-bulk caveats.

## Sources

- Open Library API index: public JSON/YAML/RDF APIs for book discovery and lookup.
- Book Search API: preferred book/work search endpoint at `https://openlibrary.org/search.json`.
- Books API: work, edition, ISBN, and legacy book lookup behavior.
- Authors API: author search, individual author JSON, and bounded works lists.
- Subjects API: subject work lists and optional facets; documented as experimental.

## Setup

No API key is required.

For regular or frequent use, Open Library asks clients to identify requests with a `User-Agent` and contact email:

```bash
export OPEN_LIBRARY_USER_AGENT="my-research-tool"
export OPEN_LIBRARY_CONTACT_EMAIL="you@example.org"
```

Without a contact email, keep usage near Open Library's documented non-identified posture of 1 request per second. With a contact-bearing request identity, Open Library documents 3 requests per second.

Direct Go installs require a Go toolchain compatible with the module's `go 1.26.4` declaration.

## Commands

```bash
open-library-pp-cli book "Designing Data-Intensive Applications" --agent
open-library-pp-cli isbn 9780131103627 --agent
open-library-pp-cli author "Ursula K. Le Guin" --limit 5 --agent
open-library-pp-cli work OL45804W --agent
open-library-pp-cli editions OL45804W --limit 10 --agent
open-library-pp-cli subjects "distributed systems" --details --agent
open-library-pp-cli sources --agent
open-library-pp-cli doctor --agent
```

## Caveats

- Do not use this CLI for borrowing, waitlists, patron data, account actions, or catalog edits.
- Do not scrape Open Library HTML pages; this CLI uses JSON endpoints.
- Do not use the API as a bulk backend. Use Open Library's monthly data dumps for bulk access.
- Subject output comes from an API documented as experimental.
- Metadata can vary across works and editions; cite returned Open Library source URLs.
