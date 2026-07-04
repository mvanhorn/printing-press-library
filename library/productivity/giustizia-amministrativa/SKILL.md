---
name: pp-giustizia-amministrativa
description: "La giurisprudenza amministrativa italiana (TAR, Consiglio di Stato) da terminale: ricerca, testo integrale in Markdown Trigger phrases: `cerca sentenze TAR`, `trova giurisprudenza consiglio di stato`, `testo integrale di una sentenza amministrativa`, `cerca ordinanze appalto`, `usa giustizia-amministrativa`, `run giustizia-amministrativa`."
author: "aborruso"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - giustizia-amministrativa-pp-cli
---

# Giustizia Amministrativa — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `giustizia-amministrativa-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install giustizia-amministrativa --cli-only
   ```
2. Verify: `giustizia-amministrativa-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Cerca sentenze, ordinanze, decreti e pareri con filtri per tipo, sede e anno; ottieni il testo integrale in Markdown pulito con il suo URL pubblico; accumula i risultati in un database SQLite locale per ricerca offline, monitoraggio nel tempo ed export di corpus.

## When to Use This CLI

Usalo quando devi cercare giurisprudenza amministrativa italiana, recuperare il testo integrale di un provvedimento in markdown, costruire un corpus citabile su un tema, o monitorare nel tempo nuove decisioni di una sede o su un argomento.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**provvedimenti** — Provvedimenti (sentenze, ordinanze, decreti, pareri) di TAR, Consiglio di Stato e CGARS.

- `giustizia-amministrativa-pp-cli provvedimenti cerca` — Cerca provvedimenti per testo, tipo, sede, anno, numero o NRG.
- `giustizia-amministrativa-pp-cli provvedimenti get` — Scarica il testo integrale di un provvedimento.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
giustizia-amministrativa-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

Nessuna autenticazione: ricerca e testi sono pubblici. Il CLF gestisce internamente l'handshake di sessione (token p_auth + cookie) del portale.

Run `giustizia-amministrativa-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  giustizia-amministrativa-pp-cli provvedimenti get mock-value --agent --select id,name,status
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
giustizia-amministrativa-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
giustizia-amministrativa-pp-cli feedback --stdin < notes.txt
giustizia-amministrativa-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/giustizia-amministrativa-pp-cli/feedback.jsonl`. They are never POSTed unless `GIUSTIZIA_AMMINISTRATIVA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GIUSTIZIA_AMMINISTRATIVA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
giustizia-amministrativa-pp-cli profile save briefing --json
giustizia-amministrativa-pp-cli --profile briefing provvedimenti get mock-value
giustizia-amministrativa-pp-cli profile list --json
giustizia-amministrativa-pp-cli profile show briefing
giustizia-amministrativa-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `giustizia-amministrativa-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add giustizia-amministrativa-pp-mcp -- giustizia-amministrativa-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which giustizia-amministrativa-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   giustizia-amministrativa-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `giustizia-amministrativa-pp-cli <command> --help`.
