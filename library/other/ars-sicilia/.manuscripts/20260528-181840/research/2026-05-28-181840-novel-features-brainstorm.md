# Novel Features Brainstorm — ARS Sicilia (audit trail)

## Customer model

### Persona 1 — Marta, giornalista di cronaca regionale a Palermo
**Oggi (senza questa CLI)**: Apre dati.ars.sicilia.it nel browser, sceglie l'archivio DDL, naviga la maschera JSP del 2008, copia-incolla numero DDL, apre 5 schede per ricostruire l'iter (commissioni, aula, voto). Quando deve verificare un nome firmatario, ripete il giro per interrogazioni, mozioni, ODG, uno per uno.

**Rito settimanale**: Lunedì mattina controlla i DDL nuovi dell'ultima settimana per il pezzo politico del martedì. Mercoledì cerca chi ha firmato cosa per i nomi che girano sui comunicati di partito. Venerdì pesca i resoconti d'aula del giovedì.

**Frustrazione**: La maschera JSP "Icaro" non sa fare cross-archivio. Per dire "tutto quello che ha fatto il deputato Tizio negli ultimi sei mesi" deve fare 7 ricerche separate, copiare a mano in un foglio, sperare di non sbagliare la trascrizione del nome.

### Persona 2 — Andrea Borruso, civic-hacker OpenDataSicilia
**Oggi (senza questa CLI)**: Mantiene `opendatasicilia/RSSdisegniLeggeAssembleaRegionaleSiciliana` (shell scraper + jq + xq) e `aborruso/ars_sicilia` (ETL Python). Ogni volta che il portale cambia un selettore HTML rifà i parser. L'RSS copre solo i DDL — gli altri 11 archivi li interroga a mano con `curl` + `pup`.

**Rito mensile**: Aggiorna i dataset opendata, scrive thread su Mastodon con anomalie trovate (deputato X firma 200 interrogazioni nello stesso giorno), risponde a richieste della community su come scaricare archivi specifici. Costruisce visualizzazioni con `duckdb` + `datasette`.

**Frustrazione**: Tiene in testa la sintassi ISIS (`E`, `O`, `VICINO`, `$`, `%`, `SEL`) perché la maschera CLI dei suoi scraper la richiede. Il copia-incolla shell → jq → xq → CSV per ogni archivio è un peso settimanale. Vuole SQL su SQLite locale, non bash su HTML.

### Persona 3 — Chiara, ricercatrice universitaria in scienze politiche
**Oggi (senza questa CLI)**: Sta scrivendo una tesi/paper sull'attività della XVIII legislatura. Per quantificare "chi parla di più in aula sui temi sanitari" deve scaricare resoconti PDF uno a uno, ctrl+F dentro, contare a mano. Per i co-firmatari fa stessa cosa su DDL e mozioni.

**Rito settimanale**: Apre Zotero, scarica 10-20 documenti ARS, li annota. Compila tabelle Excel di co-firme per le sue analisi di network politico. Cerca pattern (chi presiede quale commissione, quante volte).

**Frustrazione**: Non c'è un modo strutturato di interrogare "ORATOR" o "FIRMAT" come campi. Tutto è prosa HTML. Il portale è progettato per consultazione singola, non per analisi aggregata.

### Persona 4 — Luca, agente AI/assistente conversazionale di un consigliere regionale
**Oggi (senza questa CLI)**: Il consigliere gli chiede "come sta andando il DDL 1500?". Luca non ha un MCP né API; deve aprire il portale e raccontargli a parole. Per "tutte le interrogazioni di Tizio sul tema rifiuti" stessa cosa.

**Rito quotidiano**: Risposte rapide a domande politiche puntuali sul lavoro dell'ARS. Volume basso ma latenza zero pretesa.

**Frustrazione**: Senza un endpoint agent-native, ogni domanda è scraping ad-hoc. Niente è cacheabile, niente è componibile.

## Candidates (pre-cut)

(Full table of 14 candidates — see survivors below for the 9 that passed.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Iter completo di un DDL | `ars-sicilia ddl iter <legisl> <num>` | 9/10 | hand-code | JOIN cronologico su SQLite locale tra `bills` (221), `convocazioni_commissioni` (229), `sommari_commissioni` (230), `resoconti_aula` (217) e `laws` (201) usando `NUMDDL`+`LEGISL` come chiavi e `DATPRE`/`DATSED` per ordinare. | Brief Top Workflow #1 ("cercare e scaricare l'iter di un DDL"); User Vision: aborruso lamenta "nessun cross-reference tra archivi". | none |
| 2 | Profilo deputato cross-archive | `ars-sicilia deputato profilo "<cognome nome>"` | 9/10 | hand-code | Match su `FIRMAT` in `bills`, `interrogazioni`, `interpellanze`, `mozioni`, `ordini_giorno`, `risoluzioni` e su `ORATOR` in `resoconti_aula`; output unificato cronologico con conteggi per tipo. | Brief Top Workflow #2 ("monitorare l'attività di un deputato"); User Vision esplicita: portale non interroga programmaticamente 10 archivi su 12. | none |
| 3 | Co-firmatari di atti | `ars-sicilia analytics --type ddl --group-by cofirmatari --limit 50` | 8/10 | spec-emits | Esplode `FIRMAT` in coppie e raggruppa per pair nei `bills`, restituendo conteggi decrescenti. | Brief Data Layer: campo `FIRMAT` strutturato in più archivi; persona Chiara cita analisi di network politico. | none |
| 4 | Oratori più attivi in aula | `ars-sicilia analytics --type resoconti --group-by oratore --limit 30` | 8/10 | spec-emits | Conta righe in `resoconti_aula` raggruppando per `ORATOR`, opzionale somma lunghezza `TESTO`. | Brief Data Layer cita `ORATOR` come campo strutturato di 217; nessuno scraper esistente lo aggrega. | none |
| 5 | Dossier completo di una commissione | `ars-sicilia commissione dossier <codcom>` | 8/10 | hand-code | JOIN su `CODCOM`/`COMMIS` tra `convocazioni_commissioni` (229), `sommari_commissioni` (230), `pareri` (226) e `bills` filtrati per `RELACO`/`SETTOR`. | Brief Top Workflow #5; workflow non assorbito. | none |
| 6 | Drift dell'iter dei DDL | `ars-sicilia ddl drift --since 7d` | 8/10 | hand-code | Diff tra snapshot SQLite (`bills.ITERAT`/`ITERST`) e stato corrente post-sync, emettendo le righe cambiate nel periodo. | Brief Data Layer: `ITERAT`/`ITERST`; User Vision: oggi RSS shell mostra solo "nuovi", manca il "mossi". | none |
| 7 | Query ISIS grezza | `ars-sicilia <archivio> cerca --isis-query "<expr>"` | 8/10 | hand-code | Flag aggiunto ai comandi `cerca` esistenti che bypassa il flag-builder e inoltra l'espressione direttamente come `icaQuery` al motore Icaro. | Brief Codebase Intelligence documenta sintassi ISIS completa; User Vision: aborruso usa già ISIS direttamente. | none |
| 8 | Stato di sync degli archivi | `ars-sicilia sync stale` | 6/10 | hand-code | Legge `meta.last_synced_at` per ognuno dei 12 archivi e calcola età sync, n. record locali. | Brief Data Layer cita `meta.last_synced_at`; persona Andrea ha esigenza ops. | none |
| 9 | Cronologia inversa di una legge | `ars-sicilia legge cronologia <legisl> <num>` | 5/10 | hand-code | Inverso di `ddl iter`: parte da `laws` (201), risale al `bills` originario, raccoglie passaggi in 230/217. | Brief Top Workflow #3 + Workflow #1; persona Chiara su ricerca corpus storico. | Usare questo comando solo per una legge GIA' promulgata (archivio 201). Per un DDL ancora in iter usare `ars-sicilia ddl iter`. |

### Killed candidates

| Feature | Motivo del kill | Sibling sopravvissuto più vicino |
|---------|-----------------|----------------------------------|
| C9 — `biblioteca export --format bibtex` | Niche, una sola persona (Chiara) lo userebbe sporadicamente; format-converter senza trascendenza cross-archive. | Nessuno diretto; resta `biblioteca cerca` (assorbito #14). |
| C11 — `legge diff <legisl> <num>` | Build feasibility 1: testo storico HTML rumoroso, diff con falsi positivi alti su normalizzazione. | C9 `legge cronologia` (passaggi, non testo). |
| C12 — `commissioni agenda --week current` | Thin wrapper su `commissioni convocazioni` (assorbito #12). | C5 `commissione dossier` + `commissioni convocazioni`. |
| C13 — `analytics --type leggi --group-by anno` | Group-by triviale ottenibile in SQL su SQLite locale; non discriminante. | C3 e C4. |
| C14 — `tail --resource ddl --follow` | Già assorbito (#2 manifest). | `tail` framework. |
