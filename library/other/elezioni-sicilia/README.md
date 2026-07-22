# Elezioni Sicilia CLI

**Dati elettorali siciliani da riga di comando — senza copiare tabelle HTML.**

Il sito ufficiale della Regione Siciliana pubblica i risultati delle elezioni comunali solo in HTML e PDF. Questa CLI estrae affluenza, voti, candidati e risultati in JSON strutturato, con archivio dal 2009 e confronto storico tra anni.

Learn more at [Elezioni Sicilia](https://www.elezioni.regione.sicilia.it).

Created by [@aborruso](https://github.com/aborruso) (aborruso).

## Install

The recommended path installs both the `elezioni-sicilia-pp-cli` binary and the `pp-elezioni-sicilia` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install elezioni-sicilia
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install elezioni-sicilia --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install elezioni-sicilia --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install elezioni-sicilia --agent claude-code
npx -y @mvanhorn/printing-press-library install elezioni-sicilia --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/cmd/elezioni-sicilia-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/elezioni-sicilia-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install elezioni-sicilia --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-elezioni-sicilia --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-elezioni-sicilia --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install elezioni-sicilia --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/elezioni-sicilia-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/cmd/elezioni-sicilia-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "elezioni-sicilia": {
      "command": "elezioni-sicilia-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Tabella affluenza regionale aggiornata
elezioni-sicilia-pp-cli affluenza --json

# Comuni alle elezioni in provincia di Palermo
elezioni-sicilia-pp-cli comuni --provincia PA --json

# Voti per candidato sindaco ad Agrigento
elezioni-sicilia-pp-cli candidati Agrigento --json

# Confronto affluenza dal 2009 al 2026
elezioni-sicilia-pp-cli storico Agrigento --json

# Elezioni regionali (ARS) — Presidente 2022 con liste collegate
elezioni-sicilia-pp-cli regionali presidente --anno 2022 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Analisi temporale
- **`storico`** — Confronta affluenza, voti e candidati di uno stesso comune in tutti gli anni disponibili (2009-2026).

  _Permette analisi di trend elettorali pluridecennali su un singolo comune siciliano senza accesso a database._

  ```bash
  elezioni-sicilia-pp-cli storico Agrigento --json
  ```

### Analisi territoriale
- **`riepilogo`** — Mostra affluenza e stato scrutini per tutte le 9 province siciliane in un unico output strutturato.

  _Snapshot immediato del quadro regionale durante la notte elettorale._

  ```bash
  elezioni-sicilia-pp-cli riepilogo --json
  ```

### Monitoraggio live
- **`watch`** — Polling periodico dello stato scrutini per tutti i comuni, con alert su avanzamento.

  _Permette di monitorare l'avanzamento degli scrutini in tempo reale senza aggiornare manualmente il browser._

  ```bash
  elezioni-sicilia-pp-cli watch --intervallo 5m --json
  ```

## Usage

Run `elezioni-sicilia-pp-cli --help` for the full command reference and flag list.

## Commands

### affluenza

Dati sull'affluenza alle urne per tutti i comuni siciliani in più rilevamenti orari.

- **`elezioni-sicilia-pp-cli affluenza tabella`** - Tabella regionale completa dell'affluenza con tutti i rilevamenti orari e confronto con elezioni precedenti.

### candidati

Voti per candidato sindaco per comune.

- **`elezioni-sicilia-pp-cli candidati get`** - Voti per ogni candidato sindaco in un comune specifico.

### comuni

Elenco dei comuni che partecipano alle elezioni per una data provincia e anno.

- **`elezioni-sicilia-pp-cli comuni list`** - Lista comuni con dropdown per navigazione, con codici interni del sito.

### liste

Voti per lista elettorale collegata a ogni candidato sindaco.

- **`elezioni-sicilia-pp-cli liste get`** - Voti per lista collegata a ciascun candidato sindaco in un comune.

### risultati

Risultati finali delle elezioni per comune (disponibile a scrutinio completato).

- **`elezioni-sicilia-pp-cli risultati get`** - Risultato finale del comune: sindaco eletto, sezioni, votanti, seggi per lista.

### seggi

Ripartizione dei seggi consiliari per lista.

- **`elezioni-sicilia-pp-cli seggi get`** - Ripartizione seggi in Consiglio Comunale per ogni lista.

### regionali

Dati delle elezioni regionali siciliane (Assemblea Regionale Siciliana — ARS). Anni supportati: **2017** e **2022**.

- **`elezioni-sicilia-pp-cli regionali presidente [--anno 2022]`** - Voti per ciascun candidato Presidente della Regione, con lista regionale e liste provinciali collegate (voti, %, seggi).
- **`elezioni-sicilia-pp-cli regionali affluenza [--anno 2022]`** - Affluenza per ogni provincia con 3 rilevamenti orari (12:00, 19:00, 22-23:00) e confronto con la tornata precedente.
- **`elezioni-sicilia-pp-cli regionali seggi [--anno 2022]`** - Riparto seggi per lista provinciale: matrice 9 colonne provincia + totale regionale.
- **`elezioni-sicilia-pp-cli regionali listino [--anno 2022]`** - Candidati del listino regionale per ciascuna lista (il capolista è il candidato Presidente).
- **`elezioni-sicilia-pp-cli regionali candidati --provincia CT [--anno 2022]`** - Voti di preferenza dei candidati ARS in una provincia, raggruppati per lista provinciale.

```bash
# Voti dei candidati Presidente alle regionali 2022
elezioni-sicilia-pp-cli regionali presidente --anno 2022 --json

# Affluenza regionale 2017 con confronto
elezioni-sicilia-pp-cli regionali affluenza --anno 2017

# Voti preferenza ARS a Catania nel 2022
elezioni-sicilia-pp-cli regionali candidati --provincia CT --anno 2022 --json
```

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
elezioni-sicilia-pp-cli candidati Agrigento --provincia AG

# JSON for scripting and agents
elezioni-sicilia-pp-cli candidati Agrigento --provincia AG --json

# Filter to specific fields
elezioni-sicilia-pp-cli candidati Agrigento --provincia AG --json --select id,name,status

# Dry run — show the request without sending
elezioni-sicilia-pp-cli candidati Agrigento --provincia AG --dry-run

# Agent mode — JSON + compact + no prompts in one flag
elezioni-sicilia-pp-cli candidati Agrigento --provincia AG --agent
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
elezioni-sicilia-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/elezioni-sicilia-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Errore TLS certificate** — Il sito usa TLS self-signed — il CLI usa Surf automaticamente, non serve azione
- **Dati non disponibili per il comune** — Gli scrutini potrebbero essere ancora in corso — usa 'stato <comune>' per verificare
- **Regionali non accessibili** — Il server della Regione Siciliana non espone i dati regionali via URL diretti

## HTTP Transport

This CLI uses standard HTTP transport with HTTP/2 disabled for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**ondata/elezioni_europee_2024**](https://github.com/ondata/elezioni_europee_2024) — CSV/Python (16 stars)
- [**marcodallastella/elezioni**](https://github.com/marcodallastella/elezioni) — Python (12 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
