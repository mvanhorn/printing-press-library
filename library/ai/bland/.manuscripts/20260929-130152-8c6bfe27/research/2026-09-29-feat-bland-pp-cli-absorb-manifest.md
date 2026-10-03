# Bland Task Caller — Absorb Manifest

## Scope and evidence

User-requested scope: place a single task-driven call, monitor it, and inspect what happened. Excludes pathway authoring, batches, and account administration. Two user-authorized calls to the user's own phone were exercised via curl: the first was answered by call screening without a task exchange; the second completed the weather task and returned a transcript with the requested observations. No further live calls are authorized by this test approval.

The official Bland CLI and official MCP already cover call creation, waiting, listing, detail/log inspection, events, and stopping. This print's main possible distinction is a compact task-to-outcome flow with local recall for calls made through this CLI. That distinction is modest and requires explicit approval.

## Absorbed

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Send one task-based outbound call | Bland official CLI `call send --task`; API docs | (generated endpoint) calls create POST /v1/calls | Typed endpoint flags and JSON output; the call command is explicitly marked mutating. |
| 2 | Wait for call completion and retrieve outcome | Bland official CLI `--wait`; official MCP `wait_for_call` | bland-pp-cli calls wait | One CLI flow from returned call ID to final status/transcript/summary. |
| 3 | List active and queued calls | Bland call-list API; GET /v1/calls?completed=false | active GET /v1/calls?completed=false | Filters the supported call-history endpoint to calls not marked complete. |
| 4 | List and filter call history | Bland official CLI call list; GET /v1/calls | (generated endpoint) calls list GET /v1/calls | Typed filters and stable JSON output. |
| 5 | Inspect call status, transcript, summary, variables, and errors | Bland official CLI call get; MCP `get_call_log`; GET /v1/calls/{call_id} | (generated endpoint) calls list GET /v1/calls/{call_id} | One structured record suitable for review and automation. |
| 6 | Read event stream for progress/debugging | Bland official CLI call events; GET /v1/event_stream/{call_id} | (generated endpoint) event-stream GET /v1/event_stream/{call_id} | Direct event history access from the same authenticated CLI. |
| 7 | Stop a live call | Bland official CLI/MCP; POST /v1/calls/{call_id}/stop | (generated endpoint) calls create POST /v1/calls/{call_id}/stop | Available as an explicit, clearly mutating operation. |

## Transcendence

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Task-to-outcome runner | calls task run | 5/10 | hand-code | Creates one task call through POST /v1/calls, polls GET /v1/calls/{call_id} until a terminal status, then prints the final result and caches the returned call details locally. | User's stated errand workflow and successful authorized weather call establish the task-to-result ritual; official CLI has `--wait`, so this overlaps and the extra value is chiefly unified structured output/local recall. | none |
| 2 | Local task recall | calls search-task | 5/10 | hand-code | Searches only call tasks, transcripts, and results cached by this CLI in SQLite, with no external dependency. | Brief identifies call outcome review as the follow-up workflow; local recall can find prior CLI-originated calls by task wording without another API request. Evidence is limited to user workflow; no broad search use was established. | none |

## Candidate audit

The novelty score is intentionally modest because the official CLI/MCP already cover most core operations. The two hand-code commands above are the only recommended differentiators and both require local persistence. No stubs are planned. The `calls task run` operation makes a real outbound phone call and can incur charges; do not execute live calls during automated validation.
