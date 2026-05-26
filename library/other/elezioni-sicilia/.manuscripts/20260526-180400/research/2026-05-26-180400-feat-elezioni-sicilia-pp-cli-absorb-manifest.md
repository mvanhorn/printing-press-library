# Absorb Manifest — elezioni-sicilia CLI

## Context
Nessun competitor diretto per questo sito specifico. L'unica "alternativa" per i dati elettorali siciliani
è scaricare PDF o copiare manualmente le tabelle HTML. Alcuni tool nazionali (marcodallastella/elezioni,
onData) coprono elezioni politiche/europee ma non le comunali siciliane da questo portale.

## Absorbed (match o supera tutto ciò che esiste)

| # | Feature | Best Source | Nostra Implementazione | Added Value |
|---|---------|-------------|----------------------|-------------|
| 1 | Affluenza elettorale | Tabella HTML del sito | `affluenza [--provincia X]` con parsing tabella | JSON output, filtro per provincia, diff% vs precedenti |
| 2 | Voti candidato sindaco | HTML per-comune | `candidati <comune>` | JSON, ricerca per nome comune |
| 3 | Voti per lista | HTML per-comune | `liste <comune>` | JSON, aggregazione per candidato |
| 4 | Risultati finali | HTML per-comune | `risultati <comune>` | JSON, include sezioni, elettori, seggi |
| 5 | Ripartizione seggi | HTML per-comune | `seggi <comune>` | JSON, seggi per lista e candidato |
| 6 | Navigazione comuni | Dropdown HTML | `comuni [--provincia X]` | JSON con codici interni, ricerca per nome |
| 7 | Stato scrutinio | Campo in ogni pagina | `stato <comune>` | "in_corso", "parziale:N/M", "completo" |

## Transcendence (solo possibile con il nostro approccio)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | Confronto storico | `storico <comune>` | Richiede fetch parallelo delle pagine di 4 anni (2022-2025) e confronto strutturato — nessun utente lo fa manualmente |
| 2 | Monitoraggio scrutini live | `watch [--intervallo 5m]` | Polling periodico dello stato scrutinio per tutti i comuni, alert su avanzamento — nessun feed/API nativo |
| 3 | Riepilogo regionale | `riepilogo` | Aggrega affluenza e stato scrutini per tutte le 9 province in un singolo output strutturato |

## Note Implementative Critiche

- **Charset**: ISO-8859-15 — usare `golang.org/x/text/encoding/charmap.ISO8859_15` su ogni response
- **Transport**: browser-http (Surf) — TLS self-signed, stdlib fallisce
- **3 stati dati**: "scrutini in corso" (nessun dato), "parziale N/M sezioni", "completo"
- **URL archive**: http:// (non https://) per anni 2022-2025
- **Codici comune**: interni al sito (non ISTAT) — bootstrap da `<select name="town">`
- **secondoTurno**: non disponibile come archivio separato nei test attuali
