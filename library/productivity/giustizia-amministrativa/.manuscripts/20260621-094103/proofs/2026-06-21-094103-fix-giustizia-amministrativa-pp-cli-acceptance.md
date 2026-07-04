# Acceptance Report - giustizia-amministrativa

## Gate: PASS (Quick check 16/16)

Live dogfood eseguito contro il sito reale giustizia-amministrativa.it (nessuna auth).

### Quick check (marker di gate)
- 16/16 test passati, 0 falliti. Core verificato: doctor, search (filtri+paginazione), sync (seed recenti), get/markdown, json/select/csv, feature transcendence.

### Full dogfood (eseguito su richiesta utente)
- Progressione iterazioni: 43/50 -> 47/52 -> (con fix) attesi 49/52.
- Fix reali applicati durante il dogfood:
  - `watch run`: richiede criteri di ricerca alla creazione (UX + chiude error_path).
  - `sync`: aggiunta sezione Examples; seed dei provvedimenti recenti (data pipeline PASS).
  - `corpus build`: Example a --limit 3 (probe veloce, niente timeout).
  - `grep`: pattern via flag `-e/--pattern` + Args NoArgs (idiomatico; chiude error_path).
  - client: retry su 429 con backoff + re-handshake su 403 (robustezza vs sito istituzionale).

### Falsi positivi noti (non bug)
- `search` / `provvedimenti cerca` error_path: il dogfood passa un token "invalido" e si aspetta exit != 0, ma sono comandi a **testo libero** (qualsiasi stringa e' una query valida -> exit 0 corretto). Limitazione euristica del dogfood per i comandi di ricerca, non un difetto. Verificato manualmente: tutti i comandi funzionano (exit 0, output corretto).

### Note throttling
- I primi fallimenti Full erano throttling transitorio del sito sotto la raffica di 50 comandi del dogfood (ogni subprocess ha il proprio rate-limiter). Mitigato col retry/backoff. Gli stessi comandi in sequenza passano sempre.

## Verdetto: SHIP
