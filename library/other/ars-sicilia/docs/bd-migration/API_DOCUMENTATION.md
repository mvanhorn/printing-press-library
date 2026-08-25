Reverse engineering API portale ARS — backend nuovo /bd/

Fonte: dati.ars.sicilia.it. Mappatura via traffico (agent-browser + curl), 2026-07-21.

## Contesto

Il portale ha DUE backend:
- **Legacy Icaro/ISIS** — `GET /icaro/default.jsp?icaDB=<ID>&icaQuery=(...)` + `/icaro/shortList.jsp`. È quello che la CLI usa oggi. Per 3 archivi il suo indice è **congelato** (sommari a giu 2025, resoconti a feb 2026).
- **Nuovo /bd/** — `POST /bd/<archivio>`, dati **correnti** (fino a luglio 2026). Documentato qui.

Non è un'API JSON: risponde **HTML paginato**. Ma è regolare e parsabile.

## Flusso di chiamata

1. `GET https://dati.ars.sicilia.it/bd/<archivio>` — stabilisce il cookie di sessione (Tomcat `JSESSIONID`). La pagina è una SPA; l'HTML iniziale è un guscio.
2. `POST https://dati.ars.sicilia.it/bd/<archivio>` con `Content-Type: application/x-www-form-urlencoded` e i campi dell'archivio (sotto). Header utili: `Origin`, `Referer` = stesso URL.
3. La risposta è la **pagina risultati in HTML** (lista + form ripopolato). Ripetere il POST con `page=N` per le pagine successive.

Il token `_csrf` è presente ma **può essere vuoto** (verificato: `_csrf=` funziona).

## Convenzione nomi dei campi POST

Prefissi (stile ISIS):
- `$I<campo>` — campo **indicizzato/esatto** (es. `$Ilegislatura`, `$Icommissione_id`, `$Iseduta_numero`, `$Ispeakers`).
- `$T<campo>` — campo **testo** (es. `$TTEXT`, `$Todg`, `$Tautore`).
- `$S$T<campo>` / `$S$I<campo>` — **selettore modalità** di quel campo (valori: `all` = "tutte le parole"; altri: frase esatta / una delle parole / parziale).
- `$R<campo>` — radio (es. `$Rtipogr`).
- `$U<campo>` — full-text (es. `$Ufulltext`).
- `anno`, `page`, `_csrf` — semplici.

`anno` filtra per **anno** (non c'è un filtro data esatta/range server-side sui 3 archivi sedute: per una data precisa si filtra client-side dopo aver scaricato l'anno).

## Archivi /bd/ (esistenti, HTTP 200)

| /bd/ | Nome | Campi POST | Colonne risultato | Record tot* |
|---|---|---|---|---|
| `sommari` | Sommari Lavori Commissione | `$Ilegislatura, anno, $Iseduta_numero, $Icommissione_id, $Todg, $S$Todg, $TTEXT, $S$TTEXT, page, _csrf` | Legisl., Data, N. Seduta, Commissione e Ordine del giorno | 12.875 |
| `resoconti` | Resoconti Sedute d'Aula | `$Ilegislatura, anno, $Inrosed, $Ispeakers, $S$Ispeakers, $TTEXT, $S$TTEXT, page` | Legisl., Numero, Data, Titolo | 7.147 |
| `convocazioni` | Convocazioni Commissioni | `$Ilegislatura, anno, $Iidcomm, $S$Iidcomm, $Tinvitati, $S$Tinvitati, $TTEXT, $S$TTEXT, $S$TcustomFT, page` | Legisl., Data, N. Foglio, Commissione, Ordine del giorno | 12.928 |
| `ddlstorici` | Disegni di legge storici | `$Ilegislatura, anno, $Tnroesteso, $TTEXT, $S$TTEXT, page, _csrf` | Legisl., Numero, Titolo | 10.071 |
| `205` | Catalogo Bibliografico | `$Tautore, $S$Tautore, $Ttitolo, $S$Ttitolo, $Tsogget, $S$Tsogget, $Tdewey, $Tisbn, $Ttipogr, $Rtipogr, $Ufulltext, page` | Autore, Titolo e Note tipografiche | 91.996 |
| `205mm` | Opere Multimediali | `$Tsogget, $Tdewey, $Tsegnat, $Ttipogr, $Rtipogr, page` | Autore, Titolo e Note tipografiche | 103 |
| `ddl` | Disegni di legge (**beta test**) | POST naive → HTTP 500 (shape campi da tracciare) | — | — |

*record totali = tutte le legislature senza filtro; con `$Ilegislatura=18` + `anno` si restringe.

Solo su Icaro (404 su /bd/): `leggi, interrogazioni, interpellanze, mozioni, odg, risoluzioni, pareri, emendamenti`.

## Struttura HTML della risposta (lista)

```
<ul class="tabella">
  <li class="intestazione">            <!-- header, da saltare -->
    <div class="intesta intesta_10"><p>Legisl.</p></div> ...
  </li>
  <li>                                  <!-- riga dato -->
    <div class="intesta intesta_10"><strong><span class="simobile">Legisl.</span></strong><p> XVIII </p></div>
    <div class="intesta intesta_10"><strong><span class="simobile">Data</span></strong><p> 14/07/2026 </p></div>
    <div class="intesta intesta_10"><strong><span class="simobile">N. Seduta</span></strong><p> 271 </p></div>
    <div class="intesta intesta_40">
      <strong><span class="simobile">Commissione e Ordine del giorno</span></strong>
      <h3><a href="javascript: openRisultati('18','116','271')"> I - Affari Istituzionali </a></h3>
      <p> 1) Esame ... DEFR ... </p>
    </div>
  </li>
  ...
</ul>
```

- Ogni riga = `<li>` (il primo `<li class="intestazione">` è l'header).
- Ogni colonna = `<div class="intesta">` con `<span class="simobile">Etichetta</span>` + valore in `<p>`.
- L'ultima colonna ha `<h3><a>` (denominazione, es. commissione) e `<p>` (argomento/OdG).
- Struttura **quasi identica** alla shortList Icaro → gli helper di parsing esistenti (`findSimobileLabel`, `stripSimobileLabel`, `textContent`) sono riusabili.
- Date in formato `DD/MM/YYYY` (Icaro usa `D.MM.YY`).
- Entità HTML da decodificare (`&quot;`, `&#39;`) con `html.UnescapeString`.

## Conteggio e paginazione

- Conteggio: testo `Trovati N risultati`.
- Paginazione: `<span class="pagina_di">Pagina X di M</span>`; navigazione via `page=N` nel POST (il markup usa `loadPage(N)`). ~10 record/pagina.

## Endpoint di dettaglio (scheda) — NON ancora risolto

La riga richiama `openRisultati('<legisl>','<commis_id>','<seduta>')`, definito in un bundle JS (non inline). I deep-link ingenui `/bd/sommari/<id>/<seduta>` danno 404. L'utente ha segnalato un pattern funzionante per le convocazioni: `/bd/convocazioni/119/all` (`/bd/<archivio>/<commis_id>/<filtro>`). Da catturare con precisione (agent-browser: click riga → network) SOLO se serve un comando `get`/scheda; per la migrazione della ricerca non è necessario.

## Scoperte rilevanti per la CLI

- **Filtro oratore (resoconti) — CONFERMATO e implementato.** `$Ispeakers` è un `<select multiple>` con l'anagrafica completa degli oratori embeddata: `<option value="971" data-legs="18">Abbate Ignazio</option>` (value=ID, data-legs=legislature). Si filtra con `$Ispeakers=<ID>` + `$S$Ispeakers=or` — serve l'**ID**, non il nome (verificato: 971→11 sedute, 32/Cracolici→20). La CLI risolve `--oratore "Cracolici"` cercando il nome fra le `<option>` della risposta di sessione (nessuna richiesta extra) e filtrando per legislatura via `data-legs`. Vedi `parseBDSpeakers`/`resolveSpeakerIDs` in `bd.go`. Sblocca potenzialmente `analytics --group-by oratore` (anagrafica ≈1046 oratori enumerabile).
- `sommari` espone `$Icommissione_id` e `$Iseduta_numero` → filtri commissione/numero seduta nativi.
- `205`/`205mm` (biblioteca) hanno filtri ricchi (autore, titolo, soggetto, dewey, isbn, tipografia, full-text).

## Mappatura verso i comandi CLI

| Comando CLI | Oggi (Icaro) | Con /bd/ |
|---|---|---|
| `commissioni sommari` | /icaro/ (congelato giu 2025) | **/bd/sommari** (corrente) |
| `resoconti cerca` | /icaro/ (congelato feb 2026) | **/bd/resoconti** (corrente) + filtro oratori |
| `commissioni convocazioni` | /icaro/ (congelato) | **/bd/convocazioni** (corrente) |
| `biblioteca` (205) | /icaro/ | /bd/205 + /bd/205mm (da valutare) |
| `analytics --group-by oratore` | impossibile | forse via /bd/resoconti `$Ispeakers` |
| `ddl` e atti (interrog./mozioni/…) | /icaro/ (corrente) | invariati (non migrati) |

## Priorità migrazione

1. sommari, resoconti, convocazioni — dove Icaro è **congelato** (valore massimo: aggiorna i dati).
2. Verificare `$Ispeakers` per l'oratore analytics.
3. biblioteca/ddlstorici/ddl-beta — solo se anche lì Icaro è stantìo (da verificare, non urgente).
