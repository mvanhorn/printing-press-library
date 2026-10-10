# Bland Task Calling CLI Brief

## API Identity
- Domain: Bland AI voice calling platform; API host `https://api.bland.ai`, API key in `authorization` header.
- Users: developers and agents that trigger a specific outbound call and need to follow it through completion.
- Data profile: call metadata, active status, event records, transcripts, extracted variables, summaries, and failure details. The main useful local corpus is the calls and transcripts created by this CLI.

## Reachability Risk
- Low. Official documentation is available as Markdown and describes the relevant v1 endpoints. No OpenAPI document was linked from the docs index; candidate conventional OpenAPI URLs returned 404.
- Live read-only call-history check succeeded through `curl` with the key sourced from the final literal `.zshrc` assignment. Standard Python HTTP requests hit Cloudflare 1010; a curl GET returned 200. Two user-authorized test calls were placed: the first reached call screening and ended without the requested exchange; the second completed the weather task and returned a transcript with answers about outdoor temperature/sky and indoor temperature.
- Automated dogfood must not create calls to real phone numbers. `POST /v1/calls` directly places a potentially billed outbound call. Bland docs state international calls require credits or auto-recharge.

## Users
- **Personal errand delegator:** asks an assistant to handle a phone task such as requesting a restaurant reservation, then checks the call result instead of listening live.
- **Bland agent operator:** maintains a Bland API account, sends task-driven outbound calls during prompt/workflow iteration, and reviews status and transcripts to see whether the agent followed instructions.

## Top Workflows
1. Place one explicitly requested call with `POST /v1/calls` using `phone_number` and a natural-language `task`.
2. Keep the returned `call_id`, inspect active/queued calls, then fetch the call details until it completes.
3. Review the completed transcript, summary, extracted variables, outcome status, and error details.
4. Read the call event stream when diagnosing progress or failure; stop one active call when asked.

## Table Stakes
- Task-based call creation, call ID output, active call listing, call history filters, details/transcripts, event stream, and stopping one active call.
- Stable `--json` output for agent and shell use; API key via `BLAND_API_KEY` and persistent auth config.
- Official Bland CLI already supports `call send --task --wait`, call get/list/events/stop and an MCP server with `create_call`, `get_call_log`, and `wait_for_call`. This printed CLI should keep the user-requested simple task-call-and-follow workflow; its distinct value is still uncertain and is assessed in the absorb gate.

## Data Layer
- Primary entities: calls, statuses, transcripts, events, phone numbers, task prompt, metadata, and extracted variables.
- Cache calls produced by this CLI and fetched details to support local lookup/search; do not store or print credentials. Keep full account-wide synchronization out of the first scope unless the generated workflow requires it.
- No cursor documented for the selected endpoints; call listing supports indexed range, limit (default 1000), sort order, date/time and completion filters.

## User Vision
- “Basically make calls, monitor them, etc.” A user gives it a task such as “call this restaurant and ask for a reservation.” Initial scope is task calls only, not pathway authoring, batches, or account administration. Live test scope was a weather task call to the user's own phone, explicitly authorized by the user, followed through completion; do not place further calls without fresh authorization.
- Call placement is a real-world action and must remain visible as a mutating command in CLI/MCP safety metadata. Never trigger it during automated tests.

## Product Thesis
- Name: Bland Task Caller.
- Why it should exist: provide an agent-friendly, auditable way to make one task-driven phone call and inspect what happened from one CLI/MCP surface.

## Build Priorities
1. Send a task call to a supplied E.164 phone number and return the `call_id`.
2. Monitor: active calls, call details/status/transcripts, and event stream.
3. Allow stopping one active call and list/filter past calls.

## Sources
- User-selected task-call endpoint: https://docs.bland.ai/api-v1/post/calls-simple
- Full call endpoint: https://docs.bland.ai/api-v1/post/calls
- Active calls: filter the supported call list with `GET /v1/calls?completed=false`; the undocumented `/v1/active` path returned HTTP 404 during read-only dogfood.
- Call history: https://docs.bland.ai/api-v1/get/calls
- Call details: https://docs.bland.ai/api-v1/get/calls-id
- Event stream: https://docs.bland.ai/api-v1/get/event-stream
- Stop call: https://docs.bland.ai/api-v1/post/calls-id-stop
- Official CLI: https://docs.bland.ai/sdks/cli
- Call logs and post-call webhooks: https://docs.bland.ai/tutorials/call-logs and https://docs.bland.ai/tutorials/post-call-webhooks
