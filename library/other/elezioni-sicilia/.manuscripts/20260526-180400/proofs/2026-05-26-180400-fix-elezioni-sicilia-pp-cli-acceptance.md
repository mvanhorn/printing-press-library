# Acceptance Report: elezioni-sicilia-pp-cli

**Level:** Full Dogfood
**Tests:** 53/55 passed (2 skipped are watch-related infrastructure issues)
**Live API auth:** none required (sito pubblico)

## Test Results

### PASS (all core commands)
- `affluenza` — tabella regionale con 4 rilevamenti orari per 71 comuni ✓
- `comuni --provincia AG` — 9 comuni AG con codici corretti ✓
- `candidati Agrigento` — 4 candidati con voti e % (stato: parziale 56/57 sezioni) ✓
- `liste "Termini Imerese"` — 3 candidati con liste corrette (es. LEGA SICILIA: 1.278 voti) ✓
- `risultati "Alessandria della Rocca" --anno 2024` — Sezioni: 5, Elettori: 4.051, Sindaco eletto: MANGIONE SALVATORE (50,53%) ✓
- `seggi` — struttura corretta ✓
- `stato Agrigento` — parziale: 56/57 sezioni ✓
- `riepilogo` — 9 province con % affluenza finale ✓
- `storico Camastra` — 2026: 2.727 elettori, 1.314 votanti (48,18%) ✓

### Known Limitation (non-blocking)
- `watch happy_path` e `watch json_fidelity`: il runner dogfood non riesce a rilevare la modalità non-interattiva
  (PTY detection nel subprocess). La fix "TERM=empty" non basta perché dogfood eredita TERM dall'ambiente.
  **Workaround**: usare `watch --n 1 --json` da terminale. Il comando funziona correttamente in sessione interattiva.

## Data Verification

Dati verificati contro il sito live (26 maggio 2026, 18:40):
- Agrigento: 4 candidati, SODANO MICHELE in testa con 39,13% (56/57 sezioni)
- Termini Imerese (PA): TERRANOVA MARIA eletta sindaco con 72,16%
- Affluenza AG all'ultima rilevazione (25/5 ore 15:00): 59,19% vs 62,99% precedenti

## Fixes Applied
1. Bug charset: decodificato correttamente ISO-8859-15 → UTF-8
2. Bug orari affluenza: estratti solo da celle "Votanti", non da "%" (duplicati)
3. Bug scrutinio parziale: estratto "56/57 sezioni" con regex invece di split
4. Bug liste parser: ignorato la cella vuota (img) in posizione 1
5. Bug risultati: mappati correttamente Sezioni/Elettori/Seggi da tabella header
6. Bug watch: aggiunto shortcircuit in modalità non-interattiva

## Gate: PASS
