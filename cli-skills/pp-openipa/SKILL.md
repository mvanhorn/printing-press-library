---
name: pp-openipa
description: "Il primo CLI per l'Indice delle Pubbliche Amministrazioni — lookup istantaneo di enti, PEC, codici IPA,... Trigger phrases: `trova la PEC di un comune`, `codice IPA per fattura elettronica`, `codice destinatario SDI di un ente pubblico`, `cerca amministrazione pubblica per nome`, `verifica CF ente PA`, `usa openipa`, `indicepa`."
author: "aborruso"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - openipa-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/openipa/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# IndicePA — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `openipa-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install openipa --cli-only
   ```
2. Verify: `openipa-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/openipa/cmd/openipa-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

openipa porta sul terminale le 22 API web service di IPA che gli sviluppatori usano copia-incollando curl. Con un singolo comando `openipa cf <CF>` ottieni uffici FE, nodi NSO e domicilio digitale in parallelo — tre roundtrip in uno.

## Perché openipa?

Il portale IPA richiede navigazione manuale ente per ente. openipa risolve tre problemi concreti per gli agenti che lavorano con la PA italiana:

- **Codice destinatario SDI in un comando** — `fatturazione cf <CF>` restituisce tutti i `cod_uni_ou` abilitati, pronti per la testata XML della fattura PA.
- **Compliance check parallelo** — `cf <CF>` interroga SFE + NSO + domicilio digitale in simultanea e produce una checklist pass/fail.
- **Batch senza loop** — `fatturazione batch` legge centinaia di CF da stdin e torna NDJSON.

## Workflow Recipes

### Emettere una fattura PA

```bash
# Trova il codice IPA dell'ente
openipa-pp-cli enti cerca --nome "comune di Roma" --json | jq '.data.data[0].cod_amm'
# Ottieni cod_uni_ou per la fattura
openipa-pp-cli fatturazione cf --cf 02438750586 --json
# Verifica compliance completa
openipa-pp-cli cf 02438750586
```

### Verificare una PEC prima di inviarci notifiche

```bash
openipa-pp-cli domicilio verifica <pec-ente>
openipa-pp-cli cerca <pec-ente> --json
```

### Navigare la struttura di un ente

```bash
openipa-pp-cli enti tree agid --json
openipa-pp-cli uo list --codice agid --json
```

## When to Use This CLI

Usa openipa quando un agente deve trovare dati anagrafici, PEC, codici destinatario SDI o nodi NSO di enti della PA italiana. È il tool giusto per validare CF contro il registro IPA prima di emettere fatture, verificare se un'amministrazione è abilitata alla fatturazione elettronica, o trovare il domicilio digitale ufficiale a cui inviare notifiche.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Workflow PA in un comando
- **`cf`** — Dato un codice fiscale, verifica in un colpo se l'ente ha SFE attivo, NSO abilitato e domicilio digitale — checklist compliance PA completa.

  _Un agente che verifica la compliance PA deve sapere se un ente è pronto a ricevere fatture, ordini e notifiche digitali in un unico check._

  ```bash
  openipa-pp-cli cf 97735020584 --json
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
- **`cerca`** — Dato un indirizzo email o PEC, trova l'ente IPA titolare — AMM, AOO o UO — con cod_amm e tipo entità.

  _Un agente che riceve una PEC in ingresso può risalire all'ente mittente senza conoscere il codice IPA._

  ```bash
  openipa-pp-cli cerca <pec-ente> --json
  ```

## Command Reference

**aoo** — Aree Organizzative Omogenee degli enti

- `openipa-pp-cli aoo get` — AOO di un ente con filtro opzionale per codice AOO
- `openipa-pp-cli aoo list` — Lista delle AOO di un ente

**cerca** — Ricerca trasversale — trova entità IPA per email

- `openipa-pp-cli cerca` — Trova entità IPA (AMM/AOO/UO) associate a un indirizzo email

**domicilio** — Domicili digitali (PEC e SERC) delle entità IPA

- `openipa-pp-cli domicilio aoo` — Domicilio digitale attivo di una AOO
- `openipa-pp-cli domicilio cf` — Domicilio digitale di un ente per codice fiscale
- `openipa-pp-cli domicilio email` — Cerca entità IPA tramite indirizzo domicilio digitale (PEC)
- `openipa-pp-cli domicilio storico-aoo` — Storico domicili digitali di una AOO (inclusi cessati)
- `openipa-pp-cli domicilio storico-uo` — Storico domicili digitali di una UO (inclusi cessati)
- `openipa-pp-cli domicilio uo` — Domicilio digitale attivo di una UO per codice univoco

**enti** — Ricerca e dettagli degli enti (Pubbliche Amministrazioni)

- `openipa-pp-cli enti cerca` — Cerca enti per nome o descrizione
- `openipa-pp-cli enti get` — Dati anagrafici completi di un ente per codice IPA

**fatturazione** — Servizi di fatturazione elettronica (SFE) — ricerca uffici destinatari

- `openipa-pp-cli fatturazione cf` — Uffici destinatari fattura elettronica per codice fiscale ente
- `openipa-pp-cli fatturazione ente` — Canali SFE attivi di un ente per codice IPA

**nso** — Nodi di Smistamento Ordini (NSO) per ordini elettronici

- `openipa-pp-cli nso cf` — Nodi NSO per codice fiscale ente
- `openipa-pp-cli nso ente` — Canali NSO attivi di un ente per codice IPA

**uo** — Unità Organizzative degli enti

- `openipa-pp-cli uo get` — Dettagli di una singola UO per codice univoco
- `openipa-pp-cli uo list` — Lista delle UO di un ente

**sede** — Ricerca per nome/area (portale IPA — non API pubblica, nessun AUTH_ID richiesto)

- `openipa-pp-cli sede enti` — Cerca enti per nome, CF, area geografica, categoria
- `openipa-pp-cli sede aoo` — **Cerca AOO per nome testo libero** — unico modo per trovare AOO senza conoscerne il codice (es. prefetture, questure)
- `openipa-pp-cli sede uo` — Cerca UO per nome, area geografica, ente

Restituisce 30 risultati per pagina; aggiungere `--tutti` per scaricare tutto.

**Riepilogo modalità di ricerca AOO:**

| Comando | Input | Quando usarlo |
|---------|-------|---------------|
| `aoo list --codice <cod_amm>` | codice ente padre | vuoi tutte le AOO di un ente noto |
| `aoo cerca <cod_uni_aoo>` | codice univoco AOO | hai già il codice univoco (es. `m_it_001`) |
| `sede aoo --nome <nome>` | testo libero | cerchi per denominazione senza codice — **usa questo come fallback** |

**rtd** — Responsabile Transizione Digitale (portale IPA — non API pubblica)

- `openipa-pp-cli rtd cerca` — Cerca RTD per nominativo, ente, area geografica, categoria

Nota: l'RTD non è esposto dai web service ufficiali IPA (WS01-WS23); questo comando usa il portale PortaleServices.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
openipa-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### CF → codice destinatario fattura

```bash
openipa-pp-cli fatturazione cf --cf 80012000826 --json --select 'data[*].OU[*].cod_uni_ou'
```

Estrai il campo cod_uni_ou da usare nel campo 'Ufficio destinatario' della fattura XML SDI

### Compliance check PA completo

```bash
openipa-pp-cli cf 80012000826 --json
```

Verifica che l'ente abbia SFE attivo, NSO abilitato e domicilio digitale — unico tool che fa tutto in un colpo

### Lista enti per regione offline

```bash
openipa-pp-cli sede enti --area "Sicilia" --tutti --json | jq '.meta.total'
```

Conta gli enti siciliani — usa il portale IPA con auto-paginazione

### Verifica PEC valida

```bash
openipa-pp-cli domicilio verifica <pec-ente> --json --select 'status,tipo'
```

Controlla se una PEC è ancora il domicilio digitale attivo di un ente IPA

### Trova ente per email

```bash
openipa-pp-cli cerca <email-ente> --json --select 'tipo_entita,cod_amm,des_amm'
```

Identifica quale ente/AOO/UO usa una specifica email — utile per de-anonimizzare comunicazioni PA

### Trovare il Responsabile Transizione Digitale

```bash
openipa-pp-cli rtd cerca --ente "Comune di Roma" --json --select 'nomeResponsabile,denominazioneEnte'
```

Restituisce nome e cognome del RTD per un ente. Non disponibile via API pubblica IPA.

```bash
openipa-pp-cli rtd cerca --area "Sicilia" --json | jq '[.data[] | {rtd: .nomeResponsabile, ente: .denominazioneEnte}]'
```

Lista tutti i RTD di una regione.

### Cercare enti per area geografica e indirizzo sede

```bash
openipa-pp-cli sede enti --area "Palermo" --json --select 'denominazioneEnte,strada,numAoo,numOu'
```

Elenca tutti gli enti con sede a Palermo — non disponibile via API pubblica IPA. Usa il nome del comune senza sigla provinciale.

Con `--tutti` vengono scaricate automaticamente tutte le pagine (default: 30 risultati per pagina):

```bash
openipa-pp-cli sede enti --nome "ospedale" --tutti --json | jq '[.data[] | .denominazioneEnte]'
```

### Enti che sono AOO, non enti autonomi

**Regola generale:** se `enti cerca --nome "<nome>"` restituisce 0 risultati, la struttura cercata potrebbe essere una **AOO di un ente padre** (ministero, agenzia, etc.) anziché un ente IPA autonomo. In questo caso usare `sede aoo --nome "<nome>"` per la ricerca per testo libero.

```bash
# fallback generico quando enti cerca non trova nulla
openipa-pp-cli sede aoo --nome "<nome struttura>" --json | jq '.data[0] | {nome: .denominazioneAoo, ente: .denominazioneEnte, pec: .domicili[0].domicilioDigitale}'
```

Alcuni esempi di strutture che sono AOO, non enti autonomi (i nomi IPA esatti variano — usare sempre `sede aoo --nome` per scoprirli):

- **Prefetture** → AOO del Ministero dell'Interno (`m_it`)
- **Questure** → AOO del Ministero dell'Interno
- **Provveditorati** → AOO del Ministero dell'Istruzione

```bash
# esempio: trova il nome IPA esatto e la PEC di una prefettura
openipa-pp-cli sede aoo --nome "prefettura palermo" --json | \
  jq '.data[] | {nome: .denominazioneAoo, pec: .domicili[0].domicilioDigitale}'
```

## Auth Setup

Richiede un AUTH_ID gratuito da registrare su indicepa.gov.it (rilasciato immediatamente). Salvalo come variabile d'ambiente IPA_auth_id o in ~/.config/openipa/config.toml.

Run `openipa-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  openipa-pp-cli aoo list --codice example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
openipa-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
openipa-pp-cli feedback --stdin < notes.txt
openipa-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.openipa-pp-cli/feedback.jsonl`. They are never POSTed unless `OPENIPA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OPENIPA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
openipa-pp-cli profile save briefing --json
openipa-pp-cli --profile briefing aoo list --codice example-value
openipa-pp-cli profile list --json
openipa-pp-cli profile show briefing
openipa-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `openipa-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add openipa-pp-mcp -- openipa-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which openipa-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   openipa-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `openipa-pp-cli <command> --help`.
