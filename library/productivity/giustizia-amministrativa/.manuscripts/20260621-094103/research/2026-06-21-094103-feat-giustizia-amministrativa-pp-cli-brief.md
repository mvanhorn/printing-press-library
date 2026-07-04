# Giustizia Amministrativa CLI Brief

## API Identity
- Domain: Ricerca pubblica "Decisioni e Pareri" della Giustizia Amministrativa italiana (TAR, Consiglio di Stato, C.G.A.R.S). Pagina: /web/guest/dcsnprr
- Users: avvocati amministrativisti, magistrati, praticanti, ricercatori/accademici, giornalisti, PA, cittadini
- Data profile: ~84.845 risultati solo per "appalto"; corpus enorme di sentenze, ordinanze, decreti, pareri dal 1908 a oggi, 30 sedi
- No auth: ricerca e testi integrali pubblici, nessuna API key

## Reachability Risk
- None. probe-reachability => standard_http (200, stdlib e surf-chrome). Nessun anti-bot. Replay HTTP verificato end-to-end.

## Reverse-engineering (proven)
- Portale Liferay. Ricerca = action portlet (p_p_lifecycle=1, javax.portlet.action=search) su POST.
- Handshake: GET /web/guest/dcsnprr -> cookie ApplicationGatewayAffinity* + token CSRF `p_auth` (regex da HTML). p_auth ENFORCED (bogus => 403). Token+cookie riusabili tra richieste; refresh su 403.
- Risposta ricerca: HTML (~238KB) con "Trovati N risultati" e item strutturati.
- Paginazione: GET stesso action URL con _cur=<pagina>&changePage=true (+ tutti i param). Verificato: pagina 2 = "Risultati da 11 a 20".
- Item risultato: data-idprovv, data-nrg, data-sede (schema es. tar_rm), sezione, tipo (SENTENZA/ORDINANZA/DECRETO/PARERE/Adunanza), formato (html/pdf), numero provv., ECLI, snippet, docUrl.
- Testo integrale: GET https://mdp.giustizia-amministrativa.it/visualizzah2/?schema=<schema>&nrg=<nrg>&nomeFile=<file>&subDir=Provvedimenti -> HTML pulito (convertibile in markdown).

## Search parameters
- Full-text semplice: searchtextProvvedimenti
- Full-text avanzata: searchAllWords / searchAnyWords / searchNotWords / searchPhrase
- Filtri: TipoProvvedimentoItem (Decreto, Ordinanza, Parere, Sentenza, P=Adunanza Plenaria, C=Adunanza Generale), sedeProvvedimenti (30 sedi)
- Per numero: numeroProvvedimenti + DataYearItem/DataYearItem2 (anno o range)
- Per NRG: numeroNrg + DataNrgItem/DataNrgItem2
- pageSize, asSearchMode (provv/nrg), isAdvancedSearch

## Top Workflows
1. Ricerca full-text di giurisprudenza ("trova sentenze su appalto + soccorso istruttorio")
2. Recupero del testo integrale di un provvedimento (per NRG/numero/ECLI) e lettura/export
3. Filtraggio per tipo + sede + anno (es. ordinanze TAR Lazio 2025)
4. Monitoraggio: nuove decisioni su un tema o di una sede (diff nel tempo via store locale)
5. Estrazione massiva per ricerca/analisi (corpus su un tema -> markdown/CSV)

## Table Stakes (cosa offre il sito, da eguagliare)
- Ricerca semplice e avanzata (AND/OR/NOT/frase)
- Filtri tipo/sede/anno/numero/NRG
- Paginazione, conteggio totale
- Apertura testo integrale (html/pdf)
- Verifica appello / massima / news collegate

## Data Layer
- Primary entity: provvedimento (id, ecli, tipo, sede, schema, sezione, numero, anno, nrg, data_deposito, snippet, doc_url, nome_file, full_text)
- Sync cursor: per query salvata (tema/filtri) -> accumulo risultati nel tempo
- FTS5: full-text offline su snippet + testo integrale scaricato

## Why install this instead of the website
- Da terminale, scriptabile, --json/--select agent-native
- Output MARKDOWN pulito del testo integrale (richiesta utente) - nessun tool web lo offre
- Store SQLite + ricerca offline FTS + diff temporale
- Estrazione massiva/paginazione automatica oltre la singola pagina del form

## User Vision
- Output pulito in MARKDOWN nel CLI (richiesta esplicita): comando full-text con --format md / --markdown come feature di punta.

## Product Thesis
- Name: giustizia-amministrativa (gactl) - "La giurisprudenza amministrativa italiana da terminale"
- Why: il form web e' lento e non scriptabile; nessuno strumento offre export markdown + store locale + ricerca offline del corpus TAR/CdS.

## Build Priorities
1. Client GA: handshake (p_auth+cookie, refresh 403), search (POST + paginazione GET), HTML parsing risultati
2. Fetch testo integrale + conversione markdown (flagship)
3. Store SQLite + FTS + sync di query salvate
4. Filtri completi (tipo/sede/anno/numero/NRG), --json/--select/--csv
5. Feature transcendence (diff temporale, export corpus, ecc.)

## Public URL per atto (confermato)
- Ogni risultato ha un URL pubblico, ricostruibile da schema+nrg+nomeFile, accessibile senza auth/cookie (curl nudo => 200):
  https://mdp.giustizia-amministrativa.it/visualizzah2/?nodeRef=&schema=<schema>&nrg=<nrg>&nomeFile=<nomeFile>&subDir=Provvedimenti
- Varianti: visualizzah2/ (HTML highlighted), visualizza/ (XML/plain), .pdf per item in formato pdf.
- Requisito di base: ogni risultato (search/get/json/table/markdown) espone il campo `url`.
