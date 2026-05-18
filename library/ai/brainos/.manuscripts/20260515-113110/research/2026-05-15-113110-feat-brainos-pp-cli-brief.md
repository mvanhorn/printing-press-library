# BrainOS CLI Brief

## API Identity
- Domain: Personal AI agent orchestration backend (Supabase/PostgREST, Swagger 2.0)
- Users: One user — Devin — who runs a multi-agent AI system with BrainOS, NatureOS, Navigator trading, and MCP infrastructure
- Data profile: 35 tables, 53 endpoints. High-value entities: thoughts (with embeddings), trading sessions/premortems/postmortems, mcp_servers/tools, natureos_dual_model_tasks, agent_messages, shared_state

## Reachability Risk
- Low. The API is the user's own Supabase project. Requires service_role key for full access. Anon key gives restricted access (RLS policies apply).
- Note: spec only accessible via service_role key. CLI should support both BRAINOS_SERVICE_KEY and BRAINOS_ANON_KEY.

## Top Workflows
1. **Brain inspection** — Search/list thoughts, check active_memory by agent, browse documents with FTS
2. **Trading review** — Query today's session, review premortems, analyze postmortems, check calibration stats
3. **MCP infrastructure** — List mcp_servers and their status, review mcp_activity_logs, check auth errors
4. **Agent coordination** — List agent_messages, check natureos_task_queue status, inspect executor_briefs
5. **Offline analytics** — Cross-table queries: trading win rates by setup, MCP tool usage patterns, memory load by agent

## Table Stakes (from Supabase CLI + brain-mcp + existing tools)
- Table list with filter/search (via PostgREST)
- JSON output for every command
- CRUD operations (get, post, patch, delete) on all tables
- Auth via API key env var
- `doctor` / health check

## Data Layer
- Primary entities: thoughts, trading_premortems, trading_postmortems, trading_sessions, mcp_servers, agent_messages, natureos_dual_model_tasks
- Sync cursor: `created_at` / `updated_at` columns (present on most tables)
- FTS/search: `thoughts.content` (has embedding), `documents.content`, `agent_messages.content`, `natureos_dual_model_tasks.goal`
- Offline joins: trading win-rate analytics, MCP latency percentiles, memory load by agent

## Codebase Intelligence
- Auth: `apikey` header (PostgREST), also supports `Authorization: Bearer` JWT
- Rate limiting: None expected on user's own project
- Architecture: PostgREST thin layer over Postgres; all filters via query params (`?column=eq.value`, `?select=col1,col2`)
- brain-mcp: capture_thought, list_thoughts, search_thoughts, thought_stats commands already exist as MCP tools

## Product Thesis
- Name: `brainos-pp-cli`
- Why it should exist: The user's AI infrastructure lives in Supabase but has no unified CLI. Individual domains (trading, memory, MCP, agents) each require ad-hoc SQL or MCP calls. A single binary with offline SQLite sync enables cross-domain analytics, agent-native output, and sub-second queries without hitting the API every time.

## Build Priorities
1. Sync + offline SQLite for all high-value tables (thoughts, trading_*, mcp_*, agent_messages)
2. Domain-specific commands: `thoughts search`, `trading session`, `trading review`, `mcp servers`, `agents queue`
3. Cross-table analytics: trading performance, MCP reliability, memory load, agent throughput
