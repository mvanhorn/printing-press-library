# Giustizia Amministrativa CLI

**La giurisprudenza amministrativa italiana (TAR, Consiglio di Stato) da terminale: ricerca, testo integrale in Markdown, store locale e ricerca offline.**

Cerca sentenze, ordinanze, decreti e pareri con filtri per tipo, sede e anno; ottieni il testo integrale in Markdown pulito con il suo URL pubblico; accumula i risultati in un database SQLite locale per ricerca offline, monitoraggio nel tempo ed export di corpus.

Learn more at [Giustizia Amministrativa](https://www.giustizia-amministrativa.it).

Created by [@aborruso](https://github.com/aborruso) (aborruso).

## Install

The recommended path installs both the `giustizia-amministrativa-pp-cli` binary and the `pp-giustizia-amministrativa` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install giustizia-amministrativa
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install giustizia-amministrativa --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install giustizia-amministrativa --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install giustizia-amministrativa --agent claude-code
npx -y @mvanhorn/printing-press-library install giustizia-amministrativa --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/giustizia-amministrativa-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install giustizia-amministrativa --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-giustizia-amministrativa --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-giustizia-amministrativa --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install giustizia-amministrativa --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/giustizia-amministrativa-current).
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
    "giustizia-amministrativa": {
      "command": "giustizia-amministrativa-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Nessuna autenticazione: ricerca e testi sono pubblici. Il CLF gestisce internamente l'handshake di sessione (token p_auth + cookie) del portale.

## Quick Start

```bash
# ricerca filtrata, ogni risultato porta ECLI e url pubblico
giustizia-amministrativa-pp-cli search "appalto soccorso istruttorio" --tipo sentenza --sede roma --limit 10

# testo integrale in Markdown pulito
giustizia-amministrativa-pp-cli get IT:TARLAZ:2026:11307SENT --format md

# output agent-native filtrato sui campi utili
giustizia-amministrativa-pp-cli search "clausola sociale" --all --json --select ecli,tipo,sede,data,url

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Output agent-native
- **`get`** — Scarica il testo completo di una sentenza/ordinanza/decreto/parere e lo restituisce in Markdown pulito.

  _Quando l'agente deve leggere o citare il testo di un provvedimento senza rumore HTML._

  ```bash
  giustizia-amministrativa-pp-cli get --sede tar_rm --nrg 202600422 --file 202611307_01.html --format md
  ```

### Stato locale che si accumula
- **`watch run`** — Salva una ricerca e a ogni esecuzione mostra solo i provvedimenti nuovi dall'ultima volta.

  _Per monitorare nuove decisioni su un tema o una sede senza rileggere tutto._

  ```bash
  giustizia-amministrativa-pp-cli watch run appalti-rm --testo appalto --sede roma --limit 20
  ```
- **`corpus build`** — Assembla N provvedimenti su un tema in una cartella di Markdown + un CSV manifest (ECLI, sede, data, url).

  _Per costruire un fascicolo citabile o un dataset di ricerca in un colpo solo._

  ```bash
  giustizia-amministrativa-pp-cli corpus build --testo "soccorso istruttorio" --tipo sentenza --limit 3 --out ./corpus
  ```

### Ricerca offline
- **`grep`** — Ricerca regex/prossimita' sui testi integrali scaricati localmente, non solo sugli snippet.

  _Per trovare una frase normativa esatta dentro il corpo dei provvedimenti._

  ```bash
  giustizia-amministrativa-pp-cli grep -e "soccorso istruttorio" --select ecli,url
  ```
- **`massime`** — Estrae i paragrafi 'principio di diritto'/massima da un corpus in un unico digest.

  _Per ottenere i principi di diritto su un tema senza leggere ogni sentenza._

  ```bash
  giustizia-amministrativa-pp-cli massime --testo "clausola sociale" --limit 30
  ```

### Analisi sul corpus
- **`appeal-chain`** — Esegue il 'verifica appello' in batch e ricostruisce la catena TAR->Consiglio di Stato.

  _Per sapere quali sentenze di primo grado sono state appellate e con quale esito._

  ```bash
  giustizia-amministrativa-pp-cli appeal-chain --testo "project financing" --limit 40
  ```
- **`stats`** — Distribuzione di un tema per sede, sezione, tipo e anno.

  _Per capire quale sede/sezione decide un tema e se il volume cresce._

  ```bash
  giustizia-amministrativa-pp-cli stats --testo "appalto" --by sede,anno
  ```

## Recipes


### Testo integrale in markdown

```bash
giustizia-amministrativa-pp-cli get IT:TARLAZ:2026:11307SENT --format md
```

Recupera e converte in Markdown pulito il provvedimento.

### Output agent-native con select su risposta ricca

```bash
giustizia-amministrativa-pp-cli search "appalto" --all --json --select results.ecli,results.tipo,results.sede,results.url
```

Restringe i campi della risposta ricca della ricerca per non sprecare contesto.

### Monitoraggio nel tempo

```bash
giustizia-amministrativa-pp-cli watch run appalti-lazio --json
```

Mostra solo i provvedimenti nuovi dall'ultima esecuzione.

## Usage

Run `giustizia-amministrativa-pp-cli --help` for the full command reference and flag list.

## Commands

### provvedimenti

Provvedimenti (sentenze, ordinanze, decreti, pareri) di TAR, Consiglio di Stato e CGARS.

- **`giustizia-amministrativa-pp-cli provvedimenti cerca`** - Cerca provvedimenti per testo, tipo, sede, anno, numero o NRG.
- **`giustizia-amministrativa-pp-cli provvedimenti get`** - Scarica il testo integrale di un provvedimento.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
giustizia-amministrativa-pp-cli provvedimenti get mock-value

# JSON for scripting and agents
giustizia-amministrativa-pp-cli provvedimenti get mock-value --json

# Filter to specific fields
giustizia-amministrativa-pp-cli provvedimenti get mock-value --json --select id,name,status

# Dry run — show the request without sending
giustizia-amministrativa-pp-cli provvedimenti get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
giustizia-amministrativa-pp-cli provvedimenti get mock-value --agent
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
giustizia-amministrativa-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/giustizia-amministrativa-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 sulle ricerche** — il token p_auth e' scaduto: il CLI lo rinnova automaticamente al refresh dell'handshake; riprova.
- **nessun risultato per una query certa** — verifica i filtri --tipo/--sede e prova la ricerca avanzata --all/--phrase.
