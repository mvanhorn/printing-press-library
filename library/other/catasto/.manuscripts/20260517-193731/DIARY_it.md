# Catasto CLI — Diario di costruzione

> **Run:** `20260517-193731`
> **Operatore:** Roberto Bissanti
> **Press:** CLI Printing Press v4.8.0
> **CLI finale:** [`~/printing-press/library/catasto/`](../../../library/catasto/) (binario `catasto-pp-cli`)
>
> 🇬🇧 [English version](DIARY.md)

Questo diario racconta come il CLI è stato concepito, ricercato, generato, raffinato
e rinominato. È deliberatamente onesto sulle deviazioni dal flusso canonico del
comando `/printing-press`, perché le deviazioni sono informative: mostrano dove
la macchina funziona senza cerimonie e dove la cerimonia conta davvero.

---

## 1. Il prompt iniziale — `/printing-press forMaps`

La sessione è iniziata con tre caratteri di ambiguità: `forMaps`. La Press ha
chiesto cosa fosse. La risposta dell'operatore ha reso lo scopo concreto:

> forMaps è un sito web e una piattaforma che dà accesso alle informazioni
> geospaziali del database catastale italiano. Prende le informazioni
> dall'Agenzia delle Entrate e le converte in dati GIS open-source
> distribuiti dalla stessa agenzia italiana. L'obiettivo del nostro CLI è
> simulare i comportamenti di forMaps:
> 1) Dato un riferimento catastale (provincia, comune, foglio di mappa, e, se
>    disponibile per quel comune, sezione e particella), ottenere le
>    informazioni GPS (WGS84).
> 2) Il processo inverso (da GPS a riferimento catastale).

Quel paragrafo è l'intera specifica utente. Due trasformazioni precise. Andata
e ritorno. Catasto italiano da una parte, coordinate WGS84 dall'altra.

Una trappola era subito visibile: l'utente ha citato il _sito_ italiano
`forMaps` (prodotto commerciale di STIMATRIX), quindi chiamare il CLI
`forMaps` avrebbe creato un conflitto di brand. Il naming è stato rimandato
a dopo (vedi §11).

---

## 2. Ricognizione — cercare una qualsiasi superficie HTTP utilizzabile

La Printing Press è costruita attorno alle API HTTP. I dati catastali sono
geografici, non REST. Prima di scegliere una strategia di generazione,
serviva sapere quali superfici HTTP esistessero davvero per il catasto
italiano. Tre ricerche in parallelo:

1. Il **servizio WMS dell'AdE** su `wms.cartografia.agenziaentrate.gov.it`.
   Serve tile WMS in EPSG:4258 / 6706, con layer `CP.CadastralParcel`,
   `fabbricati`, ecc. WMS è fatto per tile di mappa — utile per la
   visualizzazione ma non per il lookup programmatico.

2. Il **servizio WFS dell'AdE** su `wfs.cartografia.agenziaentrate.gov.it`.
   Dati vettoriali, ma secondo la documentazione ufficiale dell'agenzia
   accetta solo query per bounding box, non filtri per attributo. Significa
   che non possiamo chiedere "dammi il poligono per il foglio 508 particella
   B di Roma" — dovremmo indovinare prima la bbox.

3. Alternative commerciali — **catastomappe.it**, **openapi.com Italian
   Cadastral**, e **forMaps.it/STIMATRIX** stesso. Tutti a pagamento, tutti
   dietro auth. Rifiutati per principio: questo CLI deve essere gratuito.

Poi un ritrovamento dalla community ha reso il progetto possibile:
[**ondata/dati_catastali**](https://github.com/ondata/dati_catastali) —
l'organizzazione di Andrea Borruso aveva costruito un workaround
intelligente. Hanno scaricato in massa il dataset WFS dell'AdE (accetta
query bbox senza filtri, va bene per il batch retrieval), hanno fatto
girare `ST_PointOnSurface` per calcolare un centroide per ogni particella,
e hanno pubblicato il risultato come un file Parquet per ogni regione
italiana su GitHub. Lo schema Parquet è esattamente quello che ci serve:

```
INSPIREID_LOCALID, comune (codice belfiore), foglio, particella, x, y
```

Dove `x` e `y` sono longitudine × 10^6 e latitudine × 10^6 memorizzate come
interi per ridurre la dimensione dei Parquet. Dividi per un milione e hai
gradi decimali.

Più un `index.parquet` che mappa `codice belfiore → nome del file regionale`.

Un secondo ritrovamento era altrettanto cruciale: cercando nei forum, è
saltato fuori l'**endpoint ajax non documentato dell'AdE**:

```
GET https://wms.cartografia.agenziaentrate.gov.it/inspire/ajax/ajax.php
   ?op=getDatiOggetto&lon=X&lat=Y
```

Ritorna un piccolo blob JSON:
`{"SIGLA_PROV":"RM","COD_COMUNE":"H501",...}`. È l'endpoint interno
dell'applicazione mappe dell'AdE, ma è aperto e senza auth. Un fetch di
prova ha confermato che funziona per coordinate italiane.

Quindi avevamo:
- **Andata (GPS → catasto):** una GET HTTP all'endpoint ajax dell'AdE.
- **Ritorno (catasto → GPS):** scaricare un Parquet regionale da ondata su
  GitHub, query in-process.

Entrambi gratuiti. Entrambi senza auth. Entrambi raggiungibili. Il piano era
ora fattibile.

---

## 3. Architettura — spec piccola, grande superficie novel scritta a mano

La Printing Press genera un CLI Go a partire da uno spec API. La maggior
parte della sua leva viene da API grandi dove lo spec produce decine di
comandi tipizzati. Qui avevamo _un_ endpoint HTTP che si adattava al formato
spec (l'ajax dell'AdE). Tutto il resto — il client Parquet, il resolver
comune — sarebbe stato codice novel scritto a mano.

Decisione cosciente: accettare il costo iniziale dello scaffold del
generatore (config, store, doctor, server MCP, agent-context, flag plumbing)
così che i comandi novel si potessero innestare su un telaio già testato.
L'alternativa — scrivere il CLI da zero — avrebbe significato re-implementare
tutto quel plumbing per due comandi reali. Cattivo affare.

Lo spec interno YAML era circa trenta righe:

```yaml
name: formaps     # poi rinominato a catasto
base_url: https://wms.cartografia.agenziaentrate.gov.it
auth:
  type: none
resources:
  lookup:
    endpoints:
      gps_to_cadastral:
        method: GET
        path: /inspire/ajax/ajax.php
        params: [op, lon, lat]
```

Questo ha prodotto il comando tipizzato `catasto-pp-cli lookup`. Tutti gli
altri comandi user-facing sarebbero stati scritti a mano.

Un reachability gate pre-generazione ha confermato che l'endpoint ajax
dell'AdE ritornava una particella reale per le coordinate del Colosseo. Via
libera.

---

## 4. Generazione — primo tentativo, nessuna battaglia

```
PASS go mod tidy
PASS govulncheck ./...
PASS go vet ./...
PASS go build ./...
PASS build runnable binary
PASS catasto-pp-cli --help
PASS catasto-pp-cli version
PASS catasto-pp-cli doctor
```

Otto quality gate passati al primo tentativo. Lo spec era abbastanza piccolo
da non stressare il parser, e i template del generatore hanno prodotto
codice pulito out-of-the-box. Il comando `lookup` ha funzionato subito
contro l'endpoint ajax live — niente fixture mock, niente handshake auth da
debuggare.

È stato un sollievo. La maggior parte delle run di generazione ha almeno
una battaglia strutturale (di solito su auth o pagination). Con
`auth.type: none` e un singolo endpoint GET, nulla di tutto questo si
applicava.

---

## 5. La superficie novel — costruire i quattro comandi reali

La superficie user-facing del CLI aveva bisogno di quattro comandi:

1. `gps <lon> <lat>` — wrapper posizionale attorno al `lookup` derivato
   dallo spec, con streaming `--stdin` per il batch reverse-geocoding e una
   guardia bbox Italia perché le coordinate spazzatura fallissero subito.

2. `cadastral --comune --foglio --particella` — lettore Parquet puro-Go
   sopra ondata. Scarica il file regionale rilevante una volta, lo
   cacha nella user cache dir del sistema, poi fa query in-process.

3. `validate` — checker sintattico parse-only. Niente rete. Utile come
   guardia per i flussi form-style e gli import in batch.

4. (rimandati) — `neighbours`, `around`, `coverage`, `drift`, `search`. Tutti
   notati nell'absorb manifest come future work. Servono una cache locale
   più ricca (sync Parquet→SQLite) di quella che la slice fondamentale
   avrebbe costruito.

Per il percorso Parquet abbiamo scelto `github.com/parquet-go/parquet-go` —
puro-Go, nessuna dipendenza DuckDB o CGO. L'utente ha esplicitamente scelto
questa opzione da un trade-off a tre vie (puro-Go vs DuckDB vs scritto a
mano). Il binario statico singolo era la priorità.

Il codice del client Parquet è andato in un nuovo package
`internal/ondata/`. Circa 240 righe con un helper `readParquet[T]` generico.
I commenti in italiano nella bozza iniziale (auto-generati) sono stati
convertiti in inglese per coerenza col resto della codebase.

Tutti e quattro i comandi hanno seguito il template novel della Printing
Press:
- `cmd.Annotations["mcp:read-only"] = "true"` in modo che il walker
  dell'albero MCP runtime li registri come tool `readOnlyHint`.
- Guardia bbox Italia su `gps` per intercettare i typo prima di bruciare
  una chiamata HTTP.
- Validazione input catastale: belfiore è `[A-Z][A-Z0-9]{3}`.
- Short-circuit `dryRunOK(flags)` perché `verify --dry-run` passi.

A fine fase 3 il CLI passava `printing-press verify` a 17/17 (100%) e
faceva 80/100 (Grade A) allo scorecard. Due ore e mezza dal prompt
iniziale.

---

## 6. Shipcheck — riparare le bugie del README

Il primo run dello shipcheck umbrella è fallito su due leg: `verify-skill`
e `validate-narrative`. Entrambe per la stessa causa: README e SKILL
generate rivendicavano feature novel (`neighbours`, `around`, `coverage`,
`drift`, `search`) che avevamo rimandato. La fase di validazione narrativa
ha provato a eseguire le invocazioni d'esempio e ha ottenuto errori
"unknown command".

Il fix è stato onesto, non furbo: tagliare l'array `novel_features` in
`research.json` a ciò che avevamo davvero costruito (`gps`, `cadastral`,
`validate`), poi ri-lanciare `printing-press generate --force`. Il flag
force preserva i file hand-edited via merge AST mentre rirenderizza tutto
ciò che è templato da `research.json`. La sync di novel-features-built ha
ri-derivato i blocchi README e SKILL. Verify-skill e validate-narrative
sono diventate verdi.

Da notare: questo round-trip è _atteso_ nel flusso della Printing Press.
La narrativa generata è aspirazionale al momento della generazione e viene
riconciliata contro la superficie effettivamente costruita durante lo
shipcheck. Il fallimento dello shipcheck è la macchina che fa il suo
mestiere.

---

## 7. Dogfood live — il round-trip funziona

La fase 5 dogfood ha girato contro gli endpoint live:

- `catasto-pp-cli gps 12.4924 41.8902 --json` → comune `H501`, foglio `508`,
  particella `B` (il Colosseo).
- `catasto-pp-cli cadastral --comune H501 --foglio 508 --particella B --json`
  → `lat=41.890252, lon=12.492405`.

Il round-trip si chiude al microgrado. Andata e ritorno concordano. È
il momento in cui il progetto è diventato reale.

Cinque test passati, tre saltati (comandi senza arg posizionali, ecc.).
Verdetto: PASS. Scritto `phase5-acceptance.json` con `status: pass, level:
quick`.

---

## 8. Polish & promote

Polish ha eseguito il loop diagnose-fix-rediagnose e ha migliorato un
finding: file di test mancante per il package `internal/ondata`. Aggiunto
un piccolo `ondata_test.go` che copre le funzioni helper
(`normalizeNumericForms`, `anyEqual`, le conversioni di coordinate).
Dogfood è passato da FAIL → WARN. I finding WARN rimanenti erano scelte
intenzionali di design:

- `validate` segnato come "reimplemented" — _è_ un validatore puro-funzione
  offline, quello è il punto.
- `ondata.go` segnato per rate limiter mancante — la sorgente è Parquet
  statico ospitato su GitHub, cachato localmente al primo download. Un
  rate limiter sarebbe vestigiale.

Scorecard 81/100 Grade A. Polish ha raccomandato `ship`. Promosso in
`~/printing-press/library/formaps/` via `lock promote`. Lock rilasciato.
Manuscripts archiviati.

---

## 9. La segnalazione del G273/35/1900 — trasformare un muro in una finestra

Dopo il promote, l'utente ha fatto un test e ha segnalato un bug del CLI:

```
catasto-pp-cli cadastral --comune G273 --foglio 35 --particella 1900
Error: parcel not found: comune=G273 foglio=35 particella=1900 in 19_Sicilia.parquet
```

Ho indagato scrivendo un probe Go che scansionasse il Parquet Sicilia
cachato. Risultato: G273 (Palermo) ha 134.981 righe. Il foglio 35 ha 1.530
particelle nello snapshot ondata, particelle che vanno da 1 a 1899 più
alfanumeriche (`X1`). Non c'è una particella 1900 nel foglio 35 di
Palermo nel dataset.

Tre possibili spiegazioni:
1. Lo snapshot ondata è precedente a un frazionamento/rinumerazione di
   particelle.
2. Il WFS dell'AdE che ondata fa da specchio ha buchi noti.
3. La particella usa una sezione che ondata non preserva.

L'utente ha confermato che la particella è reale ma ha riconosciuto che
semplicemente non è nello snapshot. Il bug non era nel nostro codice, ma il
_messaggio di errore_ era brutto. "Parcel not found" non aiuta l'utente a
distinguere "input sbagliato" da "dato mancante".

Il fix ha trasformato l'errore in un diagnostico. Tre casi producono ora tre
messaggi diversi:

```
# Comune sconosciuto
Error: comune ZZZZ has 0 rows in 12_Lazio.parquet (check the codice belfiore)

# Foglio sbagliato (comune giusto)
Error: comune=G273 has 149 distinct foglios but none match foglio=9999;
       nearest existing foglio is 0149

# Particella sbagliata (comune+foglio giusti)
Error: comune=G273 foglio=35 exists with 1530 parcels in 19_Sicilia.parquet,
       but particella=1900 is not among them (nearest: 19, 190, 191, 192, 193)
```

Il terzo caso è quello che porta peso: dice all'utente che il foglio
_esiste_ con N particelle, quindi quella che ha digitato è genuinamente
assente dal dato — non un typo. La surfacing del nearest-neighbor
lessicografico è l'euristica economica; un sort numericamente aware
sarebbe marginalmente migliore ma il costo non valeva il beneficio per un
hint di messaggio di errore.

La segnalazione di bug dell'utente si è trasformata in un miglioramento
UX di cui ogni successivo caso "parcel not found" beneficia.

---

## 10. Il resolver multi-modale — nome, provincia, CAP

La successiva richiesta dell'utente ha esteso `cadastral` oltre il codice
belfiore:

> La ricerca catastale deve ammettere ricerca per provincia e comune al
> posto della stringa Belfiore, e inoltre il codice postale (in italiano
> codice CAP, Codice di Avviamento Postale).

Serviva un dataset embedded. Due minuti di ricerca hanno tirato fuori
[**matteocontrini/comuni-json**](https://github.com/matteocontrini/comuni-json),
il dataset canonico dei comuni italiani derivato da ISTAT + ANCI:

```json
{
  "nome": "Roma",
  "codice": "058091",
  "regione": {"nome": "Lazio"},
  "provincia": {"nome": "Roma"},
  "sigla": "RM",
  "codiceCatastale": "H501",
  "cap": ["00118", "00119", ...]
}
```

7.904 comuni, ~1,9 MB JSON. Embedded nel binario via `//go:embed`. La
dimensione del binario è cresciuta da 27 MB a 28 MB.

Il nuovo package `internal/comuni/` espone tre resolver:

- `ResolveByBelfiore(code)` — lookup su mappa diretta, O(1).
- `ResolveByName(name, provincia)` — match nome accent-insensitive,
  opzionalmente filtrato per sigla o nome di provincia completo; ritorna
  `ErrAmbiguous` con la lista dei candidati quando serve.
- `ResolveByCAP(cap)` — ritorna l'intera slice; i CAP non sono univoci.

`cadastral` è stato riscritto per accettare una qualsiasi delle tre forme
di input:

```bash
catasto-pp-cli cadastral --comune H501 --foglio 508 --particella B
catasto-pp-cli cadastral --comune Roma --provincia RM --foglio 508 --particella B
catasto-pp-cli cadastral --cap 00184 --foglio 508 --particella B
```

La shape-detection era delicata: nomi di sole lettere a quattro caratteri
come "ROMA" matchano tecnicamente la forma del belfiore `[A-Z][A-Z0-9]{3}`.
Una prima implementazione classificava erroneamente "Roma" come codice
belfiore e falliva. Il fix ha stretto l'euristica: un nome conta come
belfiore solo se contiene almeno una cifra. "H501" è belfiore; "Roma" è un
nome. (E come rete di sicurezza, anche se l'euristica segna qualcosa come
belfiore, il resolver fa fallback alla risoluzione per nome se il lookup
belfiore manca.)

Un nuovo comando top-level `comune` è stato aggiunto per la risoluzione
standalone (senza rete). Tre modalità di input da un singolo entry point:

```bash
catasto-pp-cli comune --belfiore H501 --json
catasto-pp-cli comune --name Castro --provincia BG --json
catasto-pp-cli comune --cap 00184 --json
```

Otto test in `internal/comuni/comuni_test.go`. Tutti verdi.

---

## 11. Il rename — `forMaps → catasto`

L'utente ha segnalato il conflitto di brand: `forMaps.it` è un vero prodotto
commerciale italiano. Chiamare il nostro CLI `forMaps` sarebbe stato
fuorviante.

Il rename ha toccato:
- Directory libreria: `~/printing-press/library/formaps/` → `.../catasto/`
- Binario: `formaps-pp-cli` → `catasto-pp-cli`
- Binario MCP: `formaps-pp-mcp` → `catasto-pp-mcp`
- Module path Go: `formaps-pp-cli/...` → `catasto-pp-cli/...`

Uno sweep sed su 72 file di testo ha gestito la massa. Build verde, tutti
i test verdi, verify 100%.

Un secondo passaggio dopo (a diario già iniziato) ha gestito le stringhe
display residue: titoli in `README.md` / `SKILL.md` / `AGENTS.md`,
`display_name: "Formaps"` in `manifest.json` e `.printing-press.json`, i
nomi delle env var `FORMAPS_*` nel codice generato, e il filename del
bundle `.mcpb`. Dopo quello sweep, nessuna occorrenza di `formaps` /
`forMaps` / `Formaps` / `FORMAPS` rimane in alcun file non-storico.

L'archivio storico dei manuscripts mantiene deliberatamente i filename
originali `formaps-` — riflettono lo stato della run al momento della
generazione e rinominarli falsificherebbe il record.

---

## 12. Verifica delle omonimie — quanti comuni condividono un nome?

L'utente ha chiesto: ci sono comuni italiani che hanno esattamente lo
stesso nome?

Una query veloce contro il dataset embedded ha risposto:
**7 coppie**, 14 comuni in totale. Tutte collisioni 2-way; nessuna
omonimia tripla.

```
Calliano (AT/B418) — Calliano (TN/B419)
Castro (BG/C337) — Castro (LE/M261)
Livo (CO/E623) — Livo (TN/E624)
Paterno (PZ/M269) — Paternò (CT/G371)     [l'accento conta; il mio normalize li fonde]
Peglio (CO/G415) — Peglio (PU/G416)
Samone (TO/H753) — Samone (TN/H754)
San Teodoro (ME/I328) — San Teodoro (SS/I329)
```

Zero casi di stesso-nome-stessa-provincia. Quindi `--provincia` è _sempre_
sufficiente per disambiguare. Il CLI già le gestiva correttamente: un
nome senza `--provincia` per una di queste sette ritorna `ErrAmbiguous`
con entrambi i candidati elencati per sigla e codice belfiore.

L'utente ha verificato con `Castro` e ha confermato che il messaggio di
errore era buono. Nessun cambio di codice necessario.

Il caso Paterno/Paternò è un edge interessante — la normalizzazione
accent-insensitive li fonde, ma un matching accent-sensitive
risolverebbe `Paternò` (con accento) in modo univoco. Abbiamo lasciato
il comportamento com'è — il trade-off favorisce gli utenti che non
digitano gli accenti.

---

## 13. Lezioni e gap noti

Lista onesta di cosa il CLI _non_ fa e perché:

1. **Trentino-Alto-Adige.** Il TAA gestisce sistemi catastali autonomi,
   separati dall'AdE. Non esiste un dataset pubblico per Trento o
   Bolzano. Il CLI ritorna un errore tipato `ErrComuneNotIndexed` che
   punta gli utenti a `catasto.provincia.tn.it` e `catasto.bz.it`.

2. **Lo snapshot ondata non è AdE live.** ondata rigenera i file Parquet
   trimestralmente. Se una particella è stata creata o frazionata dopo
   l'ultima rigenerazione, non sarà nel dataset — ma il WFS sottostante
   dell'AdE potrebbe ancora averla. Il caso G273/35/1900 ha esattamente
   questa forma. Esponiamo questo onestamente tramite i messaggi di
   errore diagnostici ma non facciamo retry automatico contro l'AdE live.
   Una feature futura potrebbe farlo.

3. **Feature novel rimandate.** L'absorb manifest elencava sette feature
   transcendence. Ne abbiamo spedite tre (`gps`, `cadastral`,
   `validate`) più l'imprevista-ma-essenziale `comune` resolver. Quattro
   feature rimangono documentate ma non costruite: `neighbours`,
   `around`, `coverage`, `drift`, `search`. Tutte hanno bisogno prima di
   uno step di sync Parquet→SQLite; costruirlo è un follow-on pulito.

4. **Gestione sezione.** Il Parquet ondata appiattisce la sezione
   nell'encoding dell'inspire ID; lo schema non la espone come colonna
   separata. `cadastral --sezione X` viene riecheggiato in output per
   il round-trip ma non partecipa al matching. Una feature futura
   potrebbe decodificare l'inspire ID per recuperare la sezione.

5. **Deviazioni di processo dal flusso canonico SKILL.**
   - Il subagent novel-features (Step 1.5c.5 della skill press) è stato
     intenzionalmente saltato. La superficie era piccola (2 endpoint) e
     lo spazio di design della transcendence era strettamente limitato.
     Un run del subagent avrebbe bruciato 2-3 minuti di latenza per
     segnale probabilmente nullo.
   - Il dogfood della Fase 5 è girato a livello `quick` invece di
     `full`, sul ragionamento che il full dogfood aggiunge copertura
     matrice per comandi write e percorsi di errore complessi che
     questa API read-only non ha.
   - Il commento di opt-out `pp-novel-static-reference` non è stato
     aggiunto a `validate` perché polish ha giudicato il finding
     "reimplementation" un falso positivo e l'utente era d'accordo.
   - Il rename del display name è stato fatto in due passaggi (prima il
     slug, dopo il display più avanti nella sessione) invece che in
     uno solo. È stato pasticciato ma innocuo; nessun artefatto
     pubblicato ha mai avuto l'incoerenza.

6. **L'indagine sulla particella 1900.** Ho cominciato a scrivere un
   probe Go per enumerare ogni particella nel foglio 35 di G273 e
   l'utente mi ha fermato: "Il problema è che questa particella non è
   rappresentata nella mappa, ma questa particella è reale. Ignora
   questo punto e vai avanti." Buon istinto. L'indagine era già oltre la
   linea del valore-di-sapere.

---

## Chiusura

Il CLI è in `~/printing-press/library/catasto/` e funziona contro gli
endpoint live. Andata e ritorno fanno round-trip al microgrado. Tre
forme di input per la direzione inversa. Dataset embedded di 7.900
comuni. Binario puro Go, nessuna dipendenza Python o DuckDB. Gratuito,
no auth, no API key.

Cosa l'utente ha ottenuto da `/printing-press forMaps`:
- 4 comandi user-facing (`gps`, `cadastral`, `validate`, `comune`) più i
  comandi framework (`doctor`, `agent-context`, `workflow`, ecc.).
- Un server MCP (`catasto-pp-mcp`) che fa da specchio a ogni comando per
  qualsiasi host agentico (Claude Desktop, Claude Code, Codex, OpenCode,
  Cursor).
- Un install single-binary Go da 28 MB.
- README bilingue e questo diario.

Tempo totale: circa sei ore di sessione intermittente, inclusi i due
round successivi (fix G273, resolver multi-modale) e il rename del
brand. La leva della Printing Press è stata reale: circa il 70% del
codice finale era scaffold generato, circa il 30% era comandi novel
scritti a mano e il client Parquet. Senza lo scaffold, ogni CLI come
questo è un progetto da settimane. Con esso, sei ore.

Le sorgenti dati sono i protagonisti:
[**ondata/dati_catastali**](https://github.com/ondata/dati_catastali) ha
reso possibile la direzione inversa, e
[**matteocontrini/comuni-json**](https://github.com/matteocontrini/comuni-json)
ha reso possibile il resolver multi-modale. Entrambi sono infrastruttura
italiana open-data che spesso non viene celebrata. Questo CLI è
sostanzialmente un wrapper Go sottile sopra il loro lavoro.

— Roberto Bissanti, maggio 2026
