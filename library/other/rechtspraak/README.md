# Rechtspraak CLI

**The first CLI for Dutch court decisions with rich local narrowing for long-tail legal keywords, an appeal-chain walker, and an MCP server.**

Every Dutch ECLI from data.rechtspraak.nl, indexed locally so include/exclude/phrase/regex narrowing cuts a 1000-result topic down to a usable set — the upstream API has no free-text search, so this is the only place that workflow exists. Walks the cassation chain in one command, exports vindplaatsen as BibTeX, and exposes every command as an MCP tool — the first MCP server for Dutch case law.

## Install

The recommended path installs both the `rechtspraak-pp-cli` binary and the `pp-rechtspraak` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install rechtspraak
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install rechtspraak --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install rechtspraak --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install rechtspraak --agent claude-code
npx -y @mvanhorn/printing-press-library install rechtspraak --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rechtspraak-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-rechtspraak --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-rechtspraak --force
```

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install rechtspraak --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rechtspraak-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "rechtspraak": {
      "command": "rechtspraak-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Pull the last week of decisions into the local SQLite cache.
rechtspraak-pp-cli sync --since 7d

# List recent Hoge Raad tax-law decisions — agent-shaped output.
rechtspraak-pp-cli uitspraken search --court HR --subject belastingrecht --agent

# Pull the inhoudsindicatie (summary) for a specific decision.
rechtspraak-pp-cli uitspraken get --id ECLI:NL:HR:2024:1 --summary-only --agent

# Walk the full appeal chain (rechtbank → hof → Hoge Raad plus A-G conclusie).
rechtspraak-pp-cli chain ECLI:NL:HR:2024:1 --agent

# Export the vindplaatsen as BibTeX for a brief's footnotes.
rechtspraak-pp-cli citations ECLI:NL:HR:2024:1 --format bibtex

# Run as an MCP server for Claude Desktop or any MCP-aware agent.
rechtspraak-pp-cli mcp serve

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Graph walks across decisions
- **`chain`** — Walk the cassation, conclusie, and eerdere-aanleg graph for a Dutch court decision in both directions and print the full chain as a tree or JSON.

  _Use this whenever an agent needs the full procedural history of a Dutch decision (cassation chain, A-G conclusie, lower-court rulings) in one call instead of clicking through rechtspraak.nl tabs by hand._

  ```bash
  rechtspraak-pp-cli chain ECLI:NL:HR:2024:1 --depth 5 --agent
  ```
- **`citations`** — Extract the citation list (vindplaatsen) for a decision and emit as BibTeX, CSV, or JSON ready for legal-brief footnotes.

  _Reach for this when an agent is drafting a Dutch legal brief and needs the citation list for a cited decision without parsing raw RDF/XML._

  ```bash
  rechtspraak-pp-cli citations ECLI:NL:HR:2024:1 --format bibtex
  ```
- **`conclusie`** — Given a Hoge Raad decision ECLI, return the matching A-G conclusie (or reverse).

  _When an agent reads a Hoge Raad ruling, immediately pull the Advocate-General's conclusie so the reasoning context is in scope._

  ```bash
  rechtspraak-pp-cli conclusie ECLI:NL:HR:2024:1 --agent
  ```

### Local state that compounds
- **`dossier`** — List every decision sharing a case number across instances, sorted chronologically.

  _Use this when tracking a single Dutch case across the rechtbank, gerechtshof, and Hoge Raad — the API offers no case-file filter, only the local index does._

  ```bash
  rechtspraak-pp-cli dossier 22/00155 --agent
  ```
- **`watch`** — Poll for new decisions matching a filter, dedupe against the local SQLite, and emit only the new ECLIs — cron-friendly with --quiet.

  _Wire this into a cron or agent loop to receive only newly-published Dutch court decisions on your filter — no manual rechtspraak.nl refreshing._

  ```bash
  rechtspraak-pp-cli watch --court HR --subject belastingrecht --since 7d --quiet --agent
  ```
- **`sync archive`** — Download the official weekly full-corpus archive and bulk-insert to the local SQLite — orders of magnitude faster than paging the Atom feed for backfills.

  _Use this when bootstrapping a research corpus or rebuilding the local DB; live sync against the Atom feed is the wrong tool for backfills._

  ```bash
  rechtspraak-pp-cli sync archive --week 2024-W01
  ```

### Agent-native plumbing
- **`code`** — Bidirectional offline lookup against the Instanties controlled vocabulary — court code to full name and PSI URI, or fuzzy name to code.

  _When an agent needs to resolve a Dutch court name, code, or PSI URI without round-tripping to the API, this is the lookup._

  ```bash
  rechtspraak-pp-cli code RBAMS --agent
  ```
- **`mcp serve`** — Expose every rechtspraak command as an MCP tool — the first MCP server for Dutch case law.

  _Wire this into Claude Desktop or any MCP-aware agent to give it native access to Dutch court decisions, citations, and case-files._

  ```bash
  rechtspraak-pp-cli mcp serve
  ```
- **`ecli parse`** — Pure-local regex parse of an ECLI into country, court, year, and sequence — with validation — and `ecli url` for the deeplink.

  _Use this whenever an agent needs to validate or destructure an ECLI before fetching content or chaining into another command._

  ```bash
  rechtspraak-pp-cli ecli parse ECLI:NL:HR:2024:1 --json
  ```

### Search narrowing for long-tail legal terms
- **`narrow`** — Read an ECLI list from stdin (or piped from search/watch/sync), fetch each decision's content from the live API (one HTTP call per ECLI, paced via the shared rate limiter), apply local include/exclude/phrase/regex filters against title+summary+body, and emit the narrowed list — the defining workflow for keyword-overlap-heavy Dutch legal research.

  _Use this when a metadata-only search returns hundreds or thousands of decisions and an agent needs to narrow by phrasing — chain it after search, watch, or sync. The same vocabulary (`--keyword`, `--exclude`, `--phrase`, `--regex`) also lives on `search` directly for single-shot use._

  ```bash
  rechtspraak-pp-cli uitspraken search --subject strafrecht --court HR --from 2024-01-01 --agent | rechtspraak-pp-cli narrow --keyword huurprijs --exclude "kort geding" --phrase "huurprijswijziging" --agent
  ```

## Recipes


### Iteratively narrow an over-broad result set

```bash
rechtspraak-pp-cli uitspraken search --subject strafrecht --court HR --from 2024-01-01 --keyword huurprijs --exclude "kort geding" --phrase "huurprijswijziging" --annotate-count --agent
```

Upstream API returns metadata-only matches; without --scan-body the --keyword/--exclude/--phrase/--regex flags filter against title+summary in-memory (no fetch). With --scan-body each candidate's body is fetched and matched as well. --annotate-count prints total/fetched/post-narrow counts after the run.

### Walk the full procedural history of a Hoge Raad decision

```bash
rechtspraak-pp-cli chain ECLI:NL:HR:2024:1 --agent --select chain.ecli,chain.court,chain.date,chain.type
```

The response is deeply nested across instances; the dotted --select projection keeps the agent's context lean by emitting only the chain skeleton.

### Daily new-decision watch with narrowing

```bash
rechtspraak-pp-cli watch --court HR --subject belastingrecht --since 1d --keyword "omkering bewijslast" --quiet --agent
```

Exits silently if no new decisions match both the upstream filter (HR + belastingrecht + last 24h) AND the local keyword filter — drop-in for cron + mail.

### Pipe-compose narrowing across commands

```bash
rechtspraak-pp-cli uitspraken search --court HR --from 2024-01-01 --json | jq -r '.[] .ecli' | rechtspraak-pp-cli narrow --keyword belastingrecht --exclude conclusie --agent
```

Read ECLIs from any source, apply local filters, emit narrowed list — the same vocabulary as search but composable across pipelines.

### Bibliography export for a brief

```bash
rechtspraak-pp-cli citations ECLI:NL:HR:2024:1 --format bibtex
```

Emits the vindplaatsen as BibTeX entries with stable keys — paste directly into a LaTeX brief.

### Resolve a court name to its PSI URI

```bash
rechtspraak-pp-cli code "Rechtbank Amsterdam" --json --select code,name,psi_uri
```

Fully offline after first sync; useful in agent chains that need to construct a search filter.

## Usage

Run `rechtspraak-pp-cli --help` for the full command reference and flag list.

## Commands

### courts

Dutch courts (Instanties) controlled vocabulary - 79KB authoritative list

- **`rechtspraak-pp-cli courts`** - List every Dutch rechtsprekende instantie with PSI URI, full name, afkorting code, type, and Begin/EndDate. Begin/EndDate enable court-succession lookup (Wet Herziening Gerechtelijke Kaart).

### foreign-courts

Foreign courts (BuitenlandseInstanties) referenced by Dutch decisions

- **`rechtspraak-pp-cli foreign-courts`** - Foreign court catalog: ECHR, CJEU, and ~5000 EU member-state courts with PSI URI, multilingual name, and country code

### foreign-decisions

Non-Dutch decisions registered with LJN codes (NietNederlandseUitspraken)

- **`rechtspraak-pp-cli foreign-decisions`** - Bridge from foreign ECLIs (CJEU, ECHR) to old Dutch LJN codes - ~173k entries

### procedures

Procedure types (Proceduresoorten) controlled vocabulary

- **`rechtspraak-pp-cli procedures`** - Procedure types (cassatie, hoger beroep, kort geding, etc.) with PSI URIs

### relations

Formal relation types (FormeleRelaties) between decisions

- **`rechtspraak-pp-cli relations`** - Relation taxonomy: each relation type pairs from/to court tiers and lists the canonical AfhandelingsWijze (disposition outcomes: bekrachtiging, vernietiging, niet ontvankelijk, etc.)

### subjects

Subject areas (Rechtsgebieden) controlled vocabulary - hierarchical

- **`rechtspraak-pp-cli subjects`** - Subject-area taxonomy with PSI URIs, names, and parent-child hierarchy

### uitspraken

Search and fetch Dutch court decisions (ECLI register)

- **`rechtspraak-pp-cli uitspraken get`** - Fetch a single decision by ECLI - returns full RDF metadata plus the inhoudsindicatie summary and the uitspraak body when available.
- **`rechtspraak-pp-cli uitspraken image`** - Fetch an embedded image from a decision body by its imagedata identifier
- **`rechtspraak-pp-cli uitspraken search`** - Search the ECLI index by date, court, subject, type. Per IVO 1.15: same-type params are OR-unioned, cross-type params are AND-combined. Local --keyword/--exclude/--phrase/--regex flags filter against title+summary by default; --scan-body fetches each entry's body for matching against title+summary+body. --procedure is also filtered locally (the upstream API silently ignores procedure=).


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
rechtspraak-pp-cli courts

# JSON for scripting and agents
rechtspraak-pp-cli courts --json

# Filter to specific fields
rechtspraak-pp-cli courts --json --select id,name,status

# Dry run — show the request without sending
rechtspraak-pp-cli courts --dry-run

# Agent mode — JSON + compact + no prompts in one flag
rechtspraak-pp-cli courts --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
rechtspraak-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/rechtspraak-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **search with `--keyword` or `--phrase` returns zero results despite expected matches** — By default `uitspraken search` matches `--keyword/--exclude/--phrase/--regex` against title + Atom summary only. Party/company names and many substantive terms live in the decision body, not the headnote. Pass `--scan-body` to fetch each entry's body and match against title+summary+body. One HTTP call per entry, rate-limited via the shared limiter, so scope tightly with `--from`/`--to` and `--court` first.
- **search reports a higher total than the entries returned** — A single call fetches one page (size --max, default 100; server caps at 1000). Pass --max-pages N to sweep further pages. The CLI always emits a stderr truncation warning when upstream total > fetched.
- **search returns more results than expected even with `--procedure cassatie`** — The upstream API silently ignores `procedure=`; the CLI filters locally against Proceduresoorten metadata after sync. Confirm the procedure name resolves with `rechtspraak-pp-cli procedures list`.
- **search returns empty results from the API** — Confirm the subject area name resolves to a PSI URI with `rechtspraak-pp-cli subjects list` — many spelling variants exist.
- **get ECLI returns metadata only, no summary** — Pass `--summary` or `--full` to request `return=DOC` from the content endpoint.
- **Atom feed paging is slow for multi-year backfills** — Bulk weekly-archive ingest (`sync archive`) is deferred to v0.2 — see README ## Known Gaps. For backfills today, paginate the search endpoint with `--from-offset N` over a tight date window (server caps at 1000 results per request).
- **court name not recognised** — Run `rechtspraak-pp-cli code <name-or-code>` to see the canonical name and PSI URI.

## Known Gaps

This v0.1 ships the 9 highest-value novel commands fully implemented against the live API. Five additional commands from the original design are deferred to a follow-up polish session — each is honestly stubbed below, with the implementation sketch so a contributor can pick them up cleanly:

- **`sync archive`** — bulk-ingest the official weekly full-corpus archive. The IVO 1.15 doc references a periodic ZIP at the open-data page; the format is not yet probed in this run. The current `sync` walks the Atom feed serially. Implementation: probe the archive URL pattern, parse the ZIP, bulk-insert RDF files. Deferred because it requires understanding the archive format which the public docs only hint at.
- **`judges <name>`** — find all decisions where a contributor (rechter / raadsheer / staatsraad / A-G) participated. The schema exposes `dcterms:contributor`; this needs a local SQLite index built during sync. The Atom feed alone does not include contributors, so this requires per-ECLI content fetches during sync. Implementation: extend the sync command to populate a `contributors` table, then add `judges <name>` as a SQL query.
- **`landmark <name>` / `landmarks list`** — look up "spraakmakende zaken" by nickname (Lindenbaum/Cohen, Quint/Te Poel). Schema provides `dcterms:alternative`. Needs the same local-index pattern as judges.
- **`cites-law BWB:...` ↔ `laws <ECLI>`** — the Dutch statutory-law citation graph. Each Decision's `dcterms:references` carries embedded BWB / CVDR / CELEX / ECLI cross-references with resourceIdentifier attributes. The parser scaffolds these (see `internal/rechtspraak/parser.go`) but the XML namespace handling for body-level refs needs an extra pass.
- **`refs <ECLI>`** — extract every body-level reference (BWB, CVDR, EU CELEX, cross-jurisdiction ECLI) for a decision. Same underlying parsing as `cites-law`.

Track these in [this CLI's polish issue](https://github.com/mvanhorn/printing-press-library/issues?q=is%3Aissue+label%3Arechtspraak).

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**basm92/rechtspraak_cli**](https://github.com/basm92/rechtspraak_cli) — Shell
- [**maastrichtlawtech/rechtspraak-extractor**](https://github.com/maastrichtlawtech/rechtspraak-extractor) — Python
- [**openstate/open-rechtspraak**](https://github.com/openstate/open-rechtspraak) — Python
- [**digitalheir/rechtspraak-js**](https://github.com/digitalheir/rechtspraak-js) — JavaScript
- [**digitalheir/java-rechtspraak-library**](https://github.com/digitalheir/java-rechtspraak-library) — Java
- [**Spijkervet/dutch_jurisdiction_elastic_search**](https://github.com/Spijkervet/dutch_jurisdiction_elastic_search) — Python
- [**maastrichtlawtech/extraction_libraries**](https://github.com/maastrichtlawtech/extraction_libraries) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
