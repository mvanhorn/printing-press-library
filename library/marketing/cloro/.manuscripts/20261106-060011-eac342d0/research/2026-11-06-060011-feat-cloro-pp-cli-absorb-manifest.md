# cloro CLI Absorb Manifest

Sources surveyed: cloro's hosted MCP server (cloro-dev/mcp-server, the source of mcp.cloro.dev), the published OpenAPI 3.1 spec at https://cloro.dev/docs, and the DataForSEO CLI in this library (its AI-visibility tracker is the nearest existing CLI feature).

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | ChatGPT answer with sources | cloro-dev/mcp-server scrape_chatgpt | cloro-pp-cli chatgpt | Every include add-on as a flag, --json/--select, --dry-run |
| 2 | Google AI Mode answer with sources | cloro-dev/mcp-server scrape_google_ai_mode | cloro-pp-cli aimode | Same flags as the API, typed exit codes |
| 3 | Gemini answer with sources | cloro-dev/mcp-server scrape_gemini | cloro-pp-cli gemini | Same flags as the API, typed exit codes |
| 4 | Google Search with the AI Overview | cloro-dev/mcp-server scrape_google | cloro-pp-cli google --include-aioverview | Pages, device, location and uule targeting as flags |
| 5 | Google News | cloro-dev/mcp-server scrape_google_news | cloro-pp-cli news | --json, --select |
| 6 | Resolve a Google redirect link | cloro-dev/mcp-server resolve_google_redirect | cloro-pp-cli goto | One call, 1 credit |
| 7 | Google shopping product panel | OpenAPI spec | cloro-pp-cli product | Typed flag for the product token |
| 8 | Countries and US states for geo-targeting | cloro-dev/mcp-server list_countries, list_states | cloro-pp-cli countries, states | Free, cached GET |
| 9 | Credit balance | OpenAPI spec | cloro-pp-cli credits | Free |
| 10 | Async tasks (create, batch, get, queue status) | OpenAPI spec | cloro-pp-cli async create, batch, get, queue | 2 credits less per task than sync |
| 11 | MCP server exposing every command | cloro-dev/mcp-server | cloro-pp-mcp | Local stdio server with the same key |

Not absorbed: `DELETE /v1/async/queue` (clears every queued task of the account; left out by operator decision). Engines that cloro does not promote are not in the spec this CLI is generated from.

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Cross-engine citation check | cited | hand-code | Runs one prompt on several engines in parallel and matches a domain against each engine's cited sources; no endpoint compares engines | Use this command to see whether one domain is cited for a prompt. Do NOT use it to read the full answers; use 'chatgpt', 'aimode', 'gemini' or 'google' instead. |

Scope note: one transcendence feature instead of the default minimum of five, by operator direction: add only novel commands that can be tested, because each live run spends credits. The cut is recorded in the novel features brainstorm.
