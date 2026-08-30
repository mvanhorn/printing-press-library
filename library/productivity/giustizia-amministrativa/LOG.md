# LOG

## 2026-08-22 — quattro interventi sul client del portale

Confronto con un'altra implementazione della stessa fonte (`mcp-legal-it`, elencata ora nei
progetti correlati del README). Quattro cose da prendere, misurate prima di scriverle.

**La pagina di errore di `mdp` finiva nello store come documento.** Un `nomeFile` non piu'
valido non produce un 404: il portale risponde 200 con la propria pagina "404 - Pagina non
trovata", che `HTMLToMarkdown` converte in 136 caratteri di testo plausibile — non vuoti,
quindi non intercettati dal controllo sul testo assente. Ora `isErrorPage` guarda il body
(`<?xml`, `<GA`, `%PDF-` sono documenti veri) e `Document` restituisce un errore che dice di
rifare la ricerca.

**L'id della portlet e' letto dalla pagina.** Contiene un `_INSTANCE_<hash>` che il portale
puo' cambiare a un redeploy, ed era una costante presente in ogni ricerca e nella verifica
appello. L'handshake ora legge dalla pagina del form l'URL `action` — che porta insieme
`p_p_id`, `p_p_lifecycle` e `p_auth` — e lo replica verbatim; le costanti restano come
fallback. Il filtro anno resta server-side (`DataYearItem`): verificato, 5 risultati su 5 del
2019.

**`--sede` accetta le grafie che la CLI stessa stampa.** Emettevamo `ECLI:IT:TARLAZ:...` e poi
rifiutavamo `--sede TARLAZ`. Ora valgono citta', regione (`lazio`, `tar-sicilia`), sede
staccata (`sicilia-catania`) e codice ECLI, con i 31 codici presi dallo store e non supposti.
Quando l'alias nomina una regione con due sedi, un avviso dice quale resta fuori. `TARBOL` e
`TARABR` restano fuori di proposito: come abbreviazioni puntano a Bologna e all'Aquila.

**`get --meta` legge i metadati di registro dall'XML.** `/visualizza/` serve la sorgente XML
della stessa pagina resa da `/visualizzah2/`, e porta cio' che la resa perde: urn NIR, flag
`omissis`, oggetto tenuto dal registro, presidente ed estensore etichettati. Opt-in: costa una
seconda richiesta. Sui provvedimenti in PDF non c'e' XML, e il comando lo dichiara invece di
inventare campi.

Il primo rimedio all'opt-in era sbagliato in verso opposto, e l'ha visto la review: azzerando i
metadati *prima* della scrittura, un `get` semplice li cancellava dallo store e costringeva il
`--meta` successivo a riscaricarli. Ora si azzera solo la copia che va in output, dopo la
scrittura. Cercando lo stesso schema altrove sono usciti altri due punti che nessuno aveva
segnalato: `persistProvvedimenti` conservava il testo ma non i metadati, quindi bastava una
ricerca a cancellarli; e `corpus build`, che risolve dallo store, avrebbe scritto schede
diverse fra loro a seconda di quali provvedimenti qualcuno avesse gia' letto con `--meta`.

Tre rilievi della review, tutti veri: i metadati salvati nello store riaffioravano in un `get`
senza `--meta` (opt-in che valeva solo la prima volta), `data_pubblicazione` era documentata ma
non emessa nel front matter, e l'`action` letta dalla pagina andava ripulita dall'escape HTML
dei separatori — oggi il portale li scrive nudi, ma il nostro stesso test li aveva escapati
senza che me ne accorgessi.

Copertura misurata su 10 provvedimenti di 10 sedi diverse, dal 2010 al 2026: estensore, urn e
data di pubblicazione 10 su 10; presidente 5 su 10 (nel documento la sua firma e' spesso
vuota); oggetto di registro 4 su 10. `omissis` risulta vero sui provvedimenti anonimizzati.
L'urn ha pero' numero e data a `00000-0000` in tutti e dieci: identifica organo, sezione e
tipo, non il singolo provvedimento. Lo scrive cosi' il portale, e ora lo diciamo dove il campo
viene documentato, invece di presentarlo come chiave di citazione.

## 2026-08-18 — primo uso del bundle MCPB in Claude Desktop

### `data_deposito` e' sempre vuoto in ricerca, e nessuno lo dice

Misurato: 37 risultati su 37, sedi diverse, `data_deposito: ""`. Non e' un bug del
parsing. La data si ricava con `ExtractDataDeposito(docHTML)` (`internal/cli/ga_core.go:353`),
cioe' leggendo il documento: l'indice di ricerca del portale non la espone, e il campo si
popola solo dove il testo c'e' gia' — `get`, `massime`, `corpus build`.

Il difetto e' nel segnale, non nel dato: in ricerca il campo esce come stringa vuota, e chi
legge non distingue «il portale non la fornisce a questo endpoint» da «questo provvedimento
non ha data». Stessa forma dei problemi chiusi con la #1675 — una risposta plausibile che non
dichiara la propria incompletezza. Costo concreto osservato: per ordinare 22 pronunce per data
servirebbero 22 `get`, cioe' 22 documenti interi scaricati per ricavare una colonna. In Desktop
il modello ha ripiegato da solo su store locale piu' una query mirata, dopo tre chiamate.

**Corretto** nel branch `fix/avvisi-ricerca-e-stats-sede` (`c09c3fd`). Premessa verificata
prima di scrivere codice, non assunta: catturato l'HTML reale della ricerca e ispezionato il
blocco `<article class="ricerca--item">` — nessuna data, in nessun formato. Se il portale
l'avesse fornita sotto un nome non mappato, il fix sarebbe stato una riga di mapping e questa
voce sbagliata.

La nota va **solo su stderr**. Metterla fra gli `avvisi` dell'envelope faceva scattare
l'incapsulamento su ogni ricerca — e' una proprieta' costante dell'endpoint, non un risultato
parziale — e l'array nudo di `--json` spariva per chiunque: misurato, `type` da `array` a
`object`. Il client MCP la riceve lo stesso, perche' `warningsFromStderr` raccoglie le righe
con prefisso `Nota: `.

Corretto nello stesso branch anche `stats --by sede` senza `--sede-sweep`: non tronca
soltanto, **distorce**, perche' l'ordine del portale e' proprio quello delle sedi. Roma 29
contro 65, Brescia 4 contro 11, sedi piccole invariate. L'avviso di troncamento ora lo dice e
indica lo sweep.

### Verificato per via indipendente: `stats --by sede --sede-sweep` e' onesto

Desktop ha risposto 167 pronunce 2026 su «accesso civico generalizzato», Roma 65, CdS 22,
Napoli 19, Brescia 11. La CLI locale con gli stessi filtri da gli stessi numeri, somma 167,
`conteggi_da: "totali dichiarati dal portale per sede"`. **Senza `--sede-sweep` il confronto
non vale**: il campione unico e' ordinato per sede e taglia Roma a 29 su 65 — ci sono cascato
verificando, ed e' esattamente l'errore che lo sweep esiste per evitare.

Nota: i 48 gemelli raggruppati nel campione unico diventano 7 sotto sweep, perche' i ricorsi
identici si distribuiscono fra sedi diverse.

### Repo personale: 20 commit fermi sul disco per otto giorni

`main` locale era `ahead 20` su `origin`, con il lavoro gia' mergiato nel catalogo (#1675).
Il publish impacchetta la directory, non il remote, quindi la pubblicazione non se n'era
accorta: mancava solo il backup. Pushato. `PUBBLICARE.md` aggiornato — allineare vuol dire
anche pushare, e `main` avanti su `origin` non e' un ritardo da curare con `pull`.

Cancellati i tre cloni gestiti del publish (391 MB): tutte le PR mergiate, ogni branch gia' su
`origin`, nessun file non tracciato. Il disco resta al 99%.

### Log MCP di Claude Desktop (per debug del bundle)

`%APPDATA%\Claude\logs\mcp.log` — da WSL, sotto
`/mnt/c/Users/<utente>/AppData/Roaming/Claude/logs/`.

## 2026-08-08 (sera) — emboss: due difetti reali, e quattro allarmi che non lo erano

### Da dove riprendere

- Restano aperte le decisioni del 7 agosto: archivio open data OpenGA, semantica di `--sede-sweep`, issue #1 (Directory di Claude).
- **Non sei bloccato dall'upstream: c'e' una via, verificata.** `lock promote` e' il comando sanzionato che scrive il manifest, e sotto 4.30.1 lo riscrive a `schema_version: 2` senza perdere nulla — `creator`, `run_id`, tutte e 7 le novel features, le 24 patch. Provato in un `PRINTING_PRESS_HOME` isolato: dopo il promote, il controllo `manifest` di `publish validate` **passa**. Resta solo `phase5`, che e' proprio cio' che lo Step 4.5 del publish scrive da se' rieseguendo il dogfood live. Niente reprint, niente attesa di #3425.
- **Ma il promote in place cancella `.git`** — verificato: prima `commit=534df28`, dopo `.git=NO`, e il comando riporta `promoted: true` senza dire nulla. E' la forma che la skill stessa prescrive nel Path B della Fase 5.6, cioe' proprio per le CLI con piu' lavoro a mano. Segnalato come **#4038**. La ricetta sicura e' salvare `.git`, promuovere, rimetterlo a posto.
- ~~La PR non è partita, serve un reprint~~ — **superato**: `lock promote` ha riscritto il manifest a schema 2 e la PR #1675 è aperta e verde. Nessun reprint. Il blocco iniziale era `publish validate` su `manifest — schema_version must be 2 (found 1)`, scritto dalla press 4.24.0.
- ~~phase5 resta non verificato~~ — **fatto**: dogfood live completo eseguito due volte, **49/49, zero fallimenti**, marker riscritto col `source_fingerprint` dell'albero spedito. Il `--live-check` dello scorecard non era la stessa prova: altro runner, altra matrice, nessun fingerprint.
- **La voce già pubblicata è anch'essa `schema_version: 1`** (verificato via `gh api` sul manifest in `library/productivity/giustizia-amministrativa`). Il pavimento dello schema 2 lo impone `publish validate` del binario locale, non necessariamente la CI della libreria.
- Il numero di release lo stampa il workflow della libreria al merge: non va toccato a mano (`AGENTS.md`, "Release Ledger").

### PR #1675: aperta, review chiusa, verde

https://github.com/mvanhorn/printing-press-library/pull/1675 — Greptile 5/5, sette check su sette, `mergeable`. Non posso fare merge ne' mettere etichette: sul repo pubblico ho accesso via fork.

Porta tutto agosto, non solo oggi: il repo e' nato il 7 agosto, dopo il rilascio del 23 luglio. 68 file.

**Due cose fermate prima del commit, entrambe grazie all'appunto di ars-sicilia (`docs/interno/greptile-review-workflow.md`).**

- **60 file da non pubblicare** erano nel pacchetto: `tmp/` con harness e corpora di test, `tasks/todo.md`, `docs/future-ideas.md`, `docs/evaluation-mcp.md`, `.claude/settings.local.json`, i binari di `bin/`. E' l'issue #1381 gia' annotata nel `.gitignore` di questo repo: `publish package` copia l'intera directory ignorandolo. Tolti usando il `.gitignore` come specifica.
- **Il diff a 13406 righe** guardato prima di committare (riga 238 dell'appunto): per questo il titolo descrive due settimane di lavoro invece delle correzioni di oggi.

**Quattro rilievi in review: tre reali, uno respinto.**

- **Token di sessione Liferay** (`p_auth`, 16 occorrenze) dentro il fixture HTML di `discovery/`. Reale, sanificato. **La mia scansione l'aveva mancato** perche' cercava prefissi vendor (`sk-`, `ghp_`, `xoxb-`, `AKIA`): un token di sessione applicativa non ha prefisso. Vale per ogni CLI nata da browser-sniff, perche' quei fixture sono catture reali.
- **`.mcp.json` con path assoluto** sotto la home: non portabile, e pubblicava la struttura della home. Ora relativo.
- **Commento di `splitIDs`** finito sopra `orphanFiles`. Rimesso a posto.
- **Presunto bug byte->rune in `excerptsAround`**: respinto con la misura. In Go `for bi := range text` itera sulle posizioni di inizio rune, non sui byte — su testo italiano accentato: 51 byte, 46 rune, indice finale 46. Nessun indice puo' superare `len(runes)`, e due clamp piu' il controllo `ok1`/`ok2` rendono lo slice sicuro comunque.

Emersi mentre verificavo: quattro JSON di scratch di giugno nei manuscript, malformati, superati dal marker di oggi, con dentro i path della home. Rimossi.

**Regola resa automatica.** Gli errori "materiale locale in un repo pubblico" ora hanno un cancello, non un promemoria: `~/.claude/hooks/pp-publish-guard.sh` (PreToolUse su Bash) blocca `git commit` dentro un `.publish-repo-*` se trova path della home, token di sessione o file esclusi dal `.gitignore` del sorgente. Piu' una sezione in `~/.claude/CLAUDE.md`, caricata in ogni progetto, per cio' che l'hook non vede (la regola dell'inglese verso quel repo, la tabella del corpo PR generata dal manifest). Dettagli in MemPalace, `wing=printing-press room=howto`.

### Retro: due rilievi su tre erano gia' tracciati

Filati su `mvanhorn/cli-printing-press` dopo scansione anti-duplicati:

- **#3425** (commento) — il blocco del manifest schema 1 vs 2. L'issue esisteva con 6 CLI; ho aggiunto le 8 di questa libreria, il fatto che la voce gia' pubblicata sia anch'essa schema 1, e una "correzione" che si e' rivelata **sbagliata**: sostenevo che mancassero `auth_env_vars`, `auth_env_var_specs` e `spec_path` e che il solo intero non bastasse. Misurato dopo: il promote li lascia assenti e `publish validate` passa lo stesso. #3425 aveva ragione. Corretto pubblicamente nel thread.
- **#3459** (commento) — il falso `path_validity 0/10`. L'issue lo attribuisce ai CLI html-transport; `dogfood` sullo stesso file si basa invece su `spec_format: internal`. Questa CLI e' entrambe le cose, quindi le due cause non si separano: segnalato perche' i due rimedi hanno raggio diverso.
- **#4037** (nuovo, P1) — `dogfood --research-dir` che riscrive `internal/mcp/tools.go` e annulla i fix successivi, senza dire cosa ha sostituito. Nessun issue lo copriva. Sette CLI di questa libreria sono esposte.

Le etichette sull'issue nuovo non si sono applicate: `gh issue create --label` viene scartato in silenzio senza permessi di collaboratore. Dichiarate in fondo al corpo perche' un manutentore le metta.

### Fatto

- **Rimosso `import`.** Faceva `POST /<resource>` con il nome della risorsa preso da un posizionale arbitrario; `spec.yaml` dichiara due soli endpoint, entrambi GET, e `auth: none`. Il portale non ha una superficie di scrittura: ogni chiamata sarebbe stata 404 o 405. Il 7 agosto era già stato nascosto all'MCP, ma restava nella CLI, ed era **l'unico fallimento critico di `verify`**. Non era risolvibile mappando il path, perché il path non esiste. Via anche `--batch-size`, dichiarato e mai letto. Risponde alla domanda lasciata aperta in `tasks/todo.md`.
- **Sezione di installazione senza Node riportata al testo post-pubblicazione**, in `SKILL.md` e in `README.md`. Diceva ancora *"use the category-specific Go fallback after publish"*: la CLI è pubblicata dal 23 luglio sotto `productivity`, quindi chi non ha Node veniva rinviato a un passo futuro invece di ricevere il `go install` che già funziona. Su `SKILL.md` era anche il fallimento di `verify-skill`/`canonical-sections`; il testo è stato incollato verbatim da quello atteso, non riscritto.

### `dogfood --research-dir` ha cancellato un fix, e l'ho scoperto per caso

Eseguito con `--research-dir`, `dogfood` risincronizza `command_mirror_capabilities` in `internal/mcp/tools.go` da `research.json` — il verbale di generazione di giugno — e **annulla in silenzio tre correzioni successive**: la coppia `tool` (nome MCP, `watch_run`) + `cli_command` (grafia shell, `watch run`) ricollassa nella sola chiave `command`, `get` perde *"Passa l'ECLI in `id`"*, `stats` perde la clausola su `sede-sweep`. Quella coppia **è** il commit `6578e7a`: un agente che legge `watch run` non può chiamarlo, perché il tool si chiama `watch_run`.

Ripristinato da git. Non si corregge modificando `research.json`: è evidenza archiviata, e il suo schema ha un campo comando dove il mirror ne vuole due — la sincronizzazione appiattirebbe la coppia comunque. Annotato in `AGENTS.md` perché la prossima sessione non ci ricada.

### Quattro allarmi verificati e archiviati

Nessuno dei quattro era un difetto. Tre erano artefatti della misura, uno era una mia annotazione sbagliata di stamattina.

- **`path_validity` 0/10.** Compare solo passando `--spec spec.yaml`. Quel flag vuole **OpenAPI JSON** (lo dice il suo `--help`); questa spec è in formato interno (`spec_format: internal`), e `dogfood` infatti risponde *"internal-yaml spec: paths validated at parse time"*. Senza il flag la dimensione è N/A, che è la lettura giusta. Il punteggio dipende dall'invocazione: `--spec ""` → 85/A, `--spec spec.yaml` → 75/B. Stesso codice, −10 dal flag.
- **`--live-check` a 10s** faceva fallire `massime` e `appeal-chain` per timeout, non per rottura, e abbassava `Insight` di 3 punti. A 60s passano entrambi.
- **`watch run` "output does not contain any token from query appalti-rm".** Provato a mano: prima esecuzione 20 provvedimenti, seconda `[]`. È corretto. La sonda cerca il token `appalti-rm`, che è il *nome* della watch, non un termine di ricerca.
- **`mcp_tool_count: 2` in `.printing-press.json` è giusto.** Sono i due tool *tipizzati* di `tools-manifest.json`; i tredici sono il mirror cobratree a runtime. **Il sospetto che avevo lasciato qui stamattina era infondato**: non c'è niente da correggere. Superata anche la voce sul `.mcpb` Linux — Linux e Windows sono entrambi delle 15:16 di oggi.

### Il conto

`verify` da 96% (24/25, 1 critico) a **100% (23/23, 0 critici)**; `verify-skill` verde; `tools-audit` senza rilievi; `go build`/`vet`/`test` verdi.

Lo scorecard scende di un punto, 86 → 85: `vision` va da 8 a 7 perché conta i comandi, e ne ho tolto uno. **Rimuovere un comando rotto costa un punto di punteggio.** Il prodotto è migliore, la metrica no; è la metrica ad avere torto.

### Non fatto, per scelta

- **`cache_freshness` 5/10.** Richiederebbe un blocco `cache:` nella spec. La fonte è HTML scrapato e il sync è pesante: l'auto-refresh pre-lettura farebbe colpire il portale a ogni lettura locale. Sbagliato qui.
- **`extractPaginatedItems`, segnalata morta da `dogfood`.** Lo è, ma non da sola: l'intero sottoalbero `resolvePaginatedRead` → `resolvePaginatedReadWithStrategy` → `paginatedGet` → `extractPaginatedItems` è irraggiungibile, e `resolvePaginatedRead` non ha chiamanti. È impianto emesso dal generatore per una paginazione che questa fonte non ha. Toccarlo significa editare due file generati per un WARN che vale zero punti (`dead_code` è già 5/5): va segnalato alla macchina, non rattoppato qui.

## 2026-08-08 (mattina) — il sorgente era sano, il prodotto installato no

### Da dove riprendere

- **Da rigenerare**: `build/…-linux-amd64.mcpb` è del 21 giugno, cioè contiene lo strato MCP rotto. Quello Windows è dell'8 agosto. Serve il comando della pipeline printing-press che li impacchetta — non c'è target nel `Makefile`.
- Restano aperte le decisioni del 7 agosto: archivio open data OpenGA, semantica di `--sede-sweep`, issue #1 (Directory di Claude).

### Fatto oggi

- **Verificato un report di usabilità MCP che dichiarava l'MCP "non pronto"**: quattro dei cinque difetti non esistono nel codice. L'harness aveva `~/go/bin/…-mcp` cablato, binario del **21 giugno**, precedente ai fix di agosto (`3fe4eb1`, `bab4e65`, `107ee47`, `db055a9`). Il riesame sta in `docs/evaluation-mcp.md` — non versionato (`docs/evaluation*.md` è in `.gitignore`), quindi chi clona il repo non lo trova: qui resta il sunto.
- **Il difetto vero era negli artefatti, non nel sorgente**: erano vecchi il binario MCP (21 giu), la CLI (10 lug) e il `.mcpb` Linux (21 giu). Chi installava l'estensione aveva davvero in mano il prodotto rotto descritto dal report. Reinstallati CLI e MCP; il `.mcpb` resta da rifare. **I due binari vanno sempre aggiornati insieme e affiancati** — i tool tipizzati eseguono la CLI via `SiblingCLIPath()`.
- **Quattro correzioni, tutte della stessa famiglia** (una capacità esiste, l'agente non la raggiunge): `get` accetta `--id` come alias del posizionale, perché un mirror Cobra non può dichiarare posizionali a schema; `context` dichiarava `tool_count: 2` su tredici tool registrati e nominava i mirror con la grafia CLI (`"corpus build"`) invece del nome MCP (`corpus_build`); `sql` portava l'esempio del generatore con un `resource_type` che questo store non scrive; `import` — POST su un portale senza endpoint di scrittura — non è più esposto.
- **Nessun binario cablato nei test**: `tmp/mcp_harness.py` risolve da `GA_MCP_BIN` o da `./bin/` e fallisce se manca, invece di misurare l'ultima installazione al posto dell'ultimo commit. Anche l'harness è in `tmp/`, quindi non versionato: **la correzione della causa prima non è sotto git**, da decidere se spostarla in un percorso tracciato.
- **Da guardare**: `.printing-press.json` dichiara `mcp_tool_count: 2` e `mcp_public_tool_count: 2` con tredici tool registrati. È lo stesso errore corretto in `context`, ma sta in un file scritto dal generatore: non toccato a mano.
- **Il controllo che mancava, ed era gratis**: Go stampa il commit dentro ogni binario. `go version -m <bin> | grep vcs.revision` avrebbe smentito il report in dieci secondi. Aggiunto in `AGENTS.md` insieme al resto della procedura di test MCP.
- **`.mcp.json` di progetto**: registra il server per Claude Code puntando a `bin/`, non a `~/go/bin` — così `make build-mcp` cambia davvero ciò che l'host esegue. `bin/` è ignorato da git: dopo un clone il server non parte finché non compili, ed è il fallimento rumoroso che si vuole.

## 2026-08-07 — repo autonomo, 16 difetti corretti, documentazione OKF

### Da dove riprendere

- **Decisione aperta**: dove vive l'archivio degli open data OpenGA. Dentro questa CLI o progetto separato? Vedi `docs/limiti-della-fonte.md`, sezione "Cosa offrono in più gli open data". Prototipo DuckLake funzionante in `/tmp/gafix/dl/` (effimero, da rifare).
- **Decisione aperta**: semantica di `--sede-sweep`. Oggi quota uguale a tutte le 31 sedi, quindi Consiglio di Stato e TAR Valle d'Aosta pesano identico. Documentata, non risolta.
- **Issue #1**: sottomettere l'estensione alla Directory di Claude. Mancano privacy policy, `title` sui tool, contatto di supporto.

### Fatto oggi

- **Creato il repo** `aborruso/giustizia-amministrativa-pp-cli` (prima il codice viveva solo in una working copy non versionata) e **riconciliato col catalogo pubblico**: la copia locale era ferma a giugno, mancava il lavoro di luglio su `sweepYears`/`Warnings`.
- **16 difetti corretti**, quasi tutti della stessa famiglia — risposte plausibili ma incomplete, senza segnale. I principali: tool MCP tipizzati che restituivano HTML invece di dati; ricerca senza sede che dà solo TAR Lazio dichiarando un totale nazionale (risolto con `--sede-sweep` + avviso); `grep` che restituiva il testo integrale di ogni match (308 KB → 8 KB); `stats` che presentava come distribuzione un campione troncato; risultato zero indistinguibile da filtro troppo stretto; data di deposito assente nel 96% dei casi (l'estrattore conosceva una sola delle due diciture).
- **Provvedimenti in PDF (13%) ora leggibili**: non era un limite del formato, era una nostra riscrittura dell'URL che sostituiva l'endpoint corretto con uno che sui PDF dà errore.
- **`corpus build --ids`**: archivia una selezione già curata riusando i testi in store, senza altre richieste al portale.
- **Documentazione come bundle OKF v0.2** in `docs/`: `come-cercare.md` (regole e vincoli, prima esistevano solo nei messaggi d'errore), `limiti-della-fonte.md`, `index.md`, `log.md`.

### Come sono stati trovati

Non leggendo il codice. Provando il tool da Claude Desktop, e poi con due agenti che non sapevano nulla del progetto: uno che lo usava come avvocato amministrativista, uno che osservava il log. Hanno trovato in venti minuti cose sfuggite in otto ore, incluse due su cui mi ero sbagliato. **Da rifare a ogni giro importante.**

Verdetto dell'avvocato: *"Serve a leggere, non a decidere cosa leggere"* — affidabile per verificare estremi, ricostruire la sorte in appello e leggere i testi; non per affermare "l'orientamento è questo", perché manca il ranking per pertinenza.
