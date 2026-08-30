# Keenable CLI Absorb Manifest

## Absorbed (match or beat everything found)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Authenticated web search | Keenable OpenAPI `POST /v1/search`; official CLI `search` | (generated endpoint) web_search search_post | Typed request/response, JSON/agent output, retries and API-key env support |
| 2 | Keyless web search | Keenable OpenAPI `POST /v1/search/public` | (generated endpoint) web_search search_post_public | Uses `X-Keenable-Title`, no credential required |
| 3 | Search query | Official CLI and SDK | (behavior in keenable-pp-cli web_search search_post) query positional/flag | Natural-language query with validation and dry-run |
| 4 | Site restriction | API docs, official CLI, MCP | (behavior in keenable-pp-cli web_search search_post) --site | Domain-scoped search |
| 5 | Acquired date filters | API docs, official CLI, MCP | (behavior in keenable-pp-cli web_search search_post) --acquired-after/--acquired-before | Absolute and relative date values preserved |
| 6 | Published date filters | API docs, official CLI, MCP | (behavior in keenable-pp-cli web_search search_post) --published-after/--published-before | Freshness windows without client-side guessing |
| 7 | Point-in-time search | API docs, official CLI, MCP | (behavior in keenable-pp-cli web_search search_post) --query-time | Reproducible index cutoff |
| 8 | Snippet length control | API docs, official CLI | (behavior in keenable-pp-cli web_search search_post) --snippet-max-length | Bounded context size |
| 9 | Result count control | API docs, official CLI | (behavior in keenable-pp-cli web_search search_post) --max-results | Bounded result sets |
| 10 | Authenticated page fetch | Keenable OpenAPI `GET /v1/fetch`; official CLI `fetch` | (generated endpoint) fetch fetch | Markdown page extraction with metadata |
| 11 | Keyless page fetch | Keenable OpenAPI `GET /v1/fetch/public` | (generated endpoint) fetch fetch_public | Public fetch with application title header |
| 12 | Fetch character bound | API docs, official CLI, MCP | (behavior in keenable-pp-cli fetch fetch) --max-chars | Prevents unbounded context growth |
| 13 | Live page fetch | API docs, official CLI, MCP | (behavior in keenable-pp-cli fetch fetch) --live | Supports URLs outside the indexed copy |
| 14 | Prompt extraction | API docs and official CLI | (behavior in keenable-pp-cli fetch fetch) --prompt | Delegates targeted extraction to Keenable while preserving a bounded prompt |
| 15 | Agent-native structured output | Official CLI YAML mode, SDK `to_context`, MCP tools | (behavior in keenable-pp-cli web_search search_post) --json/--agent/--select | Composable JSON and compact context surfaces |
| 16 | Human-readable output | Official CLI `--pretty` | (behavior in keenable-pp-cli web_search search_post) human output | Readable terminal tables without breaking machine output |
| 17 | API key environment support | Official SDK/CLI and docs | (behavior in keenable-pp-cli auth status) KEEENABLE_API_KEY | Canonical env var and safe auth diagnostics |
| 18 | Login/status/logout lifecycle | Official CLI authentication commands | keenable-pp-cli auth login; keenable-pp-cli auth status; keenable-pp-cli auth logout | Secure local credential lifecycle; no secret values in output |
| 19 | MCP configuration guidance | Official CLI `configure-mcp`; official MCP repo | keenable-pp-cli mcp configure | Emits client-specific remote/stdio snippets without requiring a resident browser |
| 20 | MCP reset guidance | Official CLI `reset` | keenable-pp-cli mcp reset | Removes or previews generated client configuration safely |
| 21 | Search mode configuration | Official CLI `config set/get/unset` | keenable-pp-cli config mode set/get/unset | Local defaults for agent pipelines; validates allowed modes |
| 22 | Update guidance | Official CLI `update` | keenable-pp-cli self-update | Print-by-default upgrade guidance; no implicit binary mutation under harness |
| 23 | OpenAI-compatible tool definitions | Official SDK `TOOLS`, `run_tool_call` | (behavior in keenable-pp-cli tools) --json | Export stable search/fetch tool schemas for agent orchestration |
| 24 | MCP read-only tools | Official MCP `search_web_pages`, `fetch_page_content` | (behavior in keenable-pp-cli mcp tools) | Generated command tree is MCP-reachable with read-only hints |
| 25 | Error contracts | OpenAPI responses and official CLI | (behavior in keenable-pp-cli web_search search_post) typed errors | Clear 400/401/402/403/404/422/429 handling with retry hints |
| 26 | Rate-limit visibility | API docs, SDK, official CLI | (behavior in keenable-pp-cli doctor) headers and retry metadata | Agents can distinguish empty results from throttling |
| 27 | Credit awareness | Credits docs and MCP `_meta["keenable/usage"]` | (behavior in keenable-pp-cli doctor) usage metadata | Exposes usage/credit fields when the API returns them |
| 28 | Site/date search integrations | LangChain, LlamaIndex, Haystack, Mastra, Vercel AI SDK, NeMo, Pi, OpenCode, n8n, Dify, Langflow, RAGFlow, Convex, SearXNG, OpenClaw, Hermes | (behavior in keenable-pp-cli tools) stable search/fetch JSON contracts | One CLI surface replaces per-framework wrappers and remains scriptable |
| 29 | Search + fetch competitor parity | Tavily, Exa, Firecrawl, Brave Search, Perplexity Sonar | (behavior in keenable-pp-cli research) local evidence workflows | Adds citations, snapshots, batch retrieval, and offline recall beyond raw provider calls |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---|---|---|---|---|
| 1 | Reproducible research snapshot | research snapshot | hand-code | Persists exact search/fetch inputs, responses, timestamps, and hashes as an immutable local research run. | Use this command to save a deterministic research run for later inspection. Do NOT use it to rerun a prior query and compare fresh results; use `research replay` instead. |
| 2 | Snapshot replay | research replay | hand-code | Re-executes saved query-time/date recipes and compares current results with the local baseline. | Use this command to rerun a saved research recipe and measure change. Do NOT use it to create the first durable run; use `research snapshot` instead. |
| 3 | Citation bundle export | research citations | hand-code | Joins rank, URL, metadata, fetched content, and snapshot identity into portable Markdown/JSON citations. | Use this command to export source-linked evidence from saved runs. Do NOT use it to compare two runs’ additions and removals; use `research diff` instead. |
| 4 | Bounded multi-page fetch | research fetch-many | hand-code | Adds concurrency caps, partial-failure accounting, retry-after handling, and local persistence over the single-page fetch endpoint. | Use this command for a known URL list that needs bounded retrieval. Do NOT use it to rerun a saved search recipe; use `research replay` instead. |
| 5 | Offline local FTS recall | research local-search | hand-code | Searches saved result and Markdown corpora without spending an upstream request or pretending local data is fresh. | Use this command for recall over saved local research. Do NOT use it when fresh upstream results are required; first create or refresh a run with `research snapshot`. |
| 6 | Result/content drift comparison | research diff | hand-code | Computes added/removed URLs, rank movement, metadata changes, and fetched-content hash changes across snapshots. | Use this command to inspect changes between saved runs. Do NOT use it to archive a new run; use `research snapshot` first. |
| 7 | Domain/source coverage report | research coverage | hand-code | Computes domain diversity, result share, rank distribution, publication/acquisition coverage, and missing metadata from local evidence. | Use this command to inspect source breadth and metadata completeness for a saved run. Do NOT use it to emit source-linked citations; use `research citations` instead. |

## Killed candidates

| Feature | Reason |
|---|---|
| research extract-many | Prompt extraction is already absorbed and batching it adds an LLM-dependent surface without a stronger mechanical contract. |
| research evidence-report | Prose synthesis is an LLM dependency; deterministic citation export is safer and verifiable. |
| research freshness | Narrow timestamp view is covered by coverage and diff. |
| research dedupe | Useful persistence plumbing, not a strong recurring user ritual; fold into snapshot/fetch-many. |
| research matrix | Unbounded configurable grid is scope creep; snapshots plus replay cover the verifiable workflow. |
| research rate-status | Rate headers and retries are already table stakes and preserved in diagnostics. |
| research configure-mcp | Official parity, not transcendence; absorbed under the MCP command surface. |

## Approval Notes
- No approved stubs.
- No remote mutation endpoints exist in the Keenable HTTP API. Local credential/config/MCP commands must print by default and refuse side effects under verification or dogfood harnesses.
- The supplied authenticated credential failed a live probe as invalid; public endpoints are reachable and have been exercised successfully. Authenticated dogfood must report this honestly rather than fabricating a keyed pass.
