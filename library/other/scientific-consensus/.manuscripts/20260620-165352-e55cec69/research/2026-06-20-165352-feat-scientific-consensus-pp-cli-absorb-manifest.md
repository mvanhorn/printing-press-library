# scientific-consensus CLI — Absorb Manifest

Binary: `scientific-consensus`. Primary source: OpenAlex. Secondaries: PubMed, Crossref, Europe PMC, Semantic Scholar (optional).

## Tools surveyed
- **Consensus.app** — AI search + "consensus meter" (yes/no/mixed), study snapshots, study-type filters. (closed, paywalled, web-only)
- **Scite.ai** — Smart Citations: supporting / contrasting / mentioning. (closed, paywalled)
- **Elicit** — data extraction, systematic-review automation. (closed, freemium)
- **Semantic Scholar** — citation graph, TLDR, influential citations. (free API, rate-limited)
- **PubMed / Europe PMC** — canonical biomedical search, MeSH, pub types. (free)
- **Litmaps / Connected Papers / ResearchRabbit** — citation-network maps. (freemium)
- **SDKs:** `pyalex` (OpenAlex, Py), `metapub`/Biopython Entrez (PubMed), `habanero`/`crossref-commons` (Crossref), `semanticscholar` (S2), `europepmc` clients.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Full-text scholarly search | OpenAlex / PubMed / Europe PMC | `scientific-consensus search "<q>"` | Cross-source + DOI dedup, offline FTS, `--json`/`--select`, ranked |
| 2 | Rank by citation impact | Semantic Scholar / OpenAlex | `(behavior in scientific-consensus search "<q>" --sort citations)` | Offline, composable, `--limit` |
| 3 | Top authors for a topic | Semantic Scholar / OpenAlex | `scientific-consensus rank-authors "<q>"` | Topic publication leaders via group_by, offline; (generated endpoint) authors search covers name lookup |
| 4 | Top institutions for a topic | OpenAlex | `scientific-consensus rank-institutions "<q>"` | Topic institution leaders via group_by, offline |
| 5 | Top journals/venues | OpenAlex / Crossref | `scientific-consensus rank-journals "<q>"` | Topic venue leaders via group_by |
| 6 | Works citing a paper | Semantic Scholar / OpenAlex | `scientific-consensus cited-by <id>` | Cross-source resolution by DOI/PMID |
| 7 | Publication timeline / history | OpenAlex `counts_by_year` | `scientific-consensus timeline "<q>"` | Milestone-year detection, growth curve |
| 8 | Topic trend analysis | Consensus / proprietary | `scientific-consensus trends "<q>"` | YoY growth, emerging vs declining subtopics |
| 9 | Curated reading list + export | Elicit / Litmaps / Zotero | `scientific-consensus curate "<q>"` | Markdown/BibTeX/JSON, ranked, deduped |
| 10 | Landmark / most-influential papers | Semantic Scholar | `scientific-consensus landmark "<q>"` | Citation-impact ranked, field-defining flag |
| 11 | Export results | Zotero / reference mgrs | `scientific-consensus export "<q>" --format md\|json\|csv\|html\|bibtex` | 5 formats, agent-native |
| 12 | TLDR / AI summary | Semantic Scholar / Elicit | `(behavior in search/curate --summarize)` | Keyless S2 TLDR; AI-key enhanced when present; degrades gracefully |
| 13 | Sync to local store | (none — our moat) | `scientific-consensus sync` | Per-source incremental sync into SQLite |
| 14 | Offline SQL/search over corpus | (none) | `(generated endpoint) + scientific-consensus sql` | FTS5 + SQL over works/authors/etc. |
| 15 | List/inspect data sources | (none) | `scientific-consensus sources` | Per-source status, coverage, rate-limit info |
| 16 | Health check + key detection | (none) | `scientific-consensus doctor` | Per-source reachability + optional-key detection |

## Transcendence (only possible with our cross-source + local-store approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Consensus engine | `consensus "<claim>"` | hand-code | Requires stance classification across many works + evidence-tier weighting; no single API returns a consensus verdict | Use to answer "what does the evidence say about X". Produces Consensus Score, Confidence, Evidence Strength, study/pub/citation counts. |
| 2 | Evidence engine + pyramid | `evidence "<query>"` | hand-code | Requires study-design classification (PubMed pubtype → S2 → heuristics) then aggregation into a pyramid; OpenAlex `type` alone is too coarse | Use to see the evidence base distribution (meta-analysis → case report). Do NOT use for stance; use `consensus`. |
| 3 | Claim comparison | `compare "<q1>" "<q2>"` | hand-code | Requires two consensus runs in one local pass and a normalized side-by-side | Use to compare two interventions/claims head-to-head. |
| 4 | Research gap detector | `gaps "<query>"` | hand-code | Requires cross-tabulating study designs, populations, recency over the local corpus | Use to find understudied populations, missing RCTs/replication, future directions. |
| 5 | Controversy detector | `controversies "<query>"` | hand-code | Requires detecting opposing stances + rapidly changing evidence across works | Use to surface conflicting studies and scientific disagreement. |
| 6 | Emerging topics | `emerging` | hand-code | Requires velocity analysis over topic counts in the local store | Use to find fastest-growing research areas / exploding publication trends. |
| 7 | Topic drift | `drift "<query>" --from <y> --to <y>` | hand-code | Requires comparing topic distributions between two time windows | Use to spot emerging vs fading subtopics within a field. |
| 8 | Reproducibility signals | `reproducibility "<query>"` | hand-code | Requires detecting replication studies, sample sizes, pre-registration cues across abstracts | Use to estimate how well-replicated a finding is. |
| 9 | Study quality estimate | `quality "<query>"` | hand-code | Requires combining study design, journal prestige, sample-size cues, citation mass | Use for a rough quality signal per topic/work. |
| 10 | Funding analysis | `funding "<query>"` | hand-code | Requires Crossref funder data joined with works in the local store | Use to analyze who funds research on a topic (funder concentration). |
| 11 | Topic watch / alerting | `watch "<query>"` | hand-code | Requires storing a baseline snapshot and diffing against new pubs on re-run | Use to monitor a topic and surface major new publications since last run. |

## Stubs
None. All transcendence rows are shipping scope (hand-code). AI-enhanced summarization is an optional *layer* inside existing commands (`--summarize`), not a stub — it degrades to keyless TLDR/heuristics.

## Hand-code commitment
- 11 transcendence commands, all `hand-code` (the engines + analytics). Each ~80–200 LoC of Go + `root.go`/parent wiring.
- Plus shared engine packages: `internal/evidence/` (study-type classifier), `internal/consensus/` (stance + scoring), `internal/sources/{pubmed,crossref,europepmc,semanticscholar}/` (secondary clients), `internal/dedup/` (DOI/PMID merge).
- Absorbed analytics commands (authors, institutions, journals, timeline, trends, curate, landmark) are partly generator-emitted (OpenAlex group_by endpoints) and partly hand-wired aggregation.

## Risks / things to worry about before approving
- **Semantic Scholar is 429 keyless** — treated as optional best-effort; never blocks a command. (Confirmed on probe.)
- **Stance classification without an AI key is heuristic** — lexical (negation/effect cues), not a trained classifier. Honest labeling: `consensus` reports method=`heuristic` vs `ai-assisted`. This is the right v1 scope; AI keys upgrade it.
- **Evidence classification accuracy** depends on PubMed pubtype availability; non-PubMed works fall back to title/abstract heuristics (clearly tier-tagged with `method`).
- **Scope is large** (16 absorbed + 11 transcendence). All are in the original spec. Foundation (sync/store/search) is generator-assisted; engines are the hand-built core.
