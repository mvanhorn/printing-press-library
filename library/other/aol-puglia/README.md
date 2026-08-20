# Aol Puglia CLI

API per l'Albo Pretorio Online della Sanità Pugliese.
Accesso pubblico (bandi, concorsi, delibere, determine) tramite OAuth2 client_credentials.
13 aziende sanitarie pugliesi disponibili.

Learn more at [Aol Puglia](https://sanita.puglia.it/aol/).

Created by [@aborruso](https://github.com/aborruso) (aborruso).

## Install

The recommended path installs both the `aol-puglia-pp-cli` binary and the `pp-aol-puglia` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install aol-puglia
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install aol-puglia --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install aol-puglia --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install aol-puglia --agent claude-code
npx -y @mvanhorn/printing-press-library install aol-puglia --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/aol-puglia-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install aol-puglia --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-aol-puglia --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-aol-puglia --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install aol-puglia --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/aol-puglia-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Non è necessario inserire credenziali: il server MCP si autentica automaticamente tramite OAuth2 pubblico.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "aol-puglia": {
      "command": "aol-puglia-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Nessuna configurazione richiesta

Il CLI si autentica automaticamente tramite OAuth2 client_credentials pubblico.
Nessun token da ottenere, nessuna variabile d'ambiente da impostare.

### 3. Verify Setup

```bash
aol-puglia-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
# Lista le 13 aziende sanitarie con i loro identificatori
aol-puglia-pp-cli orgs

# Cerca bandi per la ASL Bari
aol-puglia-pp-cli atti search --azienda "ASL Bari" --tipo-item bando

# Esporta i risultati in CSV
aol-puglia-pp-cli fileexport --azienda "ASL Bari" --tipo-item bando --output bari-bandi.csv
```

## Usage

Run `aol-puglia-pp-cli --help` for the full command reference and flag list.

## Commands

### atti

Manage atti

- **`aol-puglia-pp-cli atti download-allegato`** - Restituisce il contenuto dell'allegato come base64 con il nome originale del file.
L'allegato ID è disponibile nel campo listaAllegati di ogni atto.
- **`aol-puglia-pp-cli atti get-atto`** - Restituisce il dettaglio completo di un singolo atto inclusa la lista allegati.
- **`aol-puglia-pp-cli atti get-trasparenza`** - Restituisce le sezioni di trasparenza disponibili per l'azienda.
- **`aol-puglia-pp-cli atti search`** - Ricerca paginata degli atti pubblicati: bando, concorso, delibera,
determina (la disponibilità per tipo varia per azienda; alcune ASL non pubblicano delibere/determine).
Il valore `repertorio` restituisce la vista aggregata di tutti i tipi. Supporta filtri per data, proponente, oggetto, CIG.

### config

Manage config

- **`aol-puglia-pp-cli config get-configurazione-item`** - Restituisce la configurazione di durata pubblicazione e proroga per ogni tipo di atto dell'azienda.
- **`aol-puglia-pp-cli config list-proponenti`** - Restituisce tutti i proponenti configurati per l'azienda e il tipo di documento.
- **`aol-puglia-pp-cli config list-proponenti-attivi`** - Come getProponenti ma filtra solo i proponenti con attivo=true.

### fileexport

Manage fileexport

- **`aol-puglia-pp-cli fileexport`** - Esporta i risultati di una ricerca come file CSV.
Accetta gli stessi parametri di getListaAttiPaginata (senza page/numElementi).


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
aol-puglia-pp-cli atti search --azienda "ASL Bari" --tipo-item bando

# JSON for scripting and agents
aol-puglia-pp-cli atti search --azienda "ASL Bari" --tipo-item bando --json

# Filter to specific fields
aol-puglia-pp-cli atti search --azienda "ASL Bari" --tipo-item bando --json --select id,oggetto,dataAdozione

# Dry run — show the request without sending
aol-puglia-pp-cli atti search --azienda "ASL Bari" --tipo-item bando --dry-run

# Agent mode — JSON + compact + no prompts in one flag
aol-puglia-pp-cli atti search --azienda "ASL Bari" --tipo-item bando --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
aol-puglia-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/albo-online-puglia-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `AOL_PUGLIA_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `aol-puglia-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `aol-puglia-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $AOL_PUGLIA_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
