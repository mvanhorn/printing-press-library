# Novel Features Brainstorm — openipa CLI

## Survivors (scoring >= 5/10)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| S1 | Doctor compliance PA | `openipa doctor <CF>` | 9/10 | Chiama WS01_SFE_CF + WS14_NSO_CF + WS23_DOM_DIG_CF in parallelo, produce checklist pass/fail per SFE/NSO/domicilio digitale — tre sezioni del portale web collassate in un comando |
| S2 | Batch CF → destinatari FE | `openipa fatturazione batch` | 8/10 | Legge CF da stdin (NDJSON/TSV), chiama WS01_SFE_CF in parallelo, emette NDJSON CF+cod_uni_ou+stato_canale — nessun tool ha batch mode |
| S3 | Stats aggregati per regione | `openipa stats --regione <R>` | 7/10 | Join SQLite locale su enti+aoo+uo+sfe_channels — aggregati geografici che nessun tool espone |
| S4 | Enti senza SFE attivo | `openipa report sfe-mancante` | 7/10 | Anti-join SQLite locale: enti senza canale SFE attivo, filtrabile per regione/categoria |
| S5 | Struttura gerarchica ente | `openipa enti tree <cod_amm>` | 7/10 | WS05_AMM + WS02_AOO + WS03_OU in parallelo, output ad albero Ente → AOO[N] → UO[M] |
| S6 | Ricerca AOO per nome | `openipa aoo cerca <nome>` | 6/10 | WS18_AOO (DESCR_AOO) — endpoint non nell'absorb manifest, gap asimmetrico |
| S7 | Verifica domicilio digitale | `openipa domicilio verifica <pec>` | 6/10 | WS13_DOM_DIG + WS07_EMAIL in sequenza — classifica PEC come attiva/storica/sconosciuta |

## Killed candidates
| Candidate | Kill reason |
|-----------|-------------|
| CF universale (dump grezzo) | Sibling di S1 senza logica pass/fail |
| Cerca nome → PEC | Dipende da WS20 HTTP 500 |
| Export enti per regione | Subset di S3 |
| Storico PEC timeline | WS21 HTTP 500 |
| Confronto due enti | Speculativo, <settimanale |
| PEC di AOO | Duplicato domicilio aoo assorbito |
| Watch stato canale | Scope creep |
| Sync incrementale | Complexity alta per valore moderato |
| Disambigua fuzzy offline | Ridondante con enti cerca FTS5 |
