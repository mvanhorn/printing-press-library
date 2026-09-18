Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

# Build log openbdap-pp-cli

## Priority 0 - strato dati
- `internal/cli/openbdap_catalogo.go`: modello locale del dataset (UUID, nome, titolo, note, tema, tag, licenza, data di aggiornamento) piu' i campi derivati che l'API non espone: anno, periodo, regione, serie, famiglia MOP, identificativo OData, URL del CSV e pagina del portale. Lettura parallela di `package_show` con errori che viaggiano per dataset.
- `internal/cli/openbdap_odata.go`: schema (`DataColumns`), traduzione dei nomi leggibili in `colUniqueId`, costruzione del filtro OData (`campo=valore`, `campo~testo`, apostrofi raddoppiati), lettura righe con rietichettatura delle colonne e conteggio via `count=true`.
- `allinea`: popola la tabella `catalogo` dello store SQLite generato, che ha gia' l'indice FTS su nome, titolo e note. 128/128 dataset del tema opere pubbliche allineati.

## Priority 1 - funzioni assorbite
- Generate dal generatore: `catalogo elenco`, `catalogo dettaglio`, `catalogo ricerca`, `gruppi elenco`, `gruppi dettaglio`, `tag`, `licenze`, `scarica`, `dati metadati`, `dati colonne`, `dati misure`, `dati righe`, `dati conta`.
- Scritte a mano perche' il valore sta nella traduzione dei nomi: `cerca` (ricerca offline con filtri tema/tag/anno/regione/famiglia), `colonne`, `righe` (`--dove`, `--campi`, `--tutte`), `conta` (`--dove`), `cup`.

## Priority 2 - funzioni originali (7/7)
1. `dossier <cup>` - progetto dal dataset nazionale, poi pagamenti, gare, partecipanti, piano dei costi e soggetti titolari dai regionali. Verificato su I77H11000120009: progetto 1, gare 13, pagamenti 3, soggetti titolari 1, nessuna sezione mancante.
2. `serie [testo] [--copertura]` - annualita', mensilita' e regioni ricavate dai titoli.
3. `opere --cf <cf>` - 13.489 opere per il codice fiscale 80208450587, conteggio reale da `count=true`.
4. `novita [--da 30d]` - dataset aggiornati nella finestra, letti dall'archivio locale.
5. `campi <testo> [--aggiorna]` - indice locale degli schemi; l'aggiornamento e' esplicito perche' serve una chiamata OData per dataset.
6. `mop [--regione] [--famiglia]` - mappa regione/ruolo verso identificativi OData.
7. `cig <codice>` - fan-out sui dataset di gare e partecipanti.

## Decisioni
- Il conteggio usa il parametro non standard `count=true` scoperto in browser-sniff: `$inlinecount` restituisce sempre zero.
- I comandi lunghi non usano `boundCtx`: il client generato applica gia' `--timeout` a ogni richiesta, e legare l'intero comando troncava l'allineamento e le estrazioni impaginate.
- Le famiglie MOP si ricavano dal nome del dataset. I codici verificati sul catalogo sono `prg` progetti, `gar` gare, `pga` partecipanti, `sal` pagamenti, `pdc` piano dei costi, `sog` soggetti titolari, `loc` localizzazione: i primi due erano stati dedotti male e sono stati corretti contro i dati reali.
- L'URL del CSV si ricostruisce sempre in https: l'indirizzo http pubblicato nei metadati non risponde.
- La pagina del portale si ricava dal titolo, non dal nome del dataset.

## Rinviato
- La versione "Totale" esiste solo per i progetti: per le altre famiglie il fan-out resta regionale.
- L'indice dei campi non copre i 3857 dataset del catalogo: richiederebbe altrettante chiamate OData.

## Limiti del generatore incontrati
- `dati righe` e `dati conta` non srotolano `d.results` nonostante `response_path`, mentre `dati colonne` lo fa: i comandi a mano `righe` e `conta` restituiscono comunque righe pulite.
- `browser-sniff --har` non ricava risorse dall'HAR quando gli XHR usano un doppio slash nel percorso (`//ODataProxy`).
