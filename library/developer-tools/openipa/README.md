# IndicePA CLI

**Il primo CLI per l'Indice delle Pubbliche Amministrazioni — lookup istantaneo di enti, PEC, codici IPA, fatturazione elettronica e ordini elettronici.**

openipa porta sul terminale le 22 API web service di IPA che gli sviluppatori usano copia-incollando curl. Con un singolo comando `openipa cf <CF>` ottieni uffici FE, nodi NSO e domicilio digitale in parallelo — tre roundtrip in uno.

## Perché openipa?

Il portale IPA richiede navigazione manuale ente per ente. Gli sviluppatori che integrano fatturazione elettronica, ordini NSO o notifiche PEC perdono ore a trovare codici destinatario, verificare abilitazioni e controllare PEC valide.

openipa risolve tre problemi concreti:

- **Codice destinatario SDI in un comando** — `fatturazione cf <CF>` restituisce tutti i `cod_uni_ou` abilitati, pronti per la testata XML della fattura PA.
- **Compliance check parallelo** — `cf <CF>` interroga SFE + NSO + domicilio digitale in simultanea e produce una checklist pass/fail in meno di 3 secondi.
- **Batch senza loop** — `fatturazione batch` legge centinaia di CF da stdin e torna NDJSON, senza scrivere un loop o chiamare curl in sequenza.

## Workflow Recipes

### Emettere una fattura PA

```bash
# 1. Trova il codice IPA dell'ente
openipa-pp-cli enti cerca 'comune di Roma' --json | jq '.[0].cod_amm'
# → "c_h501"

# 2. Verifica che l'ente sia abilitato SFE e ottieni il cod_uni_ou
openipa-pp-cli fatturazione cf 02438750586 --json | jq '.[0].OU[0].cod_uni_ou'
# → "ONVE0B"

# 3. Verifica compliance completa (SFE + NSO + domicilio)
openipa-pp-cli cf 02438750586
```

### Batch lookup per lista CF

```bash
# Legge CF da file, restituisce NDJSON con stato fatturazione
cat lista_cf.txt | openipa-pp-cli fatturazione batch --json
```

### Verificare una PEC prima di inviarci notifiche

```bash
# Classifica PEC come attiva / storica / sconosciuta
openipa-pp-cli domicilio verifica <pec-ente>
# → ✗ PEC: <pec-ente> — SCONOSCIUTO (trovata come email registrata)

# Trova l'ente titolare dell'email
openipa-pp-cli cerca <pec-ente> --json
```

### Navigare la struttura di un ente

```bash
# Vista Ente → AOO[N] → UO[M] in un comando
openipa-pp-cli enti tree agid --json

# Lista completa UO con cod_uni_ou per un ente
openipa-pp-cli uo list --codice agid --json
```

## Install

The recommended path installs both the `openipa-pp-cli` binary and the `pp-openipa` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install openipa
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install openipa --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openipa-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-openipa --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-openipa --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-openipa skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-openipa. The skill defines how its required CLI can be installed.
```

## Authentication

Richiede un AUTH_ID gratuito da registrare su indicepa.gov.it (rilasciato immediatamente). Salvalo come variabile d'ambiente IPA_auth_id o in ~/.config/openipa/config.toml.

## Quick Start

```bash
# Trova il codice IPA di un ente per nome
openipa-pp-cli enti cerca 'comune di Roma'


# Dati anagrafici completi di un ente per codice IPA
openipa-pp-cli enti get c_h501 --json


# Codice destinatario SDI (cod_uni_ou) per fatturazione elettronica
openipa-pp-cli fatturazione cf 80012000826 --json


# Tutti i canali PA (FE + NSO + domicilio digitale) in un colpo solo
openipa-pp-cli cf 97735020584 --json


# Sync offline e lista enti per regione
openipa-pp-cli sync && openipa-pp-cli enti list --regione Lazio --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Workflow PA in un comando
- **`doctor`** — Dato un codice fiscale, verifica in un colpo se l'ente ha SFE attivo, NSO abilitato e domicilio digitale — checklist compliance PA completa.

  _Un agente che verifica la compliance PA deve sapere se un ente è pronto a ricevere fatture, ordini e notifiche digitali in un unico check._

  ```bash
  openipa-pp-cli doctor 97735020584 --json
  ```
- **`fatturazione batch`** — Legge CF da stdin, chiama WS01_SFE_CF in parallelo, restituisce NDJSON con CF + cod_uni_ou + stato_canale per pipeline di fatturazione.

  _Un agente che emette fatture PA in batch deve trovare tutti i codici destinatario in un solo passaggio senza loop manuali._

  ```bash
  cat lista_cf.txt | openipa-pp-cli fatturazione batch --json
  ```
- **`enti tree`** — Vista ad albero di un ente con tutte le sue AOO e UO associate — Ente → AOO[N] → UO[M] in output testuale o JSON annidato.

  _Un agente che deve capire la struttura organizzativa di un ente PA ottiene tutto in un comando invece di navigare tre endpoint separati._

  ```bash
  openipa-pp-cli enti tree agid --json
  ```
- **`domicilio verifica`** — Controlla se una PEC è il domicilio digitale attivo di un ente, storico (cessato) o sconosciuta — produce stato classificato.

  _Un agente che invia notifiche PA deve sapere se una PEC è ancora valida prima di usarla — inviare a PEC cessata invalida la comunicazione._

  ```bash
  openipa-pp-cli domicilio verifica <pec-ente> --json
  ```
- **`cerca`** — Dato un indirizzo email o PEC, trova l'ente IPA titolare — AMM, AOO o UO — con cod_amm, tipo entità e tipo email.

  _Un agente che riceve una PEC in ingresso può risalire all'ente mittente senza conoscere il codice IPA._

  ```bash
  openipa-pp-cli cerca <pec-ente> --json
  ```

- **`cf`** — Dato il codice fiscale di un ente PA, verifica in parallelo SFE, NSO e domicilio digitale e produce una checklist compliance completa.

  _Un agente che deve validare un ente prima di emettere fattura o ordine ottiene tutto in un unico roundtrip parallelo._

  ```bash
  openipa-pp-cli cf 97735020584 --json
  ```

## Orchestrazione AI

Questa CLI è progettata per essere usata da agenti AI in pipeline automatizzate, non solo da umani al terminale.

### `which` — discovery semantica per agenti

Un agente non sa a priori quale comando usare. `which` risolve una query in linguaggio naturale al comando giusto, senza richiedere che l'agente legga tutta la documentazione:

```bash
openipa-pp-cli which "trovare il responsabile transizione digitale"
# → rtd cerca

openipa-pp-cli which "codice destinatario fattura per un ente"
# → fatturazione cf

openipa-pp-cli which "enti in un comune"
# → sede enti
```

Exit code `0` = match trovato, `2` = nessun match → fallback a `--help`. Questo permette a un agente di auto-orientarsi senza logica hardcoded sul nome dei comandi.

```bash
# pattern tipico per un agente orchestratore
CMD=$(openipa-pp-cli which "verifica PEC attiva" --json | jq -r '.matches[0].entry.command')
openipa-pp-cli $CMD --agent ...
```

L'indice copre 11 capability chiave. Per vedere l'elenco completo:

```bash
openipa-pp-cli which --json | jq '[.matches[] | .entry.command]'
```

### Modalità agente

Aggiungi `--agent` a qualsiasi comando per output ottimizzato per AI: JSON compatto, nessun prompt interattivo, nessun colore, errori su stderr.

```bash
openipa-pp-cli rtd cerca --ente "Comune di Milano" --agent
openipa-pp-cli sede enti --area "Roma" --agent | jq '.data[:5]'
```

### MCP server

Tutti i comandi sono esposti come tool MCP tramite `openipa-pp-mcp` — il server usa stdio transport e non richiede configurazione di rete:

```json
{
  "mcpServers": {
    "openipa": {
      "command": "openipa-pp-mcp",
      "env": { "IPA_auth_id": "<tuo-auth-id>" }
    }
  }
}
```

## Usage

Run `openipa-pp-cli --help` for the full command reference and flag list.

## Commands

### aoo

Aree Organizzative Omogenee degli enti

- **`openipa-pp-cli aoo get`** - AOO di un ente con filtro opzionale per codice AOO
- **`openipa-pp-cli aoo list`** - Lista delle AOO di un ente

### cerca

Ricerca trasversale — trova entità IPA per email

- **`openipa-pp-cli cerca email`** - Trova entità IPA (AMM/AOO/UO) associate a un indirizzo email

### domicilio

Domicili digitali (PEC e SERC) delle entità IPA

- **`openipa-pp-cli domicilio aoo`** - Domicilio digitale attivo di una AOO
- **`openipa-pp-cli domicilio cf`** - Domicilio digitale di un ente per codice fiscale
- **`openipa-pp-cli domicilio email`** - Cerca entità IPA tramite indirizzo domicilio digitale (PEC)
- **`openipa-pp-cli domicilio storico-aoo`** - Storico domicili digitali di una AOO (inclusi cessati)
- **`openipa-pp-cli domicilio storico-uo`** - Storico domicili digitali di una UO (inclusi cessati)
- **`openipa-pp-cli domicilio uo`** - Domicilio digitale attivo di una UO per codice univoco

### enti

Ricerca e dettagli degli enti (Pubbliche Amministrazioni)

- **`openipa-pp-cli enti cerca`** - Cerca enti per nome o descrizione
- **`openipa-pp-cli enti get`** - Dati anagrafici completi di un ente per codice IPA

### fatturazione

Servizi di fatturazione elettronica (SFE) — ricerca uffici destinatari

- **`openipa-pp-cli fatturazione cf`** - Uffici destinatari fattura elettronica per codice fiscale ente
- **`openipa-pp-cli fatturazione ente`** - Canali SFE attivi di un ente per codice IPA

### nso

Nodi di Smistamento Ordini (NSO) per ordini elettronici

- **`openipa-pp-cli nso cf`** - Nodi NSO per codice fiscale ente
- **`openipa-pp-cli nso ente`** - Canali NSO attivi di un ente per codice IPA

### uo

Unità Organizzative degli enti

- **`openipa-pp-cli uo get`** - Dettagli di una singola UO per codice univoco
- **`openipa-pp-cli uo list`** - Lista delle UO di un ente

### sede

Ricerca per indirizzo sede (portale IPA — non disponibile via API pubblica)

- **`openipa-pp-cli sede enti`** - Cerca enti per nome, CF, area geografica, categoria
- **`openipa-pp-cli sede aoo`** - Cerca AOO per nome, area geografica, ente
- **`openipa-pp-cli sede uo`** - Cerca UO per nome, area geografica, ente

Filtri disponibili: `--nome`, `--cf`, `--area`, `--categoria`, `--codice`/`--codice-ente`. Paginazione: `--pagina N` (30 risultati per pagina).

### rtd

Responsabile Transizione Digitale (portale IPA — non disponibile via API pubblica)

- **`openipa-pp-cli rtd cerca`** - Cerca RTD per nominativo, ente, area geografica

Filtri disponibili: `--nominativo`, `--ente`, `--codice-ente`, `--area`, `--categoria`.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
openipa-pp-cli aoo list --codice example-value

# JSON for scripting and agents
openipa-pp-cli aoo list --codice example-value --json

# Filter to specific fields
openipa-pp-cli aoo list --codice example-value --json --select id,name,status

# Dry run — show the request without sending
openipa-pp-cli aoo list --codice example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
openipa-pp-cli aoo list --codice example-value --agent
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
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-openipa -g
```

Then invoke `/pp-openipa <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add openipa openipa-pp-mcp -e IPA_auth_id=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openipa-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `IPA_auth_id` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "openipa": {
      "command": "openipa-pp-mcp",
      "env": {
        "IPA_auth_id": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
openipa-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/openipa/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `IPA_auth_id` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `openipa-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $IPA_auth_id`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Errore 902: Parametro AUTH_ID errato** — export IPA_auth_id=<tuo_auth_id> oppure registra un nuovo AUTH_ID su indicepa.gov.it
- **Errore 900: Parametro AUTH_ID mancante** — Imposta la variabile d'ambiente IPA_auth_id o aggiungi auth_id al file ~/.config/openipa/config.toml
- **HTTP 500 su comandi pec** — I web service WS20/WS21/WS22 di IPA hanno instabilità note; riprova più tardi o usa openipa sync per i dati bulk
- **Nessun risultato da 'enti cerca'** — Usa parole parziali (es. 'Roma' non 'Comune di Roma'); esegui 'openipa sync' per abilitare ricerca FTS offline

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**FatturaElettronica.IndicePA**](https://github.com/FatturaElettronica/FatturaElettronica.IndicePA) — C# (45 stars)
- [**nicogis/FatturazioneElettronica-IPA**](https://github.com/nicogis/FatturazioneElettronica-IPA) — C# (8 stars)
- [**pbertera/rubripa.it**](https://github.com/pbertera/rubripa.it) — Python (5 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
