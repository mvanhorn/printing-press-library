# Normattiva CLI

**Italian law in your terminal: resolve citations, read any article at any date, diff versions and track amendments.**

Type citations the way lawyers write them ("art. 2043 c.c.", "d.lgs. 196/2003 art. 13") and get the authoritative Normattiva text as clean text (or Markdown with --formato markdown), at today's date or any past date. On top of search and reading: verifica checks citations in a draft, diff shows how an article changed, novita reports changes to the acts you follow with segui add, and fascicolo builds date-pinned context packs for filings and AI agents.

## Install

The recommended path installs both the `normattiva-pp-cli` binary and the `pp-normattiva` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install normattiva
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install normattiva --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install normattiva --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install normattiva --agent claude-code
npx -y @mvanhorn/printing-press-library install normattiva --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.10 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/normattiva/cmd/normattiva-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/normattiva-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install normattiva --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-normattiva --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-normattiva --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install normattiva --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/normattiva-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/normattiva/cmd/normattiva-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "normattiva": {
      "command": "normattiva-pp-mcp"
    }
  }
}
```

</details>

### Remote MCP over HTTP

`normattiva-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `NORMATTIVA_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `NORMATTIVA_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export NORMATTIVA_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
normattiva-pp-mcp --transport http
```

Set `NORMATTIVA_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Authentication

No account or API key needed: the Normattiva Open Data API is public. Be gentle with it — the CLI caches article text locally and paces its requests. Every result carries a permanent link on www.normattiva.it (https://www.normattiva.it/uri-res/N2Ls?<URN>).

## Quick Start

```bash
# Check the CLI is installed and configured, without touching the network
normattiva-pp-cli doctor --dry-run

# List well-known acts you can call by name, like the codice civile
normattiva-pp-cli fonti codice

# Read one article as clean text
normattiva-pp-cli testo "codice civile" --articolo 2043

# Search acts by keyword
normattiva-pp-cli cerca "intelligenza artificiale" --per-pagina 5

# Check a citation was in force on a date
normattiva-pp-cli verifica "art. 3 L. 241/1990" --al 2024-01-01

```

## Unique Features

### Citation checking and context packs
- **`verifica`** — Check every Italian law citation in a draft or an AI answer: does it exist, and was it in force on a given date?

  _Reach for this before trusting any generated legal citation; non-zero exit means at least one citation is wrong or no longer in force._

  ```bash
  normattiva-pp-cli verifica "art. 2043 c.c." "art. 3 L. 241/1990" --al 2024-01-01 --agent
  ```
- **`fascicolo`** — Assemble several articles from different laws, all pinned to one reference date, into one Markdown document.

  _Use it to give an agent or a filing the exact text of every relevant norm as of one date._

  ```bash
  normattiva-pp-cli fascicolo "art. 2043 c.c." "art. 3 L. 241/1990" --al 2021-05-10
  ```

### Law across time
- **`diff`** — See word by word what an article said on one date versus another, or versus its original text.

  _Use it when the applicable version of a norm depends on the date of the facts (tempus regit actum)._

  ```bash
  normattiva-pp-cli diff "d.lgs. 196/2003" --articolo 2-ter --dal 2019-01-01
  ```
- **`novita`** — Follow the acts you care about (segui add) and get a report of which ones and which articles changed since your last check.

  _Use it for recurring compliance monitoring instead of scanning the whole national updates feed._

  ```bash
  normattiva-pp-cli novita --agent
  ```

## Recipes

### Verify citations in an AI answer

```bash
normattiva-pp-cli verifica --file risposta.txt --agent
```

Extracts every citation, resolves it and exits non-zero if any is missing or no longer in force.

### What changed in an article

```bash
normattiva-pp-cli diff "d.lgs. 196/2003" --articolo 2-ter --dal 2019-01-01
```

Word-level diff between two point-in-time versions, with both vigenza windows.

### Lean search results for agents

```bash
normattiva-pp-cli cerca "protezione dati" --agent --select totale,risultati.descrizione,risultati.urn,risultati.data_gu
```

Search responses carry long titles and facets; select only the fields you need.

### Weekly compliance check

```bash
normattiva-pp-cli novita
```

After adding acts with segui add "d.lgs. 231/2001" --articoli 5,6, reports which followed acts and articles changed since the last check.

### Bulk export, gated

```bash
normattiva-pp-cli esporta --tipo LEGGE --anno 1990 --numero 241 --formato AKN
```

Prints the export request only; add --avvia to queue it on Normattiva and --attendi --output file.zip to download. scarica likewise downloads only with --output.

## Usage

Run `normattiva-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `NORMATTIVA_CONFIG_DIR`, `NORMATTIVA_DATA_DIR`, `NORMATTIVA_STATE_DIR`, or `NORMATTIVA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `NORMATTIVA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export NORMATTIVA_HOME=/srv/normattiva
normattiva-pp-cli doctor
```

Under `NORMATTIVA_HOME=/srv/normattiva`, the four dirs resolve to `/srv/normattiva/config`, `/srv/normattiva/data`, `/srv/normattiva/state`, and `/srv/normattiva/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:NORMATTIVA_HOME = "C:\srv\normattiva"
normattiva-pp-cli doctor
```

Under `$env:NORMATTIVA_HOME = "C:\srv\normattiva"`, the four dirs resolve to `C:\srv\normattiva\config`, `C:\srv\normattiva\data`, `C:\srv\normattiva\state`, and `C:\srv\normattiva\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "normattiva": {
      "command": "normattiva-pp-mcp",
      "env": {
        "NORMATTIVA_HOME": "/srv/normattiva"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `NORMATTIVA_DATA_DIR` overrides an explicit `--home` for that kind. Use `NORMATTIVA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `NORMATTIVA_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `normattiva-pp-cli doctor --fail-on warn` to check path warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. The learn journal lives under `learn/` in the state directory. `normattiva-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `normattiva-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `NORMATTIVA_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `NORMATTIVA_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### collezioni

Prebuilt collections of acts (e.g. Codici)

- **`normattiva-pp-cli collezioni`** - List prebuilt collections with their act counts

### dettaglio

Raw act and article detail endpoints (use 'testo' and 'atto' for rendered text)

- **`normattiva-pp-cli dettaglio gazzetta`** - Read an act by Gazzetta Ufficiale coordinates (current text only)
- **`normattiva-pp-cli dettaglio urn`** - Read an act or article by URN:NIR (supports ~artN, @originale, !vig=YYYY-MM-DD)

### esportazioni

Bulk exports of acts as ZIP archives

- **`normattiva-pp-cli esportazioni <token>`** - Check the status of an asynchronous export request

### ricerca

Search Italian legislation (raw search endpoints; prefer cerca)

- **`normattiva-pp-cli ricerca aggiornati`** - Acts whose consolidated text changed between two dates (max 12 months)
- **`normattiva-pp-cli ricerca avanzata`** - Advanced search by act type, title, text, dates, vigenza and number
- **`normattiva-pp-cli ricerca predefinite`** - List predefined searches offered by the Normattiva portal
- **`normattiva-pp-cli ricerca semplice`** - Full-text search over act titles and text

### tipologiche

Code dictionaries used by Normattiva search filters

- **`normattiva-pp-cli tipologiche classi`** - List act vigenza classes (no updates, updated, repealed)
- **`normattiva-pp-cli tipologiche denominazioni`** - List act type names and codes (LEGGE, DECRETO-LEGGE, DECRETO LEGISLATIVO, ...)
- **`normattiva-pp-cli tipologiche estensioni`** - List export formats (AKN, XML, JSON, HTML, PDF, EPUB, RTF, URI/ELI)


### diff


- **`normattiva-pp-cli diff`** - See word by word what an article said on one date versus another, or versus its original text.


### fascicolo


- **`normattiva-pp-cli fascicolo`** - Assemble several articles from different laws, all pinned to one reference date, into one Markdown document.


### novita


- **`normattiva-pp-cli novita`** - Follow the acts you care about (segui add) and get a report of which ones and which articles changed since your last check.


### verifica


- **`normattiva-pp-cli verifica`** - Check every Italian law citation in a draft or an AI answer: does it exist, and was it in force on a given date?




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`normattiva-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`normattiva-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`normattiva-pp-cli learnings list`** - Inspect taught rows
- **`normattiva-pp-cli learnings forget <query>`** - Undo a teach
- **`normattiva-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`normattiva-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`normattiva-pp-cli teach-pattern`** - Install a query/resource template up front
- **`normattiva-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `NORMATTIVA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `normattiva-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
normattiva-pp-cli collezioni

# JSON for scripting and agents
normattiva-pp-cli collezioni --json
# Filter to specific fields
normattiva-pp-cli collezioni --json --select nomeCollezione,formatoCollezione,descrizioneFormatoCollezione

# Dry run — show the request without sending
normattiva-pp-cli collezioni --dry-run

# Agent mode — JSON + compact + no prompts in one flag
normattiva-pp-cli collezioni --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited or blocked, `10` config error.

## Health Check

```bash
normattiva-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `normattiva-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/normattiva-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\normattiva-pp-cli\config.toml` on Windows; `--home`, `NORMATTIVA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `NORMATTIVA_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `NORMATTIVA_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Connection reset by peer** — Wait a minute and retry; the Normattiva gateway drops bursts. Cached reads keep working offline.
- **Atto non trovato for a URN** — Read it from its Gazzetta coordinates (codice_redazionale and data_gu are in every search result): normattiva-pp-cli testo --codice 090G0294 --data-gu 1990-08-18
- **periodo di esportazione superiore a 12 mesi (code 1501)** — Use a shorter window, e.g. normattiva-pp-cli aggiornati --since 90d.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**normattiva_2_md**](https://github.com/ondata/normattiva_2_md) — Python (22 stars)
- [**gov-it-legal-mcp**](https://github.com/SimonBerg255/gov-it-legal-mcp) — TypeScript (11 stars)
- [**normattiva-mcp (adellorto)**](https://github.com/adellorto/normattiva-mcp) — TypeScript (4 stars)
- [**normattiva-sdk**](https://github.com/IreneBurresi/normattiva-sdk) — Python
- [**normattiva-mcp (avvocati-e-mac)**](https://github.com/avvocati-e-mac/normattiva-mcp) — Python
- [**mcp-it-normattiva**](https://github.com/pipeworx-io/mcp-it-normattiva) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
