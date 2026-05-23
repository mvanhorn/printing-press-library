# Vibecode CLI Brief

## API Identity
- Domain: AI-powered app building and cloud deployment platform
- Users: Developers, AI agents (Claude Code, Cursor, Codex, Gemini, OpenClaw), no-code builders
- Data profile: Projects (web/mobile apps), sandboxes (cloud VMs), deployments, agent sessions, git commits

## Reachability Risk
- **Low** - The official vibecode-cli exists and works with a bearer token auth. The API is accessible via `VIBECODE_API_KEY` env var.

## Top Workflows
1. **Build and Deploy (yolo)** - Single command to create project, send prompt to AI agent, and deploy
2. **Project Management** - Create, list, rename, delete projects
3. **Sandbox Development** - Acquire sandbox VM, SSH access, send prompts to agent iteratively
4. **Deployment Management** - Deploy, check status, set custom subdomains/domains, HTTP auth
5. **Debug and Monitor** - SSH into sandboxes/deployments, view logs, check commits

## Table Stakes (from official vibecode-cli)
- Bearer token auth via VIBECODE_API_KEY
- Project CRUD (create webapp/mobile/openclaw, list, get, rename, delete)
- Sandbox management (acquire, kill, SSH, tunnel links)
- Deployment lifecycle (deploy, destroy, ready polling, SSH)
- Agent prompting (send prompts, stop tasks)
- Custom domains and subdomains
- HTTP Basic Auth for deployments
- Git commit history viewing
- Text/JSON output modes

## Data Layer
- Primary entities: Projects, Sandboxes, Deployments, Commits, Links (tunnels)
- Sync cursor: Project ID serves as the primary key for all operations
- FTS/search: Project search by name/query

## Product Thesis
- Name: vibecode-pp-cli
- Why it should exist: The official vibecode-cli is designed primarily for AI agents to consume (skill file output). A Printing Press CLI adds:
  - Offline SQLite storage of projects, sandboxes, deployments
  - Full-text search across all entities
  - Historical tracking of deployments and commits
  - Agent-native structured output (--json, --select, --compact)
  - Typed exit codes for automation
  - Local caching with freshness checks
  - Cross-entity queries (find projects by deployment status, commits by date range)

## Build Priorities
1. Core auth and API client matching official CLI's bearer token pattern
2. Data layer: SQLite store for projects, sandboxes, deployments, commits
3. All absorbed features from official CLI (projects, sandboxes, deployments, agent)
4. Transcendence: offline search, historical tracking, cross-entity analysis, batch operations
5. MCP server for Claude Desktop / agent integration

## Source Analysis
The official vibecode-cli (github.com/vibecode/vibecode-cli) is a Go binary with ~40 commands covering:
- User auth
- Projects (list, get, create, rename, delete, commits)
- Sandboxes (list, get, acquire, kill, ssh, links/list, links/create, links/delete)
- Deployments (list, get, deploy, destroy, ready, ssh, auth/*, subdomain/*, domain/*, links/*)
- Agent (send, stop)
- Yolo (combined agent+deploy)

No competing CLIs or MCP servers found specifically for vibecode.dev platform.

## SDK/Wrapper Analysis
- @vibecodeapp/webapp - npm SDK for web app integration
- @vibecodeapp/v - npm API package
- No Python SDK found
- No official OpenAPI spec found

## Auth Pattern
- Type: Bearer token
- Header: Authorization: Bearer $VIBECODE_API_KEY
- Env var: VIBECODE_API_KEY
- Key source: vibecode.dev/key
