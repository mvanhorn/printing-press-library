---
name: pp-contact-goat
description: "Super LinkedIn for the terminal. Search, enrich, and map warm-intro paths across LinkedIn (stickerdaniel/linkedin-mcp-server subprocess), Happenstance (cookie-first free quota with bearer-API fallback), and Deepline (paid enrichment). Two Happenstance auth surfaces coexist: Chrome cookie session (free monthly allocation) and HAPPENSTANCE_API_KEY bearer (paid credits, deeper schema). Use when the user asks who they know at a company, how to get a warm intro, who to prospect, or wants cross-source dossiers, network diffs, or waterfall enrichment."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["contact-goat-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/cmd/contact-goat-pp-cli@latest","bins":["contact-goat-pp-cli"],"label":"Install via go install"}]}}'
---

# Contact Goat - Printing Press CLI

Super LinkedIn for the terminal. One CLI that fans out across three sources to answer "who do I know at X", "who can intro me to Y", and "who should I prospect for Z" - then enriches, dedupes, and ranks the answers without you stitching three tools together.

## When to Use This CLI

Reach for this when the user wants:

- coverage of a company (who you already know there, ranked by relationship strength)
- a warm-intro path to a target person (mutual connections across sources)
- prospecting (fan-out search with cross-source dedupe)
- a unified dossier (LinkedIn profile + Happenstance research + optional Deepline enrichment)
- network diffs over time (what's new in your graph in the last N days)
- waterfall enrichment that walks free sources before paid ones

Skip it when the user has a workflow that lives entirely inside LinkedIn Sales Navigator, or when they only need raw LinkedIn scraping with no Happenstance or Deepline overlay (use the LinkedIn MCP directly in that case).

## Two Auth Surfaces

Happenstance has two parallel auth paths and the CLI uses both:

| Surface | Auth | Cost | Default? |
|---------|------|------|----------|
| Cookie web app | Chrome session cookies | Free monthly allocation | YES (auto-prefer) |
| Public REST API | HAPPENSTANCE_API_KEY (Bearer) | 2 credits/search, 1 credit/research | Fallback only |

The auto router prefers cookies until quota is exhausted, then falls back to bearer with an explicit "cost spent" log line on stderr. Use `--source api` on `coverage`, `hp people`, `prospect`, or `warm-intro` to opt into bearer explicitly (e.g. for the richer research schema or scoped group searches). Use `--source hp` to force the cookie surface.

The `api hpn *` subcommands always use the bearer surface and always cost credits. Provision and rotate keys at https://happenstance.ai/settings/api-keys.

## Argument Parsing

Parse `$ARGUMENTS`:

1. Empty, `help`, or `--help` -> run `contact-goat-pp-cli --help`
2. Starts with `install` and ends with `mcp` -> MCP installation (see below)
3. Starts with `install` -> CLI installation (see below)
4. Anything else -> Direct Use (map the request to the best command and run it)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+).
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/cmd/contact-goat-pp-cli@latest
   ```

   If `@latest` installs a stale build (the Go module proxy cache can lag the repo by hours after a fresh merge), install from main directly:
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/cmd/contact-goat-pp-cli@main
   ```
3. Verify: `contact-goat-pp-cli --version`.
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Auth setup - both Happenstance surfaces are optional; configure either or both:
   ```bash
   # Cookie surface (free monthly allocation, requires Chrome on macOS)
   contact-goat-pp-cli auth login --chrome --service happenstance

   # Bearer surface (paid credits, deeper schema, no browser required)
   export HAPPENSTANCE_API_KEY="hpn_live_personal_..."
   ```
6. Verify: `contact-goat-pp-cli doctor` reports both surfaces' status side by side.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/cmd/contact-goat-pp-mcp@latest
   ```

   If `@latest` installs a stale build, install from main directly:
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/cmd/contact-goat-pp-mcp@main
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add -e DEEPLINE_API_KEY=value -e HAPPENSTANCE_API_KEY=value contact-goat-pp-mcp -- contact-goat-pp-mcp
   ```
   Ask the user for actual values of required API keys before running.
3. Verify: `claude mcp list`.

The MCP server exposes 16 tools. The four bearer-API tools added in this release are:

- `api_search` - Run a Happenstance public-API search (costs 2 credits)
- `api_research` - Run a deep-research dossier (costs 1 credit on completion)
- `api_groups_list` - List Happenstance groups for the caller (free)
- `api_usage` - Show live credit balance and usage history (free)

The other 12 tools cover the cookie surface (search, friends, feed, notifications, dossier, etc.) and the LinkedIn / Deepline integrations.

## Direct Use

1. Check if installed: `which contact-goat-pp-cli`. If not found, offer CLI installation (above).
2. Discover commands: `contact-goat-pp-cli --help`. Drill into subcommand help with `contact-goat-pp-cli <command> --help` or `contact-goat-pp-cli api hpn <subcommand> --help`.
3. Match the user query to the best command (see Notable Commands below).
4. Execute with the `--agent` flag for structured, token-efficient output:
   ```bash
   contact-goat-pp-cli <command> [args] --agent
   ```
5. The `--agent` flag sets `--json --compact --no-input --no-color --yes`.

Source routing (cookie vs bearer) is automatic. The auto router prefers the free cookie surface and falls back to the paid bearer surface only when cookie quota is exhausted, logging a "cost spent" notice on stderr. Pass `--source api` to opt into bearer explicitly (richer schema, group-scoped searches), or `--source hp` to force cookies.

## Notable Commands

| Command | What it does |
|---------|--------------|
| `coverage <company>` | Who you know at a company across LinkedIn + Happenstance, ranked by relationship strength |
| `hp people <query>` | Happenstance graph people-search (1st / 2nd / 3rd degree) |
| `prospect <query>` | Fan-out search across LinkedIn + Happenstance (+ opt-in Deepline), deduped |
| `warm-intro <target>` | Mutual connections across sources who could intro you to a target |
| `api hpn search <text>` | Bearer-API search (costs 2 credits, async with poll) |
| `api hpn research <description>` | Bearer-API deep dossier (costs 1 credit on completion) |
| `api hpn usage` | Live credit balance, purchases, recent usage events (free) |
| `doctor` | Check CLI health, both Happenstance surfaces, LinkedIn, and Deepline |

Run any command with `--help` for full flag documentation.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue, including bearer 402 out-of-credits) |
| 7 | Rate limited (cookie 429 or bearer 429; auto-fallback may apply) |
