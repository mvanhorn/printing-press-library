# Groq CLI Novel-Features Brainstorm (subagent audit trail)

Subagent: general-purpose (session ses_fb3f4a5e4ffe4vqeKX2QWTQzHF)
Run: 20260829-110906-2137b57d

## Customer model

- **Maya — prompt-iterating ML engineer.** Today: hops between Playground, docs, API reference; tests prompt tweaks via curl/python in terminal; copies usage numbers into scratch files. Weekly ritual: re-tunes prompts against eval set, compares llama vs gpt-oss vs qwen speed/quality, hand-rolls loops to run one prompt on several models. Frustration: 429s mid-experiment with no remaining-budget visibility; no one-liner to run one prompt across models and rank; cost per experiment is a manual multiplication she never does.
- **Dev — CI / agent tool author.** Today: non-interactive scripts calling Groq from cron, GH Actions, agent harnesses; needs structured output, stable exit codes, streamed completions. Weekly ritual: nightly eval job fires hundreds of chat calls; weekly token-spend reconciliation across projects; grooms pinned models. Frustration: manual spreadsheet-bound cost accounting; shared CI key rate-limit blowups kill nightly runs; can't answer "how much of this window's budget is left?" from the shell.
- **Priya — audio pipeline engineer.** Today: transcribes podcasts, translates interview audio, generates TTS voiceovers with whisper + Orpheus; drives API with shell loops; keeps a text-file manifest of success/failure. Weekly ritual: new episode batches → transcribe→clean→synthesize pipeline with per-model rate-limit juggling. Frustration: long runs die mid-batch on rate limits with no pacing and no resume point; re-runs whole folder re-paying for done files.
- **Sam — batch / fine-tuning power user.** Today: assembles large jsonl batch workloads, uploads, polls status, downloads results, greps for failures; manages training files and closed-beta fine-tuning. Weekly ritual: weekly eval batches; jq-check jsonl, upload, poll with curl, download results, tabulate per-line status/errors by hand. Frustration: a single malformed jsonl line fails the whole batch after upload; error-breakdown tabulation from multi-thousand-line results files by grep; big-batch cost surprises arrive only after submission.

## Candidates (pre-cut)

- C1 Rate-limit budget `rate-limits [model]` (a/b/e) — KEEP
- C2 Cost ledger analytics `costs` (c/e) — KEEP
- C3 Multi-model prompt runner `compare "<prompt>" --models ...` (a/e) — KEEP
- C4 Batch request pre-flight `batch validate <file.jsonl>` (a/b) — KEEP
- C5 Batch results diagnosis `batch diagnose <batch_id>` (a/c) — KEEP
- C6 Paced bulk audio runner `audio batch <dir|files...>` (a/b) — KEEP
- C7 Files ↔ fine-tunings linkage map (c) — KILL (closed-beta, occasional, two list calls suffice)
- C8 TTS voice catalog (b) — KILL (thin rename of `search --type models`)
- C9 Auth health check (e) — KILL (`models list` already exercises auth)
- C10 Task-based model picker (b/e) — KILL (subsumed by compare + search)
- C11 History frequency report (c) — KILL (weak insight, no persona demand)
- C12 Global 429 backoff (e) — KILL (implementation detail, folded into rate-limits + audio batch)
- C13 Batch cost pre-flight (b) — KILL (redundant with batch validate columns)

## Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Rate-limit budget | `rate-limits [model]` | 10/10 | hand-code | Joins per-model RPM/RPD/TPM/TPD/ASH/ASD limits from the synced models catalog with `x-ratelimit-*` remaining headers captured from every API response into a local ledger table, with no external dependencies. | Brief Users; Product Thesis; spec documents x-ratelimit-* headers + per-model rate-limit columns | Use this command to inspect per-model rate-limit limits and observed consumption in the current window. Do NOT use it to pace or run bulk jobs; use 'audio batch' for paced bulk audio work. |
| 2 | Cost ledger analytics | `costs [--since <dur>] [--group-by model\|day]` | 8/10 | hand-code | Joins local completion-history token usage with per-model input/output price columns in the synced models catalog to compute spend grouped by model and day. | Brief Users (agent/tool author cost accounting); Product Thesis | Use this command for aggregate token and dollar-cost analytics across your local completion history, joined to catalog prices. Do NOT use it to compare models on a single prompt; use 'compare' instead. |
| 3 | Multi-model prompt runner | `compare "<prompt>" --models <m1,m2,...>` | 8/10 | hand-code | Runs the same prompt through N chat-completions calls, collects per-model usage/latency/tokens-per-second, joins catalog prices, renders ranked comparison with agent-shaped --json. | Product Thesis; Build Priorities; Top Workflow #3 | Use this command to run one prompt across multiple models and rank them by latency, tokens/sec, usage, and cost. Do NOT use it for aggregate spend over time; use 'costs' instead. |
| 4 | Batch request pre-flight | `batch validate <file.jsonl>` | 7/10 | hand-code | Validates each `.jsonl` request line against embedded per-endpoint request schemas and estimates per-line and total tokens/cost from catalog pricing, zero API calls (// pp:data-source computed). | Top Workflow #5; batch power user persona | Use this command to pre-flight a batch request file before upload. Do NOT use it to analyze a completed batch's results; use 'batch diagnose' instead. |
| 5 | Batch results diagnosis | `batch diagnose <batch_id> [--file <results.jsonl>]` | 7/10 | hand-code | Downloads a batch's results via the API (or reads a local results file), parses each line's status_code/error fields, tabulates failure breakdowns and retry-worthy errors joined to batch metadata. | Top Workflow #5; batch power user persona | Use this command to tabulate per-line status and error breakdowns of a completed batch. Do NOT use it to validate a request file before submission; use 'batch validate' instead. |
| 6 | Paced bulk audio runner | `audio batch <dir\|files...> --action transcribe\|translate\|speech [--pace]` | 6/10 | hand-code | Loops the transcribe/translate/speech endpoints over a file set, pacing calls from the local rate-limit ledger, and writes a per-file success/failure manifest. | Brief Users (audio engineer); Top Workflow #2; absorb covers only single-file audio | Use this command to run transcription/translation/speech synthesis across many audio files with rate-limit-aware pacing and a results manifest. Do NOT use it to inspect your remaining rate-limit budget; use 'rate-limits' instead. |

## Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|--------------------------|
| Files ↔ fine-tunings linkage map | Fine-tuning closed-beta + infrequent; linkage visible via two list calls | batch diagnose |
| TTS voice catalog | Thin rename of `search --type models` | audio batch |
| Auth health check | `models list` already exercises auth | rate-limits |
| Task-based model picker | Subsumed by compare's empirical ranking + search sorting | compare |
| History frequency report | Weak exact-string insight; costs/compare cover real questions | costs |
| Global 429 backoff | Implementation detail — folded into rate-limits + audio batch | rate-limits |
| Batch cost pre-flight | Redundant with batch validate per-line columns | batch validate |
