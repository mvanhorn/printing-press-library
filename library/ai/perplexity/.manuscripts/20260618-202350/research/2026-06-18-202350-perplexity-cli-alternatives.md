# Perplexity CLI Alternatives

## Discovered Tools

| Tool | Language | Stars | Surface | Gap |
|------|----------|-------|---------|-----|
| `dawid-szewc/perplexity-cli` | Python | 172 | API wrapper | Assumes API auth; no browser-session export trail |
| `xerexcoded/pplx-cli` | Python | 32 | API wrapper | Focused on query execution, not trace export |
| `noQuli/perplexity-cli` | TypeScript | 32 | CLI | Wrapper-style surface; no local research archive |
| `sgaunet/pplx` | Go | 18 | CLI | Minimal command surface; no session preservation |
| `nileshr/pplx-cli` | TypeScript | 3 | CLI | Small wrapper, not an archive tool |

## Pattern Analysis
- Most existing tools are thin API wrappers.
- None of the alternatives centers browser-session trace export as the primary value.
- JSON output and scripting help are common, but durable local research capture is not.

## Gap Analysis
- Browser-session auth is not the main path in the existing tools.
- Trace export into local knowledge stores is missing.
- Recent-thread browsing and transcript reading are not packaged as an archive workflow.

## Recommendation
- **Proceed-with-gaps.**
- The tool is still worth building because the real differentiator is preservation of the user's own research trail, not another query wrapper.
