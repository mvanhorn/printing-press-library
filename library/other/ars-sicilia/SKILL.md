---
name: pp-ars-sicilia
description: "L'unica CLI per il portale dell'Assemblea Regionale Siciliana: cerca Trigger phrases: `ars sicilia`, `assemblea regionale siciliana`, `disegni di legge sicilia`, `interrogazioni ars`, `mozioni siciliane`, `resoconti aula sicilia`, `use ars-sicilia`, `run ars-sicilia`."
author: "aborruso"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ars-sicilia-pp-cli
---

# ARS Sicilia — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ars-sicilia-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install ars-sicilia --cli-only
   ```
2. Verify: `ars-sicilia-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/cmd/ars-sicilia-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Usa ars-sicilia-pp-cli quando devi cercare, scaricare o aggregare atti dell'Assemblea Regionale Siciliana (leggi regionali, disegni di legge, interrogazioni, mozioni, resoconti d'aula, lavori di commissione) e quando hai bisogno di output strutturato JSON/CSV per pipeline downstream o per assistenti AI via MCP. Particolarmente utile per giornalismo politico, ricerca civica, civic-hacking opendata, e analisi cross-archivio impossibili dal portale JSP nativo.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Vista cronologica cross-archivio
- **`ddl iter`** — Ricostruisce la cronologia completa di un disegno di legge: presentazione, passaggio in commissione, lavori d'aula, eventuale promulgazione come legge regionale.

  _Quando un agente deve raccontare 'a che punto sta il DDL X', questa è l'unica chiamata che restituisce la timeline completa senza incollare 5 ricerche manuali._

  ```bash
  ars-sicilia-pp-cli ddl iter 18 1500 --json
  ```
- **`deputato profilo`** — Aggrega in un'unica vista tutti gli atti firmati o pronunciati da un deputato: DDL, interrogazioni, interpellanze, mozioni, ordini del giorno, risoluzioni e interventi in resoconti d'aula.

  _Sostituisce un workflow di 7 click manuali con un'unica chiamata strutturata: pensata per agenti che rispondono a 'che ha fatto il deputato X?'._

  ```bash
  ars-sicilia-pp-cli deputato profilo "Rossi Mario" --legisl 18 --json --select tipo,data,titolo
  ```
- **`commissione dossier`** — Vista completa su una commissione: convocazioni in calendario, sommari lavori, DDL assegnati e pareri richiesti al Governo regionale.

  _Quando segui i lavori di una commissione specifica, questa è l'unica chiamata che dà il quadro completo invece di 3 ricerche separate._

  ```bash
  ars-sicilia-pp-cli commissione dossier 5 --legisl 18 --json
  ```
- **`legge cronologia`** — Partendo da una legge regionale promulgata (archivio 201), risale al DDL originario, agli emendamenti citati nei resoconti d'aula e ai pareri di commissione: l'inverso temporale di ddl iter.

  _Per ricercatori e giornalisti che partono dalla legge promulgata e vogliono raccontare come ci si è arrivati._

  ```bash
  ars-sicilia-pp-cli legge cronologia 18 5 --json
  ```

### Analytics su campi strutturati
- **`analytics`** — Identifica i deputati che firmano insieme atti parlamentari, restituendo coppie e cluster con conteggio per analisi di network politico.

  _Per ricercatori e giornalisti che analizzano alleanze e dinamiche politiche: niente foglio Excel di trascrizioni manuali._

  ```bash
  ars-sicilia-pp-cli analytics --type ddl --group-by cofirmatari --limit 50 --json
  ```
- **`analytics`** — Classifica i deputati per numero di interventi nei resoconti d'aula, con range date e legislatura, opzionale conteggio parole.

  _Per le persone che vogliono sapere 'chi parla di più' senza scaricare 200 resoconti PDF e fare ctrl+F._

  ```bash
  ars-sicilia-pp-cli analytics --type resoconti --group-by oratore --limit 30 --csv
  ```

### Stato e monitoraggio
- **`ddl drift`** — Confronta lo stato dell'iter dei DDL nella sync corrente con la precedente e segnala i disegni di legge che si sono mossi nel periodo (passati da commissione ad aula, approvati, ritirati).

  _L'RSS shell esistente segnala solo 'nuovi'; per 'mossi' non c'è alternativa. Questo è il segnale che cercavano i journalist che seguono iter politici._

  ```bash
  ars-sicilia-pp-cli ddl drift --since 7d --json
  ```
- **`sync stale`** — Mostra per ognuno dei 12 archivi ARS: timestamp ultima sync, n. record locali, età della sync, eventuale segnalazione di staleness.

  _Per agenti che orchestrano sync automatico: decide se rinfrescare prima di rispondere o se i dati locali sono ancora freschi._

  ```bash
  ars-sicilia-pp-cli sync stale --json
  ```

## Command Reference

**biblioteca** — Catalogo Bibliografico (archivio 205) e Opere Multimediali (205multimedia).

- `ars-sicilia-pp-cli biblioteca cerca` — Cerca nel catalogo bibliografico per autore, titolo, soggetto o ISBN.
- `ars-sicilia-pp-cli biblioteca multimediali` — Cerca nelle opere multimediali.

**commissioni** — Lavori delle Commissioni: convocazioni (229) e sommari (230).

- `ars-sicilia-pp-cli commissioni convocazioni` — Convocazioni delle Commissioni.
- `ars-sicilia-pp-cli commissioni sommari` — Sommari dei lavori di commissione.

**ddl** — Disegni di Legge (archivio 221): proposte di legge presentate all'ARS.

- `ars-sicilia-pp-cli ddl cerca` — Cerca disegni di legge per legislatura, anno, firmatario, materia o testo.
- `ars-sicilia-pp-cli ddl get` — Scarica un singolo disegno di legge.

**interpellanze** — Interpellanze parlamentari (archivio 234).

- `ars-sicilia-pp-cli interpellanze cerca` — Cerca interpellanze.
- `ars-sicilia-pp-cli interpellanze get` — Scarica una singola interpellanza.

**interrogazioni** — Interrogazioni parlamentari (archivio 233).

- `ars-sicilia-pp-cli interrogazioni cerca` — Cerca interrogazioni per legislatura, firmatario o rubrica.
- `ars-sicilia-pp-cli interrogazioni get` — Scarica una singola interrogazione.

**leggi** — Leggi della Regione Siciliana (archivio 201): testo storico delle leggi regionali.

- `ars-sicilia-pp-cli leggi cerca` — Cerca leggi regionali per legislatura, anno, numero o testo.
- `ars-sicilia-pp-cli leggi get` — Scarica una singola legge regionale.

**mozioni** — Mozioni parlamentari (archivio 235).

- `ars-sicilia-pp-cli mozioni cerca` — Cerca mozioni.
- `ars-sicilia-pp-cli mozioni get` — Scarica una singola mozione.

**odg** — Ordini del Giorno (archivio 236).

- `ars-sicilia-pp-cli odg cerca` — Cerca ordini del giorno.
- `ars-sicilia-pp-cli odg get` — Scarica un singolo ordine del giorno.

**pareri** — Pareri richiesti dal Governo regionale alle Commissioni (archivio 226).

- `ars-sicilia-pp-cli pareri cerca` — Cerca pareri richiesti dal Governo.
- `ars-sicilia-pp-cli pareri get` — Scarica un singolo parere.

**resoconti** — Resoconti delle Sedute d'Aula (archivio 217).

- `ars-sicilia-pp-cli resoconti cerca` — Cerca resoconti per data, oratore o argomento.
- `ars-sicilia-pp-cli resoconti get` — Scarica un singolo resoconto.

**risoluzioni** — Risoluzioni parlamentari (archivio 238).

- `ars-sicilia-pp-cli risoluzioni cerca` — Cerca risoluzioni.
- `ars-sicilia-pp-cli risoluzioni get` — Scarica una singola risoluzione.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ars-sicilia-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Sync iniziale completo XVIII legislatura

```bash
ars-sicilia-pp-cli sync --max-pages 0 --resources ddl,leggi,interrogazioni,mozioni,interpellanze,odg,risoluzioni,pareri,resoconti,convocazioni,sommari
```

Prima sincronizzazione di tutti gli archivi politici della XVIII legislatura — i dati restano in `~/.local/share/ars-sicilia-pp-cli/store.db`.

### Iter completo di un DDL con output narrowing

```bash
ars-sicilia-pp-cli ddl iter 18 1500 --json --select fase,data,sede,oratori
```

Timeline del DDL 1500, mostrando solo i campi essenziali — riduce il payload per agenti.

### Network di co-firmatari su DDL

```bash
ars-sicilia-pp-cli analytics --type ddl --group-by cofirmatari --limit 30 --csv
```

Produce un CSV con le coppie di deputati che firmano DDL insieme — pronto per import in `duckdb` o gephi.

### Drift settimanale dei DDL

```bash
ars-sicilia-pp-cli ddl drift --since 7d --json
```

Confronta lo stato dell'iter rispetto a una settimana fa — i DDL che si sono mossi (commissione → aula, voto, ritiro) compaiono qui.

### Top cofirmatari DDL (XVIII legislatura)

```bash
ars-sicilia-pp-cli analytics --type ddl --group-by cofirmatari --limit 20 --legisl 18 --json
```

Classifica i deputati che firmano più DDL insieme (richiede sync).

## Auth Setup

Nessuna credenziale richiesta: il portale ARS è pubblico. La sessione `JSESSIONID` per la ricerca è gestita automaticamente in modo trasparente dal client.

Run `ars-sicilia-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ars-sicilia-pp-cli ddl get mock-value mock-value --agent --select id,name,status
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
ars-sicilia-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ars-sicilia-pp-cli feedback --stdin < notes.txt
ars-sicilia-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/ars-sicilia-pp-cli/feedback.jsonl`. They are never POSTed unless `ARS_SICILIA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ARS_SICILIA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ars-sicilia-pp-cli profile save briefing --json
ars-sicilia-pp-cli --profile briefing ddl get mock-value mock-value
ars-sicilia-pp-cli profile list --json
ars-sicilia-pp-cli profile show briefing
ars-sicilia-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ars-sicilia-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add ars-sicilia-pp-mcp -- ars-sicilia-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ars-sicilia-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ars-sicilia-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ars-sicilia-pp-cli <command> --help`.
