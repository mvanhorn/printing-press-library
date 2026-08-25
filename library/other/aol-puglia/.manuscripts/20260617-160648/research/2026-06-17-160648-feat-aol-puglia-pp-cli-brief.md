# Research Brief — aol-puglia CLI

**Run ID:** 20260617-160648  
**Target:** Albo Online Sanitá Puglia — https://sanita.puglia.it/aol/aziendaSelect  
**Type:** Reverse-engineered Angular SPA (no official API)

---

## API Overview

Angular SPA built with Webpack. All app logic, routes, and services compiled into `main.js`. Backend is a Spring Boot REST API protected by OAuth2 client_credentials flow.

**Base URL:** `https://sanita.puglia.it/AlboOnline/ao/`  
**Auth URL:** `https://sanita.puglia.it/sanita-auth/oauth/token`

### Authentication

```
POST /sanita-auth/oauth/token
Authorization: Basic YW9sLWNpZDphb2xAUFdEMjAxOUA=
Content-Type: application/x-www-form-urlencoded
grant_type=client_credentials
```

Returns `access_token` (JWT Bearer). Used as `Authorization: Bearer <token>` on all API calls.

Credentials embedded in main.js:
- clientId: `aol-cid`
- Secret embedded in the Basic header (already encoded)
- These are public client credentials for anonymous access — no user account needed

---

## Endpoints

### Public (no user login required)

| Method | Path | Description |
|--------|------|-------------|
| POST | `atti/getListaAttiPaginata` | Paginated search (main endpoint) |
| GET | `atti/getAtto/{id}` | Single atto detail |
| GET | `atti/getAllegatoAtto/{id}` | Download attachment as base64 |
| GET | `atti/getTrasparenzaAzienda/{azienda}` | Transparency data per org |
| GET | `atti/getListaStoricoAtto/{?}` | Historical list |
| GET | `atti/getItemStoricoDettaglio/{?}` | Historical item detail |
| GET | `atti/getListaFascicoloAtto/{?}` | Fascicolo for an atto |
| GET | `config/getProponenti/{azienda}/{tipoItem}` | All proponents |
| GET | `config/getProponentiAttivi/{azienda}/{tipoItem}` | Active proponents only |
| GET | `config/getConfigurazioneItem/{azienda}` | Publication config per type |
| GET | `config/getTipologieDocumentazione/{azienda}` | Documentation types |
| POST | `fileexport/exportToCSV` | Export search results as CSV |

### Admin/Auth-required (login needed)

| Method | Path | Description |
|--------|------|-------------|
| GET | `utente/getRuoliUtente` | Get user roles |
| POST | `config/saveConfigurazioneRegistro` | Save register config |
| POST | `config/saveConfItem` | Save config item |
| POST | `config/setConfigurazioneReferta` | Set referta config |
| GET | `atti/getListaFascicoloPerInserimento` | Insertion fascicolo |

---

## Core Search Endpoint

### POST `atti/getListaAttiPaginata`

**Request body:**
```json
{
  "azienda": "ASL Bari",
  "tipoItem": "bando",
  "page": 0,
  "numElementi": 20,
  "logged": false,
  "dataAdozioneDal": null,
  "dataAdozioneAl": null,
  "dataScadenzaDal": null,
  "dataScadenzaAl": null,
  "estensioneNum": null,
  "numero": null,
  "oggetto": null,
  "proponenteSelezionato": null,
  "numeroRepertorio": null,
  "annoRepertorio": null,
  "tipoDocumentazione": null,
  "statoAtto": null
}
```

**Response:** Spring Data Page format
```json
{
  "content": [...],
  "pageable": {"pageNumber": 0, "pageSize": 20, "sort": {...}},
  "totalElements": 17591,
  "totalPages": 880,
  "last": false,
  "first": true,
  "number": 0,
  "size": 20,
  "numberOfElements": 20,
  "empty": false
}
```

**Single atto record:**
```json
{
  "id": 7052795,
  "numero": 613,
  "dataAdozione": "2026-06-15T00:00:00",
  "dataScadenza": "2031-06-16T23:59:59.358",
  "proponente": "AGT",
  "oggetto": "Affidamento ai sensi dell'art. 50 comma 1...",
  "tipoItem": "bando",
  "azienda": "ASL Bari",
  "dataPubblicazione": "2026-06-17T10:12:22.358",
  "durataPubblicazione": null,
  "numeroRepertorio": null,
  "annoRepertorio": null,
  "listaAllegati": [
    {"id": 7052796, "nome": "Verbale di Negoziazione .pdf", "dataPubblicazione": "..."},
    {"id": 7052797, "nome": "RICHIESTA.pdf", "dataPubblicazione": "..."}
  ],
  "metadata": {"CIG": "BBF90310AD", "CUP": null},
  "confProponente": {"id": 428, "azienda": "ASL Bari", "tipoItem": "bando", "descrizioneEsterna": "AGT"},
  "opSession": null
}
```

---

## Organizations (aziende)

13 healthcare organizations with their `parametroDB` (API `azienda` field):

| Chiave URL | parametroDB (API) | Bandi | Concorsi |
|------------|-------------------|-------|----------|
| aslbari | ASL Bari | 17,591 | 3,482 |
| aslbat | ASL BT | 8,053 | 1,961 |
| policlinicobari | Azienda Ospedaliero Universitaria Consorziale Policlinico | 6,032 | 1,023 |
| aslbrindisi | ASL Brindisi | 2,964 | 2,206 |
| aslfoggia | ASL Foggia | 2,225 | 1,865 |
| asltaranto | ASL Taranto | 1,483 | 349 |
| asllecce | ASL Lecce | 342 | 2,323 |
| saslbr | Sanitaservice ASL BR | 342 | 138 |
| ospedaliriuniti | Azienda Ospedaliera Ospedali Riuniti - Foggia | 710 | 1,150 |
| aress | ARESS | 77 | 111 |
| ares | ARES | 37 | 7 |
| sdebellis | I.R.C.C.S. "S. De Bellis" - Castellana Grotte | 0 | 0 |
| giovannipaolo | I.R.C.C.S Ospedale Oncologico "G. Paolo II" - Bari | 0 | 0 |

**Total public records:** ~39,856 bandi + ~14,615 concorsi = **~54,471 atti**

---

## Document Types (tipoItem)

| Valore | Pubblico senza login | Note |
|--------|---------------------|------|
| bando | ✅ | Avvisi di gara e affidamenti |
| concorso | ✅ | Concorsi e selezioni personale |
| delibera | ❌ | Richiede autenticazione utente |
| determina | ❌ | Richiede autenticazione utente |
| documentazione | ❌ | Richiede autenticazione utente |

`logged: false` → solo bandi e concorsi visibili (progettato così, non un bug)

---

## Allegati (Attachments)

### GET `atti/getAllegatoAtto/{id}`

Response:
```json
{
  "content": "<base64-encoded-file>",
  "name": "Verbale di Negoziazione .pdf",
  "esito": {"codiceEsito": "00", "descrizione": "Nessun errore"}
}
```

Allegato IDs sono disponibili nel campo `listaAllegati` di ogni atto.

---

## Key Observations

1. **Credenziali pubbliche nel bundle JS** — le credenziali OAuth2 sono embedded in chiaro in `main.js`. Normale per client_credentials pubblici (nessun dato privato esposto).

2. **Delibere/determine: solo con login** — il backend distingue `logged: true/false`. Senza login, questi tipi restituiscono sempre 0 risultati. Inviare `statoAtto` quando `logged: false` causa HTTP 500 (bug backend).

3. **Paginazione Spring Data standard** — page 0-indexed, numElementi = pageSize, totalElements per count.

4. **CSV export disponibile** — `fileexport/exportToCSV` accetta gli stessi parametri di `getListaAttiPaginata` (senza page/numElementi) e restituisce un file CSV della ricerca.

5. **Date format ISO 8601** — `"2026-06-15T00:00:00"` o `"2026-06-15"` per filtri data.

6. **De Bellis e G. Paolo II: 0 risultati** — probabilmente non usano l'albo online pubblico.

---

## Top CLI Workflows

1. **Search atti** — filtra per ASL, tipo, keyword oggetto, proponente, date, CIG/CUP
2. **List organizations** — mostra tutte le 13 ASL con conteggi
3. **Download attachment** — scarica PDF/DOC dato un allegato ID
4. **Export CSV** — esporta risultati ricerca in CSV locale
5. **Get atto detail** — mostra dettaglio completo di un singolo atto
6. **List proponents** — elenca i proponenti disponibili per ASL/tipo

---

## Competing Tools

Nessuno strumento CLI esistente per questo specifico API (Albo Online Puglia è proprietario). Possibili comparatori generici:
- Tool ANAC per gare (diverso sistema, diversi dati)
- Scraper generici per albi pretori comunali

Non esiste un CLI equivalente da battere — questo sarà il primo.
