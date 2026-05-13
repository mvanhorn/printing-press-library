## Absorbed Features Table (inline for subagent)

### Source matrix
- **FAS** = FlowiseAI Swagger spec (`packages/api-documentation/src/yml/swagger.yml`)
- **SDK** = `FlowiseAI/FlowiseSDK` (official TS + Python)
- **MCP-M** = `MilesP46/FlowiseAI-MCP` (Python, claims complete coverage)
- **MCP-W** = `wksbx/flowise-mcp-server` (TypeScript)
- **MCP-MH** = `matthewhand/mcp-flowise` (Python)
- **DOCS** = docs.flowiseai.com/api-reference

### Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List chatflows | MCP-M `chatflow_list`, FAS GET /chatflows | `chatflows list` with `--json --select --csv --name <regex>` | Offline cache via store, regex name filter (MCP-MH parity), JSON-default for agents |
| 2 | Get chatflow by ID | MCP-M `chatflow_get`, FAS GET /chatflows/{id} | `chatflows get <id>` | `--json` typed output, `--from-store` for cached read |
| 3 | Create chatflow | MCP-M `chatflow_create`, FAS POST /chatflows | `chatflows create --stdin-json` | Idempotency via `--dry-run`, stdin-piped batch creation |
| 4 | Update chatflow | MCP-M `chatflow_update`, FAS PUT /chatflows/{id} | `chatflows update <id> --stdin-json` | `--dry-run`, structured exit codes |
| 5 | Delete chatflow | MCP-M `chatflow_delete`, FAS DELETE /chatflows/{id} | `chatflows delete <id>` | `--dry-run`, confirms idempotently |
| 6 | Get chatflow by apikey | FAS GET /chatflows/apikey/{apikey} | `chatflows get-by-apikey <apikey>` | Direct command for the secondary lookup |
| 7 | Run prediction | SDK `createPrediction`, MCP-M `prediction_run`, FAS POST /prediction/{id} | `predict <chatflowId> --question "..." --override-config <json> --history <json> --session <id>` | All body fields exposed as flags; auto-records to store |
| 8 | Stream prediction | SDK streaming + MCP-M `prediction_stream`, FAS streaming=true | `predict <id> --stream` | SSE consumer; pipes tokens to stdout as they arrive |
| 9 | File-upload predict | FAS multipart/form-data on /prediction/{id} | `predict <id> --question "..." --file path/to/x.pdf` | Multi-file via repeated `--file`; agent-friendly |
| 10 | Continue with human input | FAS humanInput field | `predict <id> --human-input <json>` | First-class flag for AgentFlow V2 HITL resume |
| 11 | List chat messages | MCP-M `chatmessage_list`, FAS GET /chatmessage/{id} | `chat-messages list <chatflowId> --since --chat-type --order` | All filters exposed; offline FTS5 search via store |
| 12 | Delete all chat messages | MCP-M `chatmessage_delete_all`, FAS DELETE /chatmessage/{id} | `chat-messages clear <chatflowId>` | `--dry-run`, `--chat-id <id>` for selective clear |
| 13 | Create attachment | MCP-M `attachment_create`, FAS POST /attachments/{chatflowId}/{chatId} | `attachments upload <chatflowId> <chatId> --file path` | Multipart-form-data idiomatic; multi-file in one call |
| 14 | Create assistant | MCP-M `assistant_create`, FAS POST /assistants | `assistants create --stdin-json` | Stdin-JSON, dry-run, structured exit codes |
| 15 | List assistants | MCP-M `assistant_list`, FAS GET /assistants | `assistants list --type --select` | Type filter, offline cache |
| 16 | Get assistant by ID | MCP-M `assistant_get`, FAS GET /assistants/{id} | `assistants get <id>` | JSON-typed, `--from-store` |
| 17 | Update assistant | MCP-M `assistant_update`, FAS PUT /assistants/{id} | `assistants update <id> --stdin-json` | Dry-run, agent-native |
| 18 | Delete assistant | MCP-M `assistant_delete`, FAS DELETE /assistants/{id} | `assistants delete <id>` | Dry-run, idempotent |
| 19 | Create document store | MCP-M `docstore_create`, FAS POST /document-store/store | `docstore create --stdin-json` | Stdin-JSON, dry-run |
| 20 | List document stores | MCP-M `docstore_list`, FAS GET /document-store/store | `docstore list --status --select` | Status filter, offline cache |
| 21 | Get document store by ID | MCP-M `docstore_get`, FAS GET /document-store/store/{id} | `docstore get <id>` | JSON-typed |
| 22 | Update document store | MCP-M `docstore_update`, FAS PUT /document-store/store/{id} | `docstore update <id> --stdin-json` | Dry-run |
| 23 | Delete document store | MCP-M `docstore_delete`, FAS DELETE /document-store/store/{id} | `docstore delete <id>` | Dry-run |
| 24 | Upsert document into store | MCP-M `docstore_upsert`, FAS POST /document-store/upsert/{id} | `docstore upsert <id> --file path --loader <name>` | First-class file flag |
| 25 | Refresh document store | MCP-M `docstore_refresh`, FAS POST /document-store/refresh/{id} | `docstore refresh <id>` | `--wait` option for sync semantics |
| 26 | Query vector store | FAS POST /document-store/vectorstore/query | `docstore query --query "..." --k 5` | Direct retrieval-only path |
| 27 | Delete loader from store | MCP-M `docstore_delete_loader`, FAS DELETE /document-store/loader/{storeId}/{loaderId} | `docstore loader delete <storeId> <loaderId>` | Dry-run |
| 28 | Delete vector store | FAS DELETE /document-store/vectorstore/{id} | `docstore vectorstore delete <id>` | Dry-run |
| 29 | List document chunks | MCP-M `docstore_get_chunks`, FAS GET /document-store/chunks/{storeId}/{loaderId}/{pageNo} | `docstore chunks list <storeId> <loaderId> --page 1` | Pagination flag |
| 30 | Edit chunk | MCP-M `docstore_update_chunk`, FAS PUT /document-store/chunks/{storeId}/{loaderId}/{chunkId} | `docstore chunks update <storeId> <loaderId> <chunkId> --stdin-json` | Dry-run |
| 31 | Delete chunk | MCP-M `docstore_delete_chunk`, FAS DELETE /document-store/chunks/{storeId}/{loaderId}/{chunkId} | `docstore chunks delete <storeId> <loaderId> <chunkId>` | Dry-run |
| 32 | Create feedback | MCP-M `feedback_create`, FAS POST /feedback | `feedback create --stdin-json` | Dry-run |
| 33 | List feedback | MCP-M `feedback_list`, FAS GET /feedback/{id} | `feedback list <chatflowId>` | Offline cache, FTS over content |
| 34 | Update feedback | MCP-M `feedback_update`, FAS PUT /feedback/{id} | `feedback update <id> --stdin-json` | Dry-run |
| 35 | Create lead | MCP-M `lead_create`, FAS POST /leads | `leads create --stdin-json` | Dry-run |
| 36 | List leads | MCP-M `lead_list`, FAS GET /leads/{id} | `leads list <chatflowId>` | Offline cache, CSV export |
| 37 | Ping server | MCP-M `ping`, FAS GET /ping | `ping` | First-class command, `--timeout` |
| 38 | Create tool | MCP-M `tool_create`, FAS POST /tools | `tools create --stdin-json` | Dry-run |
| 39 | List tools | MCP-M `tool_list`, FAS GET /tools | `tools list --select` | Offline cache |
| 40 | Get tool by ID | MCP-M `tool_get`, FAS GET /tools/{id} | `tools get <id>` | JSON-typed |
| 41 | Update tool | MCP-M `tool_update`, FAS PUT /tools/{id} | `tools update <id> --stdin-json` | Dry-run |
| 42 | Delete tool | MCP-M `tool_delete`, FAS DELETE /tools/{id} | `tools delete <id>` | Dry-run |
| 43 | Get upsert history | MCP-M `upsert_history_list`, FAS GET /upsert-history/{id} | `upsert-history list <chatflowId>` | Offline cache |
| 44 | Delete upsert history | MCP-M `upsert_history_delete`, FAS PATCH /upsert-history/{id} | `upsert-history delete <chatflowId> --ids <csv>` | Dry-run, CSV ids |
| 45 | Create variable | MCP-M `variable_create`, FAS POST /variables | `variables create --stdin-json` | Dry-run |
| 46 | List variables | MCP-M `variable_list`, FAS GET /variables | `variables list --type` | Offline cache, type filter |
| 47 | Update variable | MCP-M `variable_update`, FAS PUT /variables/{id} | `variables update <id> --stdin-json` | Dry-run |
| 48 | Delete variable | MCP-M `variable_delete`, FAS DELETE /variables/{id} | `variables delete <id>` | Dry-run |
| 49 | Vector upsert (direct) | FAS POST /vector/upsert/{id} | `vector upsert <chatflowId> --file path` | Multipart upload path |
| 50 | Regex-filter chatflows | MCP-MH whitelist/blacklist by name regex | `chatflows list --name <regex> --not-name <regex>` | Native to list command, also exposed as a `--filter` JSON |
| 51 | Per-flow API key override | SDK `apiKey` per-call | `predict <id> --api-key <key>` | Per-call header override; falls back to env |
| 52 | Base URL override | SDK `baseUrl`, MCP-MH `FLOWISE_API_ENDPOINT` | `--host` flag + `FLOWISE_BASE_URL` env | Standard pattern, per-call override |
| 53 | Static node discovery | MCP-W "node discovery" | (deferred — not in spec) | Listed for posterity; would require Flowise component-introspection endpoint not in v1 spec |

(53 absorbed rows; the 46 OpenAPI endpoints plus 7 ergonomics rows from SDK + MCP wrappers. Row 53 is in-scope only if a discovery endpoint is found in code-mode/docs probing; otherwise dropped.)
