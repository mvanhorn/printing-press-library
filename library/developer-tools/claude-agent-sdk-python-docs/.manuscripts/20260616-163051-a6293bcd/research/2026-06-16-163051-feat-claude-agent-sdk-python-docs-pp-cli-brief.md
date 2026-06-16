# Claude Agent SDK Python Docs CLI Brief

## API Identity
- Domain: Claude Code documentation, specifically the Python Agent SDK reference and neighboring Agent SDK guide pages.
- Users: Python developers and AI agents building with `claude-agent-sdk`.
- Data profile: public Markdown docs indexed by `https://code.claude.com/docs/llms.txt`; primary page is `https://code.claude.com/docs/en/agent-sdk/python.md`.

## Reachability Risk
- Low. Raw docs index and Python reference fetched with HTTP 200.
- Authentication: none required.

## Top Workflows
1. Search the Python Agent SDK reference for functions, classes, types, hooks, tools, and message blocks.
2. Fetch a specific reference section such as `ClaudeAgentOptions`, `query()`, `ClaudeSDKClient`, or `ToolUseBlock`.
3. Extract runnable examples for a concept, e.g. custom tools, streaming, sessions, or structured output.
4. Verify local code snippets or identifier usage against the current docs.
5. Build an agent-ready context bundle from a topic across the Python reference and supporting guide pages.

## Table Stakes
- Fetch raw docs without browser automation.
- Search across the full Agent SDK docs index.
- Provide concise `--json`, `--select`, and `--agent` outputs.
- Cache docs locally for offline lookup and fast repeated access.
- Preserve exact symbols, signatures, enum values, and examples from Markdown.

## Data Layer
- Primary entities: docs pages, sections, symbols, examples, cross-links.
- Sync cursor: docs page URL plus fetched timestamp and content hash.
- FTS/search: section title, symbol name, body, code blocks, and tags.

## User Vision
- Build the best CLI for exploring, reading, verifying against, and leveraging the Claude Agent SDK Python docs.

## Product Thesis
- Name: Claude Agent SDK Python Docs CLI
- Why it should exist: agents need exact, current SDK facts without browsing a docs site, guessing symbols, or pasting giant Markdown pages into context. A local searchable docs mirror with symbol and example extraction makes SDK work faster and safer.

## Build Priorities
1. Docs sync and local FTS over `llms.txt`, Python reference, and key Agent SDK guide pages.
2. `search`, `read`, `symbol`, and `examples` commands with structured output.
3. `verify` command that checks Python code for documented SDK symbols and flags likely stale names.
4. `context` command that assembles compact agent-ready bundles for implementation tasks.
5. `diff` or `watch` command that reports doc changes by content hash.
