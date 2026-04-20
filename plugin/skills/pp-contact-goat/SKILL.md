---
name: pp-contact-goat
description: "Printing Press CLI for contact-goat. Super LinkedIn for the terminal - search, enrich, and map warm-intro paths across LinkedIn (stickerdaniel/linkedin-mcp-server subprocess), Happenstance (Chrome cookie auth with Clerk JWT refresh), and Deepline (paid enrichment, hybrid subprocess+HTTP). Unified SQLite store powers warm-intro, coverage, and cross-source prospect commands no single tool has. Capabilities include: analytics, budget, clerk, config, coverage, deepline, dossier, dynamo, engagement, feed, friends, graph, groups, hp, intersect, linkedin, notifications, prospect, research, search, since, tail, uploads, user, warm-intro, waterfall. Trigger phrases: 'install contact-goat', 'use contact-goat', 'run contact-goat', 'contact-goat commands', 'setup contact-goat'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["contact-goat-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/cmd/contact-goat-pp-cli@latest","bins":["contact-goat-pp-cli"],"label":"Install via go install"}]}}'
---

# contact-goat — Printing Press CLI

Super LinkedIn for the terminal - search, enrich, and map warm-intro paths across LinkedIn (stickerdaniel/linkedin-mcp-server subprocess), Happenstance (Chrome cookie auth with Clerk JWT refresh), and Deepline (paid enrichment, hybrid subprocess+HTTP). Unified SQLite store powers warm-intro, coverage, and cross-source prospect commands no single tool has.

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `contact-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/cmd/contact-goat-pp-cli@latest
   ```
3. Verify: `contact-goat-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Auth setup — log in via browser:
   ```bash
   contact-goat-pp-cli auth login
   ```
   Run `contact-goat-pp-cli doctor` to verify credentials.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/cmd/contact-goat-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add -e DEEPLINE_API_KEY=value contact-goat-pp-mcp -- contact-goat-pp-mcp
   ```
   Ask the user for actual values of required API keys before running.
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which contact-goat-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Discover commands: `contact-goat-pp-cli --help`
   Key commands:
   - `analytics` — Run analytics queries on locally synced data
   - `budget` — Deepline credit spend: totals, top tools, and history
   - `clerk` — Get a user's profile by UUID (used for resolving friend/author references)
   - `config` — Configure contact-goat-pp-cli (BYOK providers, defaults, etc.)
   - `coverage` — Show who you know at a company (LinkedIn + Happenstance)
   - `deepline` — Deepline contact-data API: email, phone, and company enrichment (credit-priced)
   - `dossier` — Build a unified person dossier across LinkedIn, Happenstance, and Deepline
   - `dynamo` — Get a search by request_id
   - `engagement` — Score last-touch engagement with a person across all sources
   - `feed` — Get the Happenstance feed (posts from your network)
   - `friends` — List your Happenstance friends (your network's top connectors)
   - `graph` — Export or inspect the unified contact graph
   - `groups` — List your Happenstance groups (hpn-CLI parity, not in sniffed spec)
   - `hp` — Happenstance graph commands (1st / 2nd / 3rd degree people-search)
   - `intersect` — Find people in BOTH your LinkedIn 1st-degree AND Happenstance friends
   - `linkedin` — LinkedIn scraper powered by stickerdaniel/linkedin-mcp-server
   - `notifications` — List Happenstance notifications
   - `prospect` — Fan-out search across LinkedIn, Happenstance, and (opt-in) Deepline
   - `research` — List history
   - `search` — Full-text search across synced data or live API
   - `since` — Time-windowed diff of new items across LinkedIn + Happenstance
   - `tail` — Stream NEW items across LinkedIn + Happenstance (and optionally Deepline)
   - `uploads` — Get the status of the user's uploaded data sources (LinkedIn, Gmail, etc)
   - `user` — Get the current user's usage limits (searches remaining, renewal date)
   - `warm-intro` — Find who in your network can intro you to a target (cross-source)
   - `waterfall` — Clay-style waterfall enrichment: free sources first, Deepline with BYOK or managed
3. Match the user query to the best command. Drill into subcommand help if needed: `contact-goat-pp-cli <command> --help`
4. Execute with the `--agent` flag:
   ```bash
   contact-goat-pp-cli <command> [subcommand] [args] --agent
   ```
5. The `--agent` flag sets `--json --compact --no-input --no-color --yes` for structured, token-efficient output.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
