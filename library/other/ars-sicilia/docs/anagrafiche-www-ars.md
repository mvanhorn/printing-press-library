# Le anagrafiche stanno su un altro sito: mappa di www.ars.sicilia.it e cosa integrare

Sopralluogo del 13 agosto 2026. La CLI vive tutta su `dati.ars.sicilia.it` — il motore documentale (atti, sedute, testi). Le **anagrafiche** — chi è chi, chi sta in quale gruppo, con quali date — stanno su `www.ars.sicilia.it`, che la CLI non toccava mai. Sono due siti e due tecnologie diverse. Dal 16/08/2026 la CLI legge anche `www.` («Anagrafiche dal sito istituzionale»): il presente documento è la mappa di ciò che c'è di là e di ciò che è già stato integrato.

## Cosa c'è di là

`www.ars.sicilia.it` è **Drupal 10**. Nessuna API: `/jsonapi`, `/api`, `/rest/session/token` e `/sitemap.xml` rispondono tutti 404. Quindi si legge HTML, con quello che comporta — selettori da reverse-engineering, fragili a un restyling.

### L'albero, dal punto di partenza

```
/gruppi-parlamentari                     elenco dei gruppi della legislatura
  ?idLeg=71|17|16                        XVIII | XVII | XVI  (il selettore è un location.replace)
  /gruppi-parlamentari/XVIII-<slug>      scheda del gruppo: composizione
    → nome deputato, ruolo nel gruppo (Presidente), collegio di elezione
    → mailto del gruppo (GruppoFIARS@ars.sicilia.it, GruppoPD@…)
    → link alla scheda di ogni componente

/xviii-legislatura                       indice dei deputati (83 link /deputati/<slug>)
  /deputati/<slug>[?idLeg=NN]            LA SCHEDA, il pezzo pregiato (vedi sotto)

/commissioni      /governo      /consiglio-presidenza     /questori
/ex-presidenti    /ex-deputati-presentazione-contatti-gallery
/agenda           /notizie      /rubrica (513 KB)         /organigramma
```

### La scheda deputato

È la voce più ricca di tutto il sopralluogo. Su `/deputati/cracolici-antonino`:

- **anagrafica**: nato a / il, titolo di studio, professione, email istituzionale, sito web
- **le legislature passate**, con selettore: XVIII, XVII, XVI, XV, XIV, XIII
- **per ogni legislatura**: lista di elezione, collegio, **voti di preferenza** (4663), gruppo di appartenenza **con la data di iscrizione**, cariche assunte **con date di inizio e fine**
- **contatori di attività**, separati per primo firmatario e cofirmatario, su cinque archivi

## Il ponte fra i due siti: le query ISIS sono nell'HTML

I link «consulta» accanto ai contatori non puntano a una pagina qualsiasi: contengono l'**espressione ISIS** già pronta, nello stesso formato che `--isis-query` accetta.

```
icaDB=221  icaQuery=18.LEGISL E 1 ADJ2 Cracolici Antonino.FIRMAT
icaDB=221  icaQuery=(18.LEGISL E ((Cracolici Antonino.FIRMAT) NOT (1 ADJ Cracolici Antonino).FIRMAT))
```

Sulla scheda di Cracolici ce ne sono **58**: cinque archivi (221 ddl, 233 interrogazioni, 234 interpellanze, 235 mozioni, 236 odg) per sei legislature, in due varianti (primo firmatario / cofirmatario).

Verificate dal vivo contro la CLI:

- `ddl cerca --isis-query "(18.LEGISL E ((Schifani Renato.FIRMAT) NOT (1 ADJ Schifani Renato).FIRMAT))"` → **2 risultati**, esattamente il contatore «Cofirmatario / Disegni di legge: 2» del sito
- `interrogazioni cerca --isis-query "18.LEGISL E 1 ADJ2 Cracolici Antonino.FIRMAT"` → risultati coerenti (3222 dell'8.04.26, …)

Due conseguenze che valgono più della mappa:

1. **La cofirma si può misurare in diretta.** Oggi `analytics --group-by cofirmatari` risponde `[]` finché non si fa `sync --resources ddl --deep` (una richiesta per ddl, minuti). L'espressione «FIRMAT ma non primo» dà lo stesso dato con una ricerca sola, per deputato.
2. **Il nome giusto per legislatura è lì.** La scheda dice «Cracolici Antonino detto Antonello»: il portale documentale indicizza *Antonino*, la stampa scrive *Antonello*. È la forma esatta da passare a `--firmatario`, ed è il motivo per cui le ricerche per nome falliscono attraverso le legislature.

## Cosa varrebbe la pena integrare, in ordine di resa

1. **`deputato anagrafica <nome>`** — la scheda: nascita, studi, professione, email, e per ogni legislatura lista/collegio/preferenze, gruppo con data, cariche con date. È il dato che oggi manca del tutto e che nessun altro comando può ricostruire.
2. ~~**`gruppo` / `gruppi`**~~ **FATTO (16/08/2026)**: `gruppi elenco` (legisl 16/17/18, una richiesta) e `gruppi get <slug-o-nome>` (composizione: cariche, collegio, email, scheda). `gruppi elenco --deputato "<nome>"` risponde alla domanda inversa — in quale gruppo sta un parlamentare — e chiude il buco per cui da un nome di gruppo trovato negli atti non si risaliva a nessuno. Selettori e markup in `internal/wwwclient/gruppi.go`, certificati dalle fixture in `testdata/`. Nota dell'implementazione: il conteggio dei componenti dei 10 gruppi della XVIII fa 70, il numero dei deputati ARS — se un'estrazione ne perde uno, il test `TestSommaComponentiXVIIIE70` fallisce.
3. **Le espressioni ISIS come sorgente autorevole** — invece di costruirle a mano, leggerle dalla scheda: risolve insieme il nome per legislatura e la cofirma in diretta. È l'integrazione con il rapporto valore/righe migliore.
4. **`/agenda`** — sedute e convocazioni già calendarizzate. Da valutare: si sovrappone a `commissioni convocazioni`, ma quello sta sul backend `/bd/`, che è il pezzo fragile del portale documentale (e il 13/08 era giù del tutto, mentre `/agenda` rispondeva). Verificato in quell'occasione che le due fonti concordano: `sync coverage` dava convocazioni al 2026-09-02, e `/agenda` mostrava «Mercoledì 02 Settembre 2026».

## Le cautele, prima di scrivere una riga

- **Nessuna API, solo HTML Drupal**: i selettori vanno trattati come i `Columns` degli archivi Icaro — verificati contro la pagina viva e documentati, perché un restyling li rompe in silenzio.
- **Seconda fonte, seconda latenza**: `sync coverage` oggi misura la copertura di `dati.`; se entra `www.` va detto quale delle due si sta guardando.
- **Gli slug non sono derivabili dal nome**: `/deputati/pellegrino-stefano-0` ha un suffisso numerico, `la-rocca-margherita` e `dagostino-nicola` normalizzano l'apostrofo in modo diverso. L'indice `/xviii-legislatura` va letto, non indovinato.
- **`?idLeg=` non è la legislatura**: XVIII è `71`, XVII è `17`, XVI è `16`. Il numero va mappato, non calcolato. (Sull'elenco gruppi il portale accetta anche `idLeg=18` come alias di 71: la normalizzazione al numero arabo canonico — 18, 17, 16 — la fa la CLI.)
