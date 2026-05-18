# Absorb Manifest — openipa CLI

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|-------------------|-------------|--------|
| 1 | Uffici destinatari SFE per CF | FatturaElettronica.IndicePA (C#) WS01 | `openipa fatturazione cf <CF> --json` | Offline + JSON pipe-friendly | ship |
| 2 | AOO per cod_amm | FatturaElettronica.IndicePA WS02 | `openipa aoo list <cod_amm>` | Offline FTS, `--json`, `--compact` | ship |
| 3 | UO per cod_amm | FatturaElettronica.IndicePA WS03 | `openipa uo list <cod_amm>` | Offline FTS, `--json` | ship |
| 4 | SFE channels per cod_amm | FatturaElettronica.IndicePA WS04 | `openipa fatturazione ente <cod_amm>` | `--json`, exit codes | ship |
| 5 | Dati ente per cod_amm | FatturaElettronica.IndicePA WS05 | `openipa enti get <cod_amm>` | Offline, `--compact`, `--json` | ship |
| 6 | UO per codice univoco | FatturaElettronica.IndicePA WS06 | `openipa uo get <cod_uni_ou>` | Offline, dettaglio completo | ship |
| 7 | Cerca per email | FatturaElettronica.IndicePA WS07 | `openipa cerca email <email>` | `--json`, typed exit | ship |
| 8 | NSO channels per CF | nicogis/FatturazioneElettronica-IPA WS14 | `openipa nso cf <CF>` | Offline + JSON | ship |
| 9 | NSO channels per cod_amm | nicogis/FatturazioneElettronica-IPA WS15 | `openipa nso ente <cod_amm>` | `--json`, exit codes | ship |
| 10 | Cerca ente per nome | forum.italia.it scripts WS16 | `openipa enti cerca <nome>` | Offline FTS (SQLite), fuzzy | ship |
| 11 | Domicilio digitale AOO | community scripts WS09 | `openipa domicilio aoo <cod_aoo>` | `--json`, storico | ship |
| 12 | Domicilio digitale UO | community scripts WS10 | `openipa domicilio uo <cod_uni_ou>` | `--json`, storico | ship |
| 13 | Storico domicili AOO | (nessun tool) WS11 | `openipa domicilio storico-aoo <cod_aoo>` | Primo tool con storico | ship |
| 14 | Storico domicili UO | (nessun tool) WS12 | `openipa domicilio storico-uo <cod_uni_ou>` | Primo tool con storico | ship |
| 15 | Cerca per domicilio digitale | (nessun tool) WS13 | `openipa domicilio email <pec>` | Primo tool a cercarlo | ship |
| 16 | Domicilio digitale per CF | (nessun tool) WS23 | `openipa domicilio cf <CF>` | Nuovo — nessun tool | ship |
| 17 | AOO con filtro data | nicogis WS08 | `openipa aoo get <cod_amm> [--aoo <cod_aoo>]` | filtro integrato | ship |
| 18 | PEC ente | FatturaElettronica WS20 | `openipa pec ente <cod_amm>` | — | (stub: WS20 HTTP 500 sul server IPA) |
| 19 | Storico PEC ente | (nessun tool) WS21 | `openipa pec storico <cod_amm>` | — | (stub: WS21 HTTP 500 sul server IPA) |
| 20 | Storico per PEC | (nessun tool) WS22 | `openipa pec cerca <pec>` | — | (stub: WS22 probabilmente stesso problema) |
| 21 | WS18 AOO per cod_uni_aoo | (nessun tool) WS18 | `openipa aoo get-uni <cod_uni_aoo>` | — | (stub: parametro COD_UNI_AOO non documentato) |
| 22 | WS19 AOO specifica con cessazione | (nessun tool) WS19 | endpoint interno — dati aggregati in WS08 | — | (stub: da investigare) |

## Transcendence (solo possibile con il nostro approccio)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| T1 | CF → tutto in un comando | `openipa cf <CF>` | Richiede chain WS01+WS14+WS23 in parallelo: uffici FE, NSO, e domicilio digitale — dati che il portale web divide in 3 pagine separate |
| T2 | Sync bulk + ricerca offline FTS5 | `openipa sync && openipa enti cerca <nome>` | Combina CKAN bulk download (27 dataset) con SQLite FTS5 per ricerca offline su tutti i 20k enti senza rate limit |
| T3 | Verifica PEC valida/storica | `openipa pec verifica <indirizzo-pec>` | Controlla WS07+WS13 (se PEC esiste come domicilio digitale attivo) — primo tool a distinguere PEC attiva vs storica vs sconosciuta |
| T4 | Export bulk per regione | `openipa enti list --regione Sicilia --json` | Powered da SQLite locale sync, nessun tool offre filtro geografico; l'API WS non ha filtri geografici |
| T5 | Doctor/validazione CF ente | `openipa doctor --cf <CF>` | Verifica che un CF abbia sia SFE (WS01) sia NSO (WS14) sia domicilio digitale (WS23): checklist compliance PA in un comando |

## Stub Items (approvazione esplicita richiesta)

1. **PEC ente** (`openipa pec ente`) — WS20 restituisce HTTP 500 per tutti gli enti testati. Causa: bug server IPA. Il comando esiste ma restituisce errore user-friendly.
2. **Storico PEC** (`openipa pec storico`, `openipa pec cerca`) — WS21/WS22 stesso problema HTTP 500.
3. **WS18/WS19** — parametri non ancora verificati completamente. Stub fino a documentazione.
