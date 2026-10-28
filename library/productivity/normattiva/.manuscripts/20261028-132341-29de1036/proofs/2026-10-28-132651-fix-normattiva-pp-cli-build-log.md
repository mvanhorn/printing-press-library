Manifest transcendence rows: 4 planned, 4 built. Phase 3 will not pass until all 4 ship.

# normattiva-pp-cli build log (2026-10-28)

## Built
- internal/norma (pure logic, tested): URN:NIR parse/build (annex, ~art, @originale, !vig=), citation extraction from free text, well-known acts table (20 acts, URNs verified live), AKN HTML → text/markdown renderer, word-level LCS diff.
- Absorbed (hand-built): testo, atto (whole act walk incl. bis/ter, --indice, budgets), cerca (semplice/avanzata auto-switch, URN per result), fonti, urn, cita, cronologia, aggiornati (auto-split >12 months), scarica (preview without --output), esporta (print-by-default; --avvia submits; poll + download).
- Generated: tipologiche (now syncable), collezioni list, ricerca predefinite/semplice/avanzata/aggiornati, dettaglio urn/gazzetta (renamed from atto so `atto` is the whole-act reader), esportazioni stato, sync/search/sql, MCP binary.
- Transcendence: verifica (exit 1 on invalid citations), diff, novita + segui add/list/rm (watchlist + snapshots in custom tables), fascicolo.
- Local cache: every article read is cached in resources('articoli'); pinned versions served from cache in auto mode; network failure falls back to cache.

## Decisions
- Spec resource `atto` renamed `dettaglio` to free the approved `atto` command path.
- Manifest rows normalized: MCP row → (generated endpoint) MCP binary; transcendence row 3 command `novita` (segui is its group).
- Minimal request bodies and honest User-Agent: the gateway resets connections on bulky bodies; no browser-impersonating headers.

## Bugs found during acceptance
- aggiornati rows lack dataEmanazione → URN not derived → novita could never match. Fixed by building the date from giorno/mese/anno.
- Code annex articles have no article-num markup → heading taken from first text line.

## Deferred / known
- esporta --avvia not exercised live yet (queues server-side work); scheduled for Phase 5 with a one-act query.
- 2 dead generated helpers (retainCLIQueryParams, retainExplicitQueryParams) left as generated.
- scarica does not warn when the requested --modo is not offered for that collection.
