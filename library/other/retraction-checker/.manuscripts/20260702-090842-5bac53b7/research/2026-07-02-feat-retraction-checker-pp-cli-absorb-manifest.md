# Retraction Checker — Absorb Manifest

Scope is deliberately capped by the user: exactly four commands in the first PR.
No competing keyless retraction CLI exists; the closest prior art is our own
scientific-consensus (OpenAlex analytics), which does not do retraction checking.

## Absorbed (foundation, generator-emitted)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search/filter works | Crossref /works | (generated endpoint) works searchWorks | --json/--select/--csv, offline cache |
| 2 | Get work by DOI | Crossref /works/{doi} | (generated endpoint) works getWork | typed output, cache |
| 3 | Health check | framework | (behavior in retraction-checker-pp-cli doctor) | keyless reachability probe |
| 4 | Local sync/SQL/search | framework | (behavior in retraction-checker-pp-cli sync) | SQLite cache of checked works |

## Transcendence (the four shipping-scope commands)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Retraction check | check | hand-code | Parses Crossref update-to/update-by + RETRACTED title into one verdict; resolves PMID→DOI first | Use to verify a single paper is not retracted before citing it. |
| 2 | Bibliography scan | scan | hand-code | Batch line/.bib parsing + fan-out retraction lookups with partial-failure accounting | Use to audit a whole reading list or manuscript for retracted citations. |
| 3 | Superseding research | superseded | hand-code | Cross-source join: Crossref date context + OpenAlex citation-ranked related works published after the original | Use to find current best evidence when a paper is retracted or outdated. AI summary deferred (keyless first PR). |
| 4 | Retraction watch | watch | hand-code | Snapshot baseline of seen retraction notices per topic/list, diff on later runs | Use to monitor a field or personal library for newly-announced retractions. |

Stubs: none. AI synthesis on `superseded` is deferred entirely (not shipped as a stub).
Hand-code count: 4 transcendence commands. Auto-emitted: 2 endpoint commands + framework.
