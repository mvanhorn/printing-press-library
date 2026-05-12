# Orgo CLI Brief

## API Identity
- **Domain:** Cloud-hosted virtual computers for AI agents. Linux desktops accessible via VNC + a Desktop API at `localhost:8080` inside the VM, plus a control-plane REST API at `https://www.orgo.ai/api`.
- **Users:** AI agent builders (Claude Code, Cursor, OpenAI agents, custom harnesses) who need a real Linux desktop the model can click, type, and run code on. Also: dev-tool startups that want to give *their* customers an agent VM (Hermes, OpenClaw, Momentum AMP, Tracer, Magnus).
- **Data profile:** Long-lived state per computer (snapshots, files, processes, screenshots over time), short-lived control commands, and a workspace-level audit trail of what each agent did. Resources are big and stateful; commands are small and frequent.

## Reachability Risk
- **None.** `probe-reachability` reports `mode: standard_http`, confidence 0.95, stdlib transport. Bearer auth, no Cloudflare/WAF interference.
- The user's `ORGO_API_KEY` in `.zshrc` returned `{"error":"Invalid API key"}` on a probe — the key may be stale. Surface at Phase 5; user can rotate.

## Top Workflows
1. **Spin up + control a desktop** — `orgo create` → `orgo screenshot` → `orgo click 400 300` → `orgo bash "ls"` → `orgo stop`
2. **Long-running agent pinned to one VM** — find existing computer by name, ensure-running, run a series of actions, leave running
3. **Fleet ops** — list every workspace's computers, see which are running/idle/suspended, clean up
4. **Cost/quota stewardship** — which computers are oversized, which are idle, when does the user hit the quota
5. **Move workloads between workspaces** — clone a computer, move it, re-attach skills/files

## Table Stakes
- All 24 tools in `@orgo-ai/mcp` (core, admin, screen, shell, files)
- All ~10 methods on the Python SDK `Computer` class (`left_click`, `right_click`, `double_click`, `type`, `key`, `screenshot`, `scroll`, `drag`, `wait`, `bash`, `exec`, `shutdown`)
- Workspace CRUD (create / list / get / delete)
- File ops (upload / list / export / download / delete)
- RTMP streaming (start / status / stop)
- VNC password retrieval, auto-stop config, resize, move, clone

## Data Layer
- **Primary entities:** `computers` (the agent's desktop), `workspaces` (a project owning N computers), `files` (per-workspace blob store), `actions` (every screenshot/click/bash/exec — *not* exposed by the API, but reconstructable from agent invocations against the CLI itself)
- **Sync cursor:** workspace `id` → list computers → upsert by id; refresh cycle on every CLI invocation that has a `--data-source auto` flag
- **FTS/search:** `search` over computer name + status + workspace name; `search` over historical bash commands and exec snippets the CLI itself ran

## Codebase Intelligence
- **Source:** Orgo OpenAPI 3.1.0 v2.0.0 at `https://docs.orgo.ai/api-reference/openapi.json` (29 paths, 29 schemas, Bearer auth)
- **Auth:** `Authorization: Bearer sk_live_...` — flat API key auth, key is created at `https://www.orgo.ai/workspaces`. SDKs read from `ORGO_API_KEY` env var.
- **Data model:**
  - Workspace 1 → N Computers (a computer always belongs to exactly one workspace)
  - Computer has `status` ∈ {creating, starting, running, stopping, stopped, suspended, restarting, deleting, error}; `suspended` = over plan quota after a downgrade
  - Computer has CPU (1/2/4/8/16), RAM (4/8/16/32/64 GB), disk (grow-only), bandwidth, auto-stop minutes
  - Files are scoped to a workspace and optionally to a desktop
- **Rate limiting:** Not documented in spec; observed in production via 429 responses (per-key)
- **Architecture:** Control plane is Next.js on Vercel + Supabase. Each computer is a Fly.io machine running a Linux desktop with a guest Desktop API at `localhost:8080`. WebSockets at `wss://{computer_id}.orgo.dev/{terminal,audio,events}` give live shell, audio, and OS events.
- **Beyond the spec:** WebSocket APIs (terminal, audio, events) are documented in `llms.txt` but not in the OpenAPI spec. RTMP streaming endpoints (`/computers/{id}/stream/{start,status,stop}`) ARE in the spec.

## User Vision
The user is the founder of Orgo and is generating this CLI for the Orgo platform itself. They explicitly asked for a "proper" generation, not a quick one. The CLI is meant to be production-grade and shippable.

## Product Thesis
- **Name:** `orgo-pp-cli` (binary), invocable as `orgo`-suffixed subcommands.
- **Why it should exist:** Today, agents talk to Orgo through three different surfaces: the Python SDK, the npm SDK, and the MCP server. Each is a thin wrapper. None of them give you fleet ops, cost stewardship, or an offline log of what the agent did. A native Go CLI with a SQLite mirror gives agents and humans a single, fast, scriptable surface plus the *audit trail Orgo doesn't otherwise have*.
- **NOI (Non-Obvious Insight):** *Orgo isn't just VM rental. It's an audit ledger of what your agent did. Every screenshot, every bash command, every clone is a signal about whether the agent is doing the right thing — and the signal is more valuable in aggregate than any single call.*

## Build Priorities
1. Computers (full lifecycle, full action surface) — the headline resource
2. Workspaces (CRUD + member-aware listing)
3. Files (upload / list / export / download / delete)
4. Streaming (start / status / stop) — agents that need to broadcast or record
5. Local store + sync (mirror computers + workspaces + a local `actions` log of what the CLI itself did)
6. Transcendence: idle/oversized/suspended computer detection, cost-stewardship, agent action replay, quota forecasting
