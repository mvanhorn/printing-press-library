# Build Log: pdok-location-pp-cli

## What was built (Phase 2 / generation)

- Multi-spec generate succeeded: `--spec locatieserver.json --spec kadaster-location-api.json --name pdok-location`.
- Generator emitted 36 absorbed commands (4 Locatieserver endpoints + 14 OGC collection sub-commands + framework commands + `/search` + `/conformance`).
- Generator emitted Highlights block in `pdok-location-pp-cli --help` listing all 11 novel features from `research.json`. Cobra commands for those 11 do NOT exist yet — they are Phase 3 work.
- Binary builds cleanly.

## What was intentionally deferred

- All 11 novel-feature Cobra commands. They were listed in research.json so they render in README/SKILL/root-help, but actual implementation is Phase 3.

## Skipped body fields

- None. Both PDOK specs are GET-only.

## Fixes applied during generation

- `promoted_free.go` and `promoted_suggest.go` had a default `--bq` value of a
  JSON-encoded Solr boost-array string. Solr's parser rejects that with a
  400 SyntaxError. Patched both files to default `--bq` to empty so the
  server-side default applies. (RETRO CANDIDATE: array-typed Solr params
  with defaults serialize as JSON literals; should be omitted-when-default
  or sent as repeated query params.)

## Generator warnings (non-blocking)

- `warning: filtered global query param "fl" / "fq" / "wt"` — these are
  filtered from generated commands because they apply to all endpoints. We
  can still pass them via `--extra-param` or by hand-wiring in Phase 3.
- `warning: skipping GET "/": could not derive resource name` — OGC landing
  page; not needed as a CLI command.
- `warning: resource "api" / "search" would shadow framework cobra command;
  renamed to "pdok-location-api" / "pdok-location-search"` — known generator
  rename behavior; will be cleaned up with a friendly `ogc-search` alias in
  Phase 3.

## Quality-gate failure (non-blocking)

- `govulncheck ./...` failed with a Go toolchain mismatch (govulncheck wants
  go 1.25; generated code uses go 1.26 features). The Go toolchain itself
  compiles the code fine. (RETRO CANDIDATE: govulncheck quality gate breaks
  on go 1.26 codebases.)

## Verification of absorbed commands (real API calls)

- `free --q Amsterdam --rows 1` → 200, returns gemeente Amsterdam.
- `suggest --q Damrak --rows 2` → 200, returns weg, weg, adres.
- `reverse --lat 52.3731 --lon 4.8922 --type adres --rows 2` → 200, returns
  Dam 20 + Dam 4.
- `conformance` → 200, returns OGC conformance class URIs.
- `collections get` → 200, returns Location API collection list.
- `pdok-location-search --q Damrak --limit 2` → **400: requires per-collection
  opt-in via `?adres[version]=1` syntax**. Will be wrapped by a friendly
  `ogc-search` command in Phase 3 that defaults to all collections.

## Phase 3 plan

Hand-code these 11 novel commands per the absorb manifest:
1. `resolve <text> [--geojson]` — suggest→lookup chain
2. `batch geocode <file.csv> --address-col street [--out result.csv]`
3. `nearest --lat <> --lon <>` (or `--rd-x --rd-y`)
4. `convert rd-to-ll <x> <y>` / `convert ll-to-rd <lon> <lat>`
5. `convert wkt-to-geojson <wkt>` / `convert geojson-to-wkt <stdin>`
6. `gemeente get <name>` / `gemeente list --provincie <name>` / `provincie list`
7. `search <text> [--type adres,gemeente,...]`
8. `features in-bbox --bbox <x1,y1,x2,y2> --collections adres,perceel`
9. `top <query> --min-score <n> [--require-type adres]`
10. `perceel lookup --aanduiding "AMR03 N 1234"`
11. `gemeente of-point --lat <> --lon <>`

Plus a friendly `ogc-search` wrapper to fix the per-collection-opt-in quirk.
