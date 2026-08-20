# Crestron CLI Build Log

Manifest transcendence rows: 6 planned, 0 built. Phase 3 will not pass until all 6 ship.

(1 of the 6 is `spec-emits` — `search --type firmware_release` — and needs the
firmware_release resource synced with notes rather than new Cobra wiring.
The other 5 are `hand-code`.)

## Findings before writing code
- Generated `html_extract: mode: page` returns page chrome (nav banners,
  "Learn More" links), not Crestron's result markup. Every HTML endpoint needs a
  domain parser.

## Priority 0 — parser foundation (DONE)
`internal/crestronparse` parses every Crestron surface. 15 table-driven tests
run against fixtures captured live from www.crestron.com during discovery, so
the tests fail if Crestron changes its markup rather than silently returning
empty results.

| Parser | Keys off | Test coverage |
|---|---|---|
| `ParseSearchResults` | `div.search-result` + `.resource-search-{name,type,date}` | firmware rows, spec-sheet rows, empty-page negative |
| `ParseProductTiles` | `div.product-result`, `p.model-number`, `span#productCount` | model/URL presence, count consistency |
| `ParseProductPage` | schema.org JSON-LD + `data-id` | model, sku, brand, description, document id |
| `ParseSpecTable` | `td.productSpecTDHead` + 2-cell rows | section/row completeness, ≥20 rows |
| `ParseCategoryPage` | inline `var request` + option counts | dId/nId presence, count suffix stripped |
| `ParseAssets` | `/getmedia/` anchors | dedup of paired Download/PDF anchors, ≥2 kinds |
| `ParseFirmwareRelease` | text-extracted labels + `/firmware_files/` href | version, download URL, sign-in detection |
| `ParseCatalogPaths` | `/sitemap` anchors | ≥20 unique catalog paths |
| `SplitReleaseTitle` | slash/underscore model list + trailing version | 4 real titles incl. a 7-model family |
| `ExpandModelFamily` | abbreviated trailing segments | `DM-NVX-D10/D20/E10` expansion |

Notable fixes found by the tests:
- Version and Last Modified labels are wrapped in `<b>` on the live page, so
  those regexes had to run against extracted text, not raw markup.
- Firmware pages also link a standalone `/release_notes/*.pdf`; captured as
  `ReleaseNotesURL`.
- Literal zero-width/nbsp characters in source broke the Go build; replaced with
  `\u` escapes.

## Priority 1 — absorbed features wired to real parsing (DONE)
`internal/cli/crestron_extract.go` dispatches each response to the parser that
understands that page, falling back to the generic extractor for anything
unrecognized. The 12 generated endpoint commands each had one call swapped from
`extractHTMLResponse(data, opts)` to `crestronExtract(data, path, opts)`.

### Live verification after wiring
| Command | Result |
|---|---|
| `resource --query DM-NVX-385 --category 5` | 1 spec sheet with title/type/date/getmedia URL |
| `product page .../DM-NVX-360` | model, sku, brand, doc id 21965, 10 spec sections |
| `product resources --document-id 21965` | 20 assets, classified (cad/revit/guide-spec/...) |
| `product variants --series "DM-NVX-35 Series"` | 10 member models with internal ids |
| `catalog tree` | 79 category paths |
| `catalog category AV-Over-IP/DM-NVX-AV-Over-IP` | dId 30989, nId 31385, count 42, 10 subcategories |
| `catalog products --document-id 30989 --node-id 31385` | total 42, tiles parsed |

### Three bugs found and fixed by live testing (not by the build)
1. **Wrong document id.** `data-id` is used by several unrelated widgets; an
   embedded video modal's Vimeo id (`1059000133`) was being picked instead of the
   product's own (`21965`). A wrong id silently returns another product's assets.
   Fixed by preferring the favorite/support/add-to-project containers with a
   most-frequent fallback. Regression test added.
2. **Category pages parsed as products.** Both page types share the
   `/Products/Catalog/` prefix AND both embed schema.org JSON-LD of type
   `Product` — a category's JSON-LD names the category — so JSON-LD presence
   cannot disambiguate them. Fixed with `IsCategoryPage`, which tests for the
   inline request block. Regression test added.
3. **`os=0` dropped from the product-tile query.** Crestron requires `os` on
   every request including the first page, but the generator omits zero-valued
   int params, so the endpoint returned zero products. Verified by bisecting the
   query string against the live endpoint. Fixed in the spec (offset typed as
   string) and in the generated command (sent unconditionally).

Bug 3 is a **Printing Press retro candidate**: a semantically-required int
param whose valid value is 0 is silently dropped from the query.

### Regen note
The 12 dispatcher swaps and the `os` fix are edits inside generator-emitted
files; `regen-merge` will report them as reviewable drift. The parsers
themselves live in `internal/crestronparse` and are untouched by regeneration.
A generator-side post-extract hook would remove the need for these edits.

## Scope change approved mid-build: the local mirror
Novel row 2 (`search --type firmware_release`) was approved as `spec-emits`,
assuming the framework's sync + FTS. **That assumption was wrong.** The generator
emits `sync`, `sql`, `stale`, and framework `search` only for JSON endpoints, and
all 12 Crestron endpoints are `response_format: html`, so none of those commands
were generated. The local store package existed with nothing to populate it.

Rather than silently downgrade an approved feature, this was returned to the
user per the skill's re-approval rule. The user chose **build the sync**, which
also makes the README/SKILL "offline" and "local SQLite mirror" claims true.

Row 2's buildability therefore changes from `spec-emits` to `hand-code`, making
the hand-code commitment **6 of 6** rather than 5 of 6.

This is a **Printing Press retro candidate**: an all-HTML spec silently yields no
sync/search/store surface, and nothing warns the author at generate time.

## Priority 2 — transcendence features
Manifest transcendence rows: 6 planned, 6 built.

### Local mirror (new, hand-authored)
- `internal/crestronstore` — owns `crestron_products`, `crestron_categories`,
  `crestron_releases`, `crestron_release_models`, plus two FTS5 indexes.
  Self-migrating and idempotent; never writes through the generated store
  helpers so regeneration cannot clobber it. 8 tests.
- `sync` command — crawls `/sitemap` → categories → tile endpoint → products,
  and pages the firmware search into releases with their covered-model list
  expanded. `--notes` pulls release notes when signed in.
  **Live result: 69 categories, and 548 releases in 30s / 23 requests on a
  4-category smoke run.**

### Feature status
| # | Feature | Command | Live-verified |
|---|---|---|---|
| 1 | Fleet firmware status | `fleet status` | yes — resolved TSW-1070 to a release whose title and URL contain no "TSW-1070" |
| 2 | Release-note search | `search --type firmware_release` | yes — FTS over synced releases, honest empty-result notes |
| 3 | Release-note diff | `firmware diff` | built; needs two synced versions to exercise |
| 4 | Submittal builder | `submittal` | yes — 8 assets for DM-NVX-360 with a missing-class report |
| 5 | Lifecycle trace | `lifecycle` | built; End-of-Sale detection needs the mirror (see below) |
| 6 | Spec comparison | `specs compare` | built; needs the mirror for product-page resolution |

### Bugs found and fixed by live testing
4. **FTS5 rejected hyphenated model numbers.** `DM-NVX` parses as an FTS5
   expression where `-` is NOT and `NVX` is read as a column name, failing with
   "no such column: NVX". Fixed by quoting each term into a literal phrase.
   Regression test added.
5. **`crestronstore` did not import its SQLite driver.** It worked at runtime
   only because the `cli` package imported it transitively; the package's own
   test binary failed with "unknown driver". The package now owns the import.
6. **`submittal` and `lifecycle` had no live fallback.** Both needed a document
   id that only the mirror supplies. Added `assetsForModel`, which prefers the
   per-product handler and falls back to a cross-category resource search.
   `submittal` now works with no mirror at all.

### Known behaviour, not a bug
`lifecycle` cannot detect every End-of-Sale notice without a synced mirror: a
notice is often titled for the family (UC-FCM-Z's is "End-of-Sale Notice:
FlexCarts"), so it is reachable only through the product's own document id.
The command reports this in its `note` field rather than implying the model is
current.

7. **`sync` killed itself on a full crawl.** The root `--timeout` is documented
   and defaulted (60s) as a *per-request* timeout, and the generated client
   already applies it to every call — but the command also passed it to
   `boundCtx`, which bounded the entire multi-minute crawl. A full 69-category
   sync died with "writing products: context deadline exceeded" after ~4 minutes
   of successful crawling, discarding everything. Fixed by using `cmd.Context()`
   directly plus a separate `--max-duration` crawl budget (30m default), and by
   writing partial results through a fresh context so an exhausted budget saves
   what was gathered instead of throwing it away.

   This is a second **Printing Press retro candidate**: `boundCtx` is documented
   for "hand-written commands that call sibling typed clients", and applying it
   is the guidance in the skill — but for a long-running crawl it converts a
   per-request timeout into a whole-command deadline. The skill should warn that
   crawl-shaped commands need their own budget flag.

## Final state
Manifest transcendence rows: 6 planned, **6 built**. No stubs shipped.

Phase 5 live acceptance: **PASS, 111/111**.
Shipcheck: **PASS, 7/7 legs**. Sample output probe **6/6**.
Scorecard 83/100 Grade A. `go vet` clean. 16 test packages pass.

### Exit-code semantics settled during acceptance
Dogfood's error-path probe and the scorecard's sample probe pulled in opposite
directions until the two failure classes were separated:

- **Model unknown to Crestron** (bad input) → non-zero exit.
- **Model exists but the catalog is not synced** → exit 0, empty result, and a
  `run: crestron-pp-cli sync ...` hint on stderr.
- **Valid search with no matches** → exit 0 with a note. `search` carries
  `pp:no-error-path-probe` because an unmatched term is a legitimate empty
  result, not a usage error.

Both probes pass under these semantics, and they are the semantics a script
actually wants.

### Incidental proof that `submittal --out` works
The scorecard sample probe ran `submittal ... --out ./submittal` inside the
working tree and left 12 real PDFs totalling 9 MB across two models —
CAD drawings, CSI Guide Specs, spec sheets, product information, and an NRTL
safety certificate. The download path is therefore verified end-to-end against
live Crestron assets.

That output directory was promoted to the library by mistake and has been
removed; `submittal/` is now in `.gitignore` so a probe run cannot leak
downloads into a published CLI again.
