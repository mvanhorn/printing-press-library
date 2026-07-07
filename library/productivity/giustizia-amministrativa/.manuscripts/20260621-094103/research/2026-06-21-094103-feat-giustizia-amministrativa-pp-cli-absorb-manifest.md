# Giustizia Amministrativa - Absorb Manifest

## Absorbed (match or beat the web form)
| # | Feature | Source (form) | Our Implementation | Added Value |
|---|---------|---------------|--------------------|-------------|
| 1 | Ricerca full-text semplice | searchtextProvvedimenti | `search "<q>"` | --json/--select/--csv, scriptable |
| 2 | Ricerca avanzata AND/OR/NOT/frase | searchAllWords/Any/Not/Phrase | `search --all --any --not --phrase` | combinabile, ripetibile |
| 3 | Filtro per tipo | TipoProvvedimentoItem | `--tipo sentenza|ordinanza|decreto|parere|plenaria|generale` | enum validato |
| 4 | Filtro per sede | sedeProvvedimenti (30) | `--sede roma|milano|consiglio-di-stato|...` | enum validato |
| 5 | Ricerca per numero+anno | numeroProvvedimenti+DataYearItem | `search --numero N --anno YYYY[:YYYY]` | range anni |
| 6 | Ricerca per NRG+anno | numeroNrg+DataNrgItem | `search --nrg N --anno-nrg YYYY` | |
| 7 | Conteggio totale | "Trovati N risultati" | campo `total` in output | |
| 8 | Paginazione | _cur/changePage | `--page`, `--limit`, `--all` (auto-paginazione) | oltre la singola pagina |
| 9 | Testo integrale provvedimento | visualizzah2 | `get <ecli|idprovv|nrg+sede>` | offline cache |
| 10 | URL pubblico per atto | docUrl onclick | campo `url` su OGNI risultato | linkabile sempre |
| 11 | Verifica appello | resource verifica-appello | `appeal <id>` | batch (vedi transcendence) |
| 12 | Verifica massima | resource verifica-massima | `massime <id>` | aggregazione |

## Transcendence (only possible with local store + scriptable CLI)
| # | Feature | Command | Why only we can do this | Score |
|---|---------|---------|--------------------------|-------|
| 1 | Output markdown pulito del testo integrale (RICHIESTA UTENTE) | `get <id> --format md` | il form mostra HTML in pagina, mai markdown esportabile | 10 |
| 2 | Watch & Diff query salvate | `watch add` / `watch run` | il form e' stateless: non sa "cosa e' nuovo da ieri" | 9 |
| 3 | Corpus export (markdown+CSV di N provvedimenti su un tema) | `corpus build` | il form mostra un atto alla volta, nessun export massivo | 9 |
| 4 | Grep full-regex sui testi integrali scaricati | `grep <regex>` | possibile solo su full-text locale, non sugli snippet del form | 8 |
| 5 | Estrazione massime (principio di diritto) da un corpus | `massime --corpus` | richiede parsing aggregato su molti testi | 7 |
| 6 | Appeal-chain tracer batch TAR->CdS | `appeal-chain` | il "verifica appello" del form e' uno alla volta e manuale | 7 |
| 7 | Stats: distribuzione per sede/sezione/anno | `stats` | aggregazione sull'intera popolazione, il form da' solo lista+conteggio | 6 |

## Foundation (Priority 0)
- Store SQLite: entita' `provvedimento` (idprovv, ecli, tipo, sede, schema, sezione, numero, anno, nrg, data_deposito, snippet, url, nome_file, full_text)
- FTS5 su snippet + full_text
- sync di query salvate (accumulo nel tempo -> abilita watch/diff/corpus/stats)

## Stubs
- (nessuno previsto: tutte le feature sono replayable via HTTP diretto, gia' verificato)
