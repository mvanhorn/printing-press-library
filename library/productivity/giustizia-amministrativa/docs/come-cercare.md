---
type: Playbook
title: Come cercare
description: Quali ricerche sono possibili, con quali filtri, quali regole di combinazione e quali prerequisiti.
resource: https://www.giustizia-amministrativa.it/dcsnprr
tags: [ricerca, filtri, regole, prerequisiti, uso]
status: stable
generated: { by: claude-sonnet-5, at: 2026-08-07T18:20:00Z }
sources:
  - id: codice-cli
    title: Codice della CLI (internal/gaclient, internal/cli) — regole estratte dai controlli effettivi
    last_modified: 2026-08-07
---

Cosa si può chiedere a questa banca dati e a quali condizioni. Per ciò che **non** è ottenibile — ordinamento per pertinenza, esito di primo grado, data di deposito nei risultati di ricerca — vedi [Limiti della fonte](limiti-della-fonte.md).

# Modi di cercare

Quattro modi, alternativi fra loro. Scegliere quello sbagliato è la prima causa di risultati fuori tema.

| Modo | Parametro | Come si comporta | Quando usarlo |
|---|---|---|---|
| Testo libero | `testo` | Le parole in AND, ovunque nel documento, ordine e adiacenza irrilevanti | Esplorazione iniziale, tema vago |
| Locuzione esatta | `phrase` | Le parole adiacenti e nell'ordine dato | Istituto giuridico con nome fisso |
| Tutte le parole | `all` | AND esplicito (ricerca avanzata del portale) | Più concetti che devono coesistere |
| Una qualsiasi / esclusione | `any`, `not` | OR, e sottrazione di termini | Sinonimi, oppure togliere un filone estraneo |

Per un istituto con nome proprio usare `phrase`, non `testo`: cercando *accesso civico generalizzato* come testo libero il motore restituisce anche una sentenza che parla di un numero civico stradale.

# Filtri

| Filtro | Valori ammessi | Note |
|---|---|---|
| `tipo` | `sentenza`, `ordinanza`, `decreto`, `parere`, `plenaria`, `generale` | Un valore non in elenco è **rifiutato con errore**, non ignorato. Attenzione: `plenaria` a monte include anche decreti e sentenze ordinarie della Sezione P |
| `sede` | 31 valori: `roma`, `milano`, `napoli`, `consiglio-di-stato`, `cgars` e gli altri TAR | Accetta sia lo slug sia l'etichetta del portale (`Milano`). Un valore non riconosciuto è **rifiutato con errore** |
| `sede-sweep` | booleano | Interroga tutte le 31 sedi e unisce |
| `sede-quota` | `proporzionale` (predefinito), `uguale` | Come `sede-sweep` spende `limit`. Un valore non in elenco è **rifiutato con errore**; senza `sede-sweep` è rifiutato perché non c'è nulla da ripartire |
| `anno` | anno singolo | |
| `anno-from`, `anno-to` | intervallo | Itera il filtro anno, per far emergere le pronunce storiche |
| `numero`, `nrg`, `anno-nrg` | numerici | Ricerca puntuale per estremi |
| `limit` | numero | Massimo risultati da scaricare |

# Regole di combinazione

Sono verificate ed emettono un errore esplicito. Nessuna viene ignorata in silenzio.

- `sede` **oppure** `sede-sweep`, non entrambi — chiedere una sede sola e insieme tutte è contraddittorio.
- `anno` **oppure** `anno-from`/`anno-to`, non entrambi.
- `sede-sweep` **non si combina** con `anno-from`/`anno-to`: sarebbero 31 sedi per ogni anno. Restringere con `anno`, o fare uno sweep per volta.
- `sede-quota` richiede `sede-sweep`: su una sede sola non c'è una quota da ripartire.
- In `corpus build`: `ids` **oppure** i criteri di ricerca, non entrambi. Con `ids` il corpus è esattamente la lista passata, e una ricerca non deve poterla sostituire.

# Obblighi

- `corpus build` richiede `out`, la cartella di destinazione. Il percorso viene risolto in assoluto e restituito nella risposta: quello riportato è quello su disco.
- `corpus build` richiede **o** `ids` **o** almeno un criterio di ricerca.
- `get` richiede un id (ECLI o idprovv), **oppure** la terna `sede` + `nrg` + `file` per il recupero diretto senza ricerca preliminare. L'id si passa come argomento oppure con `id`: le due forme sono equivalenti.
- `stats` e le altre analisi richiedono almeno un criterio di ricerca.

# Prerequisiti

Alcuni comandi non interrogano il portale ma lo **store locale**, e senza dati restituiscono un insieme vuoto legittimo:

| Comando | Cosa serve prima |
|---|---|
| `sql` | Almeno una `sync` o una ricerca precedente |
| `grep` | Testi integrali già scaricati con `get` o `corpus build`: cerca nei testi, non negli snippet |
| `massime` | Come sopra, ma li scarica da sé se mancano |
| `get <id>` | Che l'id sia già noto allo store, cioè che una ricerca l'abbia restituito. Altrimenti usare la terna `sede`/`nrg`/`file` |

I testi già scaricati non vengono mai riscaricati: `get`, `corpus build` e `massime` li riusano dallo store. Dopo aver letto dei provvedimenti, archiviarli con `corpus build --ids` non costa altre richieste al portale.

# Come lo sweep ripartisce i risultati

`sede-sweep` interroga tutte le 31 sedi. `sede-quota` decide come `limit` si distribuisce fra loro, e risponde a due domande diverse.

**`proporzionale`** (predefinito) pesa ogni sede per il totale che il portale dichiara: il campione rispecchia dove la giurisprudenza sta davvero. Su *appalto* 2026 il portale dichiara 2187 provvedimenti così distribuiti: Roma 373, Consiglio di Stato 339, Napoli 282, Milano 141, e in coda Pescara 3 e Aosta 1. Con `limit 100` la ripartizione proporzionale dà Roma 14, Consiglio di Stato 13, Napoli 12, Milano 6, e lascia fuori le sedi la cui quota non arriva a un provvedimento.

**`uguale`** dà a ogni sede la stessa fetta — con `limit 100` sono circa 4 a testa. Risponde a *"esiste qualcosa, da qualche parte, su questo tema?"*, ed è la scelta giusta per quella domanda, perché non lascia fuori le sedi piccole. Non è però una fotografia del paese: Aosta, che ha un solo provvedimento, occuperebbe il 4% del campione contro lo 0,05% reale.

Nessuna sede riceve mai più provvedimenti di quanti ne abbia, e i posti che si liberano per quel tetto vengono ridistribuiti.

**`stats` non usa il campione.** La distribuzione che riporta viene dai totali dichiarati dal portale per ogni sede — il campo `conteggi_da` lo dichiara — quindi resta corretta con entrambe le quote.

# Avvisi da leggere

Le risposte possono contenere un campo `avvisi`. Non è decorazione: segnala i casi in cui il risultato è corretto ma **parziale**.

Con `--json`, quando ci sono avvisi lo stdout diventa `{"items": [...], "avvisi": [...]}`; senza avvisi resta l'array di provvedimenti. Serve perché gli avvisi viaggiano anche su stderr, e chi legge solo stdout non li vedrebbe. `--select` e `--compact` continuano ad applicarsi ai provvedimenti, non all'involucro.

- *"tutti i risultati mostrati sono della sede X"* — la ricerca senza `sede` restituisce una sola sede per via dell'ordinamento del portale. Non sono i più rilevanti a livello nazionale.
- *"il portale dichiara N risultati, questi sono i primi M"* — il numero di elementi restituiti è la dimensione del campione, non il totale.
- *"`limit` è più basso del numero di sedi interrogate"* — con lo sweep, un limite basso rappresenta solo le prime sedi.
- *"non ha testo estraibile"* — provvedimento pubblicato in un formato da cui il testo non si ricava; il documento riporta il link all'originale.
- *"file non elencati nel manifest"* — nella cartella del corpus ci sono residui di una selezione precedente, lasciati intatti.
