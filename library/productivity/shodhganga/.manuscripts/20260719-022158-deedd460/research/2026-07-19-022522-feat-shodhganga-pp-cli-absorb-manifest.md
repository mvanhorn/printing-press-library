# Shodhganga CLI Absorb Manifest

## Landscape
No existing programmatic tool for Shodhganga (OAI-PMH/REST/OpenSearch all disabled,
so generic DSpace clients like `sickle`/`dspace-rest-client` do not apply). The only
"competitor" is the Shodhganga JSPUI **web UI** itself. We absorb every capability
the UI offers, deliver it as structured/agent-native output, then transcend with a
local store the UI has no equivalent for.

## Absorbed (match or beat every web-UI capability)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Keyword search over all theses | Web UI `/simple-search` | `shodhganga-pp-cli search <query>` | `--json`/`--select`/`--csv`, paging via `--limit`/`--start`, total-count surfaced |
| 2 | Browse by title | Web UI `/browse?type=title` | `(behavior in shodhganga-pp-cli browse --type title)` | Structured rows, scriptable |
| 3 | Browse by author/researcher | Web UI `/browse?type=author` | `(behavior in shodhganga-pp-cli browse --type author)` | Structured, agent-native |
| 4 | Browse by subject/keyword | Web UI `/browse?type=subject` | `(behavior in shodhganga-pp-cli browse --type subject)` | Structured |
| 5 | Browse by issue date | Web UI `/browse?type=dateissued` | `(behavior in shodhganga-pp-cli browse --type dateissued)` | Structured |
| 6 | Item detail w/ Dublin Core metadata | Web UI `/handle/10603/<id>` | `shodhganga-pp-cli thesis get <handle>` | Full DC extraction: researcher, guide, university, dept, keywords, abstract, year, type — as JSON |
| 7 | University/community navigation | Web UI community pages | `shodhganga-pp-cli university list` / `university get <handle>` | Structured university directory |
| 8 | Result total counts | Web UI "Results X of N" | `(behavior in shodhganga-pp-cli search — total field in output)` | Machine-readable total |
| 9 | Handle/URI resolution | Web UI hdl.handle.net links | `(behavior in shodhganga-pp-cli thesis get — accepts handle or full URI)` | Normalizes 10603/ID or hdl.handle.net URI |
| 10 | Field-narrowed output | (none — UI is fixed HTML) | `(behavior in shodhganga-pp-cli ... --select)` | Agents pull only needed fields |

## Transcendence (only possible with local extraction + SQLite store)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Offline corpus harvest | `harvest <query>` | hand-code | Persist extracted DC records to SQLite so a research area is queryable offline without re-hitting the site (framework `sync` owns the `sync` name) | none |
| 2 | Guide-centric index (all theses by a supervisor) | `guide <name>` | hand-code | Requires local join over stored DC.contributor across many theses; UI has no guide-centric view | Use to find every thesis a research supervisor guided. Requires `sync` first to populate the local store. |
| 3 | University research profile | `university stats <name>` | hand-code | Requires aggregating stored theses by university — subject spread, year range, thesis count | none |
| 4 | Subject/year trend analysis | `trends --subject <s>` | hand-code | Requires grouping stored theses by year+subject; no single UI page provides this | none |
| 5 | Similar theses by shared subjects | `similar <handle>` | hand-code | Requires local subject-overlap scoring across the stored corpus | Use to expand a literature review from one thesis. Requires `sync` first. |
| 6 | Offline full-text metadata search | `(framework) search --local` + `sql` | spec-emits | Framework FTS over the synced store — instant, offline, composable | none |

## Stubs
None. All rows above are shipping scope. PDF full-text download is intentionally
**out of scope** (login-gated) and is NOT listed as a feature or stub.

## Notes for generation
- `response_format: html` with `html_extract mode: links` (`link_prefixes: ["/handle/10603"]`)
  gives baseline search/browse. The flagship `thesis get` DC-metadata parse is hand-built
  in `internal/source/dspace/` to guarantee researcher/guide/university/keywords/abstract
  extraction (generated `page` mode may not capture arbitrary `DC.*` meta reliably).
- Local store entity: Thesis. `sync` populates it; transcendence commands read it.
