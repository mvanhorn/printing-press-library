## Customer model

| Persona | Concrete need |
|---|---|
| Python SDK implementer | Needs exact signatures, option names, message block shapes, and examples without guessing or overloading context. |
| Agentic coding assistant | Needs compact, source-grounded bundles that combine symbols, sections, examples, links, and hashes for one implementation task. |
| SDK maintainer / doc watcher | Needs to detect when docs changed, which symbols/examples moved, and whether local snippets are now stale. |
| Team reviewer | Needs mechanical checks that PR code uses documented Claude Agent SDK Python identifiers and patterns. |

## Candidates (pre-cut)

| # | Feature | Source | Candidate command | Score estimate | Long Description |
|---|---|---|---|---:|---|
| 1 | Code usage verifier against documented symbols | user briefing | `verify PATH` | 9/10 | Use `verify` for checking local Python code against documented SDK symbols and signatures. Do not use it for reading symbol docs directly, use `symbol`. |
| 2 | Agent-ready task context bundle | user briefing | `context TOPIC` | 9/10 | Use `context` to assemble a compact implementation bundle across pages, symbols, examples, and guides. Do not use it for raw page reading, use `read`. |
| 3 | Doc change diff by content hash | user briefing, content-hash pattern | `diff --since CACHE_ID` | 8/10 | none |
| 4 | Example-to-symbol coverage map | cross-entity local query | `coverage examples` | 7/10 | none |
| 5 | Minimal recipe composer from existing examples and exact signatures | persona-driven | `recipe TOPIC` | 7/10 | Use `recipe` when the user wants a stitched implementation scaffold from existing documented snippets. Do not use it for listing raw snippets, use `examples`. |
| 6 | SDK surface map by entity type | cross-entity local query | `map --kind classes,types,options` | 7/10 | none |
| 7 | Anchor and link integrity audit | service-specific content patterns | `audit-links` | 6/10 | none |
| 8 | Staleness report for cached docs | content-hash pattern | `stale` | 5/10 | none |
| 9 | Interactive docs TUI browser | persona-driven | `browse` | 3/10 | none |
| 10 | LLM-generated doc summary | persona-driven | `summarize TOPIC` | 2/10 | none |
| 11 | Run all extracted examples locally | persona-driven | `run-examples` | 4/10 | none |
| 12 | Migration planner from arbitrary older SDK code | user briefing | `migrate PATH` | 4/10 | none |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona served | Buildability proof | Buildability | Why Only We Can Do This | Long Description |
|---|---|---|---:|---|---|---|---|---|
| 1 | Code usage verifier against documented symbols | `verify PATH` | 9/10 | Python SDK implementer, team reviewer | This uses parsed local Python files plus the cached docs symbol index to report undocumented imports, names, options, and likely stale identifiers with no external dependencies. | hand-code | It joins local source identifiers to exact docs-derived symbols, signatures, headings, and anchors rather than doing generic grep or linting. | Use `verify` for checking local Python code against documented SDK symbols and signatures. Do not use it for reading symbol docs directly, use `symbol`. |
| 2 | Agent-ready task context bundle | `context TOPIC` | 9/10 | Agentic coding assistant | This uses the local FTS index, section graph, symbol table, examples table, guide cross-links, and content hashes to emit a bounded Markdown or JSON bundle with no external dependencies. | hand-code | It can pack only the exact relevant sections, examples, symbols, and provenance hashes from the local Claude Agent SDK docs mirror. | Use `context` to assemble a compact implementation bundle across pages, symbols, examples, and guides. Do not use it for raw page reading, use `read`. |
| 3 | Doc change diff by content hash | `diff --since CACHE_ID` | 8/10 | SDK maintainer / doc watcher | This uses cached page, section, symbol, example, and link hashes from two sync snapshots to compute added, removed, moved, and changed docs entities with no external dependencies. | hand-code | It compares docs at entity granularity, not just whole-page text, so users see exactly which SDK facts changed. | none |
| 4 | Minimal recipe composer from existing examples and exact signatures | `recipe TOPIC` | 7/10 | Python SDK implementer, agentic coding assistant | This uses topic-matched cached examples plus referenced symbols and signatures to stitch a deterministic scaffold containing only documented snippets and citations with no external dependencies. | hand-code | It mechanically combines examples and signatures already extracted from the docs, producing usable implementation starting points without hallucinated API calls. | Use `recipe` when the user wants a stitched implementation scaffold from existing documented snippets. Do not use it for listing raw snippets, use `examples`. |
| 5 | SDK surface map by entity type | `map --kind classes,types,options` | 7/10 | Python SDK implementer | This uses the cached symbol index and heading hierarchy to emit grouped classes, functions, types, options, message blocks, and anchors with no external dependencies. | hand-code | It turns the docs into a navigable SDK surface inventory across entity types, including anchors and exact source sections. | none |
| 6 | Example-to-symbol coverage map | `coverage examples` | 7/10 | SDK maintainer / team reviewer | This uses extracted code blocks plus the docs symbol table to report which documented symbols have examples and which examples reference undocumented names with no external dependencies. | hand-code | It performs cross-entity joins between code snippets and docs symbols, exposing gaps that raw `examples` or `symbol` commands cannot. | none |
| 7 | Anchor and link integrity audit | `audit-links` | 6/10 | SDK maintainer / doc watcher | This uses cached Markdown links, headings, anchors, and fetched page metadata to report broken internal anchors and unresolved docs references with no external dependencies after sync. | hand-code | It validates the local docs graph at section and anchor level, making the CLI useful for trusting cached context bundles. | none |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| Staleness report for cached docs | Too thin compared with `diff`; it mostly reports old timestamps and hash mismatch without enough distinct user value. | Doc change diff by content hash |
| Interactive docs TUI browser | Scope creep, likely exceeds one-command CLI value and duplicates `read`, `search`, `symbol`, and `context`. | Agent-ready task context bundle |
| LLM-generated doc summary | Requires LLM summarization, which the rubric cuts unless reframed mechanically. | Agent-ready task context bundle |
| Run all extracted examples locally | Execution environment, dependency, and side-effect risks are outside a documentation-first CLI. | Example-to-symbol coverage map |
| Migration planner from arbitrary older SDK code | Too speculative without old-version docs or a confirmed migration source; likely becomes LLM/code-mod scope creep. | Code usage verifier against documented symbols |
