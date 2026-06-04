# Absorb Manifest — ARS Sicilia

## Sources surveyed

- **opendatasicilia/RSSdisegniLeggeAssembleaRegionaleSiciliana** (Shell): scraper DDL → CSV → RSS feed. Copre solo l'archivio 221 (Disegni di Legge). Autore: aborruso. Repo: `https://github.com/opendatasicilia/RSSdisegniLeggeAssembleaRegionaleSiciliana`.
- **aborruso/ars_sicilia** (Python): ETL Astro+Python per video sedute YouTube con trascrizioni e digest AI. Focus su resoconti video, non sugli archivi documentali ARS.
- **aborruso/regioneSiciliaNewsRSS** (Shell): RSS news Regione Siciliana, fuori scope (non ARS).
- **aborruso/iter-legis** (Python): ETL Senato italiano (Akoma Ntoso), parlamento nazionale — non ARS regionale.
- **italianparliament-mcp** (MCP nazionale): solo Camera/Senato nazionali, non Regionali.
- **OpenParlamento.it**: copertura parziale parlamenti regionali, niente di sostantivo per ARS Sicilia.
- **UI nativa dati.ars.sicilia.it**: 12 archivi via form JSP del 2008, no API documentata, no CLI ufficiale.

Conclusione: zero CLI completa, una sola shell-script community che copre 1 archivio su 12.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|-------------------|-------------|--------|
| 1 | Ricerca DDL paginata | opendatasicilia shell scraper | `ars-sicilia ddl cerca --anno 2024` | FTS5 offline, JSON/CSV, query strutturata | shipping |
| 2 | Feed nuovi DDL (RSS) | opendatasicilia shell scraper | `ars-sicilia ddl tail --since 7d` + `ars-sicilia <archivio> tail` per tutti i 12 | Generico, non solo DDL | shipping |
| 3 | Lista leggi regionali per anno | UI ARS 201.jsp | `ars-sicilia leggi cerca --anno 2024 --legisl 18` | Sync SQLite, SQL libera, FTS | shipping |
| 4 | Get singolo DDL | UI ARS 221.jsp | `ars-sicilia ddl get <legisl> <numddl>` | JSON tipato, parsing campi | shipping |
| 5 | Ricerca interrogazioni parlamentari | UI ARS 233.jsp | `ars-sicilia interrogazioni cerca --firmatario "Rossi"` | Cross-legislatura, FTS testo | shipping |
| 6 | Ricerca mozioni | UI ARS 235.jsp | `ars-sicilia mozioni cerca --legisl 18` | Idem | shipping |
| 7 | Ricerca interpellanze | UI ARS 234.jsp | `ars-sicilia interpellanze cerca` | Idem | shipping |
| 8 | Ricerca ordini del giorno | UI ARS 236.jsp | `ars-sicilia odg cerca` | Idem | shipping |
| 9 | Ricerca risoluzioni | UI ARS 238.jsp | `ars-sicilia risoluzioni cerca` | Idem | shipping |
| 10 | Ricerca pareri al Governo | UI ARS 226.jsp | `ars-sicilia pareri cerca` | Idem | shipping |
| 11 | Resoconti sedute aula | UI ARS 217.jsp | `ars-sicilia resoconti cerca --data 2024-01-15` | Range date, oratore/argomento | shipping |
| 12 | Convocazioni commissioni | UI ARS 229.jsp | `ars-sicilia commissioni convocazioni --codcom 5` | Filtro codice commissione | shipping |
| 13 | Sommari lavori commissioni | UI ARS 230.jsp | `ars-sicilia commissioni sommari --presidente "Bianchi"` | Cross commissioni, presidente | shipping |
| 14 | Catalogo bibliografico | UI ARS 205.jsp | `ars-sicilia biblioteca cerca --autore "Sciascia"` | JSON tipato | shipping |
| 15 | Opere multimediali | UI ARS 205multimedia.jsp | `ars-sicilia biblioteca multimediali` | Idem | shipping |
| 16 | Costruzione espressione ISIS | manuale (utente conosce E, O, VICINO, $, %) | Auto-traduzione da flag CLI puliti | Sintassi ISIS nascosta all'utente | shipping |
| 17 | Sessione JSESSIONID | shell scraper riusa cookie jar | `internal/icaroclient/` gestisce JSESSIONID | Per-comando trasparente | shipping |
| 18 | Parser HTML risultati | shell + `scrape-cli + jq + xq` | Parser HTML Go tipato per archivio | No dipendenze esterne | shipping |
| 19 | Sync SQLite locale | nessuno | `ars-sicilia sync --resources leggi,ddl --since 30d` | Generator-emitted con FTS5 | shipping |
| 20 | Search globale FTS cross-archive | nessuno | `ars-sicilia search "bilancio sanitario"` | Generator-emitted FTS5 | shipping |
| 21 | Output agent-native | nessuno | `--json --select --csv --agent --compact` | Generator-emitted | shipping |
| 22 | MCP server | nessuno | `ars-sicilia-mcp` espone CLI tree | Generator-emitted | shipping |
| 23 | SQL libera su store | nessuno | `ars-sicilia sql "SELECT ..."` | Generator-emitted | shipping |
| 24 | Doctor reachability | nessuno | `ars-sicilia doctor` | Generator-emitted | shipping |

## Transcendence (only possible with our approach)

Survivors from novel-features subagent (Phase 1.5c.5):

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Iter completo di un DDL | `ars-sicilia ddl iter <legisl> <num>` | 9/10 | hand-code | JOIN cronologico su SQLite locale tra `bills` (221), `convocazioni_commissioni` (229), `sommari_commissioni` (230), `resoconti_aula` (217) e `laws` (201) usando `NUMDDL`+`LEGISL` come chiavi e `DATPRE`/`DATSED` per ordinare. | Brief Top Workflow #1 ("cercare e scaricare l'iter di un DDL"); User Vision: aborruso lamenta "nessun cross-reference tra archivi". | none |
| 2 | Profilo deputato cross-archive | `ars-sicilia deputato profilo "<cognome nome>"` | 9/10 | hand-code | Match su `FIRMAT` in 6 archivi + `ORATOR` in `resoconti_aula`; output unificato cronologico con conteggi per tipo. | Brief Top Workflow #2; User Vision esplicita: 10/12 archivi non interrogabili programmaticamente. | none |
| 3 | Coppie di co-firmatari di atti | `ars-sicilia analytics --type ddl --group-by cofirmatari --limit 50` | 8/10 | spec-emits | Esplode `FIRMAT` in coppie e raggruppa per pair nei `bills`, restituendo conteggi decrescenti. | Brief Data Layer (FIRMAT strutturato); persona Chiara (network politico). | none |
| 4 | Oratori più attivi in aula | `ars-sicilia analytics --type resoconti --group-by oratore --limit 30` | 8/10 | spec-emits | Conta righe in `resoconti_aula` per `ORATOR`, opzionale somma lunghezza `TESTO`. | Brief Data Layer (campo ORATOR di 217); nessuno scraper esistente lo aggrega. | none |
| 5 | Dossier 360° di una commissione | `ars-sicilia commissione dossier <codcom>` | 8/10 | hand-code | JOIN su `CODCOM`/`COMMIS` tra 229, 230, 226 e `bills` filtrati per `RELACO`/`SETTOR`. | Brief Top Workflow #5; non assorbito. | none |
| 6 | Drift dell'iter dei DDL | `ars-sicilia ddl drift --since 7d` | 8/10 | hand-code | Diff snapshot SQLite (`bills.ITERAT`/`ITERST`) vs stato corrente post-sync, emette righe cambiate. | Brief Data Layer; oggi RSS shell mostra solo "nuovi", non "mossi". | none |
| 7 | Query ISIS grezza (escape hatch) | `ars-sicilia <archivio> cerca --isis-query "<expr>"` | 8/10 | hand-code | Flag globale ai comandi `cerca` che bypassa il flag-builder e inoltra l'espressione come `icaQuery`. | Brief Codebase Intelligence (sintassi ISIS); User Vision (aborruso usa ISIS). | none |
| 8 | Stato di sync degli archivi | `ars-sicilia sync stale` | 6/10 | hand-code | Legge `meta.last_synced_at` per i 12 archivi, calcola età e n. record locali. | Brief Data Layer; persona Andrea (ops). | none |
| 9 | Cronologia inversa di una legge | `ars-sicilia legge cronologia <legisl> <num>` | 5/10 | hand-code | Inverso di `ddl iter`: parte da `laws`, risale al `bills` originario, raccoglie passaggi in 230/217. | Brief Top Workflow #3 + #1; persona Chiara (corpus storico). | Usare questo comando solo per una legge GIA' promulgata (archivio 201). Per un DDL ancora in iter usare `ars-sicilia ddl iter`. |

**Stub commitments:** nessuna riga è in stub. Tutte e 9 le novel features sono shipping-scope.

**Hand-code commitment:** 7 hand-code (#1, #2, #5, #6, #7, #8, #9) + 2 spec-emits (#3, #4) = 9 novel features.

