# War.gov UFO CLI Brief

## API Identity
- Domain: U.S. Government declassified UAP/UFO files (PURSUE initiative)
- Users: Researchers, journalists, UFO enthusiasts, historians, OSINT analysts, AI agents
- Data profile: 162 files (120 PDFs, 28 videos, 14 images) spanning 1944-2025, ~2.3 GB for PDFs alone. Rolling releases planned. Covers 400+ incidents across FBI, DoD, NASA, State Department.

## Reachability Risk
- **HIGH** — entire war.gov is behind Akamai CDN bot protection. All direct HTTP (curl, wget, WebFetch) returns 403. Both the portal HTML and medialink download URLs are blocked. Browser-based access works. The UFO-USA GitHub project successfully downloaded 120 PDFs via curl at some point (possibly before protection was fully active, or using browser cookies).
- DVIDS API (dvidshub.net) requires a free API key and may provide an alternate video download path.

## Top Workflows
1. Browse the full file catalog — filter by agency (FBI/DoD/NASA/State), type (PDF/video/image), date range, location
2. Download all files or filtered subsets for offline research (resume, verify, organize by agency/date)
3. Search across document titles, descriptions, incident locations
4. Track new release tranches as they're published on a rolling basis
5. Cross-reference incidents by location, date, and agency
6. View incident details — read descriptions, check redaction status, find paired video/PDF files

## Table Stakes
- From UFO-USA (GitHub): Bulk download PDFs, manifest tracking, metadata preservation
- From UFOSINT Explorer: Searchable SQLite database, text search, filter by multiple dimensions, MCP server
- From NUFORC scrapers: Data normalization, geocoding, CSV/JSON export

## Data Layer
- Primary entities: files (documents, videos, images), incidents, agencies, release tranches
- Key fields per file: title, type (PDF/VID/IMG), agency, incident_date, incident_location, release_date, redacted (bool), description, download_url, dvids_video_id, video_pairing, pdf_pairing, modal_image_url
- Sync cursor: release_date + tranche identifier (release_1, release_2, etc.)
- FTS/search: titles, descriptions, locations, agencies
- Relationships: video↔PDF pairings, agency→files, tranche→files

## Source Data
- **Primary manifest**: CSV at war.gov/UFO interactive chart (162 rows, 14 columns)
- **GitHub mirror**: DenisSergeevitch/UFO-USA has the manifest as `metadata/uap-csv.csv`
- **Download URL pattern**: `www.war.gov/medialink/ufo/release_1/[filename]`
- **All behind 403**: Requires browser cookies or cleared-browser HTTP for access
- **DVIDS videos**: Some entries have DVIDS Video IDs — alternate download path via DVIDS API (free key required)

## Product Thesis
- Name: **ufo** — the declassified UAP file archive CLI
- Why it should exist: The war.gov/UFO portal launched today with zero programmatic access. The only existing tool (UFO-USA) is a Python/Gemini pipeline that converts PDFs, not a user-facing CLI. Researchers need a single binary to browse, search, download, and track the archive as new tranches are released. No CLI exists for this yet.

## Build Priorities
1. Local SQLite manifest store with full metadata for all 162+ files
2. Browse/search/filter commands (by agency, type, date, location, redaction status)
3. Download management (single file, batch, resume, verify, organize)
4. Sync command to pull new release tranches
5. Cross-reference features (paired files, incident timelines, agency breakdowns)
6. Stats/summary commands (counts by agency, type, date range)
