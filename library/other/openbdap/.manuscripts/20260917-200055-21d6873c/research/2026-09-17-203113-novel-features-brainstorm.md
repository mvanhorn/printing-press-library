# OpenBDAP CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Elenco di tutti i dataset | CKAN package_list; ondata/ckan-mcp-server | openbdap-pp-cli catalogo elenco | Sincronizzato in SQLite locale, niente 3857 chiamate a ogni uso |
| 2 | Metadati di un dataset | CKAN package_show | openbdap-pp-cli catalogo dettaglio | Risolve anche per nome/slug, cosa che l'API non fa |
| 3 | Ricerca testuale dataset | CKAN package_search; ricerca Drupal del portale | openbdap-pp-cli cerca | FTS5 offline su titolo e descrizione, conteggi veri, filtri per tema/tag/anno/regione (fq lato API e' ignorato) |
| 4 | Elenco temi | CKAN group_list/group_show | openbdap-pp-cli gruppi | Conteggi e dataset dallo store locale |
| 5 | Elenco parole chiave | CKAN tag_list | openbdap-pp-cli tag | Conteggio d'uso reale per tag |
| 6 | Licenze | CKAN license_list | (generated endpoint) licenze elenco | Output agent-native |
| 7 | Download CSV integrale | tab "Scarica" del portale | openbdap-pp-cli scarica csv | Un comando invece di tre clic; https forzato (http fallisce) |
| 8 | Colonne e schema | OData DataColumns; tab "Tabella" | openbdap-pp-cli colonne | Nome leggibile, nome fisico, id per i filtri, cardinalita' e valori distinti in una tabella sola |
| 9 | Righe filtrate | OData DataRows | openbdap-pp-cli righe | Filtri per nome leggibile tradotti in colUniqueId, paginazione automatica, CSV/JSON con intestazioni leggibili |
| 10 | Conteggio righe | OData count=true (scoperto in browser-sniff) | openbdap-pp-cli conta | L'unico conteggio affidabile: $inlinecount e' rotto |
| 11 | Ricerca per CUP | vademecum pnrr.datibenecomune.it; Italian-Builders-Org/DoveVannoINostriSoldi | openbdap-pp-cli cup | Nessuna curl a mano, nessun colUniqueId da indovinare; scriptabile |
| 12 | Misure numeriche del dataset | OData DataMeasures | (generated endpoint) dati misure | Output agent-native |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Dossier di progetto | dossier | hand-code | Unisce MOP Totale e i cinque dataset MOP regionali (pagamenti, gare, partecipanti, piano dei costi, soggetti titolari) risolvendo la regione: nessuna chiamata singola lo fa | Usa questo comando per il quadro completo di un progetto. NON usarlo per la sola anagrafica; usa 'cup'. |
| 2 | Serie storiche e copertura | serie | hand-code | Aggrega 3857 titoli sui campi derivati anno/periodo/regione/serie nello store locale; l'API non espone le serie | Usa questo comando per elencare annualita', mensilita' e regioni di una serie. NON usarlo per cercare un dataset per parole chiave; usa 'cerca'. |
| 3 | Opere di un ente | opere | hand-code | Codice fiscale del titolare come chiave trasversale ai MOP, con totale reale via count=true | Usa questo comando per le opere di un ente dal codice fiscale. NON usarlo per un singolo progetto noto; usa 'cup' o 'dossier'. |
| 4 | Novita' del catalogo | novita | hand-code | recently_changed_packages_activity e' 404: il diff esiste solo perche' lo store locale conserva lo stato precedente | Usa questo comando per vedere cosa e' cambiato dall'ultimo aggiornamento locale. NON usarlo per scaricare i metadati; usa 'sync'. |
| 5 | Indice dei campi | campi | hand-code | Il colUniqueId e' deterministico: l'indice locale delle DataColumns dice in quali dataset vive un campo, e con quale id filtrarlo | Usa questo comando per trovare in quali dataset esiste un campo. NON usarlo per i campi di un dataset che gia' conosci; usa 'colonne'. |
| 6 | Mappa della famiglia MOP | mop | hand-code | Non esiste listing OData: ruolo e regione si ricavano dai nomi dei dataset (spd_mop_gar_mon_reg19) e si legano agli id OData nello store | Usa questo comando per capire quale dataset MOP interrogare. NON usarlo per i temi del catalogo; usa 'gruppi'. |
| 7 | Ricerca per CIG | cig | hand-code | Il CIG non sta nel Totale: va cercato in fan-out sui dataset gare e partecipanti regionali | Usa questo comando per una gara dal CIG. NON usarlo per un progetto dal CUP; usa 'cup' o 'dossier'. |

Nessuno stub: tutte le righe sono scope di rilascio.
