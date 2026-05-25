# ePropertyPlus CLI Brief

## API Identity
- **Domain:** Public land-bank property inventory. ePropertyPlus (by eProperty Innovations) is a closed-source SaaS many US land banks run on; each instance exposes a public JSON API at `https://public-<slug>.epropertyplus.com/landmgmtpub/remote/public/...` with no auth.
- **Users:** Distressed-asset analysts, civic/urban researchers, land-reuse/Land-Designer tooling, investors scanning land-bank inventory across cities.
- **Data profile:** Per-instance property inventory — parcels + structures with address, geometry (lat/lng, parcel L×W, sqft), zoning, potential use, structure type, occupancy, asking price, condition custom fields, and listing images.

## Reachability Risk
- **None.** Endpoints verified live against public-kclb (KC): `searchSummaryPublicMapQuery` returns 500 rows; `getPublishedProperty?propertyId=` returns full detail; image base resolves. robots.txt is fully permissive (`Disallow:`). No auth.

## Top Workflows
1. **Enumerate an instance's inventory** — list all published properties (id + lat/lng), then hydrate detail.
2. **Filter structures vs vacant lots** — `propertyClass`/`structureType` split (KC is ~97% lots; other instances differ).
3. **Export inventory** — CSV (tabular analysis) and GeoJSON (it has lat/lng) for GIS / Land Designer.
4. **Resolve & download images** — per-property listing photos via `publicThumbImgUrl`.
5. **Cross-instance scan** — point the same commands at any land bank by slug; compare inventories.

## Table Stakes
- List/get/filter properties; pagination/limit; `--json`; field selection; CSV export; local store + offline search; `doctor` health check.

## Data Layer
- **Primary entity:** Property (keyed by `id`; `parcelNumber` is the external join key to county/Socrata condition data).
- **Sync cursor:** full re-enumerate (small sets, ~hundreds–thousands per instance); store keyed by `(instance, id)`.
- **FTS/search:** address, neighborhood, propertyClass, potentialUse, comments.

## User Vision (from launch brief)
- Multi-tenant by slug (default kclb, override via env/flag). Enumerate→hydrate→filter structures-vs-lots. CSV/GeoJSON export. Images command. Feeds the distressed-LAND inventory + Land Designer pipeline (parcel geometry + zoning + potentialUse). Custom-field decode for condition signals.

## Source Priority
- Single source (ePropertyPlus). Spec authored from verified reverse-engineering; no browser-sniff needed.

## Product Thesis
- **Name:** epropertyplus-pp-cli — "Every land bank's public inventory, one command."
- **Why it should exist:** No open tool talks to ePropertyPlus today; its data is locked behind per-instance web UIs. A multi-tenant CLI turns dozens of land banks into a uniform, queryable, exportable distressed-land dataset — the targeting layer for distressed-asset intelligence and the Land Designer.

## Build Priorities
1. **P0 data layer:** Property store keyed by (instance, id); sync via enumerate→hydrate; FTS.
2. **P1 absorbed/table-stakes:** `list`, `get`, `custom-fields`, `image`, filtering, `--json`/`--select`, CSV, `doctor`, `sync`, `search`.
3. **P2 transcendence:** multi-instance registry, structure/lot classifier filter, GeoJSON export, bulk hydrate-all, parcel/condition join hooks (parcelNumber), Land-Designer land export, cross-instance compare.
