# Flight GOAT Printed CLI Agent Guide

This directory is a generated `flight-goat-pp-cli` printed CLI. It was produced by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), so treat systemic fixes as upstream Printing Press fixes first. Keep local edits narrow and document why a generated-tree patch belongs here.

## Local Operating Contract

Start by asking the generated CLI for current runtime truth:

```bash
flight-goat-pp-cli doctor --json
flight-goat-pp-cli agent-context --pretty
```

Use runtime discovery instead of relying on a copied command list:

```bash
flight-goat-pp-cli which "<capability>" --json
flight-goat-pp-cli <command> --help
```

Add `--agent` to command invocations for JSON, compact output, non-interactive defaults, no color, and confirmation-safe scripting:

```bash
flight-goat-pp-cli <command> --agent
```

Before running an unfamiliar command that may mutate remote state, inspect its help and prefer a dry run:

```bash
flight-goat-pp-cli <command> --help
flight-goat-pp-cli <command> --dry-run --agent
```

Use `--yes --no-input` only after the target, arguments, and side effects are clear.

For install, auth, examples, and longer product guidance, read `README.md` and `SKILL.md`. This file intentionally stays small so repo-local agents get invariant local guidance without duplicating the generated docs.

## Local Customizations

If you modify this CLI beyond what the generator produced, record each customization so it isn't lost on the next regen and is visible to the next reader.

1. **Mark every changed site** in source with a comment summarizing the deviation:

    ```go
    // PATCH: <one-line summary>
    ```

    Include an upstream reference inline when there is one (e.g. `// PATCH(upstream cli-printing-press#<issue>): ...`). `grep -rn 'PATCH' .` from this directory then surfaces every customization.

2. **Catalog the change** in a `.printing-press-patches.json` at this CLI's root (parallel to `.printing-press.json`). Minimum shape:

    ```json
    {
      "schema_version": 1,
      "applied_at": "YYYY-MM-DD",
      "base_run_id": "<copy from .printing-press.json>",
      "base_printing_press_version": "<copy from .printing-press.json>",
      "patches": [
        {
          "id": "short-identifier",
          "summary": "What changed (one sentence).",
          "reason": "Why this customization was needed (one or two sentences).",
          "files": ["internal/cli/foo.go"],
          "validated_outcome": "Optional: non-obvious test result that confirms the fix.",
          "upstream_issue": "Optional: https://github.com/mvanhorn/cli-printing-press/issues/<n>"
        }
      ]
    }
    ```

This file is an **index of customizations**, not a second copy of the diff. Diffs live in `git`; code lives in the source files; the inline `// PATCH:` comment carries the local semantics. Keep `summary` and `reason` short -- if you find yourself writing tables of field renames or code transformations, that detail belongs in the source comment or commit message, not here.

## Airport alias maintenance

When an IATA airport code is retired or replaced (a closure, a city splitting its traffic to a new field, an ICAO/IATA renumbering), `internal/gflights/airport_alias.go` carries the remap from the retired code to the current one. Google's `GetShoppingResults` silently returns empty for unknown codes, so without an entry the CLI returns zero results with no signal that anything is wrong.

When you add an entry:

1. Append it to the `airportAliases` map in `internal/gflights/airport_alias.go` and the `retiredAirportAliases` map in `internal/kayak/kayak.go` (they are intentionally separate to avoid an import cycle).
2. Update the block-level comment above `airportAliases` with the change date and the airport-name context.
3. Add a case to `internal/gflights/airport_alias_test.go` so the remap is locked in.

Verify by running the deprecated-code path live:

```bash
flight-goat-pp-cli flights SEA <retired-code> 2026-12-24 --return 2027-01-01 --agent
```

The response should populate `airport_remapped: {destination: {from: "<old>", to: "<new>"}}` and `query.destination` should still echo the user-supplied code unchanged.

## Airline URL maintenance

Each `Flight` result carries `booking_urls` with an optional airline-direct URL when the itinerary is operated end-to-end by a single carrier in the curated table at `internal/gflights/booking_urls.go` (`airlineTemplates` map). Source of truth for each entry is `internal/gflights/testdata/airline_url_captures.md`, which classifies each URL as:

- `prefill` — the carrier's booking form pre-fills route + dates from URL params. Verified by visiting the URL in a browser. Today: DL, WN, LH, LX.
- `landing` — the URL points at the carrier's booking entry; user may need to retype dates. All other carriers default to landing.

To upgrade a landing entry to prefill: visit the carrier's booking page in a browser, submit a sample search, and observe whether the resulting URL contains the origin/destination/date as params. If yes, generalize the URL pattern with `{origin}`, `{destination}`, `{depart}`, `{return}`, `{pax}` placeholders (plus carrier-specific `{trip_type_*}` if needed) and update both the `airlineTemplates` map and the captures markdown with the new pattern, date, and source.

Carrier-specific trip-type placeholder variants currently supported:

- `{trip_type}` — `OneWay` / `RoundTrip` (AA, AS)
- `{trip_type_int}` — `1` / `2` (UA)
- `{trip_type_dl}` — `ONE_WAY` / `ROUND_TRIP` (DL)
- `{trip_type_wn}` — `oneway` / `roundtrip` (WN)

When the carrier discriminator is one of these forms, reuse the existing placeholder. When it's something new (rare), add it to the replacer in `buildAirlineURL`.

## Parser fixture refresh

`internal/gflights/testdata/*.json` contains captured `GetShoppingResults` responses used as regression fixtures. Refresh them by re-running the build-tagged capture tests:

```bash
go test -tags capture -run TestCaptureSeaKtiResponse ./internal/gflights/...
go test -tags capture -run TestCaptureSeaBkkResponse ./internal/gflights/...
```

Fixtures are network-dependent and intentionally not run in normal CI. Recapture when Google materially changes the `GetShoppingResults` response shape and the regular parser tests start failing.
