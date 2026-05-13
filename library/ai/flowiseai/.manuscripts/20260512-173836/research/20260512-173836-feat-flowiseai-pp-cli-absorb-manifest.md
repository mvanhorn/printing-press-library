# FlowiseAI CLI Absorb Manifest

## Source legend
- **FAS** = FlowiseAI Swagger (`packages/api-documentation/src/yml/swagger.yml`)
- **SDK** = `FlowiseAI/FlowiseSDK` (TS + Python)
- **MCP-M** = `MilesP46/FlowiseAI-MCP` (claims complete coverage)
- **MCP-W** = `wksbx/flowise-mcp-server`
- **MCP-MH** = `matthewhand/mcp-flowise`

## Absorbed (match or beat everything that exists) — 52 rows

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List chatflows | MCP-M / FAS GET /chatflows | `chatflows list --json --select --csv --name <regex>` | Offline cache, regex name filter (MCP-MH parity), JSON-default |
| 2 | Get chatflow by ID | FAS GET /chatflows/{id} | `chatflows get <id>` | `--json`, `--from-store` cached read |
| 3 | Create chatflow | FAS POST /chatflows | `chatflows create --stdin-json` | Dry-run, stdin-piped batch creation |
| 4 | Update chatflow | FAS PUT /chatflows/{id} | `chatflows update <id> --stdin-json` | Dry-run, structured exit codes |
| 5 | Delete chatflow | FAS DELETE /chatflows/{id} | `chatflows delete <id>` | Dry-run, idempotent |
| 6 | Get chatflow by API key | FAS GET /chatflows/apikey/{apikey} | `chatflows get-by-apikey <apikey>` | Direct command |
| 7 | Run prediction | SDK + FAS POST /prediction/{id} | `predict <chatflowId> --question ... --override-config ... --history ... --session ...` | All body fields as flags; auto-records to local store |
| 8 | Stream prediction | SDK streaming + FAS streaming=true | `predict <id> --stream` | SSE consumer pipes deltas to stdout |
| 9 | File-upload predict | FAS multipart/form-data | `predict <id> --file path/to/x.pdf` (multi-file) | Multipart idiomatic |
| 10 | Human-input resume (HITL) | FAS humanInput field | `predict <id> --human-input <json>` | First-class AgentFlow V2 flag |
| 11 | List chat messages | FAS GET /chatmessage/{id} | `chat-messages list <chatflowId> --since --chat-type --order` | FTS5 search via store |
| 12 | Clear chat messages | FAS DELETE /chatmessage/{id} | `chat-messages clear <chatflowId>` | Dry-run, selective by chat-id |
| 13 | Create attachment | FAS POST /attachments/{cfId}/{chatId} | `attachments upload <cfId> <chatId> --file path` (multi) | Multi-file multipart |
| 14 | Create assistant | FAS POST /assistants | `assistants create --stdin-json` | Dry-run |
| 15 | List assistants | FAS GET /assistants | `assistants list --type --select` | Type filter, offline cache |
| 16 | Get assistant by ID | FAS GET /assistants/{id} | `assistants get <id>` | `--from-store` |
| 17 | Update assistant | FAS PUT /assistants/{id} | `assistants update <id> --stdin-json` | Dry-run |
| 18 | Delete assistant | FAS DELETE /assistants/{id} | `assistants delete <id>` | Dry-run |
| 19 | Create document store | FAS POST /document-store/store | `docstore create --stdin-json` | Dry-run |
| 20 | List document stores | FAS GET /document-store/store | `docstore list --status --select` | Status filter, offline cache |
| 21 | Get document store | FAS GET /document-store/store/{id} | `docstore get <id>` | `--from-store` |
| 22 | Update document store | FAS PUT /document-store/store/{id} | `docstore update <id> --stdin-json` | Dry-run |
| 23 | Delete document store | FAS DELETE /document-store/store/{id} | `docstore delete <id>` | Dry-run |
| 24 | Upsert document | FAS POST /document-store/upsert/{id} | `docstore upsert <id> --file path --loader <name>` | First-class file flag |
| 25 | Refresh document store | FAS POST /document-store/refresh/{id} | `docstore refresh <id>` | `--wait` sync semantics |
| 26 | Query vector store | FAS POST /document-store/vectorstore/query | `docstore query --query "..." --k 5` | Retrieval-only path |
| 27 | Delete loader from store | FAS DELETE /document-store/loader/{storeId}/{loaderId} | `docstore loader delete <storeId> <loaderId>` | Dry-run |
| 28 | Delete vector store | FAS DELETE /document-store/vectorstore/{id} | `docstore vectorstore delete <id>` | Dry-run |
| 29 | List document chunks | FAS GET /document-store/chunks/{storeId}/{loaderId}/{pageNo} | `docstore chunks list <storeId> <loaderId> --page 1` | Pagination |
| 30 | Edit chunk | FAS PUT /document-store/chunks/{storeId}/{loaderId}/{chunkId} | `docstore chunks update <storeId> <loaderId> <chunkId> --stdin-json` | Dry-run |
| 31 | Delete chunk | FAS DELETE /document-store/chunks/{storeId}/{loaderId}/{chunkId} | `docstore chunks delete <storeId> <loaderId> <chunkId>` | Dry-run |
| 32 | Create feedback | FAS POST /feedback | `feedback create --stdin-json` | Dry-run |
| 33 | List feedback | FAS GET /feedback/{id} | `feedback list <chatflowId>` | Offline cache, FTS |
| 34 | Update feedback | FAS PUT /feedback/{id} | `feedback update <id> --stdin-json` | Dry-run |
| 35 | Create lead | FAS POST /leads | `leads create --stdin-json` | Dry-run |
| 36 | List leads | FAS GET /leads/{id} | `leads list <chatflowId>` | CSV export |
| 37 | Ping server | FAS GET /ping | `ping` | `--timeout` |
| 38 | Create tool | FAS POST /tools | `tools create --stdin-json` | Dry-run |
| 39 | List tools | FAS GET /tools | `tools list --select` | Offline cache |
| 40 | Get tool by ID | FAS GET /tools/{id} | `tools get <id>` | `--from-store` |
| 41 | Update tool | FAS PUT /tools/{id} | `tools update <id> --stdin-json` | Dry-run |
| 42 | Delete tool | FAS DELETE /tools/{id} | `tools delete <id>` | Dry-run |
| 43 | Get upsert history | FAS GET /upsert-history/{id} | `upsert-history list <chatflowId>` | Offline cache |
| 44 | Delete upsert history | FAS PATCH /upsert-history/{id} | `upsert-history delete <chatflowId> --ids <csv>` | CSV ids batch delete |
| 45 | Create variable | FAS POST /variables | `variables create --stdin-json` | Dry-run |
| 46 | List variables | FAS GET /variables | `variables list --type` | Type filter, offline cache |
| 47 | Update variable | FAS PUT /variables/{id} | `variables update <id> --stdin-json` | Dry-run |
| 48 | Delete variable | FAS DELETE /variables/{id} | `variables delete <id>` | Dry-run |
| 49 | Vector upsert (direct) | FAS POST /vector/upsert/{id} | `vector upsert <chatflowId> --file path` | Multipart |
| 50 | Regex-filter chatflows by name | MCP-MH whitelist/blacklist | `chatflows list --name <regex> --not-name <regex>` | Native; preserved across `sync` |
| 51 | Per-flow API key override | SDK `apiKey` per-call | `--api-key <key>` flag per call | Falls back to env |
| 52 | Base URL override | SDK + MCP env | `--host` flag + `FLOWISE_BASE_URL` env | Per-call override |

## Transcendence (only possible with our approach) — 10 features

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Newsletter compose | `newsletter compose --plan plan.yml --out draft.md [--dry-run]` | 10/10 | Reads YAML plan; for each section calls POST `/prediction/{chatflowId}` with the section's question + overrideConfig; concatenates `text` fields; records every chatId in local SQLite. `--dry-run` validates flows against the cache. | Brief §"Top Workflows" #1 + §"User Vision" + §"Build Priorities" P2 |
| 2 | Prediction replay | `predict replay <chatId> [--diff]` | 8/10 | Local lookup of original question + chatflowId by chatId; re-fires the prediction; `--diff` shows delta vs recorded. | Brief §"Top Workflows" #5 |
| 3 | Cross-prediction FTS search | `predict search <query> --since 7d --used-tool <name> --cited-store <id> [--chatflow <id>]` | 9/10 | FTS5 over `predictions.question` + `response_text` with JSON1 filters on `usedTools` and `sourceDocuments`. | Brief §"Data Layer" + Sam's audit ritual |
| 4 | Newsletter audit report | `newsletter audit --since 7d --format csv` | 8/10 | SQL join across `predictions` + `chat_messages` + `upsert_history` for the window; per-chatId rows with flow name, source doc count, tool list. | Brief §"Top Workflows" #5 + §"User Vision" CSV |
| 5 | Docstore folder ingest | `docstore ingest <docstoreId> <folder> --pattern '*.pdf' --vector-upsert` | 10/10 | Walks folder; batches multipart POST `/document-store/upsert/{id}`; triggers POST `/vector/upsert/{id}`; records local upsert_history rows. | Brief §"User Vision" "batch import pattern" + §"Build Priorities" P2 |
| 6 | Predict batch fan-out | `predict batch <chatflowId> --input questions.csv --concurrency 4 --out results.ndjson` | 8/10 | CSV/NDJSON of questions → N concurrent predictions → NDJSON output with chatIds. | Brief §"Build Priorities" P2 |
| 7 | RAG corpus drift report | `docstore drift --since 7d` | 8/10 | Joins `document_stores` + `upsert_history`; lists stores with new upserts in the window + the chatflows that reference each store (parsed from flowData). | Brief §"Codebase Intelligence" upsert-history cursor |
| 8 | Chatflow dependency graph | `chatflow deps <chatflowId>` | 7/10 | Parses cached `flowData` JSON; emits tools/assistants/variables/docstores it references; flags missing references against local cache. `--show-overrides` lists overrideConfig keys the flow accepts. | Brief §"Codebase Intelligence" flowData hash + Priya's pre-delete ritual |
| 9 | Stale chatflow finder | `chatflow stale --days 60` | 7/10 | SELECT on local `chatflows.updatedDate` where age > N days; table with id, name, days-stale, deployed-status. | Brief §"Build Priorities" P2 |
| 10 | Predict resume (HITL) | `predict resume <chatId> --input <json>` | 7/10 | Local lookup of suspended AgentFlow V2 chatflow by chatId; fires prediction with `humanInput` body populated. | Brief §"Codebase Intelligence" humanInput documented |

All transcendence features ship full (no stubs).
