# subject-pp-cli — shipcheck

**Verdict: ship.** 7/7 shipcheck legs PASS · live dogfood 16/16 (quick) · 12 Go test packages green.

## The bug that justified the whole print

`route` returned zero water towers along a Los Angeles → Palm Springs corridor while a manual query with the identical bounding box returned eight. The cause: failover had reached **overpass.osm.ch**, which answers quickly and successfully but hosts a **Swiss regional extract**. It returned 0 results for California and 1 for a Swiss bounding box, verified directly.

A regional mirror in a failover list is more dangerous than a dead one: a dead host produces an error, while a regional host produces an empty result indistinguishable from "there is nothing there." It was removed from the defaults and a test now guards the list against known regional extracts.

## Other bugs found and fixed

1. **`nwr` vs nodes only** — a naive node-only query near Los Angeles found 3 water towers; querying nodes, ways and relations with `out center` found **27**. Ways and relations carry most large structures, and their coordinates live in a `center` object rather than `lat`/`lon`, so reading only `lat`/`lon` drops them silently.
2. **Overload disguised as success** — Overpass returns HTTP **200** with an HTML error page when busy. A status-code check alone treats that as a valid response; the runner inspects the body and extracts the human-readable reason.
3. **Failover starved by a shared deadline** — the whole operation inherited one `--timeout`, so the first slow mirror consumed the entire budget and the rest were never tried. Each mirror now gets its own, capped at 45 s so three mirrors cannot become a four-minute hang.
4. **`geojson --out --json` printed prose** — a confirmation sentence on stdout instead of JSON, breaking any caller parsing it.
5. **Deleted the framework's own `export.go`** while clearing scaffolds, and the novel GeoJSON command collided with the framework's data-export command by name. Renamed to `geojson`.

## Honesty properties worth preserving

- `route` builds a **rectangle around the straight line**, not a buffer around a road. The output says so and reports each result's distance off the line — one result in the verified run sits 12 km off, which is exactly what that column is for.
- Subject types with sparse OpenStreetMap coverage (`brutalist`, `art_deco`) carry a note saying so, printed with every result; an empty architectural-style search usually means thin tagging, not an empty city.
- `types` prints the exact OSM tag selectors behind every subject, so the mapping that is the product is inspectable rather than magic.
- An unknown `--type` fails with the full list rather than guessing.
