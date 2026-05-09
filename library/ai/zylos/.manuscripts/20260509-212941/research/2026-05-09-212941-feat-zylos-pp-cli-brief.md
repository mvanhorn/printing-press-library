# Zylos CLI Brief

## API Identity
- Domain: Local AI chat console at 127.0.0.1:3456
- Users: Developers running a local AI agent with a web-based chat interface
- Data profile: Conversations (messages with direction, content, timestamps), auth sessions, agent status

## Reachability Risk
- Low. Local service on fixed port. No 403/bot-detection issues. Auth is simple password-based cookie session.

## Top Workflows
1. Send a message and get an AI response (primary workflow)
2. Review conversation history and search past messages
3. Check AI agent status (busy/idle/offline)
4. Authenticate and manage sessions
5. Monitor real-time conversation stream

## Table Stakes
- Send messages to the AI agent
- Retrieve conversation history
- Check agent status
- Poll for new messages
- Authentication management (login/logout)
- Structured JSON output for all commands
- Offline access to synced conversation history

## Data Layer
- Primary entities: Messages (id, direction, content, timestamp), Sessions (auth state), Status (agent state)
- Sync cursor: message ID (since_id parameter for polling)
- FTS/search: Full-text search on message content

## Product Thesis
- Name: zylos-pp-cli
- Why it should exist: No CLI exists for Zylos Console. Users must use the web UI to interact with their local AI agent. A CLI enables scripting, automation, piping, and agent-to-agent communication through the Zylos API.

## Build Priorities
1. Core messaging: send, receive, poll, history
2. Status monitoring: check agent state
3. Auth: login/logout/session management
4. Data layer: sync conversations to SQLite, enable offline search
5. Transcendence: conversation analytics, message streaming, export/import
