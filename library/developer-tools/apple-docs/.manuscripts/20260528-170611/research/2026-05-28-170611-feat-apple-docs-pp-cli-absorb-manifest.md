# apple-docs CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Keyword search across all docs | kimsungwhee/apple-docs-mcp `search_apple_docs` (1.3k stars) | (behavior in apple-docs-pp-cli search) | Offline SQLite FTS5, --json, --select, pipe to jq |
| 2 | Get any doc page by path | kimsungwhee `get_apple_doc_content`, sosumi.ai | apple-docs-pp-cli doc get | --markdown for LLM context, --json structured |
| 3 | List frameworks/technologies | kimsungwhee (implicit), Apple `technologies.json` | (generated endpoint) technologies list | Cached locally; SQL-composable |
| 4 | Fetch framework index | kimsungwhee `search_framework_symbols` | (generated endpoint) index get | Caches full hierarchy in SQLite for offline grep |
| 5 | Get related APIs / see-also | kimsungwhee `get_related_apis` | apple-docs-pp-cli doc related | --json structured output |
| 6 | Platform compatibility for a symbol | kimsungwhee `get_platform_compatibility` | apple-docs-pp-cli doc platforms | iOS/macOS/watchOS/tvOS/visionOS table; --csv |
| 7 | Find similar APIs | kimsungwhee `find_similar_apis` | apple-docs-pp-cli doc similar | Local SQLite cosine on abstracts; offline |
| 8 | List sample code | kimsungwhee `get_sample_code`, Abdullah `samples` | apple-docs-pp-cli sample-code list | --framework filter, --json |
| 9 | WWDC video search | kimsungwhee `search_wwdc_videos` (1,260+ indexed) | apple-docs-pp-cli wwdc search | Indexes WWDC metadata referenced from doc pages |
| 10 | Markdown emission for docs | sosumi.ai | (behavior in apple-docs-pp-cli doc get --markdown) | Built-in renderer, no external service |
| 11 | Recent doc updates feed | kimsungwhee `get_documentation_updates` | apple-docs-pp-cli updates | Diffs against last sync; --since |
| 12 | Offline docset | Dash.app (paid), Zeal | (behavior in apple-docs-pp-cli sync + search) | First-class CLI; --json output Dash/Zeal can't pipe |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Token-lean doc projection | doc get --shape | hand-code | Projects 50KB+ DocC JSON down to {abstract, signature, platforms, min} fields — requires local parsed cache and shape-aware renderer; competitors return full payloads | Use this command when an agent needs the minimal viable doc payload (abstract, signature, platforms, or all three). Do NOT use it for the full rendered doc page with See Also; use 'doc get' (default shape) or 'bundle' instead. |
| 2 | Cross-platform replacement finder | port-to | hand-code | Walks the local See-Also / Replacement-Of graph until landing on a symbol that's available on the target platform AND not deprecated — requires symbol+platform+references join | Use this command to find the API that replaces an unavailable symbol on a specific platform. Do NOT use it for general similar-API lookup unrelated to platform availability; use 'doc similar' instead. Do NOT use it to enumerate all deprecated symbols in a framework; use 'deprecation-cliff' instead. |
| 3 | Framework snapshot diff | snapshot diff | hand-code | Classifies deltas between two cached framework indices (added / removed / deprecated / likely-renamed via path-stem similarity) — requires two stored snapshots; no Apple endpoint exists | Use this command for a structured added/removed/deprecated/renamed delta between two saved snapshots of one framework. Do NOT use it for the rolling 'what's new since last sync' feed; use 'updates' instead. Do NOT use it for a single-version cliff report; use 'deprecation-cliff' instead. |
| 4 | Deprecation cliff report | deprecation-cliff | hand-code | SQL aggregation across `platform_availability` where `deprecatedAt == version`, grouped by framework + symbolKind — exploits Apple's uniquely structured per-platform `deprecatedAt` field | Use this command for 'every symbol Apple deprecated in version N of platform X'. Do NOT use it for diffing two arbitrary snapshots; use 'snapshot diff' instead. Do NOT use it to find a per-symbol replacement; use 'port-to' instead. |
| 5 | Conformance graph | conformance | hand-code | Walks `relationshipsSections` rows in SQLite to enumerate concrete conformers and ancestor protocols — fixes the inheritance-hidden pain point; competitors only return untyped See Also | none |
| 6 | Cross-framework symbol grep | grep | hand-code | Regex / FTS over unified symbols table with filters on kind, platform-available, deprecated — Dash/Zeal only grep within one docset; no online competitor offers regex | Use this command to find symbols matching a pattern across every synced framework, with kind/platform/deprecation filters. Do NOT use it for plain keyword search of doc titles and abstracts; use 'search' instead. |
| 7 | WWDC → symbol reverse index | wwdc symbols | hand-code | Joins `references` rows where `kind=video` back to the symbols whose pages cite each session — kimsungwhee indexes videos but does not expose the reverse edge | Use this command to enumerate every symbol whose doc page cites a given WWDC session. Do NOT use it for keyword search across WWDC titles/abstracts; use 'wwdc search' instead. |
| 8 | Agent-shape doc bundle | bundle | hand-code | Concatenates Markdown render of a symbol + its depth-N See-Also pages from local cache, truncating to a token budget — sosumi proves Markdown demand but offers no multi-page bundling | Use this command to get a self-contained Markdown context blob ready to paste into an agent prompt. Do NOT use it for a single doc page; use 'doc get --markdown' instead. Do NOT use it for sample-code projects; use 'sample-code list' instead. |

**Source pool surveyed:** kimsungwhee/apple-docs-mcp (1.3k★, 18 MCP tools); Ahrentlov/appledeepdoc-mcp (15 tools); sosumi.ai/nshipster (4 MCP tools); Abdullah4AI/apple-developer-toolkit (Go, 8 docs subcommands); Dash.app (Kapeli); Zeal; apple/swift-docc-render; apple/swift-docc.
