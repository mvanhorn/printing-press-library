# ARS Sicilia CLI Brief

## API Identity
- **Domain:** dati.ars.sicilia.it (Assemblea Regionale Siciliana — parlamento regionale, sin dal 1947, organo legislativo della Regione Siciliana).
- **Users:** giornalisti, ricercatori, civic-hacker, OpenDataSicilia, cittadini che vogliono seguire l'iter di un DDL o gli atti di un deputato. Pubblico tecnico italiano.
- **Data profile:** 12 archivi documentali serviti dallo stesso motore di ricerca "Icaro" (Sesame/CDS-ISIS by Zed Software):
  - 201 Leggi Regionali (storico)
  - 217 Resoconti delle Sedute d'Aula
  - 221 Disegni di Legge
  - 226 Pareri Richiesti dal Governo Regionale
  - 229 Convocazioni delle Commissioni
  - 230 Sommari Lavori Commissioni
  - 233 Interrogazioni Parlamentari
  - 234 Interpellanze Parlamentari
  - 235 Mozioni Parlamentari
  - 236 Ordini del Giorno
  - 238 Risoluzioni Parlamentari
  - 205 / 205multimedia Catalogo Bibliografico / Opere Multimediali

  Ogni archivio è una "banca dati" con campi formattati propri (LEGISL, LEGANN, LEGNUM, FIRMAT, ITERAT, ITERST, DATPRE, TITOLO, TESTO, …) e supporta query con linguaggio booleano custom: `E` (AND), `O` (OR), `VICINO N` (NEAR), `$` (prefix), `%` (suffix), `IMG()` (case-sensitive), `SEL(campo *op "valore")`, intervalli numerici (`125/321`).

## Reachability Risk
- **None.** `probe-reachability` ha confermato `standard_http` (95% confidence). Sito pubblico, nessun WAF/Cloudflare. Sessione gestita via `JSESSIONID` cookie. Browser non necessario al runtime.

## Top Workflows
1. **Cercare e scaricare l'iter di un DDL** — dato titolo o anno, trovare il DDL, vederne l'iter completo (presentazione → commissione → aula → approvazione), scaricare il testo originale.
2. **Monitorare l'attività di un deputato** — tutte le interrogazioni, mozioni, ODG e DDL firmati da un certo nome, su una legislatura.
3. **Trovare la legge vigente su una materia** — full-text su 201 Leggi Regionali con filtri per anno/numero/articolo.
4. **Seguire le sedute d'aula** — resoconti per data, con argomenti trattati e oratori. Avvisi su nuovi resoconti.
5. **Lavori di commissione** — convocazioni e sommari per commissione/data/argomento. Cercare chi ha presieduto, chi era presente.

## Data Layer
- **Primary entities** (tabelle SQLite, una per archivio):
  - `laws` (LEGISL, LEGANN, LEGNUM, LEGTIA titolo, LEGEST estremi, LEGSUB, LEGSUD, TIPATT, DATGUR, NUMGUR, DATLEG, TESTO, NOTE)
  - `bills` (= disegni_legge: LEGISL, NUMDDL, TITOLO, DATPRE, FIRMAT, ITERAT, ITERST, RELACO, RELAUL, SETTOR, SOMMAR, TESTO)
  - `interrogazioni` / `interpellanze` / `mozioni` / `ordini_giorno` / `risoluzioni` (struttura affine: LEGISL, NUMORD, TITOLO, DATPRE, FIRMAT, ITERAT, ITERST, RUBRIC, TESTO)
  - `pareri` (LEGISL, COMMIS, INIZIA, NUMISC, REPERT, OGGETT, RICHIE, RIFNOR, ITERAT, ITERST)
  - `resoconti_aula` (LEGISL, ANNSED, DATSED, NUMSED, ARGOME, ORATOR, TESTO)
  - `convocazioni_commissioni` (LEGISL, CODCOM, COMMIS, DATSED, INVITA, NUMFOL, NUMINT, ODG)
  - `sommari_commissioni` (LEGISL, CODCOM, COMMIS, DATSED, NUMSED, PRESID, COMPO[AMPC], ODG, TESTO)
  - `biblioteca` (catalogo, opere multimediali)
- **Sync cursor:** `DATPRE` (data presentazione) o `DATSED` (data seduta) — record sintetico locale `meta.last_synced_at` per archivio+legislatura.
- **FTS/search:** SQLite FTS5 sui campi TESTO/TITOLO/RUBRIC/ARGOME/NOTE per ogni archivio + indice globale cross-archive (`resources_fts`).
- **Normalizzazione date:** `DD.MM.YYYY` → `YYYY-MM-DD` ISO. Fondamentale per range query.

## Codebase Intelligence
- **Endpoint pattern** (verificato via probe e curl):
  - POST `/home/cerca/<NNN>.jsp` con form fields → server costruisce espressione ISIS + risponde con JS `openIcaWindow("<NNN>", "<encoded-query>")`
  - **Shortcut**: GET `/icaro/default.jsp?icaDB=<NNN>&icaQuery=<expr>` direttamente con query costruita client-side, salva un round-trip. Crea/ripristina sessione e memorizza la query con `icaQueryId=1`.
  - GET `/icaro/shortList.jsp?setPage=<N>` → HTML con `<ul id="shortListTable">` paginato (≈10 risultati/pagina).
  - GET `/icaro/doc<NNN>-1.jsp?icaQueryId=1&icaDocId=<N>` → HTML del documento singolo (titolo, testo, metadati, allegati).
- **Sessione:** `JSESSIONID` cookie, TTL 30 min (`ALIVE_DEAD`). Acquisire nuova sessione per ogni comando va bene.
- **Query language ISIS** (da `help.jsp`):
  - Operatori: `E` (AND), `O` (OR), `VICINO` (NEAR), `$` (prefix wildcard, `$N` per limite), `%` (suffix wildcard), `IMG()` (esatto case-sensitive), `SEL(<sigla> <op> "<val>")` (range/comparison su campi formattati), `/` (range numerico).
  - Field qualifier: `(<expr>).<sigla_campo>`, es. `( bilancio ).TITOLO`.
  - Esempi: `(18.LEGISL E 2024.LEGANN) E (all)` = leggi della XVIII legislatura anno 2024.

## User Vision
- L'utente è **aborruso** (autore di `opendatasicilia/RSSdisegniLeggeAssembleaRegionaleSiciliana` shell scraper e di `aborruso/ars_sicilia` ETL Python). Sa esattamente dove fanno male le mancanze del portale ARS:
  - solo navigazione manuale o RSS preconfezionati (DDL e delibere governative)
  - nessun modo di interrogare programmaticamente i 10 archivi rimanenti
  - nessun cross-reference tra archivi
  - testo dei documenti non normalizzato (date `DD.MM.YYYY`, encoding storico)
- Vuole una CLI agent-native con sync locale che gli risparmi tutto lo shell-piping che fa oggi.

## Source Priority
Single-source: solo `dati.ars.sicilia.it`. Nessuna inversione possibile.

## Product Thesis
- **Name:** `ars-sicilia` (binario `ars-sicilia-pp-cli`)
- **Display name:** ARS Sicilia
- **Why it should exist:** Il portale ARS è l'unica fonte ufficiale sull'attività legislativa siciliana, ma è interrogabile solo via UI JSP del 2008. Esiste UN solo scraper community (shell, solo DDL, solo RSS). La nuova CLI:
  - copre tutti i 12 archivi
  - sincronizza in SQLite locale per query SQL, FTS, agent piping
  - costruisce le espressioni ISIS automaticamente da flag CLI puliti (`--anno`, `--firmatario`, `--commissione`)
  - aggrega cross-archive (attività di un deputato attraverso interrogazioni+mozioni+DDL+ODG)
  - espone MCP (Claude Desktop) per consultazione conversazionale
  - traccia drift (nuovi DDL/leggi dall'ultima sync) — sostituisce l'RSS shell con un comando integrato
  - normalizza date, sigle, nomi firmatari
  - emette CSV/JSON/JSONL per pipeline downstream (miller, duckdb, jq)

## Build Priorities
1. **Client Icaro** (hand-rolled, `internal/icaroclient/`): bootstrap sessione + costruzione espressione ISIS + paginazione shortList + fetch doc + parser HTML → struct tipate per ogni archivio.
2. **Store SQLite** per tutti i 12 archivi (data layer P0 generator-emitted, + tabelle custom per ARS-specifiche).
3. **Comandi search per archivio** (P1): `ars-sicilia leggi cerca`, `ddl cerca`, `interrogazioni cerca`, `mozioni cerca`, `interpellanze cerca`, `odg cerca`, `risoluzioni cerca`, `pareri cerca`, `resoconti cerca`, `commissioni convocazioni`, `commissioni sommari`, `biblioteca cerca`.
4. **Get per ID** (P1): `<archivio> get <legisl>/<num>` o `<archivio> get <id>`.
5. **Sync** (P1): `ars-sicilia sync --legisl 18 --dal 2024-01-01 --archivio leggi,ddl` → riempie SQLite. Hook `sync --all` per pieno scarico.
6. **Search globale offline** (P1): `ars-sicilia cerca "bilancio sanitario"` → FTS cross-archive.
7. **Transcendence** (P2): vedi absorb manifest.

