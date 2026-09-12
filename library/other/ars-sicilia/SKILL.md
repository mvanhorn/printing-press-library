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

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/cmd/ars-sicilia-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Sostituisce le 12 maschere JSP del portale ufficiale con una CLI agent-native. Sync in SQLite locale per query SQL, ricerca full-text cross-archivio, e novel commands come `ddl iter` (timeline completa di un disegno di legge) e `deputato profilo` (tutta l'attività di un parlamentare in un'unica chiamata).

## When to Use This CLI

Usa ars-sicilia-pp-cli quando devi cercare, scaricare o aggregare atti dell'Assemblea Regionale Siciliana (leggi regionali, disegni di legge, interrogazioni, mozioni, resoconti d'aula, lavori di commissione) e quando hai bisogno di output strutturato JSON/CSV per pipeline downstream o per assistenti AI via MCP. Particolarmente utile per giornalismo politico, ricerca civica, civic-hacking opendata, e analisi cross-archivio impossibili dal portale JSP nativo.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Vista cronologica cross-archivio
- **`ddl iter`** — Ricostruisce la cronologia completa di un disegno di legge: presentazione, passaggio in commissione, lavori d'aula, eventuale promulgazione come legge regionale.

  _Quando un agente deve raccontare 'a che punto sta il DDL X', questa è l'unica chiamata che restituisce la timeline completa senza incollare 5 ricerche manuali._

  Gli eventi portano **`seduta`** e, per le sedute d'Aula, un **`url`** che punta alla scheda del resoconto (la scheda dell'atto è nel campo `url` della radice). Usali sempre quando parti da una notizia: la data dell'articolo è quasi sempre il giorno **dopo** la seduta, e confonderle fa concludere che manchi un resoconto che invece c'è.

  Se l'iter si ferma a «Approvato dall'Assemblea» senza un evento «Pubblicazione Gurs», il report lo dice in `note`: i due archivi hanno ritardi diversi (il 21/08/2026 i ddl arrivavano a 24 giorni, le leggi a 30), quindi una legge appena approvata può non essere ancora nell'archivio leggi. Non concludere «non è stata promulgata» — `novita --archivi leggi` dice fin dove arriva la fonte. È lo stesso buco che `legge cronologia` copre dall'altro verso.

  In `--select` tieni sempre **`titolo`**: è il campo che dice *cosa* è successo, mentre `data`, `fase`, `sede` e `seduta` dicono solo quando e dove, e fra due eventi possono coincidere legittimamente. Nella stessa seduta d'Aula un ddl viene esaminato e poi votato («Esaminato in Aula» e «Approvato dall'Assemblea», 29 lug 2026 seduta 268 sul ddl 6030): senza `titolo` le due righe escono identiche e l'approvazione finale sembra un duplicato da scartare.

**`fase` dice dove l'evento è avvenuto, non dove il testo è diretto.** «Esitato per Aula» è l'esito del lavoro di commissione — la commissione chiude l'esame e manda il testo all'Aula — e la riga dichiara una seduta di commissione: la fase è `commissione`. Prima quel verbo bastava a farne un evento d'Aula, e chi filtrava `fase == "aula"` per trovare il voto si portava dentro una riga di commissione, con la data sbagliata di settimane (sul ddl 5030 il 16 giugno invece dell'8 luglio, che è la seduta 263). Il criterio è la seduta dichiarata dal portale, non una lista di verbi: le righe d'Aula portano il marcatore AULA al posto del nome della commissione. E vale nei due versi: «Rinviato Commissione 0400 Seduta n. 255 AULA» è il rinvio in commissione deciso in Aula (la 255 è il resoconto d'Aula del 10 giugno 2026), quindi la fase è `aula` e la commissione di destinazione resta in `sede` — sul ddl 6030 quella riga usciva come `commissione` e un filtro per fase perdeva un passaggio d'Aula. Il `titolo` resta invece la fonte verbatim, codice grezzo compreso: la commissione risolta si legge in `sede`.

  Il campo **`sede`** degli eventi dà la commissione in forma canonica — l'ordinale che gli altri comandi accettano (`commissioni sommari --commissione QUARTA`) — **sulle righe in cui il portale dichiara una seduta**, perché è lì accanto che la scrive, e la si legge da lì anche quando il verbo dell'evento dice altro o non la nomina affatto. Le commissioni speciali tengono il loro nome per esteso, e il nome d'uso resta comunque in `titolo`, che è verbatim: «Parere Commissione Bilancio» ha `sede: Commissione SECONDA`. Sulle righe **senza seduta** — le assegnazioni, gli invii — vale invece la dicitura del verbo, quindi la stessa commissione può comparire con due nomi nella stessa cronologia («Inviato Commissione Bilancio» resta `Commissione Bilancio`, il parere che ne segue è `Commissione SECONDA`). Non raggruppare una timeline per `sede` dandola per canonica.

  Nella sede la CLI ricompone la parola che la fonte spezza con un trattino e un a-capo: l'HTML della scheda del ddl 5030 scrive `Commissione</a> T-<br> ERZA`, e l'iter usciva con «Commissione TERZA» quattro volte e «Commissione T- ERZA» una — due sedi dove ce n'è una. Il testo dell'evento resta invece verbatim in `titolo`, garbled compreso: lì la fonte si legge come la scrive.

  L'ultimo evento di una legge è la pubblicazione in Gurs, e porta numero e data come li scrive la fonte: «Pubblicazione Gurs n. 44o1 del 21 agosto 2020». Il suffisso dopo il numero è la notazione del portale per i supplementi (la Gazzetta è la n. 44), non un refuso da correggere, e la data ripete quella dell'evento.

  Se due eventi d'Aula danno alla **stessa data** numeri di seduta diversi, il link viene omesso su entrambi e un hint lo dice: l'Aula tiene una seduta al giorno, quindi almeno un numero è sbagliato nella fonte (`ddl iter 17 199` dà il voto del 19 feb 2020 in «Seduta n. 179», ma la 179 è del 26 febbraio). In quel caso la chiave affidabile è la data: `resoconti cerca --legisl 17 --data 2020-02-19`.

  **La stessa seduta su più date non è un'anomalia: è una seduta fiume.** L'Assemblea apre la seduta, la sospende e la riprende finché la manovra passa, e l'archivio resoconti la indicizza al giorno di apertura. Prima di dirlo la CLI lo **verifica sull'archivio resoconti** (una richiesta per seduta candidata, solo negli iter dove un numero compare su più date). Confermata, quegli eventi portano **`seduta_pluri_giorno: true`** e, sui giorni di ripresa, **`data_apertura`** — la data d'apertura dell'archivio, in ISO — e il link al resoconto **c'è**. Un numero che l'archivio non ha, o che apre in un giorno che l'iter non dichiara, non è una ripresa ma un numero sbagliato: resta `anomalia: true` senza link. Se l'archivio non risponde la lettura si tiene e la nota dice che non è verificata. Misurato su quattro finanziarie e due legislature: L.R. 1/2019 (seduta 98, aperta il 31 gennaio, voto il 15 febbraio), L.R. 9/2020 (187, 28 aprile → 2 maggio), L.R. 28/2024 (142, 5 → 7 novembre), L.R. 1/2025 (147, 18 → 28 dicembre). È per questo che cercare l'archivio per la data del voto non trova nulla (`resoconti cerca --legisl 17 --data 2020-05-02` → `[]`) mentre `resoconti get 17 187` lo restituisce, con la data autorevole in radice (`data`, con `data_iso` accanto; resta anche in `fields.Data`). Non concludere «resoconto mancante»: cerca per numero.

  Resta `anomalia: true` per la sola contraddizione vera — stessa data con numeri di seduta diversi — dove il link è omesso e la metà affidabile è la data: `resoconti cerca --legisl 17 --data 2020-02-19`. Il motivo, in tutti e due i casi, esce sia su stderr sia nel campo `note` del report, che `--select` non può togliere.

  ```bash
  ars-sicilia-pp-cli ddl iter 18 1153 --json
  ars-sicilia-pp-cli ddl iter 17 290 --json --select data,fase,seduta,titolo,url
  ```
- **`ddl stralci`** — Elenca i disegni di legge ricavati per stralcio da un ddl base; il verso opposto è il campo `stralcio` di `ddl get` e `ddl iter`.

  _La finanziaria viene spacchettata in stralci che proseguono da soli, e la loro numerazione non segue una regola: gli stralci del ddl 1030 sono 3030…8030, quelli del 738 sono una ventina fra 7381 e 73864. Il legame lo dichiara il portale, non si calcola._

  ```bash
  ars-sicilia-pp-cli ddl stralci 18 1030 --json
  ars-sicilia-pp-cli ddl stralci 18 1030/A --json   # stessa risposta, con una nota che dice perché
  ```

  Il numero si dà **base**: sommari e stampa citano il testo emendato come `1030/A`, ma l'archivio non lo numera a parte e gli stralci sono gli stessi. Quella forma è accettata e il perché finisce in `note`; sugli altri comandi (`ddl get`, `ddl iter`) è invece un errore esplicito che indica il numero base, perché lì il documento chiesto sarebbe un altro.

  Nell'output, `base_dichiarata: false` con `di: []` significa che il documento **è** uno stralcio ma il portale non dice di quale ddl (succede su parte della XVII legislatura, dove al posto del numero base è scritto l'id interno). Non dedurre la base dalla numerazione. Uno stralcio può inoltre nascere da più ddl abbinati: `di` ha allora più voci. Su `ddl iter` la cronologia di uno stralcio può cominciare **prima** della sua presentazione (il ddl 6030 è assegnato alla QUARTA il 13 gennaio 2026 ed è presentato il 27): sono i lavori che lo hanno ritagliato dal ddl base, non un dato sballato, e il report lo dice in `note`. Non è marcato `anomalia`, che resta riservato a ciò che non può essere vero.
- **`deputato profilo`** — Aggrega in un'unica vista tutti gli atti firmati o pronunciati da un deputato: DDL, interrogazioni, interpellanze, mozioni, ordini del giorno, risoluzioni e interventi in resoconti d'aula. `--data` (range `YYYY-MM-DD:YYYY-MM-DD`) filtra per data su tutti i sotto-archivi.
  Un archivio che non risponde finisce in **`non_raggiunti`**, ed e' da leggere prima dei conteggi: i conteggi non lo comprendono. Prima quell'archivio spariva in silenzio e il profilo si presentava completo - su un periodo lungo mancavano ddl e interrogazioni, e il profilo del deputato usciva senza i suoi disegni di legge.

  _Sostituisce un workflow di 7 click manuali con un'unica chiamata strutturata: pensata per agenti che rispondono a 'che ha fatto il deputato X?'._

  ```bash
  ars-sicilia-pp-cli deputato profilo "Abbate Ignazio" --legisl 18 --json --select tipo,data,titolo
  ```
- **`commissione dossier`** — Vista completa su una commissione: convocazioni in calendario, sommari lavori, DDL assegnati e pareri richiesti al Governo regionale. Accetta il codice `1`-`6`, l'ordinale (`PRIMA`..`SESTA`) o un frammento della denominazione d'archivio. Le **commissioni speciali** (Antimafia, Statuto, Unione Europea) non hanno un codice e si raggiungono solo per denominazione, che non coincide con l'etichetta d'uso corrente: `"Antimafia"` non corrisponde a nulla, la denominazione è *«Commissione d'inchiesta e vigilanza sul fenomeno della mafia e della corruzione in Sicilia»*. Un termine che non aggancia nessuna commissione non produce un dossier vuoto: l'errore elenca le denominazioni della legislatura.

  **`conteggio` è quanto è stato scaricato, `totale` quanti ce ne sono.** Prima esisteva solo il primo, e su `--limit 100` tre sezioni su quattro dicevano `troncato` a 100 senza dire se gli atti fossero 101 o 600: chi usava il dossier per dimensionare un fenomeno non aveva il denominatore. Ora le sezioni servite dall'ISIS lo portano — sulla PRIMA della XVIII, `pareri` 82 e `ddl_assegnati` 637 — e `troncato` compare solo quando `totale > conteggio` (con `--limit 100` i pareri escono 82 su 82, non troncati). Il totale di `ddl_assegnati` è però quello di una **ricerca testuale sull'ordinale**, non dell'elenco degli assegnati: l'archivio dei ddl non espone l'assegnazione come campo, e il report lo dichiara in `note`. `convocazioni` e `sommari` restano senza totale perché il backend `/bd/` che le serve non lo pubblica: meglio il dato assente di una stima dedotta dalle pagine. La qualifica sta in `note`, e a terminale è stampata sopra le sezioni: un `100 risultati su 637` senza quella riga si legge come il numero dei ddl assegnati, che non è.

  _Quando segui i lavori di una commissione specifica, questa è l'unica chiamata che dà il quadro completo invece di 3 ricerche separate._

  ```bash
  ars-sicilia-pp-cli commissione dossier "SESTA" --legisl 18 --json
  ars-sicilia-pp-cli commissione dossier "inchiesta e vigilanza" --legisl 18 --json
  ```
- **`legge cronologia`** — Partendo da una legge regionale promulgata (archivio 201), risale al DDL originario, ai pareri di commissione e al voto d'aula: l'inverso temporale di ddl iter. Aggiungi sempre **`--anno`**: lo stesso numero di legge si ripete in anni diversi della stessa legislatura (nella XVIII ci sono due L.R. 26, ottobre 2024 e giugno 2025) e senza `--anno` l'archivio ne restituisce una sola — la cronologia esce coerente e riferita all'atto sbagliato. Un avviso su stderr dice quale legge è stata presa. In radice, `ddl_originari` porta i numeri dei ddl da cui la legge nasce (più d'uno se erano abbinati): è l'aggancio diretto per `ddl iter`, che prima andava estratto con una regex dalla frase `sede` dell'evento `ddl_originario`. Se con `--anno` già dato la legge non si trova, l'errore non ripete «aggiungi `--anno`»: nomina le due cause vere, cioè una promulgazione troppo fresca per l'archivio (`novita --archivi leggi` dice fin dove arriva la fonte: il 21/08/2026 era ferma al 22 luglio, e la L.R. 21/2026 del 4 agosto non c'era) oppure una coppia numero-anno inesistente. Per una legge recente l'iter si legge intanto dal lato ddl.

  _Per ricercatori e giornalisti che partono dalla legge promulgata e vogliono raccontare come ci si è arrivati._

  ```bash
  ars-sicilia-pp-cli legge cronologia 18 26 --anno 2025 --json
  ```

### Analytics su campi strutturati
- **`analytics`** — Identifica i deputati che firmano insieme atti parlamentari, restituendo coppie e cluster con conteggio per analisi di network politico. Richiede una **deep sync** dei ddl (`sync --resources ddl --deep`), che estrae i firmatari dalle schede di dettaglio.

  _Per ricercatori e giornalisti che analizzano alleanze e dinamiche politiche: niente foglio Excel di trascrizioni manuali._

  ```bash
  ars-sicilia-pp-cli sync --resources ddl --legisl 18 --deep
  ars-sicilia-pp-cli analytics --type ddl --group-by cofirmatari --limit 50 --json
  ```
- **`analytics`** — Classifica i deputati per numero di interventi nei resoconti d'aula, con range date e legislatura, opzionale conteggio parole.

  _Per le persone che vogliono sapere 'chi parla di più' senza scaricare 200 resoconti PDF e fare ctrl+F._

  ```bash
  ars-sicilia-pp-cli analytics --type resoconti --group-by oratore --legisl 18 --limit 30 --csv
  ```

  È una richiesta per oratore (91 nella XVIII legislatura, ~40 secondi). Se il backend non risponde per qualcuno, la classifica esce lo stesso con gli altri e un `nota:` su stderr elenca i nomi non misurati: **quei nomi non sono "zero interventi", sono "non misurati"** — ripetere il comando di solito li recupera.
- **`analytics`** — Classifica i disegni di legge per deputato **proponente** (primo firmatario) o per **gruppo** parlamentare, leggendo le viste già aggregate dal portale con **una sola richiesta** (nessuna sync). Copre la legislatura corrente (le classifiche non sono filtrabili per legislatura).

  _Per rispondere subito a 'chi presenta più DDL' / 'quale gruppo è più prolifico' senza deep sync._

  ```bash
  ars-sicilia-pp-cli analytics --type ddl --group-by proponente --limit 20
  ars-sicilia-pp-cli analytics --type ddl --group-by gruppo --json
  ```

### Anagrafiche dal sito istituzionale
- **`gruppi elenco`** — Elenca i gruppi parlamentari di una legislatura (16, 17, 18; default 18), con lo slug per aprire il dettaglio. I nomi sono gli stessi del campo gruppo delle firme sugli atti, quindi l'elenco è anche il vocabolario per costruire la join. Con `--deputato "<nome>"` legge i dettagli di tutti i gruppi della legislatura e risponde alla domanda inversa — in quale gruppo sta un parlamentare, con ruolo e collegio — a costo di una richiesta per gruppo.

  _L'anagrafica dei gruppi non sta nel motore documentale (dati.ars.sicilia.it), dove il gruppo compare solo come stringa accanto a una firma: sta sul sito istituzionale (www.ars.sicilia.it), che la CLI prima d'ora non toccava._

  ```bash
  ars-sicilia-pp-cli gruppi elenco --legisl 18 --json
  ars-sicilia-pp-cli gruppi elenco --legisl 18 --deputato "Cracolici" --json
  ```
- **`gruppi get`** — La composizione completa di un gruppo: cariche (Presidente, Vice-Presidente, Segretario, Tesoriere), collegio di elezione, email e scheda di ogni componente. Accetta lo slug (dall'elenco) o il nome del gruppo; un nome ambiguo esce con l'elenco dei candidati invece di indovinare.

  _Da un nome di gruppo trovato negli atti si risale alla sua composizione in una sola richiesta._

  ```bash
  ars-sicilia-pp-cli gruppi get XVIII-misto --json
  ars-sicilia-pp-cli gruppi get "Partito Democratico" --legisl 18 --json
  ```

### Stato e monitoraggio
- **`novita`** — Cosa è comparso negli archivi da una certa data in qua, tutti gli archivi datati in una chiamata, con accanto il **ritardo di pubblicazione della fonte** archivio per archivio.

  _È la domanda di chi monitora, e finora costava una ricerca per archivio più un filtro a mano. Diversa da `ddl drift`, che dice cosa si è **mosso**: quello richiede uno stato dell'iter da confrontare, che esiste solo sui ddl. Qui la domanda è cosa è **nuovo**, che si legge dalla data dell'atto e vale ovunque._

  ```bash
  ars-sicilia-pp-cli novita --since 7d --agent
  ars-sicilia-pp-cli novita --since 30d --archivi ddl,interrogazioni,resoconti --agent
  ars-sicilia-pp-cli novita --dal 2026-07-01 --archivi resoconti --csv
  ```

  Il ritardo accanto a ogni archivio è la parte che rende leggibile lo zero: le mozioni sono pubblicate con ~45 giorni di ritardo, quindi «gli ultimi 7 giorni» sarà vuoto a lungo, e non perché l'Assemblea sia ferma. **Quando la finestra chiesta cade tutta dentro il ritardo, il comando lo dice** invece di lasciare un elenco vuoto senza spiegazione. `conteggio` è quanti ne ha trovati, `--limit` (default 30) è quanti ne mostra: il numero non dipende da quante righe hai chiesto di vedere. **Sull'archivio `leggi` la riga è per legge, non per articolo**: il portale indicizza un articolo per riga, quindi senza aggregazione la sola L.R. 14/2026 valeva 7 novità. Ogni riga porta `articoli_trovati`, `atto` (`L.R. 14`) e `numero` (`14`, il valore che passi a `--numero`). `pareri` e `biblioteca` non sono databili e vengono dichiarati tali, non riportati vuoti.

  In `--since`, **`m` vale mesi, non minuti** (`7d`, `3w`, `2m`, `1y`, e `24h` per chi la scrive così).
- **`ddl drift`** — Confronta lo stato dell'iter dei DDL nella sync corrente con la precedente e segnala i disegni di legge che si sono mossi nel periodo (passati da commissione ad aula, approvati, ritirati). Richiede due **deep sync** (`sync --resources ddl --deep`) a distanza di tempo: solo la deep sync scrive il campo `iter` confrontato.

  _L'RSS shell esistente segnala solo 'nuovi'; per 'mossi' non c'è alternativa. Questo è il segnale che cercavano i journalist che seguono iter politici._

  ```bash
  ars-sicilia-pp-cli ddl drift --since 7d --json
  ```
- **`sync stale`** — Mostra per ognuno dei 12 archivi ARS: timestamp ultima sync, n. record locali, età della sync, eventuale segnalazione di staleness.

  _Per agenti che orchestrano sync automatico: decide se rinfrescare prima di rispondere o se i dati locali sono ancora freschi._

  ```bash
  ars-sicilia-pp-cli sync stale --json
  ```

- **`analytics --group-by cofirme`** — Quante volte ciascun deputato ha **cofirmato**, chiesto al portale in diretta: niente sync, niente deep sync.

  _Non è `--group-by cofirmatari`, che conta le **coppie** (chi firma insieme a chi) e quelle stanno solo dentro le schede di dettaglio, quindi richiede ancora `sync --resources ddl --deep`. Qui la domanda è «quanto cofirma ciascuno»._

  ```bash
  ars-sicilia-pp-cli analytics --type ddl --group-by cofirme --legisl 18 --limit 20 --agent
  ```

  Il conto lo fa il motore di ricerca, interrogato in ISIS: `(18.LEGISL E ((Nome.FIRMAT) NOT (1 ADJ Nome).FIRMAT))` — compare fra i firmatari ma non in prima posizione. Serve `--legisl` perché i nomi valgono per legislatura, ed è una richiesta per deputato (~66, ~80 s). Vale su tutti gli archivi con un campo firmatario: `ddl`, `interrogazioni`, `interpellanze`, `mozioni`, `odg`, `risoluzioni`. Verificato contro i contatori pubblicati su www.ars.sicilia.it: Cracolici 302 e Catanzaro 306 ddl cofirmati nella XVIII, uguali al singolo atto. Chi non risponde viene **nominato** su stderr, non contato zero.

- **`sync coverage`** — Dice fin dove arriva **la fonte**, archivio per archivio: la data del documento più recente che il portale espone, il ritardo in giorni rispetto a oggi e, accanto, l'ultima sync locale.

  _Serve a leggere un `[]` per quello che è. Se la notizia è del 12 agosto e l'archivio ddl è fermo al 28 luglio, la ricerca a vuoto è **latenza della fonte**, non un atto inesistente — e senza questa misura le due cose si somigliano._

  ```bash
  ars-sicilia-pp-cli sync coverage --resources ddl --json
  ars-sicilia-pp-cli sync coverage --json   # tutti i 12 archivi, ~45 s
  ```

  Il comando **non assume l'ordinamento della fonte**, che non è uniforme: `ddl` consegna dal più recente, `leggi` dal più vecchio. Legge la prima pagina, guarda se le date scendono davvero, e solo quando non lo fa scarica l'anno intero per prendere il massimo. Tre risposte non sono un numero e vanno lette come tali: `pareri` scrive le date a parole e tagliate («17 luglio 2»), quindi non è misurabile; `biblioteca` non ha proprio una colonna data; sugli archivi `/bd/` può uscire l'errore di backend, che come sempre **non è assenza di dato** — si riprova. `convocazioni` porta normalmente una data futura, perché annuncia sedute ancora da tenere: il ritardo negativo è corretto e il comando lo annota.

  Nota: `sync stale --max-age` ha default `7d` (i dati ARS non cambiano su base oraria); `doctor`'s cache section usa invece una soglia fissa di 6h, non configurabile. Le due soglie divergono di proposito — uno store che `sync stale` giudica fresco può risultare `"status": "stale"` in `doctor`. Un agente che orchestra sync automatico non deve fidarsi solo di `sync stale`: controlla anche `doctor`'s `cache.status` se vuoi il segnale più conservativo.

## Command Reference

**biblioteca** — Catalogo Bibliografico (archivio 205) e Opere Multimediali (205multimedia).

- `ars-sicilia-pp-cli biblioteca cerca` — Cerca nel catalogo bibliografico per autore, titolo, soggetto o ISBN.
- `ars-sicilia-pp-cli biblioteca multimediali` — Cerca nelle opere multimediali.

**commissioni** — Lavori delle commissioni: convocazioni (229) e sommari (230).

- `ars-sicilia-pp-cli commissioni convocazioni` — Convocazioni delle Commissioni.
- `ars-sicilia-pp-cli commissioni sommari` — Sommari dei lavori di commissione. Il filtro è `--commissione`/`--codcom`, ma in uscita **la commissione sta in `titolo`** (`I - Affari Istituzionali`): su questo archivio il titolo del record è il nome della commissione, non quello di un documento. Non esiste un campo `commissione`.

**Restringi la ricerca, su questo archivio non è un vezzo.** Il backend `/bd/` consegna intere le risposte piccole e tronca a metà quelle grandi: misurato, `--numero 270` è arrivato 10 volte su 10, la stessa ricerca senza filtri 2 volte su 8. Se sai il numero della seduta usa `--numero`; altrimenti `--anno`, poi `--commissione`. Quando una ricerca fallisce per troncatura, la CLI suggerisce quale filtro manca.

```bash
ars-sicilia-pp-cli commissioni sommari --legisl 18 --numero 270 --agent
```

`--commissione` accetta l'ordinale (`PRIMA`..`SESTA`), un frammento della denominazione (`Bilancio`) o, in alternativa, `--codcom 1`-`6`. Un termine che non corrisponde a nessuna commissione **esce con errore** e propone i nomi vicini: non restituisce una lista vuota, che si leggerebbe come "questa commissione non ha lavori".

**ddl** — Disegni di Legge (archivio 221): proposte di legge presentate all'ARS.

- `ars-sicilia-pp-cli ddl cerca` — Cerca disegni di legge per legislatura, anno, firmatario, materia o testo.
- `ars-sicilia-pp-cli ddl get` — Scarica un singolo disegno di legge.

**I valori giusti per i filtri non si indovinano, si chiedono.** `--materia` e `--firmatario` vogliono il valore come lo scrive il portale, e un valore inventato non dà errore: dà zero risultati, che si legge come «non esiste». Tre comandi elencano i valori validi, tutti istantanei e senza sync:

```bash
ars-sicilia-pp-cli ddl materie --agent      # 123 settori, da "Abrogazione di norme" a "Zootecnia"
ars-sicilia-pp-cli ddl firmatari --legisl 18 --agent   # 66 deputati della XVIII; --search "Cracolici" per cercarne uno
ars-sicilia-pp-cli ddl iniziative --agent   # Governativa, Parlamentare, Iniziativa Popolare, Consigli comunali/provinciali, Fatto proprio dalla Commissione
```

Attenzione a `ddl iniziative`: **non esiste un flag `--iniziativa`**. Il portale scrive il tipo di iniziativa nello stesso campo dei firmatari, quindi il valore si passa a `--firmatario`: `ddl cerca --legisl 18 --firmatario Governativa` restituisce i ddl del Governo (verificato: il ddl 1188 così trovato è firmato dal presidente Schifani).

**gruppi** — Gruppi parlamentari (www.ars.sicilia.it): elenco per legislatura e composizione con ruoli e collegio.

- `ars-sicilia-pp-cli gruppi elenco` — Elenca i gruppi di una legislatura (16, 17, 18); con `--deputato "<nome>"` risponde «in quale gruppo sta un parlamentare».
- `ars-sicilia-pp-cli gruppi get <slug-o-nome>` — Composizione di un gruppo: cariche, collegio di elezione, email e scheda di ogni componente.

**interpellanze** — Interpellanze parlamentari (archivio 234).

- `ars-sicilia-pp-cli interpellanze cerca` — Cerca interpellanze.
- `ars-sicilia-pp-cli interpellanze get` — Scarica una singola interpellanza.

**interrogazioni** — Interrogazioni parlamentari (archivio 233).

- `ars-sicilia-pp-cli interrogazioni cerca` — Cerca interrogazioni per legislatura, firmatario o rubrica.
- `ars-sicilia-pp-cli interrogazioni get` — Scarica una singola interrogazione.

**leggi** — Leggi della Regione Siciliana (archivio 201): cerca e scarica le leggi regionali.

- `ars-sicilia-pp-cli leggi cerca` — Cerca leggi regionali per legislatura, anno, numero o testo. Restituisce **una riga per legge**, non per articolo: l'archivio è indicizzato per articolo e senza aggregazione il `--limit` lo consumavano gli articoli della prima legge (alla domanda «quali leggi nel 2025?» rispondeva con una sola legge). `articoli_trovati` conta gli articoli agganciati **da questa ricerca**, non quelli della legge. La legge si cita con `atto` (`L.R. 14`) e si filtra con `--numero`: da oggi la riga porta anche `numero` (`14`), così il nome con cui chiedi è anche quello con cui rileggi. Con `--articoli` tornano le righe per articolo: servono con `--testo`, per sapere in quale articolo ricorre il termine. La paginazione si ferma sulle **leggi** chieste, non su un budget di righe stimato prima: le leggi lunghe (finanziarie, ~25 articoli) costano più richieste, le corte meno. **Costa tempo, e va messo in conto**: il portale accetta 2 richieste al secondo, quindi ~20 s per dieci leggi di un anno pesante e ~100 s per un elenco annuale completo (26 leggi del 2024, misurato). Se ti serve solo sapere quali sono le più recenti, restringi con `--numero` o `--anno` invece di alzare `--limit`. Resta un tetto di sicurezza sulle righe lette; se scatta prima di completare le leggi chieste, un avviso su stderr lo dice — **leggilo**, altrimenti un elenco corto sembra completo. **Anche il `--limit` raggiunto è un avviso**: `--anno 2026` col default 10 dava 10 leggi su 14 dichiarando `troncato: false`, cioè affermando una completezza che nessuno aveva verificato. Ora in quel caso `troncato` è `true` e l'avviso dice di alzare `--limit`. L'ordine di consegna del portale non è cronologico, quindi un elenco tagliato non è nemmeno «le più recenti».
- `ars-sicilia-pp-cli leggi get` — Scarica una singola legge regionale. **Usa `--anno`**: lo stesso numero di legge si ripete ogni anno della legislatura e l'archivio ne restituisce una sola. Senza `--anno`, `leggi get 17 9` apre la L.R. 9/2018 e non la 9/2020; il comando ora dice su stderr e in `nota` quale legge ha aperto, ma la data la devi leggere.

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

- `ars-sicilia-pp-cli resoconti cerca` — Cerca resoconti per data, numero, oratore o testo. `--oratore` risolve il nome sull'anagrafica del portale: se non corrisponde a nessuna voce **esce con errore e propone i nomi vicini**, invece di restituire una lista vuota che si leggerebbe come "non è mai intervenuto". Usa il solo cognome se il nome completo non aggancia.
- `ars-sicilia-pp-cli resoconti get` — Scarica un singolo resoconto. **Risponde con la scheda del backend `/bd/`, dove sta `pdf_url`: è lì il resoconto stenografico completo.** Il PDF non viene scaricato — pesa alcuni MB e supera i 200.000 caratteri di testo — ma l'URL è stabile e citabile. La scheda non ha il campo `body` e porta un campo `nota` che lo dice: l'assenza di `body` non significa «testo non disponibile». Le schede `/bd/` coprono l'archivio fin dalla XIII legislatura (seduta 1 del 25.07.2001), quindi questa è la risposta normale su qualunque seduta. **Solo se `/bd/` non ha la scheda** `get` ripiega sull'indice Icaro, che di questo archivio tiene *frammenti per punto dell'ordine del giorno*: lì c'è un `body`, ma è uno dei frammenti e può riferirsi a un'altra seduta — la `nota` di quella risposta lo dichiara e dice anche perché si è finiti su Icaro (il record manca da `/bd/`, oppure il backend non ha risposto e basta riprovare). Se il backend non risponde — capita, tronca le risposte a intermittenza — la CLI ritenta da sola (3 tentativi) e solo dopo esce con `il backend /bd/ non ha risposto …`, che è diverso da `nessun documento trovato`: quest'ultimo esce solo quando il backend ha risposto e la seduta davvero non c'è. **Non dedurre da un errore di backend che l'atto non esista.** I due percorsi hanno la **stessa forma**: `legisl`, `numero`, `data`, `data_iso`, `titolo` e `fonte` stanno in radice su entrambi, quindi lo stesso `--select numero,data_iso,titolo` rende su tutte le sedute. `fonte` dice quale dei due ha risposto: `bd` la scheda, `icaro` il frammento. Accanto al PDF la scheda dichiara **chi ha presieduto** (`presidenza`), **chi ha parlato e con quale gruppo** (`oratori`), l'**ordine del giorno** e gli eventuali **allegati** (`odg`, `allegati`, ciascuno con l'URL del proprio PDF): arrivano dalla stessa richiesta, non ne costano una in più, e i blocchi che una seduta non ha semplicemente mancano. È la via breve per «chi ha parlato nella seduta N», che altrimenti si chiede solo ad `analytics --group-by oratore`, che è per legislatura e costa una novantina di richieste. `get` è un comando **live**: il documento non sta nello store, quindi con `--data-source local` esce con `no local data source for this command` invece di andare in rete di nascosto. Con **`--con-testo`** aggiunge il campo `testo`, la versione testuale della seduta che il portale pubblica sulla stessa scheda: 66.975 caratteri sulla 17/208, e **nessuna richiesta in più** (la pagina che lo contiene è quella che `get` scarica comunque). Fuori per default perché una seduta va dalle 8.000 alle 95.000 battute. Non c'è su tutte: misurato il 2026-09-12, il testo c'è dalla seduta 13/1 del 2001 fino alla 232 del 25.02.2026, oltre la quale resta solo il PDF - **la frontiera si muove col portale e non coincide con la distinzione provvisorio/definitivo** (la 232 è provvisoria e il testo ce l'ha), quindi si guarda se c'è invece di dedurlo. Quando manca, il campo non esce vuoto: la `nota` dice perché e rimanda a `pdf_url`.

  ```bash
  ars-sicilia-pp-cli resoconti get 17 208 --agent --select pdf_url,oratori,presidenza
  # leggere la seduta: `--con-testo`, poi si cerca in locale
  ars-sicilia-pp-cli resoconti get 17 207 --con-testo --agent --select testo | grep -i sfiducia
  # poi, se serve il testo: curl -sL "<pdf_url>" -o seduta.pdf
  ```

**risoluzioni** — Risoluzioni parlamentari (archivio 238).

- `ars-sicilia-pp-cli risoluzioni cerca` — Cerca risoluzioni.
- `ars-sicilia-pp-cli risoluzioni get` — Scarica una singola risoluzione.


### Nessun argomento posizionale sui comandi di ricerca

Ogni criterio si passa come **flag**. I comandi `*/cerca`, `commissioni convocazioni|sommari` e `biblioteca multimediali` non prendono argomenti posizionali e li rifiutano con un errore: `commissioni sommari cerca --commissione X` è sbagliato (`cerca` non è un sottocomando lì), la forma giusta è `commissioni sommari --commissione X`. Prima venivano accettati e scartati in silenzio, il che faceva credere di aver invocato un comando diverso da quello realmente eseguito.

### La punteggiatura dentro un valore di ricerca: la CLI la toglie e lo dice

Il motore ISIS del portale **non accetta la punteggiatura dentro il valore di un filtro**: non la ignora, rifiuta la ricerca. Misurato il 2026-09-06 sull'archivio ddl, un carattere per volta: `' " , - / . ; : + * ? & ! ( = #` fanno rispondere una pagina d'errore («Impossibile creare la Query»). Passano lettere, cifre, spazio, il troncamento `$` e `%`.

Non è un caso di nicchia: rifiutavano `--iter "Approvato dall'Assemblea"` (il nome dello stato scritto dal portale stesso), `--firmatario "D'Agostino"` (un cognome siciliano), `--testo "dell'ambiente"`, `--materia "sanita'"`, `--testo "COVID-19"`. E il messaggio che arrivava era quello della soglia — «restringi il periodo» — cioè una strada che lì non porta da nessuna parte.

Adesso la CLI **ripulisce il valore prima di spedirlo e dichiara su stderr cosa è partito davvero**: `--firmatario «D'Agostino» è partito come «D Agostino»`. La sostituzione con uno spazio è fedele all'indice, non una resa: il portale indicizza la punteggiatura come separatore di parole, e in un valore di campo lo spazio vale adiacenza. Verificato: `--iter "Approvato dall Assemblea"` torna le stesse 16 righe di `--iter "Assemblea"` sul 2026, mentre `--iter "Assegnato Assemblea"` — due stati veri ma non adiacenti — torna vuoto.

Restano intatti i valori con parentesi (chi le scrive sta scrivendo la propria espressione), `--isis-query`, che è la via d'uscita per chi vuole comandare la sintassi, e i valori di `--data` e `--anno`, i due parametri il cui contenuto lo costruisce la CLI (`260101/261231`): lì la punteggiatura è sintassi. **L'esenzione sta sul nome del parametro, non sulla forma del valore**: un valore non dice da sé se è una data, lo dice il campo in cui sta — `--testo "2026-07-01"` cerca un documento che cita quella data, ed è una domanda legittima che va ripulita come tutte le altre.

**`--isbn` è l'eccezione, e non si deduce dalla forma del valore.** Due campi numerici dello stesso archivio si comportano al contrario: su `--dewey "340.5"` il punto separa davvero due token e la riscrittura in spazio trova il record; su un ISBN la punteggiatura è formattazione di un numero solo. E l'archivio non è coerente con sé stesso: **il 92% dei record tiene l'ISBN a gruppi separati** (3565 su ~3884) e l'8% come token unico (326), misurato il 06/09/2026 sull'archivio 205.

Dalle cifre il raggruppamento non si deduce, perché dipende dal registrante. Quindi si enumera: per un ISBN-13 il prefisso sono tre cifre e il carattere di controllo è l'ultimo, restano nove cifre da spezzare in gruppo (1-5), registrante e pubblicazione — 25 combinazioni, un numero costante. Vanno in OR con la grafia unita e **costano una richiesta sola**. `--isbn "9788898231256"` e `--isbn "978-88-98231-25-6"` producono perciò la stessa espressione e lo stesso risultato; l'avviso dice quante grafie sono partite, e `--dry-run` mostra l'espressione (circa 950 caratteri).

Fuori dall'ISBN-13 non si inventa nulla: un valore che non sia tredici cifre con prefisso 978/979 ricade sul comportamento precedente, cioè la grafia unita più quella coi separatori resi spazio, che copre chi i separatori li scrive.

**I rifiuti del portale ora si distinguono**, e chiedono mosse opposte: `(QR997)` è la soglia, il motore cede sul numero di documenti e restringere il periodo funziona; «Impossibile creare la Query» (senza codice) e `(QR999) Operando con crt non validi` sono sintassi, e restringere non cambia nulla — la stessa espressione viene rifiutata su nove mesi come su un giorno. In quel caso la CLI non spreca più lo spezzettamento in sottoperiodi. Il secondo codice porta un `QRxxx` come la soglia: distinguerli guardando solo la presenza del codice non basta.

### `ddl cerca --iter`: filtra sugli stati attraversati, non su quello attuale

Il campo indicizza **tutta la storia** dell'atto. `--iter "Assegnato"` restituisce anche i ddl che l'assegnazione l'hanno passata da un pezzo: il ddl 779, presentato nel 2024, esce da `--iter "Assegnato" --anno 2024` pur essendo molto più avanti. Per lo stato corrente di **un** atto la risposta è `ddl iter <legisl> <numero>`, non questo filtro.

Con questo in mente il filtro risponde alle domande d'insieme, che prima non avevano strada:

```bash
# quanti ddl la XVIII legislatura ha approvato in Aula
ars-sicilia-pp-cli ddl cerca --legisl 18 --iter "Approvato dall'Assemblea" --limit 300 --agent   # 112
# quanti ne sono passati dalla I Commissione
ars-sicilia-pp-cli ddl cerca --legisl 18 --iter "Assegnato per esame Commissione PRIMA" --limit 300 --agent   # 52
```

Due avvertenze prima di trarne conclusioni. Il valore è una **locuzione**, non un insieme di parole: dev'essere scritto come lo scrive il portale nell'iter dell'atto (`ddl iter` lo mostra), perché uno spazio vale adiacenza e un termine inventato torna `[]`, indistinguibile da «nessun caso reale». E **l'indice della fonte ritarda sui ddl più recenti**, proprio quelli che interessano a chi segue una notizia: il ddl 1196, assegnato il 05/08/2026, il 06/09/2026 non usciva ancora da `--iter "Assegnato" --anno 2026`. Un elenco che sembra completo e non lo è è il falso segnale peggiore: incrocia con `ddl iter` sugli atti recenti.

### Il backend `/bd/` tronca le risposte grandi

Gli archivi delle sedute — `resoconti`, `commissioni sommari`, `commissioni convocazioni` — sono serviti dal backend `/bd/` del portale, che **a intermittenza consegna il corpo della risposta tagliato a metà**: status 200, header regolari, e il contenuto che si interrompe. Non è un timeout (le risposte tagliate arrivano in due decimi di secondo) e non dipende dal protocollo (succede identico su HTTP/2 e HTTP/1.1). Dipende da **quanto è grande la risposta**: misurato su `sommari`, la ricerca di una singola seduta (24 KB) è arrivata 8 volte su 8, la stessa ricerca senza filtri (44 KB) zero volte su 8.

Cosa fa la CLI da sola: ritenta ogni lettura fino a 3 volte, e quando si arrende lo dice come guasto del backend — mai come assenza del dato. **`il backend /bd/ non ha risposto` non significa che l'atto non esista**: significa riprovare, possibilmente restringendo. Il `nessun documento trovato` invece è affidabile, esce solo quando il backend ha risposto davvero.

Cosa devi fare tu: **chiedere meno righe**. In ordine di efficacia, `--numero` (la singola seduta; su `resoconti` e `commissioni sommari`, mentre `convocazioni` non ha un numero di seduta), poi `--anno`, poi `--commissione`. Quando una ricerca fallisce per troncatura la CLI ti dice quale di questi filtri manca.

Dove la troncatura non si può evitare, viene dichiarata invece che nascosta: `analytics --group-by oratore` fa 91 richieste e, se qualcuna cade, pubblica la classifica con gli altri e nomina su stderr chi non è stato misurato — quei nomi non sono «zero interventi».

Sullo stesso backend non esistono i filtri ISIS: `--isis-query`, `--escludi` e `--frase` (più `--presidente` su `commissioni sommari`) **non sono flag di quei comandi**, perché il form `/bd/` non ha niente che li applichi. Prima erano registrati e venivano respinti a runtime: un giro a vuoto per scoprire una cosa che `--help` poteva dire subito, e per un agente che legge lo schema MCP una chiamata sprecata. La ricerca testuale lì è `--testo` (il campo full-text del form), disponibile anche su `commissioni convocazioni`; su `commissioni sommari` `--argomento` è un alias di `--testo`.

### Un atto per numero: `--numero`, mai `--testo`

Se la notizia dà il numero dell'atto, `--numero` lo aggancia sul campo (`NUMORD` per interrogazioni, interpellanze, mozioni, odg, risoluzioni; `NUMDDL` per i ddl; `LEGNUM` per le leggi) e restituisce quell'atto.

Passare il numero come testo libero aggancia invece **ogni documento che lo cita**, in ordine dal più recente: l'atto cercato può finire oltre il `--limit`. `mozioni cerca --testo "143"` mette la mozione 143 in diciassettesima posizione su diciannove — col limite di default non si vede, e sembra che non esista.

```bash
ars-sicilia-pp-cli mozioni cerca --legisl 18 --numero 143 --json
```

Un numero però non sempre aggancia **un** documento: il portale ne tiene di distinti sotto lo stesso numero, di norma versioni diverse della stessa pratica. Sul ddl 6030 sono due — uno col testo del ddl e l'iter aggiornato, l'altro la sola scheda ferma a due settimane prima — identici in ogni campo della lista, titolo e data comprese. Quando succede, `cerca` e `get` lo dicono con un hint: `get` apre il primo e ne riporta il `docno`.

### Un atto senza numero, ma con la data di una seduta: passa dai sommari

Quando la notizia racconta una seduta e non dà il numero dell'atto, la ricerca testuale sui ddl è la strada lunga: il portale ordina per data e non per pertinenza, quindi l'atto può stare fuori dalla finestra anche quando la ricerca è giusta. `ddl cerca --legisl 18 --anno 2024 --frase "enti locali"` esce troncato su dieci righe di novembre-dicembre, e il ddl 780 — quello della maratona di emendamenti del 17 settembre 2024 — non c'è.

Il sommario della commissione di quel giorno lo nomina per esteso, e da lì l'iter si chiude. La commissione non serve saperla: la sola data risponde, perché la troncatura del backend `/bd/` dipende dalla dimensione della risposta e una giornata sola è piccola (`--commissione` si aggiunge solo per restringere).

```bash
ars-sicilia-pp-cli commissioni sommari --legisl 18 --data 2024-09-17 --agent
# le 7 sedute di quel giorno; la II - Bilancio è la seduta 109, «Esame del disegno di legge ... n. 780»
ars-sicilia-pp-cli ddl iter 18 780 --agent
```

Quando `ddl cerca` esce troncato su una ricerca a testo libero, l'hint nomina questa strada.

### `docno` e `permalink`: l'unico URL che si può conservare

Sugli archivi Icaro — tutti tranne `resoconti`, `sommari` e `convocazioni` — `doc_id` e `url` **non identificano il documento**: `icaDocId` è la posizione nella short list della sessione corrente, quindi con un'altra query lo stesso valore apre un altro atto, e fuori sessione l'URL risponde 302. Non citarli e non salvarli.

`get` restituisce anche `docno` — il numero di documento interno del portale, stabile — e `permalink`, che riapre quel documento in una sessione nuova. Sono quelli da conservare in una nota o in un articolo.

```bash
ars-sicilia-pp-cli ddl get 18 6030 --agent --select docno,permalink
```

Gli `url` dei tre archivi serviti dal backend `/bd/` sono invece già citabili (`bd/resoconti/scheda/18/269` risponde 200 senza sessione), e lì `doc_id` non compare affatto.

Il campo `nota` non va **chiesto** in `--select`: c'è solo quando serve, e chiederlo dove non c'è fa comparire l'avviso «nota non esiste in questi record». Non serve chiederlo: i campi che qualificano la risposta — `troncato`, `conteggio`, `nota`, `hint`, `meta` — **sopravvivono a `--select`** in radice, perché dicono se i dati sono tutti e non sono dati fra cui scegliere. Dentro le righe di un array restano invece campi come gli altri, e `--select` li filtra.

### Le date: quattro grafie in arrivo, `data_iso` per lavorarci

Il portale scrive le date in quattro forme diverse, e due convivono nello stesso payload di `ddl iter`:

- `28.07.26` — archivi Icaro: ddl, interrogazioni, interpellanze, mozioni, odg, risoluzioni
- `5.01.2026` — archivio `leggi`
- `05/08/2026` — backend `/bd/`: resoconti, sommari, convocazioni
- `17 giu 2026` — blocco di stato dentro il documento di un DDL, cioè gli eventi di `ddl iter` e `legge cronologia`

Nessuna è ordinabile come stringa e nessuna è quella che i filtri vogliono in ingresso. Accanto a ogni data leggibile viaggia perciò **`data_iso`** (`YYYY-MM-DD`), nel JSON e come colonna nel CSV: è quello da usare per ordinare, confrontare, importare in `duckdb` o rimettere dentro un `--data`. `--csv` rende una tabella anche sui comandi aggregati, che avvolgono le righe in un oggetto: su `commissione dossier` le quattro sezioni escono concatenate, con una colonna `tipo` che dice da quale sezione viene ogni riga. Quando manca vuol dire che quel valore non è una data leggibile: il range echeggiato in radice da `deputato profilo` (`2026-06-01:2026-08-14`), che è un criterio e non una data, oppure una data che la fonte ha scritto monca. Sull'archivio `pareri` succede **riga per riga**, non per tutto l'archivio: nella stessa risposta convivono `30 gen 2026` (che dà `data_iso`) e `17 luglio 2` o `05 febbraio` (che non lo danno, perché l'anno non c'è). Quindi su `pareri` l'assenza di `data_iso` va letta come «questa riga non è databile», e le righe restano ordinabili solo in parte.

```bash
ars-sicilia-pp-cli deputato profilo "Chinnici Valentina" --legisl 18 --agent --select tipo,data_iso,titolo
```

**In ingresso i flag temporali non sono gli stessi su tutti gli archivi**:

**Periodi lunghi**: oltre un certo numero di documenti il motore rifiuta la ricerca, e il portale lo dichiara con una pagina d'errore. La CLI la riconosce e rifà la ricerca su sottoperiodi, unendo le risposte e dicendolo nell'`hint`; prima quel rifiuto usciva come `[]`, cioe' come un'affermazione falsa sull'archivio. La soglia e' sul numero di documenti e cambia da archivio ad archivio, quindi non c'e' una durata sicura da ricordare: se un periodo lungo torna un errore, restringilo. **`--anno` non e' l'alternativa sicura**: su `ddl` non esiste un campo anno, `--anno 2023` e' compilato nello stesso tipo di range di `--data`.

- **`--data`** (giorno singolo o range `YYYY-MM-DD:YYYY-MM-DD`): `ddl`, `interrogazioni`, `interpellanze`, `mozioni`, `odg`, `risoluzioni`, `resoconti`, `commissioni sommari`, `commissioni convocazioni`, e `deputato profilo` (che lo applica a tutti i sotto-archivi).
- **`--anno`**: `ddl`, `leggi`, `resoconti`, `commissioni sommari`, `commissioni convocazioni`.
- **Nessuno dei due**: `pareri` e `biblioteca`.

Su `ddl` i due flag qualificano lo stesso campo, la data di presentazione: `--anno 2026` è esattamente il range `2026-01-01:2026-12-31`. Per questo **non si usano insieme** — messi entrambi il comando si ferma con un errore, invece di restituire in silenzio zero risultati — e «quali ddl sono stati presentati questa settimana» si chiede così:

```bash
ars-sicilia-pp-cli ddl cerca --legisl 18 --data 2026-07-01:2026-08-14 --agent
```

Su `leggi` invece un intervallo più stretto dell'anno non esiste affatto: l'archivio non indicizza una data (l'unico campo temporale è `LEGANN`, l'anno), quindi nemmeno `--isis-query` lo raggiunge. Lì si chiede l'anno e si filtra a valle su `data_iso`.

### `--testo` cerca le parole, `--frase` cerca la locuzione

`--testo "aree idonee"` costruisce `(aree E idonee)`: entrambe le parole devono comparire **da qualche parte** nel documento. Su un disegno di legge lungo questo aggancia anche atti che hanno una parola all'articolo 3 e l'altra all'articolo 40 — con «aree idonee» escono peschicoltura e coworking accanto agli atti pertinenti.

`--frase "aree idonee"` costruisce `(aree adj idonee)`: parole **adiacenti, nell'ordine dato**, e restano solo gli atti che contengono davvero la locuzione (ddl 803 «Norme in materia di aree idonee e non idonee», ddl 726).

```bash
ars-sicilia-pp-cli ddl cerca --legisl 18 --frase "aree idonee" --json
```

Una parola sola passa invariata. Su `resoconti`, `sommari` e `convocazioni` (backend `/bd/`) il flag **non esiste**: lì la ricerca testuale è `--testo`. Se una ricerca a due parole restituisce troppi risultati poco pertinenti, prova `--frase` prima di concludere che l'atto non c'è.

**La congiunzione dentro la locuzione**: i titoli delle manovre sono fatti così, «Coesione e crescita», e «e» è anche l'operatore AND del portale. Prima bastava vederlo per lasciare la frase intatta, cioè il flag prometteva una locuzione e consegnava un AND senza dirlo. Adesso la congiunzione minuscola viene scartata e la distanza la tiene in conto: `--frase "coesione e crescita"` costruisce `(coesione adj2 crescita)`, le parole vicine entro quella distanza. Non è la locuzione esatta - aggancia anche due parole separate da una parola qualsiasi - e un avviso su stderr, dentro l'envelope e in `--dry-run` dice quale parola è caduta e quale espressione è partita. Misurato sul ddl 969 «prevenzione e contrasto» in legislatura 18: l'AND dà 144 risultati, `adj2` ne dà 41 e comprende il 969, mentre l'adiacenza stretta sulle sole parole superstiti ne dà 3 e il 969 lo manca, perché il portale indicizza la congiunzione come posizione. **Solo «e» e «o» si scartano.** Il vocabolario del portale contiene anche parole piene dell'italiano - «seguito», «vicino», «meno», «no», «escluso» - e toglierle non attenua la ricerca, la falsifica: `--frase "aree meno idonee"` diventerebbe «aree idonee», il contrario. Lì la frase parte com'era e l'avviso dice che la locuzione non è esprimibile, invece di tacere come faceva prima.

Un operatore scritto in **maiuscolo** resta un operatore: `--frase "aree E idonee"` passa intatta, e per un'espressione costruita a mano c'è `--isis-query`. La maiuscola vale come segnale solo se nella frase c'è anche del minuscolo: un titolo copiato in stampatello dal portale, «SVILUPPO E COESIONE», viene trattato come locuzione, altrimenti l'AND muto tornerebbe proprio sul caso che questo serve a coprire.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ars-sicilia-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. L'indice copre le 42 capacità della CLI — non solo i comandi di punta, anche le ricerche d'archivio, i `get`, i vocabolari dei filtri — e dove una capacità si distingue per una flag, la flag fa parte del nome restituito: la risposta è qualcosa da incollare.

Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. **Sotto `--json` (quindi sotto `--agent`) il no-match esce 0 con `matches: []`**, per scelta: un agente ramifica su `matches.length`, non sul codice d'uscita.

Alle domande su ciò che il portale **non pubblica** risponde con il motivo, non col silenzio: sotto `--json` arriva un campo `non_coperto`, altrove lo stesso testo su stderr. Sono i voti nominali, le presenze in aula, gli emendamenti come archivio, le spese dell'Assemblea e il gruppo di appartenenza come anagrafica interrogabile. Ogni voce dice anche la cosa più vicina che si può fare davvero — per esempio, l'esito delle votazioni esiste solo nella prosa dei resoconti d'aula.

## Recipes

### Sync iniziale completo XVIII legislatura

```bash
ars-sicilia-pp-cli sync --max-pages 0 --resources ddl,leggi,interrogazioni,mozioni,interpellanze,odg,risoluzioni,pareri,resoconti,convocazioni,sommari
```

Prima sincronizzazione di tutti gli archivi politici della XVIII legislatura — i dati restano in `~/.local/share/ars-sicilia-pp-cli/store.db`.

### Iter completo di un DDL con output narrowing

```bash
ars-sicilia-pp-cli ddl iter 18 1153 --json --select fase,data,sede,titolo,oratori
```

Timeline del DDL 1153, mostrando solo i campi essenziali — riduce il payload per agenti. `titolo` è fra gli essenziali: è ciò che dice *cosa* è successo, e senza di lui due eventi della stessa seduta escono identici.

### Network di co-firmatari su DDL

```bash
ars-sicilia-pp-cli sync --resources ddl --deep
ars-sicilia-pp-cli analytics --type ddl --group-by cofirmatari --limit 30 --csv
```

Produce un CSV con le coppie di deputati che firmano DDL insieme — pronto per import in `duckdb` o gephi. **La deep sync è obbligatoria**: i firmatari stanno solo nelle schede di dettaglio, quindi senza di essa il comando restituisce `[]` (con un hint su stderr) — risultato vuoto per mancanza di dati locali, non per assenza di co-firme.

### Drift settimanale dei DDL

```bash
ars-sicilia-pp-cli ddl drift --since 7d --json
```

Confronta lo stato dell'iter rispetto a una settimana fa — i DDL che si sono mossi (commissione → aula, voto, ritiro) compaiono qui.

### Top cofirmatari DDL (XVIII legislatura)

```bash
ars-sicilia-pp-cli sync --resources ddl --legisl 18 --deep
ars-sicilia-pp-cli analytics --type ddl --group-by cofirmatari --limit 20 --legisl 18 --json
```

Classifica i deputati che firmano più DDL insieme (richiede una **deep sync** dei ddl: i firmatari stanno solo nelle schede di dettaglio).

## Auth Setup

Nessuna credenziale richiesta: il portale ARS è pubblico. La sessione `JSESSIONID` per la ricerca è gestita automaticamente in modo trasparente dal client.

Run `ars-sicilia-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **`--envelope` on searches: read the truncation flag, do not infer absence** ⚠️ — by default `*/cerca` prints a bare array and the warnings ("results truncated", "no result has your terms in its title") go to **stderr**, so an agent parsing stdout never sees them. That is not hypothetical: a truncated 3-month search was read as "this sitting record is not indexed", and the record was there. Add `--envelope` and you get `{"risultati": [...], "troncato": true, "hint": "..."}`; `--select` still filters inside `risultati`. **A short list is never proof of absence** — check `troncato` before concluding anything does not exist.

  ```bash
  ars-sicilia-pp-cli resoconti cerca --legisl 17 --data 2019-10-01:2019-12-31 --agent --envelope --limit 10
  ```

- **Un `cerca` vuoto con `--data` che arriva agli ultimi 45 giorni avvisa che può essere latenza della fonte** — prima `interrogazioni cerca --data 2026-08-15:2026-09-04` tornava `[]` e basta, con l'archivio fermo un mese prima. L'hint (stderr e dentro `--envelope`) dice che il portale pubblica con 9-45 giorni di ritardo e rimanda a `sync coverage --resources <archivio>` per l'ultima data vera. Nessuna richiesta in più; una finestra che finisce mesi fa, o che inizia nel futuro, resta muta, e una a cavallo di oggi si valuta come se finisse oggi.
- **`--select` con nomi che non esistono da nessuna parte viene ignorato davvero** — l'avviso lo diceva già, ma la proiezione partiva lo stesso e ogni array annidato usciva come oggetti vuoti (`ddl get 18 779 --select iter` → `firmatari: [{}, {}]`; `--select pippo` faceva lo stesso a `eventi`, `atti`, `sezioni` e a ogni riga di un `cerca`). Ora se nessun nome esiste esce il payload intero e l'avviso dice il nome giusto (`fields.Iter`). Un selettore misto (`numero,pippo`) filtra sui nomi buoni, come prima.
- **List titles are cut at 256 characters: never conclude an act is off-topic from its title alone** ⚠️ — the acts with the longest titles (`Schema di progetto di legge costituzionale…`, `Disegno di legge voto…`) are the ones whose subject falls past the cut. XVII-legislature bill 199 is titled "…riconoscimento degli svantaggi derivanti dalla **condizione di insularità**", but the list shows "…svantaggi deriva". Search results whose title hits the cap without matching are ranked between the proven matches and the off-topic rows, and the "no relevant title" hint reports how many titles were cut — when it does, open the document (`ddl get`) for the full title instead of raising `--limit`.

  ```bash
  ars-sicilia-pp-cli ddl cerca --legisl 17 --testo "insularità" --agent --envelope   # ddl 199 first, hint: 1 title cut
  ```
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. On the aggregate commands (`legge cronologia`, `ddl iter`, `deputato profilo`, `commissione dossier`) the payload is an object wrapping an array, so name the fields at the level where they live: `--select data,fase` filters the events, `--select titolo` keeps the act's own title, and mixing both returns both. A name that exists nowhere is reported on stderr with the list of available fields, e con dove sta se vive un livello sotto (`--select data` su `ddl get` risponde «usa `fields.Data`»). **Se ogni nome chiesto esiste già nella radice, l'array annidato non esce**: `ddl iter --select numero,titolo` dà numero e titolo dell'atto e zero eventi, perché la radice vince. È voluto — `--select titolo` deve dare il titolo dell'atto, non quello di trentuno eventi — ma ora un avviso su stderr dice quante righe sono rimaste fuori e quali nomi usare per vederle. Se l'elenco ti serve, chiedi almeno un campo che vive nelle sue righe (`data`, `fase`, `etichetta`…). Su `ddl iter` e `legge cronologia` **`titolo` non è un campo fra gli altri**: è il contenuto dell'evento, e le coordinate (`data`, `fase`, `sede`, `seduta`) sono le stesse per eventi diversi della stessa seduta. Toglierlo non riduce il payload, rende le righe indistinguibili. Critical for keeping context small on verbose APIs:

  ```bash
  ars-sicilia-pp-cli ddl get mock-value mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending, **naming the backend that would actually serve it**. Gli archivi delle sedute (`resoconti`, `sommari`, `convocazioni`) sono migrati al backend `/bd/` del portale: escono con `backend: "bd"`, l'endpoint della POST in `would_post`, e la richiesta divisa in due: `post_fields` sono i campi che partono esattamente così — nomi del backend, non i tuoi flag (`legisl` parte come `$Ilegislatura`), più i selettori di modalità che il form porta sempre — mentre `deferred` nomina i filtri che si risolvono solo al momento della richiesta e dice in cosa si trasformano (`--data` non è affatto un campo: diventa **una richiesta per ciascun anno** dell'intervallo, enumerati in `anni` e contati in `richieste`, con `anno` fra i `post_fields` che porta il primo — il valore della prima richiesta, più un filtro sulle righe ricevute per tagliare i giorni fuori intervallo; `page` sta fra i `post_fields` e vale 1 sulla prima richiesta di ogni anno, poi cresce fino al numero di pagine che la risposta dichiara — quel numero arriva **dentro** la risposta, quindi le pagine oltre la prima non sono anteprimabili e l'anteprima dice la regola invece di inventare un conto — e se `--anno` cade fuori dall'intervallo l'anteprima dice che non resta nessun anno da interrogare; `--oratore` e `--commissione`/`--codcom` si risolvono da nome a id leggendo le `<option>` del form). Mostrarli come li hai scritti sarebbe la stessa bugia dell'endpoint sbagliato, un livello più giù; gli altri con `backend: "icaro"`, `isis_query` e `would_fetch`. Prima l'anteprima li descriveva tutti come query Icaro, cioè annunciava con sicurezza un URL che quei comandi non interrogano — su un flag che esiste per diagnosticare. I comandi che interrogano più archivi (`legge cronologia`, `deputato profilo`, `commissione dossier`) elencano una voce per richiesta sotto `requests`, nell'ordine in cui partirebbero; sul dossier è lì che si vede lo stesso argomento partire come `codcom` verso `/bd/` e come ordinale a lettere (`SESTA`) verso l'ISIS, che è la ragione per cui metà sezioni possono restare vuote. Su `legge cronologia` l'anteprima si ferma alla legge: il ddl d'origine si risolve dai campi P010/P012 della scheda trovata, quindi dipende da una risposta che il dry-run non chiede — e la `note` lo dichiara invece di tacerlo
- **Gli hint stanno su stderr, su ogni comando** — non solo sulle ricerche: l'anomalia seduta↔data di `legge cronologia`/`ddl iter`, il taglio dei risultati, lo store vecchio. **Non unire stderr a stdout in una pipe verso `jq`** (`2>&1 | jq`): l'hint precede il JSON, `jq` muore con un parse error e **esce 5** — che si legge come un guasto intermittente della CLI e non lo è. La CLI un exit 5 non lo produce mai (`0`, `1`, `2`, `3`, `7`, `10`). Tieni stderr separato (`2>/dev/null`), oppure usa `--envelope` e i tool MCP, che lo stesso testo lo riportano dentro il payload
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
