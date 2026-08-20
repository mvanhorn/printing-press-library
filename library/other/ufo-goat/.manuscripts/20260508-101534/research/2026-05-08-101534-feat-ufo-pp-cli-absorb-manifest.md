# UFO CLI Absorb Manifest

## Source Tools Analyzed
1. **UFO-USA** (GitHub, DenisSergeevitch) — Python curl downloader + Gemini PDF converter. 120 PDF downloads, TSV manifest, markdown conversion.
2. **UFOSINT Explorer** (GitHub, UFOSINT) — Flask web app + SQLite database of 618K NUFORC sightings. MCP server, search tools, timeline. Different data source (citizen sightings, not government files).
3. **NUFORC scrapers** (GitHub, timothyrenner) — Python data collection from NUFORC. CSV/JSON export, geocoding.
4. **uap-data-vis-tool** (GitHub, jamsoft) — Desktop app for importing/visualizing UAP data sources.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Download PDFs in bulk | UFO-USA curl scripts | `ufo download --type pdf` | Resume, verify checksum, progress bars, organize by agency |
| 2 | Manifest tracking | UFO-USA pdf_manifest.tsv | SQLite manifest store | Queryable, filterable, sync-aware |
| 3 | File metadata storage | UFO-USA uap-csv.csv | SQLite with FTS5 | Full-text search across titles, descriptions, locations |
| 4 | Search sightings | UFOSINT search_sightings | `ufo search "<query>"` | Searches government file descriptions, not just citizen reports |
| 5 | Filter by agency | None (manual CSV grep) | `ufo files --agency FBI` | First-class flag with autocomplete |
| 6 | Filter by type | None (manual) | `ufo files --type vid` | PDF, VID, IMG filters |
| 7 | Filter by date range | UFOSINT timeline | `ufo files --after 1960 --before 1970` | Date range on incident dates |
| 8 | Filter by location | UFOSINT by state | `ufo files --location "Iraq"` | Location search across incident locations |
| 9 | JSON output | UFOSINT API | `--json` on every command | Agent-native, pipeable |
| 10 | Stats/counts | UFOSINT get_stats | `ufo stats` | Breakdown by agency, type, date, redaction status |
| 11 | Export data | NUFORC CSV export | `ufo export --csv` / `--json` | Export filtered subsets |
| 12 | Download videos | None | `ufo download --type vid` | DVIDS API integration + direct URL fallback |
| 13 | Download images | None | `ufo download --type img` | Direct URL with browser cookie fallback |
| 14 | View file details | None | `ufo file <id>` | Full metadata, description, paired files, download status |
| 15 | Redaction status | None | `ufo files --redacted` / `--unredacted` | Filter by redaction status |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|------------------------|-------|
| 1 | Sync new releases | `ufo sync` | Fetches latest manifest from war.gov/UFO, detects new files added in subsequent release tranches, shows diff | 9/10 |
| 2 | What's new | `ufo new` | Shows files added since your last sync — "what did I miss" for rolling releases | 8/10 |
| 3 | Timeline view | `ufo timeline` | Chronological incident timeline across all agencies — FBI case from 1947 next to DoW mission report from 2024 | 8/10 |
| 4 | Agency breakdown | `ufo agencies` | Cross-agency analysis: which agency contributed what, overlap detection, coverage gaps | 7/10 |
| 5 | Paired files | `ufo pairs` | Shows video↔PDF pairings so researchers can find the document that accompanies a video and vice versa | 8/10 |
| 6 | Download progress | `ufo download --all --resume` | Resume interrupted downloads, verify completions, track what's been downloaded vs pending | 8/10 |
| 7 | Location map data | `ufo locations` | Aggregate incidents by location, export GeoJSON for mapping tools | 7/10 |
| 8 | Doctor/health check | `ufo doctor` | Verify manifest integrity, check download completeness, test API reachability | 7/10 |
| 9 | Open in browser | `ufo open <id>` | Open the war.gov page or direct file URL in default browser | 6/10 |
| 10 | Describe file | `ufo describe <id>` | Print the full description blurb with redaction notice — readable terminal output for long government descriptions | 6/10 |

## Data Source Strategy

The CLI uses a hybrid data source approach:
1. **Primary**: Download the CSV manifest from the GitHub mirror (DenisSergeevitch/UFO-USA) as the canonical file list
2. **Future**: When war.gov/UFO becomes programmatically accessible, switch to direct manifest fetch
3. **Downloads**: Attempt direct HTTP to war.gov medialink URLs; if 403, instruct user to import Chrome cookies via `ufo auth login --chrome`
4. **Videos**: Offer DVIDS API as alternate video source when DVIDS Video ID is present

## Spec Strategy

No OpenAPI spec exists. Build an internal YAML spec defining:
- File entity with all 14 CSV fields
- List/get/search/download endpoints modeled as commands
- Auth: optional (browser cookies for downloads, DVIDS API key for videos)
