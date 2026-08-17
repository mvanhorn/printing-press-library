# Peerspace Absorb Manifest

## User Vision (updated)
Tech-event planning: meetups, workshops, community events. Rank venues by headcount/budget/date/vibe, gap-check shortlists for AV/WiFi/late access, export proposal packs for Luma/Eventbrite/Slack, watch favorites for price drift. Agent-native JSON throughout.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Search venues (location, activity, space type, guests, price, availability) | Peerspace web search | peerspace-pp-cli venues list | Offline-ready, --json, scriptable |
| 2 | Natural-language venue search | Peerspace listings/natural (JS) | peerspace-pp-cli venues natural | NL query without UI |
| 3 | Batch listing total prices | Peerspace listings/prices (JS) | peerspace-pp-cli venues prices | Quote totals for shortlist |
| 4 | Single-listing deep get (from local/synced hits) | Search hit payload + local store | peerspace-pp-cli venues get | Finalist detail when no public detail API in HAR |
| 5 | List amenities / space types / styles filters | Peerspace filters API | peerspace-pp-cli venues filters | Agent-readable filter catalog |
| 6 | Default activity time windows | Peerspace default-times | peerspace-pp-cli venues default-times | Planning defaults without UI |
| 7 | Current user profile | Peerspace /profiles/me | peerspace-pp-cli profiles me | Cookie session, --json |
| 8 | Profile experience signals | Peerspace experience | peerspace-pp-cli profiles experience | Onboarding context |
| 9 | Message thread count | Peerspace messages | peerspace-pp-cli messages thread-count | Inbox pulse |
| 10 | Favorites board (saved listing ids) | Peerspace fav_board | peerspace-pp-cli projects fav-board | Shortlist IDs for offline join |
| 11 | Project details for collaborator | Peerspace projects details | peerspace-pp-cli projects details | Project metadata + location |
| 12 | Profile entitlements | Peerspace entitlements | peerspace-pp-cli entitlements profiles | Account attachments |
| 13 | Env/web config | Peerspace env | peerspace-pp-cli env web | Client config dump |
| 14 | Keyword dictionary | Peerspace env keywords | peerspace-pp-cli env keywords | Locale keyword map |
| 15 | Local SQLite sync of listings/favorites | Printing Press framework | peerspace-pp-cli sync | Offline re-query; ad-hoc SQL over listings |
| 16 | FTS search over synced data | Printing Press framework | (behavior in peerspace-pp-cli search) FTS over local listings | Offline full-text |
| 17 | SQL over local store (capacity/budget ad-hoc) | Printing Press framework | peerspace-pp-cli sql | e.g. capacity 25-60 under $180/hr |
| 18 | Doctor / health | Printing Press framework | peerspace-pp-cli doctor | Auth + reachability |
| 19 | Cookie auth via Chrome session | Printing Press cookie auth | peerspace-pp-cli auth login --chrome | Logged-in favorites/profile |

**Note:** No official listing-detail REST path appeared in the HAR. `venues get` reads the richest local/search-hit row for an id; live re-hydrate via `venues list` filters when needed. All novel commands ship with `--json` / `--agent` / `--select` and token-efficient row shapes.

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Budget scout | scout budget | hand-code | Offline price bands over synced hits | Market-level price histograms. Expose underlying listing rows for agent SQL follow-up. |
| 2 | Capacity bands | scout capacity | hand-code | Capacity distribution offline | Headcount histograms; same store powering ad-hoc sql. |
| 3 | Shortlist compare | shortlist compare | hand-code | Join fav IDs to attributes | Side-by-side shortlist table. |
| 4 | Multi-city market pulse | markets pulse | hand-code | Cross-city aggregates | Median price, density, IB %, capacity quantiles. |
| 5 | Shortlist delta | shortlist delta | hand-code | Membership history | Added/removed favorites since last snapshot. |
| 6 | Neighborhood market cut | markets neighborhoods | hand-code | GROUP BY neighborhood + tech vibe | Stats plus `--vibe tech` keyword/cluster signals (no external APIs). |
| 7 | Similar-to-shortlist | shortlist similar | hand-code | Mechanical substitutes | Neighbors by capacity/price/neighborhood. |
| 8 | Project scout pack | projects scout | hand-code | Project + listings join | Project location context + fitting listings. |
| 9 | Sync coverage | sync coverage | hand-code | Sync metadata plane | Which markets are local & fresh. |
| 10 | Coordination pulse | pulse | hand-code | Composed auth GETs | Threads + favs + profile; optional recent shortlist activity. |
| 11 | Venue fit recommender | venues recommend | hand-code | Multi-factor tech-event rank | Rank by headcount, date window, budget, vibe keywords. |
| 12 | Shortlist proposal bridge | shortlist export | hand-code | Handoff pack format | JSON + markdown for Eventbrite/Luma/Slack. |
| 13 | Shortlist/venue drift | shortlist drift | hand-code | Attribute history on favorites | Price & availability-field changes over time. |
| 14 | Amenity gap analysis | scout gaps | hand-code | Tech checklist vs shortlist | Missing WiFi/AV/late access/seating/transit. |
| 15 | Multi-city campaign scout | scout multi-city | hand-code | Shared constraints × cities | Top options per city under one constraint set. |

## Stubs
None.

## Hand-code commitment
- Novel features ≥5/10: **15** (all hand-code)
- Absorbed endpoint surface + local `venues get` + framework sync/sql/search
- Cross-cutting: every novel command supports `--json --agent --select` with compact row schemas
