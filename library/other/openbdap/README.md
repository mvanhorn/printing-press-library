# OpenBDAP CLI

**Il catalogo Open Data della Ragioneria Generale dello Stato da terminale: ricerca offline, righe filtrate per nome di colonna leggibile e opere pubbliche cercabili per CUP, CIG o codice fiscale.**

OpenBDAP pubblica migliaia di dataset di finanza pubblica dietro un'API CKAN con conteggi sbagliati e un OData con nomi di colonna offuscati e senza elenco. Questa CLI allinea il catalogo in un archivio SQLite locale con 'allinea', ci costruisce sopra una ricerca full-text su titolo e descrizione, e traduce i nomi leggibili delle colonne negli identificativi che il servizio pretende. Comandi come dossier, opere e serie fanno in un colpo cio' che oggi richiede una manciata di chiamate costruite a mano.

## Install

The recommended path installs both the `openbdap-pp-cli` binary and the `pp-openbdap` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install openbdap
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install openbdap --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install openbdap --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install openbdap --agent claude-code
npx -y @mvanhorn/printing-press-library install openbdap --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/openbdap/cmd/openbdap-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openbdap-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install openbdap --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-openbdap --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-openbdap --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install openbdap --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openbdap-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/openbdap/cmd/openbdap-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "openbdap": {
      "command": "openbdap-pp-mcp"
    }
  }
}
```

</details>

## Prima di iniziare

`cerca`, `serie`, `mop`, `novita`, `cup`, `cig`, `dossier` e `opere` leggono un archivio SQLite locale: popolalo con `openbdap-pp-cli allinea` (qualche minuto per i 3857 dataset del catalogo, o `--tema <tema>` per un sottoinsieme). `campi` usa un secondo indice, che si costruisce con `openbdap-pp-cli campi --aggiorna --tema <tema>`. I comandi che interrogano il portale dal vivo senza passare dall'archivio sono `catalogo`, `dati`, `gruppi`, `tag`, `licenze`, `scarica`, `colonne`, `righe` e `conta`.

## Quick Start

```bash
# verifica che il portale risponda
openbdap-pp-cli doctor --dry-run

# allinea il catalogo nell'archivio locale: serve a cerca, serie, mop, novita e campi
openbdap-pp-cli allinea

# cerca offline su titolo e descrizione
openbdap-pp-cli cerca "opere pubbliche"

# capisci le colonne prima di estrarre le righe
openbdap-pp-cli colonne bda1676b-62ab-44b7-8f9a-ca93b8534488

# il quadro completo di un'opera dal CUP
openbdap-pp-cli dossier I77H11000120009

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Opere pubbliche trasversali
- **`dossier`** — Il quadro completo di un'opera pubblica a partire dal CUP: progetto, pagamenti, gare, partecipanti, piano dei costi e soggetti titolari.

  _Usalo quando ti serve tutto su un'opera e hai solo il CUP, invece di cinque chiamate OData con filtri diversi._

  ```bash
  openbdap-pp-cli dossier I77H11000120009 --agent
  ```
- **`opere`** — Le opere pubbliche in capo a un ente, a partire dal suo codice fiscale, con il totale reale.

  _Usalo per capire quante e quali opere risultano a un'amministrazione._

  ```bash
  openbdap-pp-cli opere --cf 80208450587 --agent
  ```
- **`mop`** — Quale dataset MOP interrogare per ogni regione e ruolo, con il relativo identificativo OData.

  _Usalo prima di una ricerca mirata sulle opere pubbliche, per sapere dove cercare._

  ```bash
  openbdap-pp-cli mop --regione Sicilia --agent
  ```
- **`cig`** — La gara e i partecipanti a partire dal codice CIG.

  _Usalo quando parti da un CIG invece che da un CUP._

  ```bash
  openbdap-pp-cli cig 12345678AB --agent
  ```

### Catalogo che si capisce
- **`serie`** — Le annualita', le mensilita' e le regioni disponibili di una stessa serie di dataset.

  _Usalo per sapere se esiste l'annualita' che ti serve senza scorrere il catalogo._

  ```bash
  openbdap-pp-cli serie "Pagamenti Bilancio dello Stato" --agent
  ```
- **`novita`** — I dataset il cui ultimo aggiornamento cade nella finestra indicata.

  _Usalo per sapere cosa e' cambiato di recente senza riscaricare il catalogo._

  ```bash
  openbdap-pp-cli novita --da 30d --agent
  ```
- **`campi`** — In quali dataset esiste un campo, con l'identificativo pronto da usare nei filtri.

  _Usalo quando sai quale campo ti serve ma non in quale dataset vive._

  ```bash
  openbdap-pp-cli campi "codice fiscale" --agent
  ```

## Recipes

### Allineare il catalogo prima di ogni ricerca offline

```bash
openbdap-pp-cli allinea
```

Popola l'archivio SQLite locale su cui lavorano cerca, serie, mop e novita.

### Trovare i dataset di un tema e un anno

```bash
openbdap-pp-cli cerca SIOPE --anno 2024 --agent
```

Cerca offline nell'archivio locale con i filtri derivati dai titoli.

### Estrarre righe filtrate senza conoscere gli identificativi di colonna

```bash
openbdap-pp-cli righe bda1676b-62ab-44b7-8f9a-ca93b8534488 --dove "Codice CUP=I77H11000120009" --csv
```

Il nome leggibile viene tradotto nell'identificativo che il servizio pretende.

### Il quadro di un'opera, campi essenziali

```bash
openbdap-pp-cli dossier I77H11000120009 --agent --select cup,progetto
```

Riduce il documento alle sole chiavi di primo livello che servono all'agente.

### Contare le opere di un ente

```bash
openbdap-pp-cli opere --cf 80208450587 --solo-conteggio
```

Usa il conteggio reale del servizio invece di scaricare le righe.

### Costruire l'indice dei campi e cercarci dentro

```bash
openbdap-pp-cli campi --aggiorna --tema 172_opere-pubbliche
```

L'indice degli schemi si popola su richiesta: senza questo passaggio 'campi' non trova nulla.

## Usage

Run `openbdap-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `OPENBDAP_CONFIG_DIR`, `OPENBDAP_DATA_DIR`, `OPENBDAP_STATE_DIR`, or `OPENBDAP_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `OPENBDAP_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export OPENBDAP_HOME=/srv/openbdap
openbdap-pp-cli doctor
```

Under `OPENBDAP_HOME=/srv/openbdap`, the four dirs resolve to `/srv/openbdap/config`, `/srv/openbdap/data`, `/srv/openbdap/state`, and `/srv/openbdap/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "openbdap": {
      "command": "openbdap-pp-mcp",
      "env": {
        "OPENBDAP_HOME": "/srv/openbdap"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `OPENBDAP_DATA_DIR` overrides an explicit `--home` for that kind. Use `OPENBDAP_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `OPENBDAP_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `openbdap-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### catalogo

Dataset del catalogo OpenBDAP

- **`openbdap-pp-cli catalogo dettaglio`** - Metadati di un dataset (solo per UUID: i nomi non sono risolvibili)
- **`openbdap-pp-cli catalogo elenco`** - Elenca gli identificativi di tutti i dataset del catalogo
- **`openbdap-pp-cli catalogo ricerca`** - Ricerca testuale lato portale (il campo count non e' affidabile e fq viene ignorato)

### dati

Dati tabellari dei dataset via OData

- **`openbdap-pp-cli dati colonne`** - Colonne del dataset: nome leggibile, nome fisico, identificativo da usare nei filtri, tipo, cardinalita' e valori distinti
- **`openbdap-pp-cli dati conta`** - Conta le righe del dataset, rispettando il filtro
- **`openbdap-pp-cli dati metadati`** - Metadati OData del dataset (ultimo aggiornamento, stato)
- **`openbdap-pp-cli dati misure`** - Misure numeriche dichiarate dal dataset
- **`openbdap-pp-cli dati righe`** - Righe del dataset, con filtri OData

### gruppi

Temi (gruppi) del catalogo

- **`openbdap-pp-cli gruppi dettaglio`** - Dettaglio di un tema, con gli UUID dei dataset che contiene
- **`openbdap-pp-cli gruppi elenco`** - Elenca i temi del catalogo

### licenze

Licenze usate nel catalogo

- **`openbdap-pp-cli licenze`** - Elenca le licenze

### scarica

Scaricamento integrale dei dataset in CSV

- **`openbdap-pp-cli scarica <id>`** - Scarica l'intero dataset in CSV (separatore punto e virgola)

### tag

Parole chiave del catalogo

- **`openbdap-pp-cli tag`** - Elenca le parole chiave


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`openbdap-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`openbdap-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`openbdap-pp-cli learnings list`** - Inspect taught rows
- **`openbdap-pp-cli learnings forget <query>`** - Undo a teach
- **`openbdap-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`openbdap-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`openbdap-pp-cli teach-pattern`** - Install a query/resource template up front
- **`openbdap-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `OPENBDAP_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `openbdap-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
openbdap-pp-cli catalogo dettaglio --id d032b3a2-2b70-4193-a0c8-cb7eb69f8710

# JSON for scripting and agents
openbdap-pp-cli catalogo dettaglio --id d032b3a2-2b70-4193-a0c8-cb7eb69f8710 --json
# Filter to specific fields
openbdap-pp-cli catalogo dettaglio --id d032b3a2-2b70-4193-a0c8-cb7eb69f8710 --json --select id,name,title

# Dry run — show the request without sending
openbdap-pp-cli catalogo dettaglio --id d032b3a2-2b70-4193-a0c8-cb7eb69f8710 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
openbdap-pp-cli catalogo dettaglio --id d032b3a2-2b70-4193-a0c8-cb7eb69f8710 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - `feedback --stdin` legge una nota dallo standard input; questa CLI non ha comandi che scrivono sul portale
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `1` unexpected error, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

I comandi in italiano non usano il codice 3 per "nessun risultato": restituiscono 0 con una lista vuota e, quando la causa e' l'archivio locale non popolato, una `nota` che dice quale comando lanciare.

## Health Check

```bash
openbdap-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `openbdap-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/openbdap-pp-cli/config.toml`; `--home`, `OPENBDAP_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Usa `catalogo elenco` o `cerca` per vedere gli identificativi disponibili

### API-specific
- **Il servizio OData risponde 500** — Stai usando l'UUID del dataset al posto dell'identificativo OData: prendilo da 'catalogo dettaglio'.
- **L'estrazione delle righe va in timeout** — Riduci la pagina: oltre 5000 righe per chiamata il servizio non risponde.
- **La ricerca sul portale restituisce conteggi incoerenti** — Usa 'cerca', che interroga l'archivio locale: il campo count dell'API e' inaffidabile.
- **Il download CSV non parte** — L'indirizzo http non funziona, serve https: la CLI lo forza gia'.
- **cerca, serie, mop, novita, cup, cig, dossier o opere non restituiscono nulla** — L'archivio locale e' vuoto: lancia 'openbdap-pp-cli allinea'. La risposta lo dice nel campo nota e nel booleano archivio_vuoto, che distingue "il codice non c'e'" da "non ho un archivio in cui cercarlo".
- **Il CSV di scarica arriva con gli accenti corrotti** — Il portale serve il dump in latin-1; 'scarica' lo converte in UTF-8, e con --raw restituisce i byte originali.
- **dossier non dice dove ricade l'opera** — La sezione 'localizzazione' c'e' dalla versione con la famiglia MOP omonima, e porta il codice ISTAT del comune a sei cifre.
- **righe sembra restituire tutte le righe e invece ne restituisce 50** — E' il default di --limite: quando il risultato lo tocca, la risposta porta il totale vero in meta.nota e un avviso su stderr. Usa --tutte per averle tutte.
- **campi non trova il campo cercato** — L'indice degli schemi si popola a parte: lancia 'openbdap-pp-cli campi --aggiorna --tema 172_opere-pubbliche'.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**DoveVannoINostriSoldi**](https://github.com/Italian-Builders-Org/DoveVannoINostriSoldi) — TypeScript (318 stars)
- [**ckan-mcp-server**](https://github.com/ondata/ckan-mcp-server) — TypeScript (57 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
