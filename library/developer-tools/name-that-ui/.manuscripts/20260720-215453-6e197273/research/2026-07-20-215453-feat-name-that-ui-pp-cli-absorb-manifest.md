# NameThatUI CLI Absorb Manifest

## Absorbed (match or beat the public site and relevant reference tools)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Resolve a vague component description | NameThatUI semantic search | name-that-ui-pp-cli identify | Enriches ranked hits with canonical page details and source URLs |
| 2 | Two-stage retrieve then resolve ranking | NameThatUI `POST /api/search` | (behavior in name-that-ui-pp-cli identify) local candidate retrieval followed by remote resolve | Replays the site's proven ambiguity workflow with deterministic JSON |
| 3 | Browse components by platform | NameThatUI catalog | name-that-ui-pp-cli component list | Filters web/macOS records with bounded agent output |
| 4 | Get canonical component definition | NameThatUI component page | name-that-ui-pp-cli component get | Returns name, tagline, aliases, fuzzy phrases, description, and source URL |
| 5 | Inspect component anatomy and named parts | NameThatUI component parts | name-that-ui-pp-cli component anatomy | Part IDs and descriptions are directly addressable and selectable |
| 6 | Retrieve framework and platform API symbols | NameThatUI API mappings | name-that-ui-pp-cli component api | Normalizes ARIA/HTML, AppKit, and SwiftUI symbols with implementation notes |
| 7 | Copy an implementation prompt | NameThatUI component prompt | name-that-ui-pp-cli component prompt | Compact paste-ready prompt with provenance |
| 8 | Copy a debugging prompt | NameThatUI debug prompt | name-that-ui-pp-cli component debug-prompt | Turns a visual mismatch into source-backed debugging instructions |
| 9 | Follow related components | NameThatUI related entries | name-that-ui-pp-cli component related | Traversable related graph instead of manual page hopping |
| 10 | Resolve a vague visual-style description | NameThatUI style search | name-that-ui-pp-cli style identify | Returns ranked style candidates and ambiguity explicitly |
| 11 | Browse visual styles | NameThatUI style atlas | name-that-ui-pp-cli style list | Structured browsing with stable slugs and source URLs |
| 12 | Get full visual-style guidance | NameThatUI style detail | name-that-ui-pp-cli style get | Combines signals, comparisons, cautions, origins, code, and related styles |
| 13 | Inspect defining style signals | NameThatUI style signals | name-that-ui-pp-cli style signals | Separates facet, role, name, and description for agent selection |
| 14 | Distinguish commonly confused styles | NameThatUI style comparisons | name-that-ui-pp-cli style compare | Produces concise decision rules instead of a generic definition |
| 15 | Retrieve style implementation starting points | NameThatUI code guidance | name-that-ui-pp-cli style code | Stack-specific source-backed starting points |
| 16 | Retrieve accessibility and misuse cautions | NameThatUI style guidance | name-that-ui-pp-cli style cautions | Keeps risk guidance separable from aesthetic advice |
| 17 | Translate product language across AppKit and SwiftUI | NameThatUI translation table | name-that-ui-pp-cli translate | Bidirectional exact/fuzzy lookup across plain language and APIs |
| 18 | Read component-versus-component guides | NameThatUI comparison pages | name-that-ui-pp-cli component compare | Side-by-side purpose, anatomy, and choice guidance |
| 19 | Scenario-based component recommendation | UX Components MCP `recommend`; matched with NameThatUI data | name-that-ui-pp-cli recommend | Uses NameThatUI records to return choice, reasoning, states/parts, alternatives, and watch-outs |
| 20 | Natural-language intent routing | UX Components MCP `smart_query` | name-that-ui-pp-cli ask | Routes identify, recommend, compare, translation, and style intent without exposing tool-selection complexity |
| 21 | Usage states and when-to-use/avoid guidance | UX Components `lookup`; NameThatUI anatomy/prompts | name-that-ui-pp-cli component guidance | Derives explicit guidance from canonical descriptions, parts, prompts, and comparisons without claiming unsupported design-system mappings |
| 22 | Markdown and structured machine output | UX Components MCP formats; NameThatUI copy-as-Markdown | (behavior in name-that-ui-pp-cli component get) global `--json`, `--agent`, `--select`, `--compact`, `--plain`, and `--csv` output | Fits human reference and agent context budgets |
| 23 | Search aliases and colloquial names | Component Gallery and NameThatUI catalogs | (behavior in name-that-ui-pp-cli search) local FTS over names, aliases, fuzzy phrases, parts, APIs, prompts, and style signals | Works offline after sync and is SQL-composable |
| 24 | Track catalog additions and guidance changes | NameThatUI RSS and sitemap | name-that-ui-pp-cli updates | Deterministic recent-entry view with source timestamps |
| 25 | Persist public reference data locally | NameThatUI public HTML/RSS/sitemap | name-that-ui-pp-cli sync | Reproducible SQLite snapshots for offline and low-latency agent use |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Agent context pack | `context-pack --component <slug> [--style <slug>] [--framework <name>]` | 10/10 | hand-code | Uses synced component, part, API, prompt, style-signal, and caution records in local SQLite to compute one bounded provenance-preserving packet with no external dependencies. | Mira and Evan need one compact packet before weekly UI implementation and repair; the site exposes the source entities but no endpoint joins them. | `none` |
| 2 | Terminology lint | `lint <path>` | 9/10 | hand-code | Uses local FTS over synced names, aliases, fuzzy phrases, parts, and API symbols to compute source-backed findings from files with no external dependencies. | Mira and Theo repeatedly review tickets, prompts, and design specs; research surfaced colloquial naming and hallucinated terminology as core pains. | `Use this command for prose, prompt, and specification terminology checks. Do NOT use this command to identify UI APIs already present in source code; use 'inventory' instead.` |
| 3 | Framework crosswalk | `crosswalk <concept>` | 9/10 | hand-code | Uses synced element, alias, part, API-symbol, and translation tables in local SQLite to compute a unified terminology matrix with no external dependencies. | Priya translates product language into platform APIs weekly; NameThatUI exposes both component mappings and macOS translations but no unified crosswalk. | `none` |
| 4 | Project UI inventory | `inventory <path>` | 8/10 | hand-code | Uses a deterministic filesystem scan and the synced local API-symbol/name index to compute file-to-component matches with no external dependencies. | Mira and Theo inspect code before repair and maintenance; exact framework symbols make matching deterministic and testable. | `Use this command to map UI components and API symbols currently present in source code. Do NOT use this command to check prose terminology; use 'lint' instead.` |
| 5 | Catalog change impact | `impact <path> --since <snapshot>` | 8/10 | hand-code | Uses local catalog snapshots, changed component/style records, and a deterministic project symbol scan to compute affected source files with no external dependencies. | Theo's weekly frustration is identifying which internal artifacts upstream guidance changes affect; RSS, sitemap freshness, and snapshots provide the change evidence. | `Use this command to find project files affected by guidance changes between catalog snapshots. Do NOT use this command for a current-state source inventory; use 'inventory' instead.` |

## Explicit exclusions

- No stubs.
- No screenshot classification: it would require image understanding outside the public NameThatUI contract.
- No generated design tokens: NameThatUI does not provide authoritative token values.
- No arbitrary third-party design-system mappings or component installation: those require external catalogs and are outside this source-backed reference CLI.
- No persistent watcher: `updates` plus one-shot `impact` covers the workflow without a background process.
