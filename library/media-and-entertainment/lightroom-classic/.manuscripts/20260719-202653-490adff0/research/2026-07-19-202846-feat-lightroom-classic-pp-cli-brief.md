# Lightroom Classic Catalog CLI Brief

## API Identity
- Domain: Adobe Lightroom Classic catalogs (.lrcat) — local SQLite databases, no network API
- Users: photographers running Lightroom Classic; primary user is a photographer with daily photo projects (photo-a-day site, finite photo projects) who publishes via Astro static sites
- Data profile: 32k+ images; tables Adobe_images, AgLibraryFile/Folder/RootFolder, AgHarvestedExifMetadata (+ interned camera/lens), AgLibraryKeyword(Image), AgLibraryCollection(Image), Adobe_imageDevelopSettings. captureTime is ISO-ish local string; aperture/shutterSpeed stored as APEX values; pick is 1/0/-1; colorLabels free string; rating REAL or NULL.

## Reachability Risk
- None — local file access only. Risk vector is the live catalog being open in Lightroom (SQLite WAL + lock); mitigated by always opening read-only/immutable and documenting behavior when Lightroom holds the lock.

## Top Workflows
1. Photo-a-day: "what did I shoot on <date>, which are picks/4+ stars" → feed daily publish pipeline
2. Gap/streak audit: which days in a range have no photos (or no picks); current shooting streak
3. Shot-list export: filtered set (date range, rating, collection, keyword) as JSON for site build scripts
4. Catalog forensics: counts by camera/lens/year/format; find images whose master file is missing on disk
5. Resolve any image to its absolute path on disk for downstream copy/export

## Table Stakes (from ecosystem)
- lrselect-style criteria search: date range, rating op, pick/reject, color label, keyword, collection, camera, lens, ISO/aperture/shutter/focal (Lightroom-SQL-tools)
- List collections and keywords with image counts (lrcat-extractor, ExportLRCatalog)
- Full file-path resolution via rootFolder+folder+filename join (LightroomClassicCatalogReader)
- CSV/JSON export of any result set (ExportLRCatalog, LightroomClassicCatalogReader)
- Human-readable EXIF: APEX→f-stop and shutter-fraction conversion (Lightroom-SQL-tools)

## Data Layer
- Primary entities: photos (joined view over the 5 core tables), collections, keywords, cameras, lenses
- The catalog itself IS the local store. No sync path, no mirror: every command queries the .lrcat directly in read-only immutable mode. cache disabled.
- FTS/search: filename + keyword + collection name matching via SQL LIKE; catalog is authoritative

## Codebase Intelligence
- lrcat-extractor (Rust, hfiguiere): id_local/id_global conventions, genealogy paths — confirms schema stability across LR versions
- Lightroom-SQL-tools (Python, fdenivac): proven criteria-DSL over the same joins; smart-collection execution
- Direct schema introspection of the user's real v13 catalog verified all joins (32,339 images, 2016–2026)

## User Vision
- Read-only agent-native CLI. Default catalog ~/Pictures/Lightroom/Lightroom Catalog-v13-3.lrcat, path configurable. Must refuse writes; SQLite read-only/immutable mode always. Photo-a-day workflow is the headline: per-day queries, no-pick day detection, streaks, JSON shot lists.

## Product Thesis
- Name: lightroom-classic (binary lightroom-classic-pp-cli)
- Why it should exist: every existing tool is a Python/Rust library or script collection demanding SQL knowledge; none is agent-native (no --json contract, no typed exits, no help examples), none understands daily-practice workflows (gaps, streaks, per-day picks). This CLI reads the catalog Lightroom itself maintains — zero setup, zero sync, zero API keys.

## Build Priorities
1. find (criteria search) + day (per-day view) with full EXIF joins and APEX conversion
2. gaps/streak for daily practice; stats rollups
3. collections/keywords/cameras/lenses listing with counts; path resolution + missing-file audit
4. JSON shot-list export contract for static-site pipelines
