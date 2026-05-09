## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Send message | Zylos POST /api/send | send command with --stdin, --file, --dry-run | Piped input, batch, agent-native |
| 2 | Get conversation history | Zylos GET /api/conversations/recent | conversations list with --limit, --json, --select | Offline via SQLite, regex search, SQL composable |
| 3 | Poll for new messages | Zylos GET /api/poll | poll command with --since, --follow (streaming) | Streaming mode, structured output |
| 4 | Check AI status | Zylos GET /api/status | status command with --json, --watch | Periodic monitoring, exit codes for state |
| 5 | Login | Zylos POST /api/auth | auth login with --password flag or env var | Env var support, session persistence |
| 6 | Logout | Zylos POST /api/logout | auth logout | Session cleanup |
| 7 | Check auth state | Zylos GET /api/auth | auth status --json | Structured output, doctor integration |
| 8 | Session management | aichat sessions | Cookie-based session with auto-renew | Persistent config, auto-login |
| 9 | Message formatting | aichat REPL | --format table/json/markdown | Multiple output formats |
| 10 | History search | aichat Ctrl+R | search command with FTS5, regex, SQL | Offline full-text search |
| 11 | Conversation export | llm CLI logs | export command to JSON/Markdown | Portable conversation archives |
| 12 | Multiple input forms | aichat -f flags | send --stdin, send --file | Pipe and file input support |

### Transcendence (only possible with our approach)
| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | Conversation analytics | stats | Requires local SQLite aggregation across all synced messages (counts, durations, response patterns) |
| 2 | Message timeline | timeline | Requires temporal ordering and gap detection across synced conversation history |
| 3 | Conversation search with context | search --context | FTS5 returns surrounding messages for matched results — needs local store to join |
| 4 | Response time tracking | latency | Timestamps on in/out messages enable per-conversation response time analysis in SQLite |
| 5 | Conversation export/import | export / import | Round-trip JSON/Markdown conversation archive using local SQLite |
| 6 | Watch mode for status | status --watch | Periodic polling with state-change detection and exit-on-state transitions |
| 7 | Message streaming | poll --follow | Long-poll streaming with structured output, pipeable to other tools |
