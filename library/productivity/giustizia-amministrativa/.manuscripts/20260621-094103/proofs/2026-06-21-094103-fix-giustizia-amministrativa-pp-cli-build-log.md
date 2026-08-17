# Build log - giustizia-amministrativa

## Architettura
- Scaffold generato da spec YAML interna (store SQLite, FTS, cobra, MCP cobratree, README/SKILL).
- Client a mano `internal/gaclient/`: handshake Liferay (p_auth + cookie, refresh su 403), search (GET paginato), parsing risultati (regex su struttura stabile), fetch testo integrale, conversione HTML->Markdown/text (x/net/html), verifica_appello (JSON).
- Comandi a mano in `internal/cli/` (marker `// pp:client-call`), core condiviso `ga_core.go`.

## Costruito (Priority 0-2)
- P0: store provvedimenti + FTS; persistenza automatica a ogni ricerca.
- P1 (assorbite): `search` (top-level) + `provvedimenti cerca` (full-text semplice/avanzata, filtri tipo/sede/anno/numero/NRG, paginazione automatica, conteggio totale); `get`/`provvedimenti get` (testo integrale, URL pubblico per atto).
- P2 (transcendence, 7/7): `get --format md` (Markdown, flagship, richiesta utente), `watch run` (diff query salvate), `corpus build` (export Markdown+CSV), `grep` (regex su testi locali), `massime` (estrazione euristica principi di diritto), `appeal-chain` (verifica appello batch TAR->CdS), `stats` (distribuzione per sede/sezione/tipo/anno).
- `sync`: seed provvedimenti recenti + refresh watch.

## Robustezza
- Rate-limiter cortese (cliutil.AdaptiveLimiter, 2 rps) verso sito istituzionale.
- Retry su 429 con backoff (ctx-aware), re-handshake su 403 a ogni pagina.
- gaSkip: short-circuit sotto verify env (no rete, no contesa SQLite).

## Test
- internal/gaclient: parse_test.go + markdown_test.go (logica pura).
- Tutti i comandi verificati dal vivo (sequenziali) exit 0 con output corretto.

## Note
- I tipi provvedimento reali sono più ricchi dei 6 del form (es. "Sentenza Breve", "Ordinanza Collegiale"): il parser li preserva.
- get su item formato pdf: l'URL punta al pdf; markdown migliore sugli item html.
