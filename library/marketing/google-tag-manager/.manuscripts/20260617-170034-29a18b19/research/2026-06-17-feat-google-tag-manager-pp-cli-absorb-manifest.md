# Google Tag Manager CLI — Absorb Manifest (READ-ONLY scope)

## Landscape
- Official source: Google Tag Manager API v2 + `google.golang.org/api/tagmanager/v2` (method parity).
- Community tools are thin and mostly **write**-focused (cross-container copy/clean), so for the read surface the real incumbent is the **GTM web console + raw API**, neither of which can diff, bulk-query, cross-reference the graph, or export to VCS.
- This CLI absorbs full read coverage of the API, then transcends with a local cross-referenced SQLite store.

## Absorbed (match the full read surface — 22 typed GET commands, generator-emitted)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List accounts | tagmanager.accounts.list | (generated endpoint) accounts list | `--json`/`--csv`/offline, agent-native |
| 2 | List containers | accounts.containers.list | (generated endpoint) containers list | Offline, scriptable |
| 3 | Look up container by destination/GA4 id | containers.lookup | (generated endpoint) containers lookup | Find which container owns a tag id |
| 4 | Container install snippet | containers.snippet | (generated endpoint) containers snippet | Pull GTM snippet from terminal |
| 5 | List workspaces | workspaces.list | (generated endpoint) workspaces list | Offline |
| 6 | Workspace status (changes vs base) | workspaces.getStatus | (generated endpoint) workspaces status | Built-in change set, feeds `diff` |
| 7 | List tags | workspaces.tags.list | (generated endpoint) tags list | Offline, feeds audit/graph |
| 8 | List triggers | workspaces.triggers.list | (generated endpoint) triggers list | Offline, feeds audit/graph |
| 9 | List variables | workspaces.variables.list | (generated endpoint) variables list | Offline, feeds audit/graph |
| 10 | List built-in variables | workspaces.built_in_variables.list | (generated endpoint) built-in-variables list | Offline |
| 11 | List folders | workspaces.folders.list | (generated endpoint) folders list | Offline |
| 12 | List templates | workspaces.templates.list | (generated endpoint) templates list | Custom/gallery template inventory |
| 13 | List clients | workspaces.clients.list | (generated endpoint) clients list | Server-container clients |
| 14 | List zones | workspaces.zones.list | (generated endpoint) zones list | Offline |
| 15 | List gtag config | workspaces.gtag_config.list | (generated endpoint) gtag-config list | Offline |
| 16 | List destinations | containers.destinations.list | (generated endpoint) destinations list | Linked GA4 destinations |
| 17 | List environments | containers.environments.list | (generated endpoint) environments list | Offline |
| 18 | List version headers | version_headers.list | (generated endpoint) version-headers list | Version history |
| 19 | Latest version header | version_headers.latest | (generated endpoint) version-headers latest | Offline |
| 20 | Live (published) version | versions.live | (generated endpoint) versions live | Source of truth for diff |
| 21 | List user permissions | user_permissions.list | (generated endpoint) user-permissions list | Access audit |
| 22 | Get user permission | user_permissions.get | (generated endpoint) user-permissions get | Access audit |

Every row also gets `--json`, `--select` (dotted-path field narrowing), `--csv`, typed exit codes, and the local SQLite store via `pull`. Mutating endpoints (28 POST / 1 PUT / 1 DELETE) are **stripped** — the binary cannot create, update, delete, or publish.

## Transcendence (only possible with our local cross-referenced store)
Validated + scored by an independent design pass. Buildability `hand-code` unless noted.

| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|------------------------|-----------------|
| 1 | Mirror container → SQLite (foundation) | `pull [--account --container --workspace\|--live] [--all-containers]` | hand-code | 10 | Local cross-referenced store is the precondition for every offline/agent query the API can't answer. Stamps each pull with `pulled_at` so re-pulls become time-series snapshots. | none |
| 2 | Hygiene battery | `audit [--fail-on=high]` | hand-code | 9 | Dead tags (no firing trigger), orphan triggers, unused variables, paused tags, Custom-HTML/All-Pages flags need the full joined graph — impossible in the per-resource UI. Typed exit code gates CI. | Use to find config hygiene problems. For Consent Mode specifically use 'consent-report'. |
| 3 | Snapshot diff | `diff <a> <b>` | hand-code | 9 | Field-level diff of workspace vs live, version vs version, or container vs container. GTM UI has no diff at all. Stable entity keys (name+type) so renames don't read as add+remove. | Refs: `live`, `workspace:<id>`, `version:<id>`, `container:<id>`. For drift vs a past pull use 'history'. |
| 4 | Cross-container fleet matrix | `fleet [--metric ga4\|consent\|custom-html\|counts]` | hand-code | 9 | GA4/measurement-id inventory, Consent-Mode coverage %, Custom-HTML count, entity counts across MANY pulled containers in one table. UI is single-container only. | none |
| 5 | Consent Mode v2 readiness | `consent-report` | hand-code | 8 | Per-tag `consentSettings` coverage + ungated-tag flags, classified by vendor (ads/analytics), from the joined tag/consent graph. Live EEA concern. | none |
| 6 | FTS over all config | `search <term>` | hand-code | 8 | Find stray pixels, hardcoded GA4 ids, leftover URLs across every pulled container at once. FTS5 over flattened entity JSON. | none |
| 7 | Change-tracking over time | `history [--since <dur>]` | hand-code | 8 | Diff the current pull vs a prior snapshot in the same DB — drift detection / tamper-evident ledger the API never retains. | none |
| 8 | Reverse-dependency / blast radius | `uses <variable\|trigger>` | hand-code | 8 | "What references X / what breaks if I touch it" needs the reference index, not a single GET. Distinct from `fires` (runtime graph). | none |
| 9 | Runtime fire graph | `fires [--tag <id>\|--trigger <id>]` | hand-code | 7 | Bidirectional trigger↔tag↔variable walk for incident debugging ("why did this fire / what fires here"). | For "is it safe to delete" use 'uses' instead. |
| 10 | Git-friendly export | `export [--flat]` | hand-code | 7 | Deterministic flattened representation (stable key ordering) for clean VCS diffs and PR review. `--format gtm` emits container-export-compatible JSON. | none |
| 11 | Governance lint | `lint` | hand-code | 6 | Naming-convention/folder/notes/gallery-author rules across the whole estate at once. | none |

## Build plan
- **P0 foundation**: SQLite schema for all entities + reference index (tag→trigger, tag/trigger→variable) + `pull` walker.
- **P1 absorb**: 22 typed GET commands (generator-emitted) + `export`.
- **P2 transcend**: audit, diff, fleet, consent-report, search, history, uses, fires, lint.
