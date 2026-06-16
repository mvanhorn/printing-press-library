# Claude Agent SDK Python Docs CLI Absorb Manifest

## Sources
- Claude Code docs index: `https://code.claude.com/docs/llms.txt`
- Python Agent SDK reference: `https://code.claude.com/docs/en/agent-sdk/python.md`
- Supporting Agent SDK guides: overview, quickstart, custom tools, sessions, permissions, structured output, and MCP.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Fetch the complete Claude Code docs index | Claude Code `/docs/llms.txt` | claude-agent-sdk-python-docs-pp-cli sync --resources pages | Local cache, content hashes, and repeatable offline access. |
| 2 | Read the Python Agent SDK reference | Claude Code `agent-sdk/python.md` | claude-agent-sdk-python-docs-pp-cli read --page python | Section-aware output with exact Markdown provenance. |
| 3 | Search Agent SDK docs | Claude Code docs website search / Markdown grep | claude-agent-sdk-python-docs-pp-cli search | Offline FTS, `--json`, `--select`, and compact agent output. |
| 4 | Look up functions, classes, types, and message blocks | Python reference headings | claude-agent-sdk-python-docs-pp-cli symbol | Exact signatures, anchors, import hints, and related examples. |
| 5 | Extract code examples | Python reference code blocks | claude-agent-sdk-python-docs-pp-cli examples | Topic/language filters and structured snippets with citations. |
| 6 | Fetch related guide pages | Agent SDK guide docs | claude-agent-sdk-python-docs-pp-cli guide | Cross-linked guidance for sessions, MCP, hooks, permissions, structured output, and custom tools. |
| 7 | Export agent-readable docs context | Claude docs Markdown pages | (behavior in claude-agent-sdk-python-docs-pp-cli context) supports Markdown and JSON bundles | Bounded, source-cited context for downstream agents. |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Persona served | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|---:|---|---|---|---|
| 1 | Code usage verifier against documented symbols | verify PATH | 9/10 | Python SDK implementer, team reviewer | hand-code | Joins local source identifiers to exact docs-derived symbols, signatures, headings, and anchors rather than doing generic grep or linting. | Use this command for checking local Python code against documented SDK symbols and signatures. Do NOT use it for reading symbol docs directly; use `symbol` instead. |
| 2 | Agent-ready task context bundle | context TOPIC | 9/10 | Agentic coding assistant | hand-code | Packs only the relevant sections, examples, symbols, guide links, and provenance hashes from the local Claude Agent SDK docs mirror. | Use this command to assemble a compact implementation bundle across pages, symbols, examples, and guides. Do NOT use it for raw page reading; use `read` instead. |
| 3 | Doc change diff by content hash | diff --since CACHE_ID | 8/10 | SDK maintainer / doc watcher | hand-code | Compares docs at page, section, symbol, example, and link granularity so users see exactly which SDK facts changed. | none |
| 4 | Minimal recipe composer from existing examples and exact signatures | recipe TOPIC | 7/10 | Python SDK implementer, agentic coding assistant | hand-code | Mechanically combines examples and signatures already extracted from the docs, producing implementation starting points without hallucinated API calls. | Use this command when the user wants a stitched implementation scaffold from existing documented snippets. Do NOT use it for listing raw snippets; use `examples` instead. |
| 5 | SDK surface map by entity type | map --kind classes,types,options | 7/10 | Python SDK implementer | hand-code | Turns the docs into a navigable SDK surface inventory across entity types, including anchors and exact source sections. | none |
| 6 | Example-to-symbol coverage map | coverage examples | 7/10 | SDK maintainer / team reviewer | hand-code | Performs cross-entity joins between code snippets and docs symbols, exposing gaps that raw `examples` or `symbol` commands cannot. | none |
| 7 | Anchor and link integrity audit | audit-links | 6/10 | SDK maintainer / doc watcher | hand-code | Validates the local docs graph at section and anchor level, making the CLI useful for trusting cached context bundles. | none |

## Stubs
- None. All approved rows are intended as shipping scope.

## Discovery Decision
- Browser-sniff gate recorded as declined. The raw docs index and Markdown pages are the authoritative, replayable surface for this documentation CLI.
