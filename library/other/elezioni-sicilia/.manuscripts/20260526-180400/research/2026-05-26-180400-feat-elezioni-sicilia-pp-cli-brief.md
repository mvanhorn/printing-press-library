# elezioni-sicilia CLI Brief

## API Identity
- Domain: Dati elettorali delle elezioni comunali siciliane (Regione Siciliana)
- Fonte: https://www.elezioni.regione.sicilia.it (sito istituzionale Assessorato Autonomie Locali)
- Users: Giornalisti, data analyst, ricercatori, hacktivisti civici, sviluppatori open data
- Data profile: HTML statico (anni '90), nessun API ufficiale, nessun export JSON/CSV — solo HTML e PDF

## Reachability Risk
- Low — accessibile via Surf/browser-compatible HTTP (TLS self-signed bypassed da Surf)
- probe-reachability: mode=browser_http, confidence=0.85
- Nessun rate limiting rilevato, dati aggiornati in tempo reale durante scrutini

## Struttura Sito Scoperta
### URL Pattern
- Base 2026: https://www.elezioni.regione.sicilia.it/
- Archive 2025: http://www.elezioni.regione.sicilia.it/comunali2025/
- Archive 2024: http://www.elezioni.regione.sicilia.it/comunali2024/primoTurno/
- Archive 2023: http://www.elezioni.regione.sicilia.it/comunali2023/primoTurno/
- Archive 2022: http://www.elezioni.regione.sicilia.it/comunali2022/primoTurno/

### Report Types (per comune: /{PROV}/{ReportType}{PROV}{CODE}.html)
1. ReportDatiLista — candidati sindaco + liste per ogni candidato
2. ReportCandidati — voti per candidato sindaco (parziali o completi)
3. ReportCandidatiListe — voti per lista collegata a ogni candidato
4. ReportRisultati — risultato finale (sindaco eletto, sezioni, elettori, schede)
5. ReportSeggi — ripartizione seggi per lista

### Report Globali (a livello regionale/provinciale)
- ReportTabellaAffluenza.html — affluenza per tutti i comuni, 4 rilevamenti (12:00/19:00/23:00/15:00)
- ReportRisultati.html / ReportCandidati.html / etc. — navigatori a dropdown per provincia

### Province Siciliane (9): AG, CL, CT, EN, ME, PA, RG, SR, TP
### Comuni 2026: 71 comuni in 9 province (AG=9, CL=7, CT=9, EN=6, ME=17, PA=16, RG=1, SR=3, TP=3)
### Codice comune: numerico interno al sistema (es. Agrigento=11, non ISTAT)

## Dati Disponibili per Campo

### ReportTabellaAffluenza (rilevamento affluenza):
- Comune, Elettori
- Votanti ore 12:00 (24/5), % vs precedenti, diff%
- Votanti ore 19:00 (24/5), % vs precedenti, diff%
- Votanti ore 23:00 (24/5), % vs precedenti, diff%
- Votanti ore 15:00 (25/5), % vs precedenti, diff%

### ReportCandidati (voti sindaco):
- Per ogni candidato: N°, Nome, Voti, %
- Stato scrutinio (parziale: N su M sezioni, o completo)

### ReportCandidatiListe (voti lista):
- Per ogni candidato sindaco: N°, Nome
- Per ogni lista collegata: N° lista, Nome lista, N° candidati, Voti, %

### ReportRisultati (completato):
- Comune, Provincia, Pop. Legale
- Sezioni, Elettori, Seggi, Votanti (totale + %), Voti Sindaco, Voti Consiglio
- Schede non valide (totale + bianche)
- Per sindaco eletto: Nome, Voti, %, Liste + seggi
- Per altri candidati: Nome, Voti, %

### ReportSeggi (completato):
- Come ReportRisultati + seggi per ogni lista
- Seggio assegnato a candidato sindaco sconfitto (se supera soglia)

## Top Workflows (power user)
1. **Monitoraggio in tempo reale scrutini**: seguire l'avanzamento del conteggio voti durante la notte delle elezioni
2. **Confronto affluenza**: confrontare l'affluenza 2026 con le precedenti elezioni per ogni comune
3. **Risultati per provincia/comune**: estrarre dati strutturati (JSON) per analisi o visualizzazioni
4. **Trend partiti/liste**: aggregare voti per partito su tutti i comuni
5. **Export dati**: estrarre tutti i dati in CSV/JSON per uso in altri strumenti

## Table Stakes
- Query per provincia / comune
- Output JSON strutturato
- Confronto con elezioni precedenti (storico 2022-2025)
- Monitoraggio stato scrutini (parziale vs completo)
- Affluenza in tempo reale

## Data Layer
- Primary entities: Comune (province + code + nome), Elezione (anno + tipo), Candidato, Lista, VotiCandidato, VotiLista, Affluenza
- Sync cursor: timestamp ultimo aggiornamento (presente in ogni pagina)
- FTS/search: ricerca per nome comune, nome candidato, nome lista

## Codebase Intelligence
- Source: analisi diretta del sito tramite curl/python
- Auth: nessuna (sito pubblico)
- Transport: Surf/browser-compatible HTTP (TLS self-signed)
- Data model: HTML tables annidate in struttura anni '90
- Rate limiting: nessuno rilevato
- Architecture: CGI/static HTML server, pagine generate dinamicamente ma statically served

## Product Thesis
- Name: elezioni-sicilia
- Why it should exist: il sito ufficiale non offre API, non esporta JSON/CSV, non è consultabile programmaticamente. La CLI converte le tabelle HTML in output strutturato JSON/CSV, permette comparazioni temporali, e rende i dati accessibili a giornalisti e analisti senza dover copiare tabelle manualmente.

## Build Priorities
1. Affluenza: `affluenza [--anno 2026] [--provincia AG] [--json]` — tabella regionali con tutti i rilevamenti
2. Candidati: `candidati <comune> [--anno 2026] [--json]` — voti per candidato sindaco
3. Liste: `liste <comune> [--anno 2026] [--json]` — voti per lista
4. Risultati: `risultati <comune> [--anno 2026] [--json]` — risultati completi
5. Comuni: `comuni [--provincia AG] [--json]` — elenco comuni alle elezioni
6. Seggi: `seggi <comune> [--anno 2026] [--json]` — ripartizione seggi
7. Storico: `storico <comune> [--json]` — confronto tra anni
