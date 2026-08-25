---
type: Reference
title: Limiti della fonte
description: Cosa il form di ricerca non offre, cosa colmano gli open data OpenGA, e cosa resta fuori portata per entrambi.
resource: https://www.giustizia-amministrativa.it/dcsnprr
tags: [fonte, limiti, ricerca, giurisprudenza, portale, open-data]
status: stable
generated: { by: claude-sonnet-5, at: 2026-08-07T20:30:00Z }
sources:
  - id: portale-dcsnprr
    resource: https://www.giustizia-amministrativa.it/dcsnprr
    title: Decisioni e pareri — form di ricerca pubblico
  - id: prova-avvocato
    title: Sessione di prova con un avvocato amministrativista su tre ricerche reali
    author: process:prova-mcp-2026-08-07
    last_modified: 2026-08-07
  - id: openga
    resource: https://openga.giustizia-amministrativa.it
    title: OpenGA — catalogo CKAN degli open data della Giustizia Amministrativa
    last_modified: 2026-07-28
---

Serve a distinguere i difetti nostri — da correggere — dai limiti a monte, che possiamo solo dichiarare, così che l'utente non prenda una risposta parziale per una risposta completa.

Regola generale: un limite della fonte non si nasconde. Se una domanda non ha risposta con i dati disponibili, va detto. Il difetto nostro comincia quando restituiamo un risultato plausibile senza dire che è parziale.

**Attenzione, il perimetro è cambiato.** Questo documento riguarda ciò che manca al **form di ricerca pubblico**, che è l'unica fonte che questa CLI interroga oggi. La Giustizia Amministrativa pubblica però anche un catalogo di open data, [OpenGA](https://openga.giustizia-amministrativa.it), che contiene alcuni dei dati qui elencati come mancanti — in particolare **l'esito del provvedimento e la data di pubblicazione**. Due voci di questo documento sono quindi limiti del form, non della fonte nel suo complesso: vedi [Cosa offrono in più gli open data](#cosa-offrono-in-più-gli-open-data) in fondo.

# Nessun ordinamento per pertinenza

Il motore ordina per sede e, dentro la sede, per numero di provvedimento decrescente. Non esiste un punteggio di rilevanza né il modo di chiederlo.[^1]

Chi chiede "le pronunce più rilevanti su X" ottiene quindi "le più recenti fra le prime sedi". La sentenza che conta si trova leggendo, o seguendo le citazioni interne fra provvedimenti, non ordinando i risultati.

Osservato nell'uso reale: cercando l'orientamento sui costi della manodopera, la pronuncia di riferimento (Cons. Stato Sez. V 9254/2024) è emersa solo perché **citata dentro** una sentenza del 2026 letta per altro motivo, su un insieme di 959 risultati.[^2]

Cosa facciamo: `search` avverte che l'ordinamento non è per pertinenza e restituisce il totale dichiarato dal portale accanto al campione, così il rapporto fra i due resta visibile. Oltre non si può andare: un ranking richiederebbe scaricare e valutare l'intero insieme.

# Nessun esito di primo grado — nel form

L'esito di un provvedimento (accolto, respinto, inammissibile) non è un campo interrogabile **del form**. Domande del tipo "quale TAR accoglie più spesso i ricorsi in materia di accesso" non hanno risposta interrogando la ricerca, se non leggendo i provvedimenti uno per uno.

Due vie lo aggirano:

- `appeal-chain` restituisce l'esito del **grado successivo**, perché quel dato il portale lo espone dall'endpoint di verifica appello.
- Gli open data lo espongono per il **primo grado**, nel campo `ESITO_PROVVEDIMENTO`, con una tassonomia articolata: `ACCOGLIE`, `RESPINGE`, `DICHIARA INAMMISSIBILE`, `DICHIARA CESSATA MATERIA DEL CONTENDERE`, `IMPROCEDIBILE PER SOPRAVVENUTA CARENZA DI INTERESSE` e altri — 42 valori distinti sul solo TAR Lazio Roma.

# La ricerca non restituisce la data di deposito

Il campo `data_deposito` esiste nello schema dei risultati ma il motore lo lascia sempre vuoto. Ordinare o filtrare per data partendo dai soli risultati di ricerca non è quindi possibile.

Si recupera in due modi: dal testo integrale, dove compare come "Pubblicato il GG/MM/AAAA" oppure "DEPOSITATA IN SEGRETERIA / Il GG/MM/AAAA" — `get` e `corpus build` leggono entrambe le diciture, anche nei PDF; e dagli open data, campo `DATA_PUBBLICAZIONE`, in formato ISO.

Dal 2026-08-18 la ricerca lo dichiara invece di lasciarlo dedurre: quando le righe escono senza data, una nota su stderr — e nel campo `avvisi` per i client MCP — dice che il portale non espone il campo a questo endpoint e da dove si ottiene. Verificato sull'HTML dei risultati: la data non compare in alcun formato, quindi non è un campo che il parser trascura.

# Duplicati e record malformati

Il portale restituisce talvolta lo stesso provvedimento più volte, righe con ECLI vuoto — non recuperabili a valle, perché `get` e `corpus build --ids` lavorano su ECLI — e occasionali record spazzatura: osservato un `nrg` a 100000000 con allegato `.docx`.

Non deduplichiamo in modo aggressivo perché due righe possono legittimamente riferirsi a documenti distinti dello stesso ricorso. Lo sweep per anni e per sedi deduplica per ECLI/idprovv al proprio interno.

# Il filtro "plenaria" non seleziona solo le Adunanze Plenarie

Con `tipo=plenaria` il portale include anche decreti cautelari e sentenze ordinarie della Sezione P. Per un giurista "plenaria" significa pronuncia vincolante ex art. 99 c.p.a., e questi non lo sono.

È comportamento del filtro a monte: noi inoltriamo il valore che il form prevede.

# Provvedimenti in PDF — limite superato

Circa il 13% dei provvedimenti è servito come PDF anziché HTML. **Non è più un limite**: ne estraiamo il testo, e da lì anche la data di deposito. Resta il caso del PDF scansionato senza strato di testo, per il quale il documento riporta una nota esplicita con il link all'originale.

Vale come promemoria metodologico: per mesi il formato è sembrato il limite, e invece era una nostra riscrittura dell'URL che sostituiva l'endpoint corretto con uno che sui PDF risponde con un errore. **Prima di dichiarare un limite della fonte, verificare che non sia un difetto nostro travestito.**

# Cosa offrono in più gli open data

[OpenGA](https://openga.giustizia-amministrativa.it) è il catalogo CKAN della Giustizia Amministrativa: 436 dataset, uno per combinazione di sede e materia, in CSV, JSON e ODS, con una risorsa per anno. Copre tutte le 31 sedi.

Non duplica il form: lo **completa**, e i due si agganciano — `NUMERO_RICORSO` degli open data è l'`nrg` del portale, quindi da una riga si arriva all'ECLI e al testo integrale.[^3]

| | Open data | Form di ricerca |
|---|---|---|
| Esito del provvedimento | sì (`ESITO_PROVVEDIMENTO`) | no |
| Data di pubblicazione | sì, ISO (`DATA_PUBBLICAZIONE`) | no nei risultati |
| Oggetto del ricorso | sì, troncato a 500 caratteri | solo lo snippet |
| Tipo di udienza, collegio, data di deposito del ricorso | sì | no |
| Testo integrale | **no** | **sì** |
| Ordinamento e aggregazione | qualunque, è SQL | solo sede + numero decrescente |
| Carico di lavoro delle sedi | sì (pervenuti, definiti, pendenti, udienze) | no |

Freschezza: migliore di quanto suggerirebbe il termine "open data annuali". Alla verifica del 2026-08-07 le sentenze del TAR Lazio Roma arrivavano al **24 luglio 2026**, con la risorsa aggiornata il 28. Restano quindi scoperte le ultime due settimane circa, per le quali serve il form.

**Questa CLI oggi non li usa.** Le voci qui sopra non sono quindi difetti da correggere nell'immediato, ma dicono dove sta il margine: buona parte di ciò che il form non sa rispondere è già pubblicato altrove, in forma strutturata. Il ranking per pertinenza resta l'unica mancanza che nessuna delle due fonti colma.

[^1]: Verificato misurando la distribuzione delle sedi pagina per pagina sul form pubblico: pagine 1–2 interamente TAR Lazio, mescolanza reale solo dalla pagina 20 in poi.
[^2]: Sessione di prova del 2026-08-07 con un avvocato amministrativista, su tre ricerche professionali reali.
[^3]: Verificato il 2026-08-07: la riga con `NUMERO_PROVVEDIMENTO 202400196` e `NUMERO_RICORSO 202208702` corrisponde a `ECLI:IT:TARLAZ:2024:196SENT` sul portale.
