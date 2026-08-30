# Catasto CLI (italiano)

> 🇬🇧 English version: [README.md](README.md)

**CLI italiano del catasto in un singolo binario — converte tra riferimenti catastali e coordinate GPS, online o offline, con output JSON pensato per gli agenti.**

Fa da ponte tra i riferimenti catastali italiani (provincia / comune / foglio / particella) e le coordinate WGS84 usando l'endpoint pubblico ajax dell'Agenzia delle Entrate, più il dataset Parquet di centroidi mantenuto dalla community ondata. Produce JSON che si incolla direttamente nelle pipeline GIS a valle. Niente credenziali, niente Python, niente DuckDB — un singolo binario Go.

Stampato da [@robertobissanti](https://github.com/robertobissanti) (Roberto Bissanti).

## Installazione

Il percorso consigliato installa in un colpo solo sia il binario `catasto-pp-cli` sia la skill per agenti `pp-catasto` (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, e gli altri agenti supportati dal CLI upstream [`skills`](https://github.com/vercel-labs/skills)):

```bash
npx -y @mvanhorn/printing-press install catasto
```

Solo il CLI (senza skill):

```bash
npx -y @mvanhorn/printing-press install catasto --cli-only
```

Solo la skill (senza il binario CLI, utile per aggiornarla):

```bash
npx -y @mvanhorn/printing-press install catasto --skill-only
```

Per limitare l'installazione della skill a specifici agenti:

```bash
npx -y @mvanhorn/printing-press install catasto --agent claude-code
npx -y @mvanhorn/printing-press install catasto --agent claude-code --agent codex
```

### Binario pre-compilato

Scarica il binario per la tua piattaforma dalla [release più recente](https://github.com/mvanhorn/printing-press-library/releases/tag/catasto-current). Su macOS, libera dal quarantine di Gatekeeper: `xattr -d com.apple.quarantine <binario>`. Su Unix, rendilo eseguibile: `chmod +x <binario>`.

## Uso con sistemi agentici

Il CLI è progettato per essere agent-native: tutti i comandi supportano `--json`, `--select`, `--agent`, `--dry-run`; gli exit code sono tipati; il server MCP (`catasto-pp-mcp`) espone ogni comando Cobra come tool dell'agente, con `readOnlyHint` impostato sui comandi di sola lettura.

Tre vie di integrazione, scegli quella che ti torna comoda:

### 1. Claude Desktop / Claude Code (bundle `.mcpb` drag-and-drop)

La via più rapida. Il bundle compila il server MCP e lo impacchetta col manifest in un singolo ZIP.

1. Prendi il file `build/catasto-pp-mcp-darwin-arm64.mcpb` da questa repo (o scaricalo dalla release pubblicata).
2. Doppio-click sul file — Claude Desktop apre la finestra di installazione.
3. Dopo l'installazione, ogni comando appare come tool in Claude (`gps`, `cadastral`, `comune`, `validate`, `doctor`, ...). I comandi di sola lettura hanno `readOnlyHint: true`, quindi Claude non chiede permesso a ogni chiamata.

Per Claude Code specificamente, il comando di installazione sopra (`npx -y @mvanhorn/printing-press install catasto`) registra sia il CLI sia la skill (`pp-catasto`) — preferibile al bundle MCP grezzo quando vuoi anche il testo prosaico della SKILL.md.

### 2. MCP host generico (configurazione stdio)

Qualsiasi host che parli MCP via stdio (Cursor, Windsurf, Zed, integrazioni custom): puntalo al binario `catasto-pp-mcp`. Compila il binario una volta con `go build -o catasto-pp-mcp ./cmd/catasto-pp-mcp`, poi aggiungi alla configurazione MCP dell'host:

```json
{
  "mcpServers": {
    "catasto": {
      "command": "/percorso/assoluto/a/catasto-pp-mcp"
    }
  }
}
```

### 3. Codex CLI

Codex supporta server MCP via file di configurazione. Aggiungi a `~/.codex/config.toml`:

```toml
[mcp_servers.catasto]
command = "/percorso/assoluto/a/catasto-pp-mcp"
```

Da quel momento le sessioni Codex possono chiamare `gps`, `cadastral`, ecc. come tool. Verifica la [documentazione corrente di Codex](https://github.com/openai/codex) per la sintassi più aggiornata — è evoluta durante il 2026.

### 4. OpenCode

OpenCode legge la configurazione MCP da `~/.config/opencode/config.json`:

```json
{
  "mcp": {
    "catasto": {
      "type": "local",
      "command": ["/percorso/assoluto/a/catasto-pp-mcp"]
    }
  }
}
```

Per la sintassi più recente vedi [opencode.ai](https://opencode.ai/).

### 5. aider (niente MCP — shell-out)

aider non parla MCP nativamente. Due strade:

- **Aggiungi `SKILL.md` al contesto read-only di aider** (`aider --read SKILL.md`). aider impara cosa fa il CLI e lo chiama via shell.
- **Invocazioni one-shot:** chiedi ad aider di "eseguire `catasto-pp-cli <sottocomando> --agent` e usare l'output JSON". `--agent` attiva JSON, modalità compatta, e disabilita i prompt interattivi in un unico flag — output pronto per il flusso chat-tool di aider.

### 6. Qualsiasi agente — shell-out diretto (niente MCP, niente skill)

Per setup minimi, gli agenti possono chiamare il CLI e parsare il JSON:

| Comando | Invocazione agent-friendly | Restituisce |
|---|---|---|
| GPS → catasto | `catasto-pp-cli gps <lon> <lat> --agent` | Provincia, comune, foglio, particella |
| GPS batch | `catasto-pp-cli gps --stdin --agent < punti.csv` | Un oggetto JSON per riga |
| Catasto → GPS | `catasto-pp-cli cadastral --comune <belfiore-o-nome> [--provincia <sigla>] [--cap <codice>] --foglio <n> --particella <n> --agent` | Lat/lon WGS84 + metadati comune |
| Risoluzione comune | `catasto-pp-cli comune --belfiore <codice> --json` / `--name <nome> --provincia <sigla> --json` / `--cap <codice> --json` | Metadati del comune (niente rete) |
| Validazione input | `catasto-pp-cli validate --comune <c> --foglio <f> --particella <p> --json` | `{valid: true}` o lista errori |
| Health check | `catasto-pp-cli doctor --json` | Stato di raggiungibilità degli endpoint |
| Auto-descrizione | `catasto-pp-cli agent-context` | Albero completo dei comandi + flag in JSON |

Exit code standard: `0` successo, `2` errore di utilizzo, `3` non trovato, `5` errore API, `7` rate-limit, `10` errore di config. Gli errori vanno su stderr; l'output strutturato va su stdout.

Pre-carica un agente con `catasto-pp-cli agent-context` per dargli la superficie completa — flag, esempi, exit code — in un colpo solo. Utile per agenti che hanno bisogno della discovery prima di poter usare il CLI in modo produttivo.

## Uso standalone (senza agente)

Il CLI funziona come strumento ordinario da riga di comando per uso umano. I flussi più comuni:

```bash
# 1. Verifica che gli endpoint upstream siano raggiungibili.
catasto-pp-cli doctor

# 2. Risolvi un punto italiano qualsiasi al suo riferimento catastale.
catasto-pp-cli gps 12.4924 41.8902

# 3. Recupera le coordinate di una particella dal suo riferimento catastale.
catasto-pp-cli cadastral --comune Roma --provincia RM --foglio 508 --particella B

# 4. Risolvi i metadati di un comune (senza rete).
catasto-pp-cli comune --cap 90121 --json | jq '.[].nome'

# 5. Reverse-geocode batch di un CSV di coordinate.
cat coords.csv | catasto-pp-cli gps --stdin --json | jq -r '"\(.lon),\(.lat),\(.result.COD_COMUNE),\(.result.FOGLIO),\(.result.NUM_PART)"' > arricchito.csv
```

In terminale l'output di default è una tabella leggibile dall'umano; metti in pipe (o passa `--json`) per ottenere JSON per altri strumenti.

## Autenticazione

Nessuna credenziale richiesta. Entrambe le sorgenti dati (l'ajax dell'Agenzia delle Entrate e i Parquet di ondata su GitHub) sono accessibili pubblicamente senza registrazione.

## Quick Start

```bash
# Verifica che l'AdE sia raggiungibile prima di affidarsi all'output.
catasto-pp-cli doctor

# Forward lookup: coordinate GPS → riferimento catastale (Colosseo).
catasto-pp-cli gps 12.4924 41.8902 --json

# Reverse lookup: riferimento catastale → coordinate del centroide.
catasto-pp-cli cadastral --comune H501 --foglio 508 --particella B --json

# Controllo sintassi parse-only; nessuna chiamata API.
catasto-pp-cli validate --comune H501 --foglio 508 --particella B --json
```

## Funzionalità uniche

Capacità non disponibili in nessun altro strumento per questa API.

### Composabilità agent-native
- **`gps`** — Risolve un punto WGS84 lon/lat al suo riferimento catastale (provincia, comune, foglio, particella). Supporta singolo punto e streaming batch.

  ```bash
  catasto-pp-cli gps 12.4924 41.8902 --json
  ```

### Stato locale che si compone
- **`cadastral`** — Reverse lookup: dato un comune (codice belfiore, nome+provincia, o CAP) + foglio + particella, ritorna le coordinate WGS84 del centroide. Alimentato dal dataset Parquet di ondata/dati_catastali, cachato localmente al primo uso.

  ```bash
  catasto-pp-cli cadastral --comune H501 --foglio 508 --particella B --json
  ```

- **`comune`** — Resolver standalone per il comune (no network). Tre modalità di input: codice belfiore, nome+provincia, o CAP. Embedded ~7.900 comuni italiani con dati ISTAT + ANCI.

  ```bash
  catasto-pp-cli comune --cap 00184 --json
  ```

### Ergonomia da campo
- **`validate`** — Validatore parse-only per riferimenti catastali. Spiega le regole di forma senza chiamare alcuna API.

  ```bash
  catasto-pp-cli validate --comune H501 --foglio 508 --particella B --json
  ```

## Uso

Esegui `catasto-pp-cli --help` per il riferimento completo dei comandi e dei flag.

## Output

```bash
# Tabella umana (default in terminale, JSON quando in pipe).
catasto-pp-cli gps 12.4924 41.8902

# JSON per script e agenti.
catasto-pp-cli gps 12.4924 41.8902 --json

# Filtra solo i campi necessari.
catasto-pp-cli gps 12.4924 41.8902 --json --select COD_COMUNE,FOGLIO,NUM_PART

# Dry run — mostra la richiesta senza inviarla.
catasto-pp-cli gps 12.4924 41.8902 --dry-run

# Modalità agente — JSON + compact + nessun prompt in un solo flag.
catasto-pp-cli gps 12.4924 41.8902 --agent
```

## Health Check

```bash
catasto-pp-cli doctor
```

Verifica configurazione e raggiungibilità dell'API.

## Configurazione

File di config: `~/.config/catasto-pp-cli/config.toml`.

Header HTTP statici si configurano sotto `headers`; eventuali override per-comando hanno precedenza.

## Cookbook

Ricette pratiche per i casi più comuni.

### Round-trip: confermare che un punto corrisponda alla sua particella

```bash
# Forward: dalle coordinate al riferimento
catasto-pp-cli gps 12.4924 41.8902 --json --select COD_COMUNE,FOGLIO,NUM_PART
# {"COD_COMUNE":"H501","FOGLIO":"508","NUM_PART":"B"}

# Reverse: dal riferimento alle coordinate
catasto-pp-cli cadastral --comune H501 --foglio 508 --particella B --json --select lat,lon
# {"lat":41.890252,"lon":12.492405}
```

### Risolvere un indirizzo romano da un CAP

```bash
catasto-pp-cli comune --cap 00184 --json --select nome,codice_belfiore,provincia_sigla
# [{"nome":"Roma","codice_belfiore":"H501","provincia_sigla":"RM"}]
```

### Reverse-geocode batch di un CSV

```bash
# coords.csv: una riga "lon,lat" per linea
cat coords.csv | catasto-pp-cli gps --stdin --json --select result.COD_COMUNE,result.FOGLIO,result.NUM_PART
```

### Disambiguare un nome omonimo

L'Italia ha 7 casi di comuni omonimi (es. Castro in BG e LE). Aggiungi `--provincia`:

```bash
catasto-pp-cli cadastral --comune Castro --provincia BG --foglio 1 --particella 1 --json
# risolve a C337 (Castro, Bergamo)
```

Oppure usa il resolver standalone per vedere prima i candidati:

```bash
catasto-pp-cli comune --name Castro
# Error: multiple comuni match: name="Castro" → 2 candidates: Castro (BG, C337); Castro (LE, M261)
```

### Pre-flight: validare prima di importare un foglio Excel

```bash
catasto-pp-cli validate --comune H501 --foglio 508 --particella B --json
# {"valid":true,"comune":"H501",...}
# exit 0

catasto-pp-cli validate --comune ROMA --foglio abc --particella ""
# {"valid":false,"errors":["foglio \"abc\" is not numeric","particella is required"]}
# exit 2
```

### Errori diagnostici quando una particella non si trova

Il CLI distingue tre modalità di fallimento, così sai distinguere errori di input da gap nel dataset:

```bash
# Comune sconosciuto
catasto-pp-cli cadastral --comune ZZZZ --foglio 1 --particella 1
# Error: comune ZZZZ has 0 rows in 12_Lazio.parquet (check the codice belfiore)

# Foglio inesistente
catasto-pp-cli cadastral --comune H501 --foglio 9999 --particella 1
# Error: comune=H501 has N distinct foglios but none match foglio=9999; nearest existing foglio is ...

# Particella inesistente (comune e foglio giusti)
catasto-pp-cli cadastral --comune G273 --foglio 35 --particella 1900
# Error: comune=G273 foglio=35 exists with 1530 parcels, but particella=1900 is not among them (nearest: ...)
```

L'ultimo caso è particolarmente utile: se il CLI dice "il foglio esiste con N particelle" allora la particella che hai inserito davvero non esiste nello snapshot ondata — probabilmente perché lo snapshot è precedente a un frazionamento/rinumerazione, o perché la particella usa una sezione che il dataset ondata non espone.

## Troubleshooting

**Errori "not found" (exit code 3)**
- Controlla che il codice della risorsa sia corretto.
- Esegui il comando `list` per vedere gli elementi disponibili.

### Specifici per l'API
- **Lookup di un comune del Trentino-Alto-Adige ritorna 'comune not in ondata index'** — TAA gestisce un catasto autonomo, separato dall'AdE; non c'è dataset pubblico. Usa i portali catastali provinciali (catasto.provincia.tn.it / catasto.bz.it).
- **AdE ajax ritorna TIPOLOGIA: STRADA invece di una particella** — Il punto è caduto su una strada, non su una particella. Sposta leggermente la coordinata dentro l'area di proprietà, oppure accetta il risultato strada se è quello che ti serve.
- **AdE ritorna risposta vuota** — Verifica che le coordinate siano dentro l'Italia (lon 6.6–18.5, lat 35.5–47.1). AdE non ritorna errore — semplicemente niente — per punti fuori dalla copertura catastale. Passa `--strict` per convertire le risposte vuote in NotFound (exit 3).
- **La prima chiamata di `cadastral` è lenta** — Al primo uso il file Parquet regionale (pochi MB fino a ~50MB) viene scaricato e cachato nella cache utente del sistema. Le chiamate successive nella stessa regione sono istantanee.

---

## Fonti e ispirazione

Questo CLI è stato costruito studiando questi progetti e risorse:

- [**ondata/dati_catastali**](https://github.com/ondata/dati_catastali) — Python: query DuckDB+Parquet sulla cartografia catastale vettoriale dell'AdE
- [**pigreco/workshop-estate-gis-2021**](https://github.com/pigreco/workshop-estate-gis-2021) — Workshop sull'uso del WMS catastale in QGIS
- [**enricofer/catasto**](https://github.com/enricofer/catasto) — Plugin QGIS per navigare il WMS catastale dell'AdE
- [**matteocontrini/comuni-json**](https://github.com/matteocontrini/comuni-json) — Dataset JSON dei comuni italiani con ISTAT + CAP + codice belfiore (embedded in questo CLI)

Generato dalla [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press).
