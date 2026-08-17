# Placeit Novel-Features Brainstorm (subagent audit trail)

## Customer model

**Maya — Etsy/POD apparel seller (dominant persona).** 200-listing Etsy shop, POD tees/hoodies/totes via Printify.
- Today: hand-browses Placeit category by category, 30+ mockup tabs, copies stage links to a spreadsheet by hand.
- Weekly ritual: launches a design across 8-12 product mockups; needs consistent best-selling mockup styles.
- Frustration: no way to know which mockups sell well, no bulk export of deep links, Printify-compatible mockups aren't filterable in the UI.

**Devon — Twitch/YouTube streamer.** Rebranding their channel.
- Today: separately hunts overlay, panels, emotes, webcam frame — UI never groups them; ends with a mismatched kit.
- Weekly ritual: refreshes channel assets per season/game; wants a consistent matched kit.
- Frustration: UI gives no way to assemble/verify a complete matched set or bulk-grab deep links.

**Priya — small-business solopreneur / logo seeker.**
- Today: types industry into logo search, scrolls hundreds of near-identical results, no popularity sense.
- Weekly ritual: occasional — finds a logo, later needs matching social/business-card templates.
- Frustration: can't see the industry taxonomy as a map, can't sort logos by real popularity within her industry.

**Sam — social-media manager / freelancer.** 6 small-business clients.
- Today: re-finds the same template families weekly; flat unsearchable bookmark list.
- Weekly ritual: batch Instagram/Facebook story+post sets per client; tracks newly published templates.
- Frustration: no saved-search/watchlist for "new templates matching a niche," no offline cross-facet querying, no diff of what's new.

## Candidates (pre-cut)
(14 candidates generated across sources a/b/c/e — see Survivors and Killed tables for dispositions.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Long Description |
|---|---------|---------|-------|--------------|--------------|------------------|
| 1 | Best-seller rank within a niche | `top <query> [--category --device]` | 8/10 | spec-emits | Sorts local-mirror FTS matches by `purchases` desc | none |
| 2 | Printify POD-ready filter + export | `pod <query>` | 8/10 | hand-code | Filters mirror on `is_printify=1`, emits stage/editor/thumbnail links | Use for finding Printify-compatible mockups for a POD listing pipeline. To rank any mockup by popularity regardless of POD, use `top`. |
| 3 | Twitch matched-kit builder | `kit <style\|bundle>` | 7/10 | hand-code | Joins mirror on shared `bundle_tags`/style family; reports overlay/panel/emote/frame slots present vs missing | Use to assemble a streamer asset set from one style family. For generic cross-category siblings of any template, use `family`. |
| 4 | Industry taxonomy map with counts | `industry-map [<industry>]` | 6/10 | hand-code | Joins Industries taxonomy against the mirror; prints tree with per-node template counts | Use to navigate the industry taxonomy with counts. The framework `industries` lists entries flat with no counts. |
| 5 | Saved search / watchlist + new-since-sync diff | `watch add <query>` / `watch run` | 7/10 | hand-code | Persists named queries; `watch run` diffs current mirror matches vs ids/published_date at last run | Use for recurring niche monitoring across syncs. |
| 6 | Cross-facet gap finder | `gaps --facet <a> --by <b>` | 6/10 | hand-code | Local SQLite pivot across two tag-array facets to surface under-served combinations | none |
| 7 | Template popularity percentile | `rank <id>` | 6/10 | hand-code | Computes the template's `purchases` percentile within its category + primary tag cohort | Use to judge one template's popularity in context. To rank a whole result set, use `top`. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| `whatsnew` new-since delta | Too close to `search --sort newest`; `watch` captures the genuinely novel value | `watch run` |
| `export` bulk links | Wrapper over generic `--csv`/`--select`/`--json` | `pod` |
| `family` style-family spanner | Overlaps `kit`; weaker/occasional persona pull | `kit` |
| `free` radar | Covered by `search --free` + `top` | `top` |
| `trending` climbers | Needs two prior historical syncs; unverifiable on first run | `top` |
| `board` bookmark brand board | Thin atop `bookmarks` + generic `sql`/FTS | `bookmarks` |
| `mockups` blank-matcher | Collapses into `top --device` + `pod` | `top`, `pod` |
