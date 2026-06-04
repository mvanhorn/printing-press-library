# ARS Sicilia CLI

**L'unica CLI per il portale dell'Assemblea Regionale Siciliana: cerca, sincronizza in locale e interroga tutti i 12 archivi documentali con SQL, FTS e MCP.**

Sostituisce le 12 maschere JSP del portale ufficiale con una CLI agent-native. Sync in SQLite locale per query SQL, ricerca full-text cross-archivio, e novel commands come `ddl iter` (timeline completa di un disegno di legge) e `deputato profilo` (tutta l'attività di un parlamentare in un'unica chiamata).

Learn more at [ARS Sicilia](https://dati.ars.sicilia.it).

Created by [@aborruso](https://github.com/aborruso) (aborruso).

## Install

The recommended path installs both the `ars-sicilia-pp-cli` binary and the `pp-ars-sicilia` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ars-sicilia
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ars-sicilia --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ars-sicilia --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ars-sicilia --agent claude-code
npx -y @mvanhorn/printing-press-library install ars-sicilia --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/cmd/ars-sicilia-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ars-sicilia-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ars-sicilia --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-ars-sicilia --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install ars-sicilia --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ars-sicilia-current).
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
    "ars-sicilia": {
      "command": "ars-sicilia-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Nessuna credenziale richiesta: il portale ARS è pubblico. La sessione `JSESSIONID` per la ricerca è gestita automaticamente in modo trasparente dal client.

## Quick Start

```bash
# Verifica raggiungibilità del portale e stato del database locale.
ars-sicilia-pp-cli doctor

# Sincronizza in locale leggi e DDL degli ultimi 30 giorni in SQLite.
ars-sicilia-pp-cli sync --resources leggi,ddl --max-pages 0

# Cerca i DDL della XVIII legislatura presentati nel 2024.
ars-sicilia-pp-cli ddl cerca --anno 2024 --legisl 18 --json

# Ricerca full-text cross-archivio sui documenti già sincronizzati.
ars-sicilia-pp-cli search "bilancio sanitario" --limit 20

# Timeline completa del DDL 1500 della XVIII legislatura.
ars-sicilia-pp-cli ddl iter 18 1500 --json

# Tutta l'attività parlamentare di un deputato in un'unica chiamata.
ars-sicilia-pp-cli deputato profilo "Rossi Mario" --json --select tipo,data,titolo

```

## Known Gaps

- **HTTP error exit codes**: Non-429 HTTP errors from the Icaro portal (404, 5xx) exit with code 1 rather than typed exit codes (e.g. exit 3 for not-found, exit 5 for server error). Rate-limit responses (HTTP 429) correctly return exit 7. Scripts that branch on specific exit codes should use `ars-sicilia-pp-cli doctor` to check connectivity first.
- **`legge cronologia` date filtering**: The sommari search finds committee meetings that mention the law number in free text without a date ceiling. A committee meeting held after the law's promulgation date may appear in the timeline if it references the same number. Filter results by the `data` field when you need only pre-promulgation events.

## Unique Features

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

## Recipes

### Sync iniziale completo XVIII legislatura

```bash
ars-sicilia-pp-cli sync --full --resources leggi,ddl,interrogazioni,mozioni,interpellanze,odg,risoluzioni,pareri,resoconti_aula,convocazioni_commissioni,sommari_commissioni
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

### Analytics sui cofirmatari

```bash
ars-sicilia-pp-cli analytics --type ddl --group-by cofirmatari --limit 20 --legisl 18 --json
```

Top 20 coppie di deputati che firmano insieme DDL nella XVIII legislatura — aggregazione locale sul DB sincronizzato.

### Ricerca per tema (vocabolario materie)

```bash
# Scopri le materie disponibili, filtra per parola chiave
ars-sicilia-pp-cli ddl materie | grep -i "sanit\|salut\|lavoro\|ambiente"

# Tutti i DDL sull'ambiente nella XVIII
ars-sicilia-pp-cli ddl cerca --legisl 18 --materia "Ambiente" --json | \
  jq -r '.[] | "\(.data) — \(.title)"'
```

Utile per giornalisti che seguono un tema: la lista completa delle 123 materie è navigabile offline senza aprire il portale.

### Veterani del parlamento — chi dura di più

```bash
ars-sicilia-pp-cli ddl firmatari --json | \
  jq -r 'group_by(.nome)[] | select(length >= 4) | "\(length) legislature — \(.[0].nome)"' | \
  sort -rn | head -10
```

Identifica i parlamentari con la carriera più lunga: quante e quali legislature hanno coperto. Cracolici Antonino è il record attuale con 6 legislature consecutive (XIII→XVIII).

### Seguire un deputato — carriera e attività

```bash
# In quali legislature ha operato?
ars-sicilia-pp-cli ddl firmatari --search "Scoma" --json | jq -r '.[].legisl' | sort | tr '\n' ' '

# Tutti i DDL presentati nella XVIII
ars-sicilia-pp-cli ddl cerca --legisl 18 --firmatario "Scoma Francesco" --json | \
  jq -r '.[] | "\(.data) — \(.title)"'
```

### Nuovi deputati — chi è al primo mandato

```bash
ars-sicilia-pp-cli ddl firmatari --json | \
  jq -r 'group_by(.nome)[] | select(length == 1 and .[0].legisl == "18") | .[0].nome'
```

Filtra i deputati presenti solo nella XVIII — al loro primo mandato regionale.

### Iniziative parlamentari vs governative

```bash
# Tipi di iniziativa disponibili
ars-sicilia-pp-cli ddl iniziative

# DDL a iniziativa governativa nella XVIII
ars-sicilia-pp-cli ddl cerca --legisl 18 \
  --isis-query "(18.LEGISL E Governativa.FIRMAT)" --limit 50 --json | jq 'length'
```

Distingue le proposte dei deputati (parlamentare) da quelle dell'esecutivo regionale (governativa).

## Usage

Run `ars-sicilia-pp-cli --help` for the full command reference and flag list.

## Commands

### biblioteca

Catalogo Bibliografico (archivio 205) e Opere Multimediali (205multimedia).

- **`ars-sicilia-pp-cli biblioteca cerca`** - Cerca nel catalogo bibliografico per autore, titolo, soggetto o ISBN.
- **`ars-sicilia-pp-cli biblioteca multimediali`** - Cerca nelle opere multimediali.

### commissioni

Lavori delle Commissioni: convocazioni (229) e sommari (230).

- **`ars-sicilia-pp-cli commissioni convocazioni`** - Convocazioni delle Commissioni.
- **`ars-sicilia-pp-cli commissioni sommari`** - Sommari dei lavori di commissione.

### ddl

Disegni di Legge (archivio 221): proposte di legge presentate all'ARS.

- **`ars-sicilia-pp-cli ddl cerca`** - Cerca disegni di legge per legislatura, anno, firmatario, materia o testo.
- **`ars-sicilia-pp-cli ddl get`** - Scarica un singolo disegno di legge.

### interpellanze

Interpellanze parlamentari (archivio 234).

- **`ars-sicilia-pp-cli interpellanze cerca`** - Cerca interpellanze.
- **`ars-sicilia-pp-cli interpellanze get`** - Scarica una singola interpellanza.

### interrogazioni

Interrogazioni parlamentari (archivio 233).

- **`ars-sicilia-pp-cli interrogazioni cerca`** - Cerca interrogazioni per legislatura, firmatario o rubrica.
- **`ars-sicilia-pp-cli interrogazioni get`** - Scarica una singola interrogazione.

### leggi

Leggi della Regione Siciliana (archivio 201): testo storico delle leggi regionali.

- **`ars-sicilia-pp-cli leggi cerca`** - Cerca leggi regionali per legislatura, anno, numero o testo.
- **`ars-sicilia-pp-cli leggi get`** - Scarica una singola legge regionale.

### mozioni

Mozioni parlamentari (archivio 235).

- **`ars-sicilia-pp-cli mozioni cerca`** - Cerca mozioni.
- **`ars-sicilia-pp-cli mozioni get`** - Scarica una singola mozione.

### odg

Ordini del Giorno (archivio 236).

- **`ars-sicilia-pp-cli odg cerca`** - Cerca ordini del giorno.
- **`ars-sicilia-pp-cli odg get`** - Scarica un singolo ordine del giorno.

### pareri

Pareri richiesti dal Governo regionale alle Commissioni (archivio 226).

- **`ars-sicilia-pp-cli pareri cerca`** - Cerca pareri richiesti dal Governo.
- **`ars-sicilia-pp-cli pareri get`** - Scarica un singolo parere.

### resoconti

Resoconti delle Sedute d'Aula (archivio 217).

- **`ars-sicilia-pp-cli resoconti cerca`** - Cerca resoconti per data, oratore o argomento.
- **`ars-sicilia-pp-cli resoconti get`** - Scarica un singolo resoconto.

### risoluzioni

Risoluzioni parlamentari (archivio 238).

- **`ars-sicilia-pp-cli risoluzioni cerca`** - Cerca risoluzioni.
- **`ars-sicilia-pp-cli risoluzioni get`** - Scarica una singola risoluzione.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ars-sicilia-pp-cli ddl get mock-value mock-value

# JSON for scripting and agents
ars-sicilia-pp-cli ddl get mock-value mock-value --json

# Filter to specific fields
ars-sicilia-pp-cli ddl get mock-value mock-value --json --select id,name,status

# Dry run — show the request without sending
ars-sicilia-pp-cli ddl get mock-value mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ars-sicilia-pp-cli ddl get mock-value mock-value --agent
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
ars-sicilia-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/ars-sicilia-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **I comandi `cerca` restituiscono 0 risultati ma il sito ne mostra molti.** — Verifica la legislatura: senza `--legisl` la query usa il default. Il portale ARS richiede sempre una legislatura nel criterio. Esempio: `--legisl 18` per XVIII.
- **Errore di sessione o redirect inatteso.** — Il portale resetta la sessione dopo 30 minuti di inattività. Riprova il comando: il client acquisisce una nuova `JSESSIONID` automaticamente.
- **Comando `ddl iter` o `deputato profilo` non trova nulla.** — Le viste cross-archivio leggono dal DB locale: esegui prima `ars-sicilia-pp-cli sync --resources ddl,leggi,resoconti,convocazioni_commissioni,sommari_commissioni`.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**opendatasicilia/RSSdisegniLeggeAssembleaRegionaleSiciliana**](https://github.com/opendatasicilia/RSSdisegniLeggeAssembleaRegionaleSiciliana) — Shell
- [**aborruso/ars_sicilia**](https://github.com/aborruso/ars_sicilia) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
