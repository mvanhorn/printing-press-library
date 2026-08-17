# Conductor Cloud CLI Brief

## API Identity

- Domain: remote coding-agent workspaces and sessions in Conductor Cloud.
- Users: a solo technical operator launching bounded implementation work from Linear; an engineering lead supervising several coding-agent sessions; and an analyst reviewing what agents changed without opening the desktop app.
- Data profile: projects contain workspaces; workspaces contain sessions; sessions contain ordered messages and transcripts. Workspace and session status change asynchronously.

## Contract

- Official source: <https://api.conductor.build/v0/openapi.json>, retrieved 2026-08-05.
- Raw SHA-256: `d7eb7819e0d1338a693f3e52d01ab7249e514ee4c3c40eeaa804f9e1b011163d`.
- OpenAPI 3.0.3 with 20 paths and 21 operations.
- Base host: `https://api.conductor.build`; paths include `/v0` except `/me`.
- Bearer auth via `CONDUCTOR_API_KEY`.
- The live spec labels the full surface experimental.
- List endpoints use `limit` and `offset`, and return `data`, `offset`, and `hasMore`.
- Structured errors expose `userMessage`, `debugMessage`, `retryable`, `source`, details, and nested causes.

## Reachability Risk

- Decision: PASS.
- Evidence: unauthenticated `GET https://api.conductor.build/me` returned HTTP 401 with the expected structured auth error on 2026-08-05.
- Beta drift is the main risk. The pinned raw spec and digest make later changes visible.

## Top Workflows

1. A solo operator starts implementation from a Linear brief: create a workspace and first session, send the task, monitor transcript changes until work starts and later returns to idle, then collect the final transcript and deep link.
2. An engineering lead checks several running sessions during the day, sends steering messages, cancels a bad turn, and confirms cancellation by polling to idle.
3. A planner opens a separate review session in the same workspace, hands an approved plan to an implementation session, then compares their outputs without mixing context.
4. An operator searches recent transcripts across repositories and turns the result into a daily report without opening Conductor.
5. A cleanup pass archives completed sessions and workspaces while preserving an audit trail and avoiding accidental queued-message loss.

## Table Stakes

- Identity and auth doctor.
- Project, workspace, session, message, status, cancel, archive, sleep, rename, and transcript-query commands.
- Full pagination, agent-native JSON, explicit dry-run behavior, structured errors, useful exit codes, and a distinct User-Agent.
- Request-body JSON fallback for the workspace creation union.
- Current model, effort, agent, and channel enums must come from the pinned contract, not a hand-written cross-product.

## Ecosystem Findings

- No existing Conductor Cloud API CLI, SDK, or MCP server surfaced in the public-library search or targeted web searches.
- Search results for `conductor-oss` and Microsoft Conductor refer to unrelated products and were excluded.
- Conductor's own docs and desktop app are the incumbent surfaces. The CLI must match the API itself, then add safe orchestration.

## Data Layer

- Primary entities: projects, workspaces, sessions, messages, status snapshots, and transcript query results.
- Sync cursor: list pagination offset plus message `after` cursor where available.
- Local state should retain session observations and transcript cursors so monitor can distinguish queued false-idle from real completion and can emit incremental updates.

## Product Thesis

- Name: Conductor Cloud CLI.
- Why it should exist: the API exposes the primitives, but operators need bounded session orchestration that handles asynchronous status, queued-message races, cancellation, transcript collection, and agent-friendly output correctly.

## Build Priorities

1. Generate every official endpoint with bearer auth, pagination, errors, and JSON output.
2. Add `launch`, `monitor`, `steer`, `run`, `plan-implement`, and `daily-report` workflows.
3. Test the queued false-idle race, transcript-change completion, async cancellation, archive/reopen behavior, and enum validation.
4. Shipcheck, publish, and install durably for Operator and ENG-526.

## User Vision

ENG-525 defines a programmatic Conductor Cloud control surface for Operator. ENG-526 will consume it for Linear issue to Conductor session workflows and must not duplicate transport or polling logic.
