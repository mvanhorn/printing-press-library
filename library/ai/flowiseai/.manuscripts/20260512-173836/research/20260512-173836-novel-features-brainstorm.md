# Novel Features Brainstorm (subagent audit trail)

## Customer model

### Persona 1: Hermes Agent "Marcus" — the autonomous marketing manager

**Today (without this CLI).** Marcus is a Hermes agent assigned the role of marketing manager for a real-estate brokerage's weekly newsletter. Without a purpose-built CLI, Marcus must either write ad-hoc Python against `flowise-sdk`, shell out to `curl` with hand-rolled `Authorization: Bearer` headers, or wire up MCP servers that only Claude Desktop can reach. Each newsletter cycle costs hundreds of context tokens just to remember which chatflow ID maps to "market summary" versus "neighborhood spotlight."

**Weekly ritual.** Every Monday morning Marcus has to (1) pull the latest MLS export and market PDFs into a Flowise document store, (2) trigger five section chatflows in sequence, (3) concatenate the `text` field of each response into a single markdown draft, (4) hand that draft to a distribution chatflow that fires SendGrid, and (5) record `chatId` for every prediction so the human supervisor can audit what was generated.

**Frustration.** Marcus has no way to express "compose newsletter from these five flows" as one shell command. He fans out manually, parses JSON manually, and his transcripts balloon with raw API response bodies that he only needs three fields from. When something goes wrong in a 5-flow run, replaying it is impossible — the chatId is buried in a prior turn's tool output that may have been compacted away.

### Persona 2: Priya, the developer-operator who owns the Flowise instance

**Today (without this CLI).** Priya runs the self-hosted Flowise server that the Hermes fleet calls into. She authors the visual chatflows in the Flowise UI but has no command-line way to see what's on the server. To audit "which chatflows haven't been touched in 60 days" or "which document stores have stale upsert history," she opens the Flowise web UI and clicks through each item.

**Weekly ritual.** Friday afternoons she pulls a maintenance pass: deletes stale chatflows, re-indexes document stores whose source PDFs changed, reviews lead capture from the public chatbot, and exports the past week's chat messages for a compliance archive.

**Frustration.** The Flowise UI doesn't surface staleness, cross-entity joins, or batch operations. She has to query the underlying SQLite by hand or click 40 times. There's no CLI that respects her workflow — the official `flowise` npm package is a server lifecycle tool, not an API client.

### Persona 3: Sam, the QA engineer auditing agent-generated newsletters

**Today (without this CLI).** Sam is the human-in-the-loop checking that Marcus's agent didn't hallucinate listing prices. Each week Sam wants to read every prediction the agent made, see which source documents the RAG layer cited, and confirm tool calls landed where they should.

**Weekly ritual.** Sam grabs the previous week's chatIds from Marcus, calls `/chatmessage/{id}` for each one, manually diffs `sourceDocuments` against the actual MLS export, and writes a one-paragraph audit note.

**Frustration.** Sam can't full-text-search across a week of predictions. He can't ask "which predictions cited document store X?" or "show me every response that used the SendGrid tool." The chat history endpoint returns one chatflow at a time and dumps raw JSON.

## Candidates (pre-cut)

(See full subagent transcript — 19 candidates generated across sources a/b/c/e/f; cut to 10 survivors below.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Newsletter compose (fan-out) | `newsletter compose --plan plan.yml --out draft.md [--dry-run]` | 10/10 | Reads YAML plan; for each section calls POST `/prediction/{chatflowId}` with the section's question + overrideConfig; concatenates `text` fields; records every chatId in local `predictions` SQLite. `--dry-run` validates flows exist + are deployed against local cache. Persona: Marcus. | Brief §"Top Workflows" #1 + §"User Vision"; brief §"Build Priorities" P2 lists it first |
| 2 | Prediction replay | `predict replay <chatId> [--diff]` | 8/10 | Local SQLite lookup of original question + chatflowId by chatId; re-fires POST `/prediction/{id}`; optional `--diff` shows text + sourceDocuments delta against the recorded response. Persona: Sam, Marcus. | Brief §"Top Workflows" #5 ("audit what was generated, recover from a failed run, or re-issue a corrected version") |
| 3 | Cross-prediction FTS search | `predict search <query> --since 7d --used-tool <name> --cited-store <id> [--chatflow <id>]` | 9/10 | FTS5 over `predictions.question` + `response_text` with JSON1 filters on `usedTools` and `sourceDocuments` columns. Persona: Sam. | Brief §"Data Layer" explicitly lists FTS5 on these columns; brief §"Top Workflows" #5 + Sam's audit ritual |
| 4 | Newsletter audit report | `newsletter audit --since 7d --format csv` | 8/10 | SQL join across `predictions` + `chat_messages` + `upsert_history` for the time window; emits per-chatId rows with flow name, source doc count, tool list. Persona: Sam. | Brief §"Top Workflows" #5; brief §"User Vision" agent-first ergonomics calls out CSV output |
| 5 | Docstore folder ingest | `docstore ingest <docstoreId> <folder> --pattern '*.pdf' --vector-upsert` | 10/10 | Walks folder, batches files into multipart POST to `/document-store/upsert/{id}`, then triggers POST `/vector/upsert/{id}`, records each ingestion row in local `upsert_history`. Persona: Priya, Marcus. | Brief §"User Vision" calls it a "batch import pattern"; brief §"Build Priorities" P2 lists `docstore ingest` |
| 6 | Predict batch fan-out | `predict batch <chatflowId> --input questions.csv --concurrency 4 --out results.ndjson` | 8/10 | Reads CSV/NDJSON of questions, runs N concurrent POST `/prediction/{id}` calls, streams results as NDJSON with chatIds preserved. Persona: Marcus. | Brief §"Build Priorities" P2 lists `predict batch (CSV/NDJSON fan-out)` |
| 7 | RAG corpus drift report | `docstore drift --since 7d` | 8/10 | Joins local `document_stores` + `upsert_history`; lists stores with new upserts in the window plus the chatflows that reference each store (parsed from `chatflows.flowData`). Persona: Priya. | Brief §"Codebase Intelligence" calls upsert-history the change cursor; brief §"Data Layer" lists upsert_history |
| 8 | Chatflow dependency graph | `chatflow deps <chatflowId>` | 7/10 | Parses cached `flowData` JSON for the flow; emits the tools/assistants/variables/docstores it references; flags missing references against local cache. Persona: Priya. | Brief §"Codebase Intelligence" notes flowData hash is stored; Priya's Friday "before I delete this flow" ritual |
| 9 | Stale chatflow finder | `chatflow stale --days 60` | 7/10 | SELECT on local `chatflows.updatedDate` where age > N days; emits table with id, name, days-stale, deployed-status. Persona: Priya. | Brief §"Build Priorities" P2 lists `chatflow stale --days N` |
| 10 | Predict resume (HITL) | `predict resume <chatId> --input <json>` | 7/10 | Local SQLite lookup of suspended AgentFlow V2 chatflow by chatId; fires POST `/prediction/{chatflowId}` with `humanInput` body populated. Persona: Marcus, Sam. | Brief §"Codebase Intelligence" documents the humanInput field; absorb manifest row 10 confirms HITL pattern is first-class |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C8 predict profile preset | Marginal value over `--override-config '{...}'` flag; local profile store is yet-another-config-format with no weekly use | Use shell aliases or env-var passthrough |
| C10 predict stream tee | Stream-to-file is one-flag scope; C1 and C7 already record output to disk | C1 newsletter compose (writes to --out) |
| C13 leads trace | Narrower than C4 audit and not a weekly need; lead followups aren't in Marcus's workflow | C4 newsletter audit |
| C15 newsletter dry-run | Not a standalone command — folded into C1 as `--dry-run` flag | C1 newsletter compose |
| C16 newsletter regen | Requires a newsletter-draft persistence layer that's scope creep; C1 + C2 cover the recovery path | C1 + C2 |
| C17 tokens estimate | Historical mean is weak signal; agent can derive this from C3 with `--select` | C3 predict search |
| C18 explain-config | Folded into C8 chatflow deps' adjacent capability via `chatflow deps --show-overrides` flag, not a separate command | C8 chatflow deps |
| C19 assistants bundle | Scope creep — bundle format adds protocol complexity for a rare migration use case | API-native `assistants get` + `tools get` chained in shell |
| C12 tools usage heatmap (standalone) | Folded into C4 audit's tool column | C4 newsletter audit |
