---
name: pp-contact-goat
description: "Super LinkedIn for the terminal. Search, enrich, and map warm-intro paths across LinkedIn (stickerdaniel/linkedin-mcp-server subprocess), Happenstance (cookie-first free quota with bearer-API fallback), and Deepline (paid enrichment). Two Happenstance auth surfaces coexist: Chrome cookie session (free monthly allocation) and HAPPENSTANCE_API_KEY bearer (paid credits, deeper schema). Use when the user asks who they know at a company, how to get a warm intro, who to prospect, or wants cross-source dossiers, network diffs, or waterfall enrichment."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - contact-goat-pp-cli
    install:
      - kind: go
        bins: [contact-goat-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/cmd/contact-goat-pp-cli
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

## Enrichment Preflight (read this before running any enrichment command)

These commands spend Deepline credits and REQUIRE `DEEPLINE_API_KEY` or a BYOK setup:

- `waterfall` (unless you pass `--byok` and have BYOK providers configured)
- `dossier --enrich-email`
- `deepline find-email` / `enrich-person` / `email-find` / `phone-find`
- `deepline search-people` / `search-companies` / `enrich-company`

Before invoking any of these, verify auth by running doctor first:

```bash
contact-goat-pp-cli doctor --agent | grep -i deepline
```

`deepline_env` shows the resolution source:

- `set (env)` — `DEEPLINE_API_KEY` exported in the current shell
- `set (flag)` — `--deepline-key` passed on the command line
- `set (file:~/.local/deepline/<host>/.env)` — auto-discovered from the official Deepline CLI's persisted key (the user authenticated with `deepline auth register` or `deepline auth status`; no shell export needed)
- `not set` — none of the above. Ask the user for a key, or for a BYOK Hunter/Apollo key

The auto-discovery path means a user who has the Deepline CLI installed and authenticated does NOT need to re-export the key into their shell — contact-goat reads `~/.local/deepline/code-deepline-com/.env` directly (mode 0600, owned by the user). Don't ask the user to re-export when `set (file:...)` is reported. If `deepline_discovery_skipped` is also reported, those are candidate files the resolver rejected for a security reason (wrong mode, missing prefix); surface them so the user can fix the underlying issue.

Provider chain by target kind (waterfall):

| Target | Primary | Fallback 1 | Fallback 2 |
|--------|---------|-----------|-----------|
| LinkedIn URL | apollo_people_match | hunter_people_find | contactout_enrich_person |
| Email | apollo_people_match | hunter_people_find | - |
| Name + --company | dropleads_email_finder | hunter_email_finder | datagma_find_email |

Notes:
- Name targets MUST pass `--company <domain>` (or set `CONTACT_GOAT_COMPANY` env).
- Apollo returns `personal_emails[]` when available; treat `email_status: "unavailable"` as "no verified work email on file" (the personal email is still usable).
- Dropleads returns `status: "catch_all"` for domains on Google Workspace; the email is a pattern guess, not a verified mailbox.
- Provider-level 403s are surfaced as "Provider not connected" rather than "Check DEEPLINE_API_KEY"; they do not abort the chain. The next provider is tried automatically.

## Notable Commands

| Command | What it does |
|---------|--------------|
| `coverage <company>` | Who you know at a company across LinkedIn + Happenstance, ranked by relationship strength |
| `coverage --location <city>` | Who you know in a city. Bearer-only (cookie surface has no city-search); use `--source api`. |
| `hp people <query>` | Happenstance graph people-search (1st / 2nd / 3rd degree). `--csv` emits flat CSV with semicolon-joined bridges. |
| `prospect <query>` | Fan-out search across LinkedIn + Happenstance (+ opt-in Deepline), deduped |
| `warm-intro <target>` | Mutual connections across sources who could intro you to a target |
| `waterfall <target> [--company X]` | Free-sources-first enrichment, falls through to Deepline provider chain. Requires DEEPLINE_API_KEY or --byok. Bare-name targets need --company |
| `dossier <target> [--enrich-email]` | Unified LinkedIn + Happenstance + (optional) Deepline dossier. --enrich-email requires DEEPLINE_API_KEY |
| `deepline find-email "<name>" --company <domain>` | Single-call work-email lookup via dropleads_email_finder |
| `deepline enrich-person <linkedin-url>` | Full person record via apollo_people_match (includes personal_emails[]) |
| `api hpn search <text>` | Bearer-API search (costs 2 credits, async with poll). `--first-degree-only` keeps only 1st-degree matches; `--min-score N` drops weak signals (see docs/scoring.md); `--all --max-results N` auto-paginates. |
| `api hpn research <description>` | Bearer-API deep dossier (costs 1 credit on completion) |
| `api hpn usage` | Live credit balance, purchases, recent usage events (free) |
| `doctor` | Check CLI health, both Happenstance surfaces, LinkedIn, and Deepline. Reports `happenstance_graph_status` (ok / stale / very_stale) on the LinkedIn upload age. |

Run any command with `--help` for full flag documentation.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields, with dotted-path support (see below)
- **Previewable** — `--dry-run` shows the request without sending
- **Cacheable** — GET responses cached for 5 minutes, bypass with `--no-cache`
- **Non-interactive** — never prompts, every input is a flag


### Filtering output

`--select` accepts dotted paths to descend into nested responses; arrays traverse element-wise:

```bash
contact-goat-pp-cli <command> --agent --select id,name
contact-goat-pp-cli <command> --agent --select items.id,items.owner.name
```

Use this to narrow huge payloads to the fields you actually need — critical for deeply nested API responses.


### Response envelope

Data-layer commands wrap output in `{"meta": {...}, "results": <data>}`. Parse `.results` for data and `.meta.source` to know whether it's `live` or local. The `N results (live)` summary is printed to stderr only when stdout is a TTY; piped/agent consumers see pure JSON on stdout.


## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue, including bearer 402 out-of-credits) |
| 7 | Rate limited (cookie 429 or bearer 429; auto-fallback may apply) |
