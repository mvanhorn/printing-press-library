# Groq CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Chat completions (one-shot + SSE streaming, tools, structured output, reasoning) | groq-python / groq-mcp-server groq_ttt | (generated endpoint) chat completions | Local history, usage+latency stats, agent-native --json/--select |
| 2 | Responses API (agentic, remote MCP tools, built-in tools) | groq-python / docs responses-api | (generated endpoint) responses | Remote MCP tool orchestration via API |
| 3 | Vision image analysis | Groq MCP groq_vision | groq-pp-cli vision analyze | Offline model pick, select/compact output |
| 4 | Audio transcription | Groq MCP groq_stt | (generated endpoint) audio transcriptions | Model pick, --json output |
| 5 | Audio translation to English | Groq MCP groq_stt | (generated endpoint) audio translations | |
| 6 | Text-to-speech | Groq MCP groq_tts | (generated endpoint) audio speech | Output path control, voice/model pick |
| 7 | Batch create/list/status/results/cancel | Groq MCP groq_batch | (generated endpoint) batches create/retrieve/list/cancel | Local job history, dry-run |
| 8 | List models | MCP list_chat_models / SDK | groq-pp-cli models list | Offline catalog sync + search |
| 9 | Retrieve model detail | SDK | (generated endpoint) models retrieve | |
| 10 | Delete model | SDK | (generated endpoint) models delete | |
| 11 | Embeddings | groq-python | (generated endpoint) embeddings | |
| 12 | Reranking | groq-python / docs | (generated endpoint) reranking | |
| 13 | Files upload/list/get/delete/download | SDK / MCP groq_batch | (generated endpoint) files upload/list/retrieve/delete/download | |
| 14 | Fine-tuning list/create/get/delete (beta) | groq-python | (generated endpoint) fine_tunings list/create/get/delete | Closed-beta messaging |
| 15 | Chat session history | groq-code-cli, groq-chat | groq-pp-cli history | Persisted SQLite + FTS search (novel/local) |
| 16 | Token usage + tps stats | groq-chat, prab-cli | (behavior in groq-pp-cli chat) | Persisted usage ledger |
| 17 | Model switching mid-session | groq-code-cli, groq-chat | groq-pp-cli chat --model + config default | |
| 18 | Reasoning toggle | groq-code-cli | groq-pp-cli chat --reasoning-effort | |
| 19 | Temp / max tokens / system prompt options | groqchat, groq-code-cli | groq-pp-cli chat flags | |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Rate-limit budget | rate-limits [model] | 10/10 | hand-code | Joins per-model RPM/RPD/TPM/TPD/ASH/ASD limits from the synced models catalog with `x-ratelimit-*` remaining headers captured from every API response into a local ledger table | Brief Users; Product Thesis; spec x-ratelimit-* headers + per-model rate limits | Use this command to inspect per-model rate-limit limits and observed consumption in the current window. Do NOT use it to pace or run bulk jobs; use 'audio batch' for paced bulk audio work. |
| 2 | Cost ledger analytics | costs [--since <dur>] [--group-by model\|day] | 8/10 | hand-code | Joins local completion-history token usage with per-model input/output price columns in the synced models catalog to compute spend grouped by model and day | Brief Users (agent/tool author); Product Thesis | Use this command for aggregate token and dollar-cost analytics across your local completion history, joined to catalog prices. Do NOT use it to compare models on a single prompt; use 'compare' instead. |
| 3 | Multi-model prompt runner | compare "<prompt>" --models <m1,m2,...> | 8/10 | hand-code | Runs the same prompt through N chat-completions calls, collects per-model usage/latency/tokens-per-second, joins catalog prices, renders ranked comparison with agent-shaped --json | Product Thesis; Build Priorities; Top Workflow #3 | Use this command to run one prompt across multiple models and rank them by latency, tokens/sec, usage, and cost. Do NOT use it for aggregate spend over time; use 'costs' instead. |
| 4 | Batch request pre-flight | batch validate <file.jsonl> | 7/10 | hand-code | Validates each `.jsonl` request line against embedded per-endpoint request schemas and estimates per-line and total tokens/cost from catalog pricing, zero API calls (computed) | Top Workflow #5; batch power user persona | Use this command to pre-flight a batch request file before upload. Do NOT use it to analyze a completed batch's results; use 'batch diagnose' instead. |
| 5 | Batch results diagnosis | batch diagnose <batch_id> [--file <results.jsonl>] | 7/10 | hand-code | Downloads a batch's results via the API (or reads a local results file), parses each line's status_code/error fields, tabulates failure breakdowns and retry-worthy errors joined to batch metadata | Top Workflow #5; batch power user persona | Use this command to tabulate per-line status and error breakdowns of a completed batch. Do NOT use it to validate a request file before submission; use 'batch validate' instead. |
| 6 | Paced bulk audio runner | audio batch <dir\|files...> --action transcribe\|translate\|speech [--pace] | 6/10 | hand-code | Loops the transcribe/translate/speech endpoints over a file set, pacing calls from the local rate-limit ledger, and writes a per-file success/failure manifest | Brief Users (audio engineer); Top Workflow #2; absorb covers only single-file audio | Use this command to run transcription/translation/speech synthesis across many audio files with rate-limit-aware pacing and a results manifest. Do NOT use it to inspect your remaining rate-limit budget; use 'rate-limits' instead. |
