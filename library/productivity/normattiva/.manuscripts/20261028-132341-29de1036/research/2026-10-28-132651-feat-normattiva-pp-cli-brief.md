# Normattiva CLI Brief

## API Identity
- Domain: Italian national legislation — Normattiva, the official database of State acts published in the Gazzetta Ufficiale (Presidenza del Consiglio / Camera / Senato, operated by IPZS). Covers laws, decree-laws, legislative decrees, DPRs, regi decreti, the Constitution, with consolidated ("vigente"), original and point-in-time ("multivigente") texts.
- API: Normattiva Open Data REST API (launched 2025). Base `https://api.normattiva.it/t/normattiva.api/bff-opendata/v1/api/v1`. Test env `pre.api.normattiva.it`. Official spec document: "Normattiva — Specifiche API Open Data" (mirrored at github.com/adellorto/normattiva-mcp/docs/Normattiva_API_OpenData_Specifiche.md). The doc references an `openapi-bff-opendata-<VERSION>.json` descriptor that is not publicly linked; the spec will be authored from the doc + live probes.
- Users: lawyers, legal-tech developers, public administration staff, compliance officers, journalists, students, and AI agents needing authoritative citations.
- Data profile: JSON metadata (act lists with facets), article text as HTML (AKN-derived classes), bulk ZIP exports (AKN, XML, JSON, HTML, PDF, EPUB, RTF, URI/ELI).

## Endpoints (verified live 2026-10-28 unless noted)
| Method | Path | Purpose | Live |
|---|---|---|---|
| GET | `/tipologiche/estensioni` | export formats | 200 |
| GET | `/tipologiche/classe-provvedimento` | act class codes | doc |
| GET | `/tipologiche/denominazione-atto` | act type names/codes | doc |
| GET | `/collections/collection-predefinite` | prebuilt collections (e.g. Codici) | doc |
| GET | `/ricerca/predefinita` | predefined searches | doc |
| POST | `/ricerca/semplice` | full-text search `{testoRicerca, orderType: recente|vecchio, paginazione{paginaCorrente, numeroElementiPerPagina}, filtriMap?}` | 200 |
| POST | `/ricerca/avanzata` | advanced search (denominazioneAtto, titoloRicerca, testoRicerca, date ranges emanazione/pubblicazione, vigenza, classeProvvedimento, anno/mese/giorno/numeroProvvedimento) | doc |
| POST | `/ricerca/aggiornati` | acts updated between two datetimes (max 12 months, max 7000 acts; error codes 1501/1502/1503) | doc |
| POST | `/atto/dettaglio-atto` | text by Gazzetta coordinates `{codiceRedazionale, dataGU}` (+ idArticolo…); always current text | 200 (minimal body) |
| POST | `/atto/dettaglio-atto-urn` | text by URN:NIR `{urn}`; supports `~artN`, `@originale`, `!vig=YYYY-MM-DD` | 200 |
| POST | `/ricerca-asincrona/nuova-ricerca` → `conferma-ricerca` → GET `check-status/{token}` (303 + `x-ipzs-location`) → GET `collections/download/collection-asincrona/{token}` | async bulk export ZIP | doc |
| GET | `/collections/download/collection-preconfezionata?nome=&formato=&formatoRichiesta=O|V|M` | prebuilt collection ZIP | doc |

Response notes: search returns `listaAtti[]` (codiceRedazionale, dataGU, denominazioneAtto, numeroProvvedimento, annoProvvedimento, descrizioneAtto, titoloAtto in brackets, dataUltimaModifica…), `facetMap` (anno_provvedimento, codice_tipo_provvedimento, descrizione_emettitore), `numeroPagine`, `numeroAttiTrovati`. Detail returns `data.atto` with `titolo`, `sottoTitolo`, `articoloHtml`, `articoloDataInizioVigenza`/`articoloDataFineVigenza` (YYYYMMDD, `99999999` = open), GU coordinates. Errors: Spring-style `{timestamp,status,error,path}` or `{message,code}`.

## Reachability Risk
- Low. No authentication. Production answered 200 to plain requests with an honest User-Agent and minimal JSON bodies. One connection reset seen when sending the documentation's full detail body (extra fields like `idGruppo`, `sottoArticolo1`) — use minimal bodies. The host root resets connections (WAF), so the CLI must never probe `/`. Do not send browser-impersonating headers.
- Probe-safe endpoint used: `GET /tipologiche/estensioni`.

## Top Workflows
1. "Read article N of law X" — `testo "d.lgs. 196/2003" --articolo 13` / by URN, rendered as clean plain text/markdown in the terminal.
2. "What did article N say on date D?" — point-in-time text via `!vig=` URN, plus the version window (in force from/to).
3. Search acts by keyword/title/type/year with facets; jump from a result straight to its text.
4. Monitor changes: acts updated in a date range (`aggiornati`), e.g. weekly digest; diff of an article between two dates.
5. Bulk export / prebuilt collections (Codici) as ZIP in AKN/JSON/XML for offline corpora.

## Table Stakes (from competitors)
- normattiva-sdk (Python, CLI `normattiva`): testo, cerca, avanzata, cronologia (walks every version of an article), aggiornati, esporta (async), collezioni, scarica, dizionario, identificatore (URN compose/parse offline), noti (well-known acts by name).
- pipeworx mcp-it-normattiva: search, get_act (as_of / original), get_article (article/annex, as_of).
- adellorto/normattiva-mcp, avvocati-e-mac/normattiva-mcp (CLI+MCP to read, verify, cite norms), ondata/normattiva_2_md (law → markdown for LLMs), dej611/italian-laws-history (law as git history).
- Must have: URN:NIR builder/parser, citation-alias resolution ("codice civile", "GDPR nazionale" → URN), HTML→text/markdown rendering, vigenza windows, original text, JSON output for agents.

## Data Layer
- Primary entities: atti (search results keyed by codiceRedazionale+dataGU, with URN when derivable), articoli (text snapshots keyed by URN+vigenza), aggiornamenti (update feed).
- Sync cursor: `dataUltimaModifica` / the `aggiornati` date window.
- FTS/search: local SQLite FTS over cached act titles and fetched article text, so repeated lookups and "grep across laws I've read" work offline.

## User Vision
- (none provided — "Partiamo")

## Product Thesis
- Name: `normattiva-pp-cli` — "Normattiva da terminale".
- Why it should exist: no Go/single-binary CLI exists; the Python SDK requires a Python env. A fast, agent-native CLI that resolves human citations ("art. 2043 c.c.", "L. 241/1990 art. 3") to authoritative text, shows point-in-time versions and diffs, renders clean Markdown, and caches locally for offline search — ideal for lawyers and for AI agents that must cite Italian law accurately.

## Build Priorities
1. `atto`/`testo` by URN or citation alias, with `--articolo`, `--vigenza`, `--originale`, markdown/plain/json rendering.
2. `cerca` (semplice + avanzata flags, facets, pagination) with result → URN.
3. `aggiornati` (updates feed) and `cronologia`/`diff` of an article across versions.
4. `urn` offline builder/parser + `alias` list of well-known acts (Costituzione, codice civile, codice penale, c.p.c., c.p.p., GDPR d.lgs. 196/2003, CAD, codice appalti…).
5. Dictionaries (`tipologiche`), prebuilt collection download, async export with polling.

## Public permalinks
- Article and act permalinks use the public portal resolver `https://www.normattiva.it/uri-res/N2Ls?<urn:nir:...>` (verified 2026-10-28; this is the link the API itself embeds in article HTML). Gazzetta-coordinate links use `https://www.normattiva.it/atto/caricaDettaglioAtto?...`.
