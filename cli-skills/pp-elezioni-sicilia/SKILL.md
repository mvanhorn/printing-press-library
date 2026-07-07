---
name: pp-elezioni-sicilia
description: "Dati elettorali siciliani da riga di comando — senza copiare tabelle HTML. Trigger phrases: `elezioni sicilia`, `risultati elezioni comunali sicilia`, `affluenza elettorale sicilia`, `candidati sindaco sicilia`, `voti lista sicilia`."
author: "aborruso"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - elezioni-sicilia-pp-cli
    install:
      - kind: go
        bins: [elezioni-sicilia-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/cmd/elezioni-sicilia-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/elezioni-sicilia/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Elezioni Sicilia — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `elezioni-sicilia-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install elezioni-sicilia --cli-only
   ```
2. Verify: `elezioni-sicilia-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/cmd/elezioni-sicilia-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Usa questa CLI durante le notti elettorali per monitorare in tempo reale i risultati comunali siciliani. Utile anche per analisi storiche: confronta come ha votato un comune dal 2009 ad oggi.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## HTTP Transport

This CLI uses standard HTTP transport with HTTP/2 disabled for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**affluenza** — Dati sull'affluenza alle urne per tutti i comuni siciliani in più rilevamenti orari.

- `elezioni-sicilia-pp-cli affluenza` — Tabella regionale completa dell'affluenza con tutti i rilevamenti orari e confronto con elezioni precedenti.

**candidati** — Voti per candidato sindaco per comune.

- `elezioni-sicilia-pp-cli candidati` — Voti per ogni candidato sindaco in un comune specifico.

**comuni** — Elenco dei comuni che partecipano alle elezioni per una data provincia e anno.

- `elezioni-sicilia-pp-cli comuni` — Lista comuni con dropdown per navigazione, con codici interni del sito.

**liste** — Voti per lista elettorale collegata a ogni candidato sindaco.

- `elezioni-sicilia-pp-cli liste` — Voti per lista collegata a ciascun candidato sindaco in un comune.

**risultati** — Risultati finali delle elezioni per comune (disponibile a scrutinio completato).

- `elezioni-sicilia-pp-cli risultati` — Risultato finale del comune: sindaco eletto, sezioni, votanti, seggi per lista.

**seggi** — Ripartizione dei seggi consiliari per lista.

- `elezioni-sicilia-pp-cli seggi` — Ripartizione seggi in Consiglio Comunale per ogni lista.

**regionali** — Dati delle elezioni regionali siciliane (ARS). Anni: 2017, 2022.

- `elezioni-sicilia-pp-cli regionali presidente [--anno 2022]` — Candidati Presidente con lista regionale e liste provinciali collegate (voti, %, seggi).
- `elezioni-sicilia-pp-cli regionali affluenza [--anno 2022]` — Affluenza per provincia con 3 rilevamenti orari e confronto con tornata precedente.
- `elezioni-sicilia-pp-cli regionali seggi [--anno 2022]` — Riparto seggi per lista provinciale (matrice 9 province + totale).
- `elezioni-sicilia-pp-cli regionali listino [--anno 2022]` — Candidati del listino regionale per ciascuna lista (capolista = Presidente).
- `elezioni-sicilia-pp-cli regionali candidati --provincia CT [--anno 2022]` — Voti di preferenza ARS in una provincia.


**Hand-written commands**

- `elezioni-sicilia-pp-cli affluenza` — Mostra l'affluenza elettorale per tutti i comuni siciliani con confronto rispetto alle elezioni precedenti.
- `elezioni-sicilia-pp-cli comuni [--provincia AG]` — Elenca i comuni alle elezioni con codice, provincia e nome.
- `elezioni-sicilia-pp-cli candidati <comune> [--provincia AG] [--anno 2026]` — Mostra i voti per candidato sindaco in un comune.
- `elezioni-sicilia-pp-cli liste <comune> [--provincia AG] [--anno 2026]` — Mostra i voti per lista elettorale in un comune.
- `elezioni-sicilia-pp-cli risultati <comune> [--provincia AG] [--anno 2026]` — Mostra il risultato finale delle elezioni in un comune (richiede scrutinio completato).
- `elezioni-sicilia-pp-cli seggi <comune> [--provincia AG] [--anno 2026]` — Mostra la ripartizione dei seggi consiliari in un comune.
- `elezioni-sicilia-pp-cli stato <comune> [--provincia AG] [--anno 2026]` — Controlla lo stato dello scrutinio: in corso, parziale (N/M sezioni), o completo.
- `elezioni-sicilia-pp-cli regionali presidente [--anno 2022]` — Voti dei candidati Presidente della Regione (ARS), con lista regionale e liste provinciali collegate. Anni: 2017, 2022.
- `elezioni-sicilia-pp-cli regionali affluenza [--anno 2022]` — Affluenza regionale per provincia con 3 rilevamenti orari e confronto con elezione precedente.
- `elezioni-sicilia-pp-cli regionali seggi [--anno 2022]` — Riparto seggi ARS per lista (matrice provincia × lista).
- `elezioni-sicilia-pp-cli regionali listino [--anno 2022]` — Candidati del listino regionale.
- `elezioni-sicilia-pp-cli regionali candidati --provincia <XX> [--anno 2022]` — Voti di preferenza dei candidati ARS in una provincia.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
elezioni-sicilia-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Affluenza in JSON

```bash
elezioni-sicilia-pp-cli affluenza --json
```

Tutte le rilevazioni orarie per ogni comune in formato machine-readable

### Voti per lista a Palermo

```bash
elezioni-sicilia-pp-cli liste Palermo --anno 2025 --json
```

Voti per ogni lista elettorale nell'anno specificato

### Riepilogo regionale

```bash
elezioni-sicilia-pp-cli riepilogo --json --select province.affluenza
```

Snapshot aggregato per tutte le province

### Trend storico

```bash
elezioni-sicilia-pp-cli storico Messina --json
```

Serie temporale del voto dal 2009 al 2026

### Regionali — Presidente con liste collegate

```bash
elezioni-sicilia-pp-cli regionali presidente --anno 2022 --json
```

Per ogni candidato Presidente: voti, %, lista regionale, e tutte le liste provinciali collegate (voti, %, seggi).

### Regionali — Voti di preferenza ARS in una provincia

```bash
elezioni-sicilia-pp-cli regionali candidati --provincia CT --anno 2022 --json
```

Candidati ARS raggruppati per lista provinciale, con voti di preferenza per ciascuno.

## Auth Setup

No authentication required.

Run `elezioni-sicilia-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  elezioni-sicilia-pp-cli candidati Agrigento --provincia AG --agent --select data.candidati
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
elezioni-sicilia-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
elezioni-sicilia-pp-cli feedback --stdin < notes.txt
elezioni-sicilia-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.elezioni-sicilia-pp-cli/feedback.jsonl`. They are never POSTed unless `ELEZIONI_SICILIA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ELEZIONI_SICILIA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
elezioni-sicilia-pp-cli profile save briefing --json
elezioni-sicilia-pp-cli --profile briefing candidati Agrigento --provincia AG
elezioni-sicilia-pp-cli profile list --json
elezioni-sicilia-pp-cli profile show briefing
elezioni-sicilia-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `elezioni-sicilia-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/elezioni-sicilia/cmd/elezioni-sicilia-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add elezioni-sicilia-pp-mcp -- elezioni-sicilia-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which elezioni-sicilia-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   elezioni-sicilia-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `elezioni-sicilia-pp-cli <command> --help`.
