## Absorbed Manifest (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Keyword search models | Apify search scraper; MakerWorld web (`select/design2`) | makerworld-pp-cli search <kw> | offline FTS over local mirror, --json/--select/--csv, orderBy score/hotScore |
| 2 | Model (design) details | Apify details scraper; bambu-mcp source | (generated endpoint) designs get | --select deep fields, --compact high-gravity fields |
| 3 | Browse trending / by category | MakerWorld web UI (`select/design/nav`) | makerworld-pp-cli browse --nav Trending | full navKey enum, pagination, agent output |
| 4 | List categories | web UI (`homepage/nav`) | (generated endpoint) categories list | offline, stable keys |
| 5 | Designer profile + their models | Apify maker scraper | makerworld-pp-cli designers models <uid> | local mirror, counts |
| 6 | Related designs | bambu-mcp source (`/design/{id}/relate`) | (generated endpoint) designs related | composable IDs |
| 7 | Remixes of a design | bambu-mcp source (`/design/{id}/remixed`) | (generated endpoint) designs remixes | lineage-ready |
| 8 | Comments & ratings | bambu-mcp source (`comment-service/commentandrating`) | (generated endpoint) designs ratings | aggregate star stats |
| 9 | Download 3MF file | bambu-mcp `makerworld_download` | makerworld-pp-cli download <designId> | auth tier (bearer); auto-resolve default instance |
| 10 | Favorites / liked designs (account) | bambu-mcp; web UI | makerworld-pp-cli favorites (auth tier) | offline mirror of your library |
| 11 | Recommended-for-you | web UI (`recommand/youlike`) | (generated endpoint) recommend list | agent output |
